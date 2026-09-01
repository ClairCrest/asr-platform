package job

import "errors"

// Sentinel errors for the job service. internal/http maps each of these to
// exactly one HTTP status code; no other package should re-derive status
// codes from these errors.
var (
	ErrNotFound          = errors.New("job: not found")
	ErrDuplicateRequest  = errors.New("job: duplicate idempotency key")
	ErrInvalidTransition = errors.New("job: invalid state transition")
	ErrNotCancellable    = errors.New("job: not cancellable in its current state")
	ErrNotRetryable      = errors.New("job: not retryable in its current state")
)
