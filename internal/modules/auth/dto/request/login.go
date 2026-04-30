package request

// LoginRequest is the DTO for the POST /auth/login endpoint.
type LoginRequest struct {
	Email             string `json:"email" validate:"required,email"`
	Password          string `json:"password" validate:"required"`
	DeviceID          string `json:"deviceId,omitempty"`
	DeviceName        string `json:"deviceName,omitempty"`
	DeviceFingerprint string `json:"deviceFingerprint,omitempty"`
	RememberMe        bool   `json:"rememberMe,omitempty"`
}
