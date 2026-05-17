package response

import (
	"time"

	entities "erg.ninja/internal/modules/auth/domain/entity"
)

// AuthResponse is the DTO returned on successful login / register.
type AuthResponse struct {
	User         ProfileResponse  `json:"user"`
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	ExpiresIn    int64            `json:"expiresIn"`
	TokenType    string           `json:"tokenType"`
	Session      SessionDeviceDTO `json:"session"`
	Permissions  []string         `json:"permissions"`
	Portals      []string         `json:"portals,omitempty"`
	AccountType  string           `json:"accountType"`
	AccessLevel  string           `json:"accessLevel"`
}

// TokenResponse is the DTO for token refresh responses.
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	TokenType    string `json:"tokenType"`
}

// SessionDeviceDTO describes a login device/session without exposing token hashes.
type SessionDeviceDTO struct {
	SessionID     string `json:"sessionId"`
	DeviceID      string `json:"deviceId,omitempty"`
	DeviceName    string `json:"deviceName,omitempty"`
	IPAddress     string `json:"ipAddress,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`
	Current       bool   `json:"current"`
	Revoked       bool   `json:"revoked"`
	RevokedReason string `json:"revokedReason,omitempty"`
	CreatedAt     string `json:"createdAt"`
	LastSeenAt    string `json:"lastSeenAt"`
	ExpiresAt     string `json:"expiresAt"`
}

// NewSessionDeviceDTO constructs a safe session response.
func NewSessionDeviceDTO(session *entities.UserSession, current bool) SessionDeviceDTO {
	if session == nil {
		return SessionDeviceDTO{}
	}
	lastSeen := session.LastActiveAt
	if lastSeen.IsZero() {
		lastSeen = session.CreatedAt
	}
	return SessionDeviceDTO{
		SessionID:     session.SessionID,
		DeviceID:      session.DeviceID,
		DeviceName:    session.DeviceName,
		IPAddress:     session.IPAddress,
		UserAgent:     session.UserAgent,
		Current:       current,
		Revoked:       session.RevokedAt != nil,
		RevokedReason: session.RevokedReason,
		CreatedAt:     formatTime(session.CreatedAt),
		LastSeenAt:    formatTime(lastSeen),
		ExpiresAt:     formatTime(session.ExpiresAt),
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
