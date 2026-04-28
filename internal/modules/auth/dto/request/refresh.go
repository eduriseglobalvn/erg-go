package request

// RefreshRequest is the DTO for the POST /auth/refresh endpoint.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}
