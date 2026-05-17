// Package dto provides request/response types for the audit module.
package dto

import "time"

// ─── Query DTOs ──────────────────────────────────────────────────────────────

// AuditLogQueryParams holds filter options for GET /api/audit/logs.
type AuditLogQueryParams struct {
	Page         int    `query:"page"`
	Limit        int    `query:"limit"`
	Action       string `query:"action"`
	UserID       string `query:"user_id"`
	ResourceType string `query:"resource_type"`
	StartDate    string `query:"start_date"`
	EndDate      string `query:"end_date"`
}

// ─── Response DTOs ───────────────────────────────────────────────────────────

// AuditLogResponse is the public-facing audit log document.
type AuditLogResponse struct {
	ID           string    `json:"id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id,omitempty"`
	UserID       string    `json:"user_id"`
	UserEmail    string    `json:"user_email,omitempty"`
	IPAddress    string    `json:"ip_address,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	Changes      string    `json:"changes,omitempty"`
	Metadata     string    `json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}
