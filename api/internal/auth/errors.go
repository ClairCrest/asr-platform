package auth

import "errors"

// Sentinel errors for the auth service. internal/http maps each of these to
// exactly one HTTP status code; no other package should re-derive status
// codes from these errors.
var (
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	ErrUserNotFound       = errors.New("auth: user not found")
	ErrAPIKeyNotFound     = errors.New("auth: api key not found")
	ErrInvalidAPIKey      = errors.New("auth: invalid api key")
)
