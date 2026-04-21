package common

import (
	"time"

	"github.com/google/uuid"
)

// Auth

type User struct {
	Id           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Session struct {
	Id        uuid.UUID `json:"id"`
	UserId    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Conversations
type Conversation struct {
	Id      uuid.UUID   `json:"id"`
	Members []uuid.UUID `json:"members"`
}

type ConversationWithUsernames struct {
	Id      uuid.UUID `json:"id"`
	Members []string  `json:"members"`
}

// Messages

type ShortMessage struct {
	Id       uuid.UUID `json:"id"`
	Sender   string    `json:"sender"`
	Content  string    `json:"content"`
}
