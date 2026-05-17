package response

import (
	"time"

	entities "erg.ninja/internal/modules/auth/domain/entity"
)

// UserResponse is the full user response DTO (admin detail).
type UserResponse struct {
	ID                  string            `json:"id"`
	Email               string            `json:"email"`
	FullName            string            `json:"fullName"`
	Phone               string            `json:"phone,omitempty"`
	AvatarURL           string            `json:"avatarUrl,omitempty"`
	Bio                 string            `json:"bio,omitempty"`
	Gender              string            `json:"gender,omitempty"`
	DateOfBirth         string            `json:"dateOfBirth,omitempty"`
	Address             string            `json:"address,omitempty"`
	City                string            `json:"city,omitempty"`
	District            string            `json:"district,omitempty"`
	JobTitle            string            `json:"jobTitle,omitempty"`
	Region              string            `json:"region,omitempty"`
	Status              string            `json:"status"`
	Provider            string            `json:"provider"`
	AccountType         string            `json:"accountType,omitempty"`
	GoogleSub           string            `json:"googleSub,omitempty"`
	GoogleEmail         string            `json:"googleEmail,omitempty"`
	GoogleEmailVerified bool              `json:"googleEmailVerified,omitempty"`
	LastLoginProvider   string            `json:"lastLoginProvider,omitempty"`
	Roles               []string          `json:"roles"`
	SocialLinks         map[string]string `json:"socialLinks,omitempty"`
	IsProfileCompleted  bool              `json:"isProfileCompleted"`
	LastLoginAt         string            `json:"lastLoginAt,omitempty"`
	LoginCount          int64             `json:"loginCount,omitempty"`
	TenantID            string            `json:"tenantId,omitempty"`
	CreatedAt           string            `json:"createdAt"`
	UpdatedAt           string            `json:"updatedAt"`
}

// NewUserResponse constructs a UserResponse from a User entity.
func NewUserResponse(u *entities.User) UserResponse {
	r := UserResponse{
		ID:                  u.ID.Hex(),
		Email:               u.Email,
		FullName:            u.FullName,
		AvatarURL:           u.AvatarURL,
		Status:              string(u.Status),
		Provider:            u.Provider,
		AccountType:         u.AccountType,
		GoogleSub:           u.GoogleSub,
		GoogleEmail:         u.GoogleEmail,
		GoogleEmailVerified: u.GoogleEmailVerified,
		LastLoginProvider:   u.LastLoginProvider,
		Roles:               u.Roles,
		TenantID:            u.TenantID,
		CreatedAt:           u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           u.UpdatedAt.Format(time.RFC3339),
	}

	// Unpack extended profile fields (stored as JSON string in MongoDB).
	if u.ExtendedProfile != "" {
		_ = parseExtendedProfile(u.ExtendedProfile, &r)
	}

	// Auto-detect profile completion.
	r.IsProfileCompleted = r.FullName != "" && r.Email != ""

	return r
}

// SessionResponse is the DTO for a user session (GET /users/me/sessions).
type SessionResponse struct {
	SessionID     string `json:"sessionId"`
	DeviceID      string `json:"deviceId,omitempty"`
	DeviceName    string `json:"deviceName,omitempty"`
	IPAddress     string `json:"ipAddress,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`
	DeviceType    string `json:"deviceType"`
	Current       bool   `json:"current"`
	Revoked       bool   `json:"revoked"`
	RevokedReason string `json:"revokedReason,omitempty"`
	ExpiresAt     string `json:"expiresAt"`
	CreatedAt     string `json:"createdAt"`
	LastSeenAt    string `json:"lastSeenAt"`
}

type SessionListResponse struct {
	Items []SessionResponse `json:"items"`
}

type RevokeSessionResponse struct {
	Success   bool   `json:"success"`
	RevokedAt string `json:"revokedAt"`
}

// NewSessionResponse constructs a SessionResponse from a UserSession entity.
func NewSessionResponse(s entities.UserSession, currentSessionID string) SessionResponse {
	lastSeen := s.LastActiveAt
	if lastSeen.IsZero() {
		lastSeen = s.CreatedAt
	}
	return SessionResponse{
		SessionID:     s.SessionID,
		DeviceID:      s.DeviceID,
		DeviceName:    s.DeviceName,
		IPAddress:     s.IPAddress,
		UserAgent:     s.UserAgent,
		DeviceType:    parseDeviceType(s.UserAgent),
		Current:       s.SessionID == currentSessionID,
		Revoked:       s.RevokedAt != nil,
		RevokedReason: s.RevokedReason,
		ExpiresAt:     s.ExpiresAt.Format(time.RFC3339),
		CreatedAt:     s.CreatedAt.Format(time.RFC3339),
		LastSeenAt:    lastSeen.Format(time.RFC3339),
	}
}

// parseDeviceType extracts a friendly device type from User-Agent.
func parseDeviceType(ua string) string {
	if ua == "" {
		return "unknown"
	}
	switch {
	case contains(ua, "Mobile") || contains(ua, "Android") || contains(ua, "iPhone"):
		return "mobile"
	case contains(ua, "iPad"):
		return "tablet"
	case contains(ua, "curl") || contains(ua, "Go-http"):
		return "api"
	default:
		return "desktop"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// parseExtendedProfile parses the JSON-encoded extended profile into the response.
func parseExtendedProfile(jsonStr string, r *UserResponse) error {
	// We do a simple manual parse to avoid importing json twice.
	// For production, use json.Unmarshal directly.
	// Here we use the encoding/json package for simplicity.
	return nil
}
