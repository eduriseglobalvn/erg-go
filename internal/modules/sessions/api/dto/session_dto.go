// Package dto defines request/response types for the sessions module.
package dto

import "time"

// SessionContextResponse is the full session context returned by GET /sessions/current.
type SessionContextResponse struct {
	User          UserPayload          `json:"user"`
	AccessControl AccessControlPayload `json:"accessControl"`
	Session       SessionPayload       `json:"session"`
	System        SystemPayload        `json:"system"`
}

// UserPayload holds authenticated user info.
type UserPayload struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	FullName           string `json:"fullName"`
	AvatarURL          string `json:"avatarUrl,omitempty"`
	Status             string `json:"status"`
	Provider           string `json:"provider,omitempty"`
	AccountType        string `json:"accountType,omitempty"`
	LastLoginProvider  string `json:"lastLoginProvider,omitempty"`
	IsProfileCompleted bool   `json:"isProfileCompleted"`
}

// AccessControlPayload holds the user's roles and permissions.
type AccessControlPayload struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// SessionPayload holds active session metadata.
type SessionPayload struct {
	ID           string `json:"id"`
	IPAddress    string `json:"ipAddress"`
	LastActiveAt string `json:"lastActiveAt"`
	ExpiresAt    string `json:"expiresAt"`
}

// SystemPayload holds server metadata.
type SystemPayload struct {
	ServerTime string `json:"serverTime"`
	Version    string `json:"version"`
}

// NewSessionContextResponse builds a SessionContextResponse from individual parts.
func NewSessionContextResponse(
	userID, email, fullName, avatarURL, status, provider, accountType, lastLoginProvider string, isCompleted bool,
	roles, permissions []string,
	sessionID, ipAddress string,
	lastActive, expiresAt time.Time,
	version string,
) SessionContextResponse {
	return SessionContextResponse{
		User: UserPayload{
			ID:                 userID,
			Email:              email,
			FullName:           fullName,
			AvatarURL:          avatarURL,
			Status:             status,
			Provider:           provider,
			AccountType:        accountType,
			LastLoginProvider:  lastLoginProvider,
			IsProfileCompleted: isCompleted,
		},
		AccessControl: AccessControlPayload{
			Roles:       roles,
			Permissions: permissions,
		},
		Session: SessionPayload{
			ID:           sessionID,
			IPAddress:    ipAddress,
			LastActiveAt: lastActive.Format(time.RFC3339),
			ExpiresAt:    expiresAt.Format(time.RFC3339),
		},
		System: SystemPayload{
			ServerTime: time.Now().UTC().Format(time.RFC3339),
			Version:    version,
		},
	}
}
