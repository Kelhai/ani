package client

import (
	"errors"
	"time"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
)

// types
type RegisterResultMsg struct { Err error }
type LoginResultMsg struct {
	Username string
	Err      error
}
type ConversationsLoadedMsg struct {
	Conversations []common.ConversationWithUsernames
	Err           error
}
type ConversationCreatedMsg struct {
	Id  uuid.UUID
	Err error
}
type MessagesLoadedMsg struct {
	Messages      []common.DecryptedMessage
	LastMessageId *uuid.UUID
	Err           error
}
type MessageSentMsg struct {
	Err       error
	MessageId uuid.UUID
}
type PollTickMsg struct{}
type ChatLine struct {
	Text      string
	ArrivedAt time.Time
	FromPoll  bool // true = came from polling, false = optimistic own send
}


// globals
var (
	User *common.User
)

// errors
var (
	ErrUsernameTaken = errors.New("Username taken")
	ErrLoginFailed   = errors.New("Failed to login")
	ErrUserNotFound  = errors.New("Failed to find user")
	ErrNoAuthToken   = errors.New("No session token")
	ErrUnknownErr    = errors.New("Unknown error")
	ErrJsonUnmarshal = errors.New("Failed to unmarshal json")
)

type Screen int

const (
	ScreenAuthChoice Screen = iota
	ScreenUsername
	ScreenPassword
	ScreenConversations
	ScreenNewChat
	ScreenChat
)

