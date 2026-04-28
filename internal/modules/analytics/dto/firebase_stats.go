// Package dto provides request/response types for the analytics module.
package dto

// FirebaseSyncStatsParams holds query params for Firebase sync stats.
type FirebaseSyncStatsParams struct {
	From string `query:"from"` // YYYY-MM-DD
	To   string `query:"to"`   // YYYY-MM-DD
}

// FirebaseEventStat represents a single Firebase event type summary.
type FirebaseEventStat struct {
	EventName string `json:"eventName" bson:"event_name"`
	Count     int64  `json:"count" bson:"count"`
}

// FirebaseSyncStatsResponse holds enriched Firebase event stats.
type FirebaseSyncStatsResponse struct {
	TotalEvents  int64               `json:"totalEvents"`
	TopEvents    []FirebaseEventStat `json:"topEvents"`
	TopCountries []FirebaseEventStat `json:"topCountries"`
	TopPlatforms []FirebaseEventStat `json:"topPlatforms"`
	DateFrom     string              `json:"dateFrom"`
	DateTo       string              `json:"dateTo"`
}
