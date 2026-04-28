// Package dto provides request/response types for the analytics module.
package dto

// TrackEventRequest is the payload for POST /api/analytics/behavior.
type TrackEventRequest struct {
	SessionInternalID string                 `json:"sessionInternalId" validate:"required"`
	EventType         string                 `json:"eventType" validate:"required"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// TrackEventResponse is the response after tracking an event.
type TrackEventResponse struct {
	Success   bool   `json:"success"`
	Timestamp string `json:"timestamp"`
}

// FinishSessionRequest is the payload for PUT /api/analytics/session/:id/finish.
type FinishSessionRequest struct {
	Duration int `json:"duration" validate:"gte=0"`
}

// IdentifyRequest is the payload for POST /api/analytics/identify.
type IdentifyRequest struct {
	SessionID string `json:"sessionId" validate:"required"`
	UserID    int64  `json:"userId" validate:"required"`
}
