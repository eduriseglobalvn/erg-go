// Package dto provides request/response types for the analytics module.
package dto

// VisitorStatsParams holds query params for GET /api/analytics/stats.
type VisitorStatsParams struct {
	Range string `query:"range"` // 7d, 30d, 90d
	From  string `query:"from"`  // YYYY-MM-DD
	To    string `query:"to"`    // YYYY-MM-DD
}

// VisitorStat represents daily visitor count broken down by device type.
type VisitorStat struct {
	Date    string `json:"date"`
	Desktop int    `json:"desktop"`
	Mobile  int    `json:"mobile"`
}

// VisitorStatsResponse is the response for GET /api/analytics/stats.
type VisitorStatsResponse struct {
	StatusCode int           `json:"status_code"`
	Message    string        `json:"message"`
	Data       []VisitorStat `json:"data"`
}

// ExportParams holds query params for GET /api/analytics/export.
type ExportParams struct {
	From string `query:"from" validate:"required"`
	To   string `query:"to"   validate:"required"`
}
