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
	CodeInvalidCredentials      = "AUTH_INVALID_CREDENTIALS" // #nosec G101 -- public error code, not a credential value.
	CodeAuthSessionReplaced     = "AUTH_SESSION_REPLACED"
	CodeAuthDeviceLimitReached  = "AUTH_DEVICE_LIMIT_REACHED"
	CodeUserNotFound            = "USER_NOT_FOUND"
	CodeEmailExists             = "EMAIL_EXISTS"
	CodeInvalidToken            = "INVALID_TOKEN"
	CodeTokenExpired            = "TOKEN_EXPIRED"
	CodeInvalidPIN              = "INVALID_PIN"
	CodePINExpired              = "PIN_EXPIRED"
	CodeRateLimited             = "RATE_LIMITED"
	CodeInvalidRequest          = "INVALID_REQUEST"
	CodeInternalError           = "INTERNAL_ERROR"
	CodeAccountLocked           = "ACCOUNT_LOCKED"
	CodeEmailNotVerified        = "EMAIL_NOT_VERIFIED"
	CodeAuthTooManyAttempts     = "AUTH_TOO_MANY_ATTEMPTS"
	CodeAuthIPBlocked           = "AUTH_IP_BLOCKED"
	CodeAuthGeoBlocked          = "AUTH_GEO_BLOCKED"
	CodeFirewallAllowlistNeeded = "FIREWALL_ALLOWLIST_REQUIRED"
)
