package response

// Canonical error codes used across all modules.
// These must match the codes consumed by API clients (frontend, mobile, etc.).
const (
	ErrCodeBadRequest     = "BAD_REQUEST"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
	ErrCodeForbidden      = "FORBIDDEN"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeValidation     = "VALIDATION_ERROR"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeGone           = "GONE"
	ErrCodeTooManyReqs    = "TOO_MANY_REQUESTS"
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeServiceUnavail = "SERVICE_UNAVAILABLE"
)
