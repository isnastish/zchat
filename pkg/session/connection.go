package session

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
)

type connectionState int8

const (
	pendingState   connectionState = 0
	connectedState connectionState = 0x1
)

var connStateTable []string

type connection struct {
	netConn                net.Conn
	ipAddr                 string
	participant            *types.Participant
	channel                *types.Channel
	timeout                time.Duration
	ctx                    context.Context
	cancel                 context.CancelFunc
	abortConnectionTimeout chan struct{}
	state                  connectionState
}

type connectionMap struct {
	connections map[string]*connection
	mu          sync.RWMutex
}

func init() {
	connStateTable = make([]string, 2)
	connStateTable[pendingState] = "offline"
	connStateTable[connectedState] = "online"
}

func newConn(conn net.Conn, timeout time.Duration) *connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &connection{
		netConn:                conn,
		ipAddr:                 conn.RemoteAddr().String(),
		participant:            &types.Participant{},
		channel:                &types.Channel{},
		timeout:                timeout,
		ctx:                    ctx,
		cancel:                 cancel,
		abortConnectionTimeout: make(chan struct{}),
		state:                  pendingState,
	}
}

func newConnectionMap() *connectionMap {
	return &connectionMap{
		connections: make(map[string]*connection),
	}
}

func (c *connection) matchState(state connectionState) bool {
	return c.state == state
}

func (c *connection) disconnectIfIdle() {
	timer := time.NewTimer(c.timeout)
	for {
		select {
		case <-timer.C:
			// The timer has fired, close the net connection manually,
			// and invoke the cancel() function in order to send a message to the client
			// insie a select statement in reader::read() procedure.
			c.netConn.Close()
			c.cancel()
			return
		case <-c.abortConnectionTimeout:
			// A signal to abort the timeout process was received, probably due to the client being active
			// (was able to send a message that the session received).
			// In this case we have to stop the current timer and reset it.
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(c.timeout)
		case <-c.ctx.Done():
			// A signal to unblock this procedure was received,
			// so we can exit gracefully without having go routine leaks.
			return
		}
	}
}

func (cm *connectionMap) _doesConnExist(connIpAddr string) bool {
	_, exists := cm.connections[connIpAddr]
	return exists
}

func (cm *connectionMap) addConn(conn *connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections[conn.ipAddr] = conn
}

func (cm *connectionMap) removeConn(connIpAddr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm._doesConnExist(connIpAddr) {
		log.Logger.Panic("Connection {%s} doesn't exist", connIpAddr)
	}

	delete(cm.connections, connIpAddr)
}

func (cm *connectionMap) hasConnectedParticipant(username string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, conn := range cm.connections {
		if conn.matchState(connectedState) {
			if conn.participant.Username == username {
				return true
			}
		}
	}
	return false
}

func (cm *connectionMap) empty() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections) == 0
}

func (cm *connectionMap) markAsConnected(connIpAddr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm._doesConnExist(connIpAddr) {
		log.Logger.Panic("Connection {%s} doesn't exist", connIpAddr)
	}

	cm.connections[connIpAddr].state = connectedState
}

// Pointers to interfaces: https://stackoverflow.com/questions/44370277/type-is-pointer-to-interface-not-interface-confusion
func (cm *connectionMap) broadcastMessage(msg interface{}) int {
	var sentCount int

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	switch msg := msg.(type) {
	case *types.ChatMessage:
		senderWasSkipped := false
		// Convert message into a canonical form, which includes the name of the sender and the time when the message was sent.
		canonChatMsg := bytes.NewBuffer([]byte(util.Fmtln("{%s:%s} %s", msg.Sender, msg.SentTime, msg.Contents.String())))
		for _, conn := range cm.connections {
			if conn.matchState(connectedState) {
				if !senderWasSkipped && strings.EqualFold(conn.participant.Username, msg.Sender) {
					senderWasSkipped = true
					continue
				}

				n, err := util.WriteBytes(conn.netConn, canonChatMsg)
				if err != nil || (n != msg.Contents.Len()) {
					log.Logger.Error("Failed to send a chat message to the participant: %s", conn.participant.Username)
				} else {
					sentCount++
				}
			}
		}

	case *types.SysMessage:
		// canonSysMsg := bytes.NewBuffer([]byte(util.Fmtln("{system:%s} %s", msg.SentTime, msg.Contents.String())))
		if msg.Recipient != "" {
			conn, exists := cm.connections[msg.Recipient]
			if exists {
				n, err := util.WriteBytes(conn.netConn, msg.Contents)
				if err != nil || (n != msg.Contents.Len()) {
					log.Logger.Error("Failed to send a system message to the connection ip: %s", conn.ipAddr)
				} else {
					sentCount++
				}
			}
		} else {
			// A case where messages about participants leaving broadcasted to all the other connected participants
			for _, conn := range cm.connections {
				if conn.matchState(connectedState) {
					n, err := util.WriteBytes(conn.netConn, msg.Contents)
					if err != nil || (n != msg.Contents.Len()) {
						log.Logger.Error("Failed to send a system message to the participant: %s", conn.participant.Username)
					} else {
						sentCount++
					}
				}
			}
		}

	default:
		log.Logger.Panic("Invalid message type")
	}

	return sentCount
}
