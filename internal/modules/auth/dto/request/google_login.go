package request

// GoogleLoginRequest is the trusted payload forwarded by the frontend OAuth bridge.
type GoogleLoginRequest struct {
	Email         string `json:"email" validate:"required,email"`
	FullName      string `json:"fullName"`
	AvatarURL     string `json:"avatarUrl"`
	GoogleSub     string `json:"googleSub" validate:"required"`
	EmailVerified bool   `json:"emailVerified"`
}
