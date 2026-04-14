package client

import (
	"errors"

	"github.com/Kelhai/ani/common"
)

// types
type RegisterResultMsg struct{ Err error }

// globals
var (
	User *common.User
)

// errors
var (
	ErrUsernameTaken = errors.New("Username taken")
)

