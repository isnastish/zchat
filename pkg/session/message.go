package session

import (
	"fmt"
	t "time"
)

type Message struct {
	Sender   string
	Contents []byte
	SendTime string
	// This fields are mutully exclusive,
	// we cannot specify a Receiver and SendToAllParticipants, and vice-versa.
	Receiver              string
	SendToAllParticipants bool

	sender         string
	ownedBySession bool
	recipient      string
	hasRecipient   bool
	data           []byte
	time           t.Time
}

func MakeMessage(data []byte, sender string, rest ...string) *Message {
	m := &Message{
		data:   data,
		sender: sender,
		time:   t.Now(), // pass as a function argument?
	}

	if len(rest) != 0 {
		m.hasRecipient = true
		m.recipient = rest[0]
	}

	return m
}

func FmtMessage(m *Message) []byte {
	if m.ownedBySession {
		return []byte(fmt.Sprintf("%s: %s", m.time.Format(t.TimeOnly), string(m.data)))
	}
	return []byte(fmt.Sprintf("%s: [%18s]: %s", m.time.Format(t.TimeOnly), m.sender, string(m.data)))
}
