package common

import "errors"

var (
	ErrUuidFailed     = errors.New("failed to generate UUID")
	ErrInvalidLogin   = errors.New("password hashes do not match")
	ErrNotFound       = errors.New("not found")
	ErrPgInsertFailed = errors.New("Postgres insert failed")
)
