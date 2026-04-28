// Package dto provides request/response types for the analytics module.
package dto

// OverviewParams holds query params for GET /api/analytics/overview.
type OverviewParams struct {
	From string `query:"from"` // YYYY-MM-DD
	To   string `query:"to"`   // YYYY-MM-DD
}

// PostSummaryParams holds query params for GET /api/analytics/posts/summary.
type PostSummaryParams struct {
	Range string `query:"range"` // 7d, 30d, 90d, 180d, 365d
}

// TopContentParams holds query params for GET /api/analytics/top-content.
type TopContentParams struct {
	From  string `query:"from"`
	To    string `query:"to"`
	Limit int    `query:"limit"`
}

// InsightsParams holds query params for GET /api/analytics/insights.
type InsightsParams struct {
	From string `query:"from"`
	To   string `query:"to"`
}

// TrafficSourcesParams holds query params for GET /api/analytics/traffic-sources.
type TrafficSourcesParams struct {
	From  string `query:"from"`
	To    string `query:"to"`
	Limit int    `query:"limit"`
}

// UserJourneyParams holds query params for GET /api/analytics/user-journey/:sessionId.
type UserJourneyParams struct {
	SessionID string `param:"session_id"`
}

// SessionParams holds query params for GET /api/analytics/sessions/:sessionId.
type SessionParams struct {
	SessionID string `param:"session_id"`
}
