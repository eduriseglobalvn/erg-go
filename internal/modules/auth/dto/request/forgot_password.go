package request

// ForgotPasswordRequest is the DTO for the POST /auth/forgot-password endpoint.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}
