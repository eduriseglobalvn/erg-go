// Package dto provides request/response types for the analytics module.
package dto

import "time"

// MetricWithGrowth holds a metric value with period-over-period comparison.
type MetricWithGrowth struct {
	Value    float64 `json:"value"`
	Previous float64 `json:"previous"`
	Growth   float64 `json:"growth"`
}

// TrafficDataPoint holds a single data point in the traffic chart.
type TrafficDataPoint struct {
	Label   string `json:"label"`
	Mobile  int    `json:"mobile"`
	Desktop int    `json:"desktop"`
	Total   int    `json:"total"`
}

// LocationStat holds geographic visitor statistics.
type LocationStat struct {
	City    string `json:"city"`
	Country string `json:"country"`
	Count   int    `json:"count"`
}

// DeviceTypeStat holds device type statistics.
type DeviceTypeStat struct {
	Name       string `json:"name"`
	Count      int    `json:"count"`
	Percentage int    `json:"percentage"`
}

// DeviceStats holds device statistics breakdown.
type DeviceStats struct {
	Types    []DeviceTypeStat `json:"types"`
	OS       []DeviceTypeStat `json:"os"`
	Browsers []DeviceTypeStat `json:"browsers"`
}

// ContentItem holds a single content page's analytics.
type ContentItem struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Views    int    `json:"views"`
	EntityID string `json:"entityId,omitempty"`
}

// ContentStats holds content analytics.
type ContentStats struct {
	TopPages       []ContentItem `json:"topPages"`
	TopCourses     []ContentItem `json:"topCourses"`
	TopPosts       []ContentItem `json:"topPosts"`
	InterestByType []struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	} `json:"interestByType,omitempty"`
	InterestByCategory []struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	} `json:"interestByCategory,omitempty"`
	TrendingPosts []ContentItem `json:"trendingPosts,omitempty"`
}

// PeakHour holds hourly traffic statistics.
type PeakHour struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

// TrafficSource holds traffic source statistics.
type TrafficSource struct {
	Source     string `json:"source"`
	Count      int    `json:"count"`
	Percentage int    `json:"percentage"`
}

// InteractionStat holds interaction event statistics.
type InteractionStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DashboardSummary holds the overview summary metrics.
type DashboardSummary struct {
	TotalVisits MetricWithGrowth `json:"totalVisits"`
	ActiveUsers MetricWithGrowth `json:"activeUsers"`
	NewUsers    MetricWithGrowth `json:"newUsers"`
	AvgDuration MetricWithGrowth `json:"avgDuration"`
	BounceRate  MetricWithGrowth `json:"bounceRate"`
}

// DashboardOverviewResponse is the full dashboard response for GET /api/analytics/overview.
type DashboardOverviewResponse struct {
	DateRange struct {
		Current struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"current"`
		Previous struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"previous"`
	} `json:"dateRange"`
	Summary        DashboardSummary   `json:"summary"`
	TrafficChart   []TrafficDataPoint `json:"trafficChart"`
	Locations      []LocationStat     `json:"locations"`
	Devices        DeviceStats        `json:"devices"`
	Content        ContentStats       `json:"content"`
	PeakHours      []PeakHour         `json:"peakHours"`
	TrafficSources []TrafficSource    `json:"trafficSources"`
	Interactions   []InteractionStat  `json:"interactions,omitempty"`
}

// PostSummaryResponse is the response for GET /api/analytics/posts/summary.
type PostSummaryResponse struct {
	MonthlyStats []struct {
		Month string `json:"month"`
		Posts int    `json:"posts"`
		Views int    `json:"views"`
	} `json:"monthlyStats"`
	CategoryDistribution []struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	} `json:"categoryDistribution"`
	Overview struct {
		TotalPosts     int `json:"totalPosts"`
		PublishedPosts int `json:"publishedPosts"`
		DraftPosts     int `json:"draftPosts"`
	} `json:"overview"`
}

// TopContentResponse is the response for GET /api/analytics/top-content.
type TopContentResponse struct {
	TopPages   []ContentItem `json:"topPages"`
	TopCourses []ContentItem `json:"topCourses"`
	TopPosts   []ContentItem `json:"topPosts"`
}

// TrafficSourcesResponse is the response for GET /api/analytics/traffic-sources.
type TrafficSourcesResponse struct {
	Sources []TrafficSource `json:"sources"`
}

// UserJourneyResponse is the response for GET /api/analytics/user-journey/:sessionId.
type UserJourneyResponse struct {
	SessionID string      `json:"sessionId"`
	Events    []EventItem `json:"events"`
}

// EventItem holds a single event in a user journey.
type EventItem struct {
	EventName  string                 `json:"eventName"`
	EventType  string                 `json:"eventType"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// SessionResponse is the response for GET /api/analytics/sessions/:sessionId.
type SessionResponse struct {
	SessionID    string    `json:"sessionId"`
	VisitID      string    `json:"visitId,omitempty"`
	PageURL      string    `json:"pageUrl,omitempty"`
	DeviceType   string    `json:"deviceType,omitempty"`
	DurationSecs int       `json:"durationSeconds,omitempty"`
	PageViews    int       `json:"pageViews,omitempty"`
	CreatedAt    time.Time `json:"createdAt,omitempty"`
}
