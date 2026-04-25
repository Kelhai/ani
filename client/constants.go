package client

import (
	"errors"

	"github.com/Kelhai/ani/common"
)

var (
	User *common.User
)

// errors
var (
	ErrUsernameTaken = errors.New("Username taken")
	ErrLoginFailed   = errors.New("Failed to login")
	ErrNoAuthToken   = errors.New("No session token")
	ErrUnknownErr    = errors.New("Unknown error")
	ErrJsonUnmarshal = errors.New("Failed to unmarshal json")
	ErrUserNotFound = errors.New("user not found")
)

