package common

import "errors"

// errors
var (
	ErrUuidFailed     = errors.New("failed to generate UUID")
	ErrInvalidLogin   = errors.New("password hashes do not match")
	ErrNotFound       = errors.New("not found")
	ErrPgInsertFailed = errors.New("Postgres insert failed")
)

// key types
var (
	MlDsa87 = "ML-DSA-87"
)
