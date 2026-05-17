package request

// ResendPinRequest is the DTO for the POST /auth/resend-pin endpoint.
type ResendPinRequest struct {
	Email   string `json:"email" validate:"required,email"`
	Purpose string `json:"purpose" validate:"omitempty,oneof=register forgot_password verify"`
}
