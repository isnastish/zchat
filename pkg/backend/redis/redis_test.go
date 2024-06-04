package redis

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/isnastish/chat/pkg/testsetup"
)

func TestMain(m *testing.M) {
	var result int
	var redisHasStarted bool

	redisHasStarted, _ = testsetup.SetupRedisMock()
	result = m.Run()

	// Make sure we always tear down the running redis-mock container
	// if one of the tests panics.
	defer func() {
		if redisHasStarted {
			testsetup.TeardownRedisMock()
		}
		os.Exit(result)
	}()
}

var redisEndpoint = "127.0.0.1:6379"

func TestRegisterParticipant(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	defer func() {
		// Make sure all the participants are deleted so it does't affect the subsequent tests.
		for _, p := range testsetup.Participants {
			backend.deleteParticipant(&p)
			assert.False(t, backend.HasParticipant(p.Username))
		}
	}()
	for _, p := range testsetup.Participants {
		backend.RegisterParticipant(&p)
		assert.True(t, backend.HasParticipant(p.Username))
	}

	participants := backend.GetParticipants()
	assert.True(t, testsetup.Match(participants, testsetup.Participants, testsetup.ContainsParticipant))
}

func TestParticipantAlreadyExists(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	defer backend.deleteParticipant(&testsetup.Participants[0])
	backend.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, backend.HasParticipant(testsetup.Participants[0].Username))
	assert.Panics(t, func() { backend.RegisterParticipant(&testsetup.Participants[0]) })
}

func TestRegisAuthenticateParticipant(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	defer backend.deleteParticipant(&testsetup.Participants[0])
	backend.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, backend.HasParticipant(testsetup.Participants[0].Username))
	assert.True(t, backend.AuthParticipant(&testsetup.Participants[0]))
}

func TestRegisterChannel(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	defer func() {
		for _, ch := range testsetup.Channels {
			backend.DeleteChannel(ch.Name)
			assert.False(t, backend.HasChannel(ch.Name))
		}
	}()
	for _, ch := range testsetup.Channels {
		backend.RegisterChannel(&ch)
		assert.True(t, backend.HasChannel(ch.Name))
	}

	channels := backend.GetChannels()
	assert.True(t, testsetup.Match(channels, testsetup.Channels, testsetup.ContainsChannel))
}

func TestChannelAlreadyExists(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	defer backend.DeleteChannel(testsetup.Channels[0].Name)
	backend.RegisterChannel(&testsetup.Channels[0])
	assert.True(t, backend.HasChannel(testsetup.Channels[0].Name))
	assert.Panics(t, func() { backend.RegisterChannel(&testsetup.Channels[0]) })
}

func TestStoreGeneralMessages(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)

	for _, msg := range testsetup.GeneralMessages {
		backend.StoreMessage(&msg)
	}

	chatHistory := backend.GetChatHistory()
	assert.True(t, testsetup.Match(chatHistory, testsetup.GeneralMessages, testsetup.ContainsMessage))
}
