package common

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	Id uuid.UUID
	Username string
	PasswordHash []byte
}

type Session struct {
	UserId uuid.UUID
	Token string
	ExpiresAt time.Time
}

