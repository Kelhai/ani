package common

import (
	"time"

	"github.com/google/uuid"
)

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
	Id        uuid.UUID
	UserId    uuid.UUID
	ExpiresAt time.Time
}

type Conversation struct {
	Id      uuid.UUID   `json:"id"`
	Members []uuid.UUID `json:"members"`
}
