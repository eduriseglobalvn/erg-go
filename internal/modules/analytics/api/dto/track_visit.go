// Package dto provides request/response types for the analytics module.
package dto

// TrackVisitRequest is the payload for POST /api/analytics/session/begin.
type TrackVisitRequest struct {
	URL        string `json:"url" validate:"required,url"`
	Referrer   string `json:"referrer,omitempty"`
	EntityType string `json:"entityType,omitempty"`
	EntityID   string `json:"entityId,omitempty"`
}

// TrackVisitResponse is the response after starting a visit.
type TrackVisitResponse struct {
	VisitID   string `json:"visitId"`
	SessionID string `json:"sessionId,omitempty"`
	Timestamp string `json:"timestamp"`
}
