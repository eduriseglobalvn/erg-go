package response

// ErrorResponse is the standard error shape for all auth endpoints.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewErrorResponse creates an ErrorResponse.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{Code: code, Message: message}
}

// Common codes.
const (
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeUserNotFound       = "USER_NOT_FOUND"
	CodeEmailExists        = "EMAIL_EXISTS"
	CodeInvalidToken       = "INVALID_TOKEN"
	CodeTokenExpired       = "TOKEN_EXPIRED"
	CodeInvalidPIN         = "INVALID_PIN"
	CodePINExpired         = "PIN_EXPIRED"
	CodeRateLimited        = "RATE_LIMITED"
	CodeInvalidRequest     = "INVALID_REQUEST"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeAccountLocked      = "ACCOUNT_LOCKED"
	CodeEmailNotVerified   = "EMAIL_NOT_VERIFIED"
)
