package request

// RegisterRequest is the DTO for the POST /auth/register endpoint.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	FullName string `json:"fullName" validate:"required,min=2,max=100"`
}
