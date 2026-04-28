package request

// LoginRequest is the DTO for the POST /auth/login endpoint.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
