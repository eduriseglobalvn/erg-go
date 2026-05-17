package request

// ResetPasswordRequest is the DTO for the POST /auth/reset-password endpoint.
type ResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email"`
	Code        string `json:"code" validate:"required,len=6"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}
