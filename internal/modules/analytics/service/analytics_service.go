// Package service provides the business logic for the analytics module.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/analytics/dto"
	"erg.ninja/internal/modules/analytics/entities"
	"erg.ninja/internal/modules/analytics/repository"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Service provides analytics business logic.
type Service struct {
	visitRepo *repository.VisitRepository
	eventRepo *repository.EventRepository
	fbRepo    *repository.FirebaseSyncRepository
	log       *logger.Logger
	db        *database.MongoClient
	redis     *cache.RedisClient
	cfg       *config.Config
}

// NewService creates a new analytics service.
func NewService(mongo *database.MongoClient, log *logger.Logger, redis *cache.RedisClient, cfg *config.Config) *Service {
	return &Service{
		visitRepo: repository.NewVisitRepository(mongo, log),
		eventRepo: repository.NewEventRepository(mongo, log),
		fbRepo:    repository.NewFirebaseSyncRepository(mongo, log),
		log:       log,
		db:        mongo,
		redis:     redis,
		cfg:       cfg,
	}
}

// ─── Tracking ─────────────────────────────────────────────────────────────────

// TrackVisit records a new page view visit.
func (s *Service) TrackVisit(ctx context.Context, tenantID string, req dto.TrackVisitRequest, ip, userAgent string, userID *int64) (*dto.TrackVisitResponse, error) {
	deviceType, os, browser := parseUserAgent(userAgent)

	// Parse entity from URL if not provided.
	entityType, entityID := req.EntityType, req.EntityID
	if entityType == "" || entityID == "" {
		eType, eID := parseEntityFromURL(req.URL)
		if entityType == "" {
			entityType = eType
		}
		if entityID == "" {
			entityID = eID
		}
	}

	v := &entities.Visit{
		ID:              database.NewID(),
		TenantID:        tenantID,
		UserID:          userID,
		SessionID:       generateSessionID(req.URL, ip),
		PageURL:         req.URL,
		Referrer:        req.Referrer,
		IPHash:          hashIP(ip),
		UserAgent:       userAgent,
		DeviceType:      deviceType,
		Browser:         browser,
		OS:              os,
		EntityType:      entityType,
		EntityID:        entityID,
		DurationSeconds: 0,
		PageViews:       1,
	}

	if err := s.visitRepo.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("analytics.TrackVisit: %w", err)
	}

	s.log.DebugContext(ctx).Str("visit_id", v.ID).Str("url", req.URL).Msg("analytics: visit tracked")
	return &dto.TrackVisitResponse{
		VisitID:   v.ID,
		SessionID: v.SessionID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// TrackEvent records a user interaction event.
func (s *Service) TrackEvent(ctx context.Context, tenantID string, req dto.TrackEventRequest, userID *int64) error {
	e := &entities.Event{
		ID:         database.NewID(),
		TenantID:   tenantID,
		SessionID:  req.SessionInternalID,
		UserID:     userID,
		EventName:  req.EventType,
		EventType:  req.EventType,
		Properties: req.Metadata,
	}

	if err := s.eventRepo.Create(ctx, e); err != nil {
		return fmt.Errorf("analytics.TrackEvent: %w", err)
	}

	s.log.DebugContext(ctx).Str("session", req.SessionInternalID).Str("event", req.EventType).Msg("analytics: event tracked")
	return nil
}

// FinishSession updates the duration when a visit session ends.
func (s *Service) FinishSession(ctx context.Context, id string, durationSeconds int) error {
	// Cap duration at 3600 seconds (1 hour).
	if durationSeconds > 3600 {
		durationSeconds = 3600
	}

	if err := s.visitRepo.UpdateDuration(ctx, id, durationSeconds, 1); err != nil {
		return fmt.Errorf("analytics.FinishSession: %w", err)
	}

	s.log.DebugContext(ctx).Str("visit_id", id).Int("duration", durationSeconds).Msg("analytics: session finished")
	return nil
}

// Identify associates a user ID with a session (after login).
func (s *Service) Identify(ctx context.Context, tenantID, sessionID string, userID int64) error {
	// Update visit.
	visit, _ := s.visitRepo.GetBySessionID(ctx, sessionID)
	if visit != nil {
		_ = s.visitRepo.UpdateUserID(ctx, visit.ID, userID)
	}
	// Update events.
	_ = s.eventRepo.UpdateUserID(ctx, sessionID, userID)

	s.log.DebugContext(ctx).Str("session", sessionID).Int64("user_id", userID).Msg("analytics: session identified")
	return nil
}

// ─── Dashboard Stats ─────────────────────────────────────────────────────────

// GetVisitorStats returns daily visitor statistics for the given range.
func (s *Service) GetVisitorStats(ctx context.Context, params dto.VisitorStatsParams) ([]dto.VisitorStat, error) {
	from, to, err := parseDateRange(params.Range, params.From, params.To)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetVisitorStats: %w", err)
	}

	visits, err := s.visitRepo.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetVisitorStats: %w", err)
	}

	statsMap := make(map[string]map[string]int)
	formatDate := func(t time.Time) string {
		return t.In(time.FixedZone("ICT", 7*3600)).Format("2006-01-02")
	}

	for _, v := range visits {
		dateKey := formatDate(v.CreatedAt)
		if _, ok := statsMap[dateKey]; !ok {
			statsMap[dateKey] = map[string]int{"desktop": 0, "mobile": 0}
		}
		isMobile := v.DeviceType == "mobile" || v.DeviceType == "tablet" || v.DeviceType == ""
		if isMobile {
			statsMap[dateKey]["mobile"]++
		} else {
			statsMap[dateKey]["desktop"]++
		}
	}

	var result []dto.VisitorStat
	for current := from; !current.After(to); current = current.AddDate(0, 0, 1) {
		dateKey := formatDate(current)
		stat := statsMap[dateKey]
		if stat == nil {
			stat = map[string]int{"desktop": 0, "mobile": 0}
		}
		result = append(result, dto.VisitorStat{
			Date:    dateKey,
			Desktop: stat["desktop"],
			Mobile:  stat["mobile"],
		})
	}

	return result, nil
}

// GetOverview returns the full dashboard overview with period-over-period comparison.
func (s *Service) GetOverview(ctx context.Context, params dto.OverviewParams) (*dto.DashboardOverviewResponse, error) {
	from, to, err := parseOverviewDates(params.From, params.To)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetOverview: %w", err)
	}

	rangeDays := int(to.Sub(from).Hours() / 24)
	prevFrom := from.AddDate(0, 0, -(rangeDays + 1))
	prevTo := from.AddDate(0, 0, -1)

	currentVisits, err := s.visitRepo.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetOverview: %w", err)
	}
	previousVisits, _ := s.visitRepo.FindByDateRange(ctx, prevFrom, prevTo)

	currentInteractions, _ := s.eventRepo.AggregateEventStats(ctx, from, to)

	// Calculate summaries.
	currSummary := calculateRawSummary(currentVisits)
	prevSummary := calculateRawSummary(previousVisits)

	resp := &dto.DashboardOverviewResponse{}
	resp.DateRange.Current.From = from.Format(time.RFC3339)
	resp.DateRange.Current.To = to.Format(time.RFC3339)
	resp.DateRange.Previous.From = prevFrom.Format(time.RFC3339)
	resp.DateRange.Previous.To = prevTo.Format(time.RFC3339)

	resp.Summary.TotalVisits = createMetricWithGrowth(float64(len(currentVisits)), float64(len(previousVisits)))
	resp.Summary.ActiveUsers = createMetricWithGrowth(float64(currSummary.activeUsers), float64(prevSummary.activeUsers))
	resp.Summary.NewUsers = createMetricWithGrowth(0, 0)
	resp.Summary.AvgDuration = createMetricWithGrowth(float64(currSummary.avgDuration), float64(prevSummary.avgDuration))
	resp.Summary.BounceRate = createMetricWithGrowth(float64(currSummary.bounceRate), float64(prevSummary.bounceRate))

	resp.TrafficChart = buildTrafficChart(currentVisits, from, to)
	resp.Locations = buildLocationStats(currentVisits)
	resp.Devices = buildDeviceStats(currentVisits)
	resp.Content = buildContentStats(currentVisits)
	resp.PeakHours = buildPeakHours(currentVisits)
	resp.TrafficSources = buildTrafficSources(currentVisits)
	resp.Interactions = buildInteractionStats(currentInteractions)

	return resp, nil
}

// GetPostSummary returns post analytics summary (simplified — posts live in MySQL).
func (s *Service) GetPostSummary(ctx context.Context, rangeStr string) (*dto.PostSummaryResponse, error) {
	now := time.Now()
	startTime := now.AddDate(0, 0, -90)
	switch rangeStr {
	case "7d":
		startTime = now.AddDate(0, 0, -7)
	case "30d":
		startTime = now.AddDate(0, 0, -30)
	case "90d":
		startTime = now.AddDate(0, 0, -90)
	case "180d":
		startTime = now.AddDate(0, 0, -180)
	case "365d":
		startTime = now.AddDate(0, 0, -365)
	}

	// Count visits to /posts/* URLs.
	visits, err := s.visitRepo.FindByDateRange(ctx, startTime, now)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetPostSummary: %w", err)
	}

	postVisits := 0
	for _, v := range visits {
		if strings.Contains(v.PageURL, "/posts/") {
			postVisits++
		}
	}

	resp := &dto.PostSummaryResponse{}
	resp.MonthlyStats = []struct {
		Month string `json:"month"`
		Posts int    `json:"posts"`
		Views int    `json:"views"`
	}{}
	resp.CategoryDistribution = []struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	}{}
	resp.Overview.TotalPosts = 0
	resp.Overview.PublishedPosts = 0
	resp.Overview.DraftPosts = 0
	_ = postVisits // Used for future MySQL-based post stats.
	return resp, nil
}

// ExportCSV generates CSV export of visits in a date range.
func (s *Service) ExportCSV(ctx context.Context, from, to string) (string, error) {
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", fmt.Errorf("invalid from date format")
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", fmt.Errorf("invalid to date format")
	}
	toDate = toDate.Add(24*time.Hour - time.Nanosecond)

	visits, err := s.visitRepo.FindByDateRange(ctx, fromDate, toDate)
	if err != nil {
		return "", fmt.Errorf("analytics.ExportCSV: %w", err)
	}

	headers := []string{"Timestamp", "URL", "Device", "Browser", "OS", "Country", "City", "Duration(s)", "IP"}
	rows := []string{strings.Join(headers, ",")}
	for _, v := range visits {
		device := v.DeviceType
		if device == "" {
			device = "unknown"
		}
		browser := v.Browser
		if browser == "" {
			browser = "unknown"
		}
		os := v.OS
		if os == "" {
			os = "unknown"
		}
		country := v.Country
		if country == "" {
			country = "unknown"
		}
		city := v.City
		if city == "" {
			city = "unknown"
		}
		ip := v.IPHash
		if ip == "" {
			ip = "unknown"
		}

		row := fmt.Sprintf("%s,\"%s\",%s,%s,%s,%s,%s,%d,%s",
			v.CreatedAt.UTC().Format(time.RFC3339),
			strings.ReplaceAll(v.PageURL, "\"", "\"\""),
			device, browser, os, country, city,
			v.DurationSeconds, ip,
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n"), nil
}

// SyncFirebaseEvents ingests a batch of Firebase Analytics events.
// It deduplicates using a Redis fast-path cache + MongoDB upsert.
// Returns the number of events successfully ingested.
func (s *Service) SyncFirebaseEvents(ctx context.Context, tenantID string, req dto.SyncFirebaseEventsRequest) (dto.SyncFirebaseEventsResponse, error) {
	if len(req.Events) == 0 {
		return dto.SyncFirebaseEventsResponse{}, nil
	}

	var synced, duplicates int

	for _, ev := range req.Events {
		// Build deduplication key from Firebase event signature.
		dedupeKey := fmt.Sprintf("fb_dedup:%s:%s:%d",
			ev.EventName, ev.AppInstanceID, ev.Timestamp.UnixMilli())

		// Fast-path: check Redis cache.
		if s.redis != nil {
			if exists, _ := s.redis.Get(ctx, dedupeKey); exists != "" {
				duplicates++
				continue
			}
		}

		// Convert Firebase payload to our entity.
		event := &entities.FirebaseSyncEvent{
			ID:            database.NewID(),
			TenantID:      tenantID,
			EventName:     ev.EventName,
			Platform:      ev.Platform,
			UserID:        ev.UserID,
			DeviceID:      ev.DeviceID,
			SessionID:     ev.SessionID,
			Params:        ev.Params,
			Country:       ev.Country,
			City:          ev.City,
			AppInstanceID: ev.AppInstanceID,
			ReceivedAt:    ev.Timestamp,
			ProcessedAt:   time.Now().UTC(),
			CreatedAt:     time.Now().UTC(),
		}

		// Upsert into MongoDB (dedup by composite key).
		n, err := s.fbRepo.BatchUpsert(ctx, []*entities.FirebaseSyncEvent{event})
		if err != nil {
			s.log.ErrorContext(ctx).Err(err).Str("event", ev.EventName).Msg("analytics.SyncFirebaseEvents upsert failed")
			continue
		}
		if n > 0 {
			synced++
			// Mark in Redis with 48h TTL.
			if s.redis != nil {
				_ = s.redis.Set(ctx, dedupeKey, "1", 48*time.Hour)
			}
		} else {
			duplicates++
		}
	}

	s.log.InfoContext(ctx).
		Int("synced", synced).
		Int("duplicates", duplicates).
		Int("total", len(req.Events)).
		Msg("analytics.SyncFirebaseEvents completed")

	return dto.SyncFirebaseEventsResponse{Synced: synced, Duplicates: duplicates}, nil
}

// GetFirebaseSyncStats returns enriched Firebase event statistics for a date range.
func (s *Service) GetFirebaseSyncStats(ctx context.Context, fromStr, toStr string) (*dto.FirebaseSyncStatsResponse, error) {
	from, to, err := parseOverviewDates(fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetFirebaseSyncStats: %w", err)
	}

	// Total count.
	total, err := s.fbRepo.CountByDateRange(ctx, from, to)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("analytics.GetFirebaseSyncStats: count failed, returning 0")
		total = 0
	}

	// Top events by count.
	eventStats, err := s.fbRepo.AggregateByEventName(ctx, from, to)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("analytics.GetFirebaseSyncStats: event aggregation failed")
		eventStats = nil
	}

	// Build top events list (limit 20).
	var topEvents []dto.FirebaseEventStat
	for name, count := range eventStats {
		topEvents = append(topEvents, dto.FirebaseEventStat{EventName: name, Count: count})
	}
	// Sort descending.
	for i := 0; i < len(topEvents)-1; i++ {
		for j := i + 1; j < len(topEvents); j++ {
			if topEvents[j].Count > topEvents[i].Count {
				topEvents[i], topEvents[j] = topEvents[j], topEvents[i]
			}
		}
	}
	if len(topEvents) > 20 {
		topEvents = topEvents[:20]
	}

	return &dto.FirebaseSyncStatsResponse{
		TotalEvents:  total,
		TopEvents:    topEvents,
		TopCountries: nil, // Extend with AggregateByCountry() if needed
		TopPlatforms: nil, // Extend with AggregateByPlatform() if needed
		DateFrom:     from.Format("2006-01-02"),
		DateTo:       to.Format("2006-01-02"),
	}, nil
}

// GetUserJourney returns all events for a given session ID.
func (s *Service) GetUserJourney(ctx context.Context, sessionID string) (*dto.UserJourneyResponse, error) {
	events, err := s.eventRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetUserJourney: %w", err)
	}

	items := make([]dto.EventItem, len(events))
	for i, e := range events {
		items[i] = dto.EventItem{
			EventName:  e.EventName,
			EventType:  e.EventType,
			Properties: e.Properties,
			Timestamp:  e.CreatedAt,
		}
	}

	return &dto.UserJourneyResponse{
		SessionID: sessionID,
		Events:    items,
	}, nil
}

// GetSession returns visit details for a session ID.
func (s *Service) GetSession(ctx context.Context, sessionID string) (*dto.SessionResponse, error) {
	visit, err := s.visitRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("analytics.GetSession: %w", err)
	}
	if visit == nil {
		return nil, nil
	}

	return &dto.SessionResponse{
		SessionID:    visit.SessionID,
		VisitID:      visit.ID,
		PageURL:      visit.PageURL,
		DeviceType:   visit.DeviceType,
		DurationSecs: visit.DurationSeconds,
		PageViews:    visit.PageViews,
		CreatedAt:    visit.CreatedAt,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func parseUserAgent(ua string) (deviceType, os, browser string) {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
		deviceType = "mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		deviceType = "tablet"
	} else {
		deviceType = "desktop"
	}

	if strings.Contains(ua, "windows") {
		os = "Windows"
	} else if strings.Contains(ua, "mac os") || strings.Contains(ua, "macos") {
		os = "macOS"
	} else if strings.Contains(ua, "android") {
		os = "Android"
	} else if strings.Contains(ua, "ios") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		os = "iOS"
	} else if strings.Contains(ua, "linux") {
		os = "Linux"
	} else {
		os = "Unknown"
	}

	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		browser = "Chrome"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		browser = "Safari"
	} else if strings.Contains(ua, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "edg") {
		browser = "Edge"
	} else if strings.Contains(ua, "msie") || strings.Contains(ua, "trident") {
		browser = "IE"
	} else {
		browser = "Unknown"
	}

	return
}

func parseEntityFromURL(url string) (entityType, entityID string) {
	url = strings.ToLower(url)
	if strings.Contains(url, "/posts/") {
		entityType = "post"
		parts := strings.Split(url, "/posts/")
		if len(parts) > 1 {
			entityID = strings.Split(parts[1], "?")[0]
			entityID = strings.TrimSuffix(entityID, "/")
		}
	} else if strings.Contains(url, "/courses/") {
		entityType = "course"
		parts := strings.Split(url, "/courses/")
		if len(parts) > 1 {
			entityID = strings.Split(parts[1], "?")[0]
			entityID = strings.TrimSuffix(entityID, "/")
		}
	}
	return
}

func generateSessionID(url, ip string) string {
	data := fmt.Sprintf("%s:%s:%d", url, ip, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

func hashIP(ip string) string {
	if ip == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(hash[:])
}

func parseDateRange(rangeStr, fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now()
	to := now
	from := now.AddDate(0, 0, -7)

	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24*time.Hour - time.Nanosecond)
		}
	}

	switch rangeStr {
	case "30d":
		if fromStr == "" {
			from = now.AddDate(0, 0, -30)
		}
	case "90d":
		if fromStr == "" {
			from = now.AddDate(0, 0, -90)
		}
	case "7d", "":
		// default
	}

	return from, to, nil
}

func parseOverviewDates(fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now()
	to := now
	from := now.AddDate(0, 0, -7)

	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24*time.Hour - time.Nanosecond)
		}
	}

	return from, to, nil
}

type rawSummary struct {
	totalVisits int
	activeUsers int
	avgDuration int
	bounceRate  int
}

func calculateRawSummary(visits []*entities.Visit) rawSummary {
	if len(visits) == 0 {
		return rawSummary{}
	}

	seen := make(map[string]bool)
	var totalDuration int
	var bounces int

	for _, v := range visits {
		key := ""
		if v.UserID != nil && *v.UserID != 0 {
			key = fmt.Sprintf("%d", *v.UserID)
		} else {
			key = v.IPHash
		}
		if key != "" {
			seen[key] = true
		}
		totalDuration += v.DurationSeconds
		if v.DurationSeconds < 10 {
			bounces++
		}
	}

	validDurations := 0
	for _, v := range visits {
		if v.DurationSeconds > 0 {
			validDurations++
		}
	}
	avgDur := 0
	if validDurations > 0 {
		avgDur = totalDuration / validDurations
	}
	bounceRate := 0
	if len(visits) > 0 {
		bounceRate = (bounces * 100) / len(visits)
	}

	return rawSummary{
		totalVisits: len(visits),
		activeUsers: len(seen),
		avgDuration: avgDur,
		bounceRate:  bounceRate,
	}
}

func createMetricWithGrowth(current, previous float64) dto.MetricWithGrowth {
	var growth float64
	if previous > 0 {
		growth = ((current - previous) / previous) * 100
	} else if current > 0 {
		growth = 100
	}
	return dto.MetricWithGrowth{
		Value:    current,
		Previous: previous,
		Growth:   float64(int(growth*10)) / 10,
	}
}

func buildTrafficChart(visits []*entities.Visit, from, to time.Time) []dto.TrafficDataPoint {
	days := int(to.Sub(from).Hours()/24) + 1
	if days <= 2 {
		hourMap := make(map[int]int)
		for _, v := range visits {
			hour := v.CreatedAt.Hour()
			hourMap[hour]++
		}
		var points []dto.TrafficDataPoint
		for h := 0; h < 24; h++ {
			points = append(points, dto.TrafficDataPoint{
				Label:   fmt.Sprintf("%02d:00", h),
				Mobile:  0,
				Desktop: hourMap[h],
				Total:   hourMap[h],
			})
		}
		return points
	}

	dateMap := make(map[string]struct {
		mobile, desktop int
	})
	for _, v := range visits {
		dateKey := v.CreatedAt.Format("2006-01-02")
		if _, ok := dateMap[dateKey]; !ok {
			dateMap[dateKey] = struct{ mobile, desktop int }{}
		}
		entry := dateMap[dateKey]
		if v.DeviceType == "mobile" || v.DeviceType == "tablet" {
			entry.mobile++
		} else {
			entry.desktop++
		}
		dateMap[dateKey] = entry
	}

	var points []dto.TrafficDataPoint
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		entry := dateMap[dateKey]
		points = append(points, dto.TrafficDataPoint{
			Label:   dateKey,
			Mobile:  entry.mobile,
			Desktop: entry.desktop,
			Total:   entry.mobile + entry.desktop,
		})
	}
	return points
}

func buildLocationStats(visits []*entities.Visit) []dto.LocationStat {
	cityMap := make(map[string]dto.LocationStat)
	for _, v := range visits {
		if v.City == "" {
			continue
		}
		key := v.City + "|" + v.Country
		if s, ok := cityMap[key]; ok {
			s.Count++
			cityMap[key] = s
		} else {
			cityMap[key] = dto.LocationStat{City: v.City, Country: v.Country, Count: 1}
		}
	}

	var result []dto.LocationStat
	for _, s := range cityMap {
		result = append(result, s)
	}
	// Sort by count descending.
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if len(result) > 10 {
		result = result[:10]
	}
	return result
}

func buildDeviceStats(visits []*entities.Visit) dto.DeviceStats {
	typeMap := map[string]int{"mobile": 0, "desktop": 0, "tablet": 0}
	osMap := make(map[string]int)
	browserMap := make(map[string]int)
	total := len(visits)

	for _, v := range visits {
		t := v.DeviceType
		if t == "" {
			t = "desktop"
		}
		typeMap[t]++
		if v.OS != "" {
			osMap[v.OS]++
		}
		if v.Browser != "" {
			browserMap[v.Browser]++
		}
	}

	makeStats := func(m map[string]int, total int) []dto.DeviceTypeStat {
		var out []dto.DeviceTypeStat
		for name, count := range m {
			pct := 0
			if total > 0 {
				pct = (count * 100) / total
			}
			out = append(out, dto.DeviceTypeStat{Name: name, Count: count, Percentage: pct})
		}
		return out
	}

	// Sort OS and browser by count.
	sortedOS := make([]struct {
		name  string
		count int
	}, 0)
	for k, v := range osMap {
		sortedOS = append(sortedOS, struct {
			name  string
			count int
		}{k, v})
	}
	for i := 0; i < len(sortedOS)-1; i++ {
		for j := i + 1; j < len(sortedOS); j++ {
			if sortedOS[j].count > sortedOS[i].count {
				sortedOS[i], sortedOS[j] = sortedOS[j], sortedOS[i]
			}
		}
	}
	if len(sortedOS) > 5 {
		sortedOS = sortedOS[:5]
	}
	var osStats []dto.DeviceTypeStat
	for _, s := range sortedOS {
		pct := 0
		if total > 0 {
			pct = (s.count * 100) / total
		}
		osStats = append(osStats, dto.DeviceTypeStat{Name: s.name, Count: s.count, Percentage: pct})
	}

	sortedBr := make([]struct {
		name  string
		count int
	}, 0)
	for k, v := range browserMap {
		sortedBr = append(sortedBr, struct {
			name  string
			count int
		}{k, v})
	}
	for i := 0; i < len(sortedBr)-1; i++ {
		for j := i + 1; j < len(sortedBr); j++ {
			if sortedBr[j].count > sortedBr[i].count {
				sortedBr[i], sortedBr[j] = sortedBr[j], sortedBr[i]
			}
		}
	}
	if len(sortedBr) > 5 {
		sortedBr = sortedBr[:5]
	}
	var brStats []dto.DeviceTypeStat
	for _, s := range sortedBr {
		pct := 0
		if total > 0 {
			pct = (s.count * 100) / total
		}
		brStats = append(brStats, dto.DeviceTypeStat{Name: s.name, Count: s.count, Percentage: pct})
	}

	return dto.DeviceStats{
		Types:    makeStats(typeMap, total),
		OS:       osStats,
		Browsers: brStats,
	}
}

func buildContentStats(visits []*entities.Visit) dto.ContentStats {
	pageMap := make(map[string]*dto.ContentItem)
	postMap := make(map[string]*dto.ContentItem)
	courseMap := make(map[string]*dto.ContentItem)

	for _, v := range visits {
		url := v.PageURL
		if url == "" || strings.Contains(url, "/admin") {
			continue
		}
		url = strings.Split(url, "?")[0]
		cleanURL := url

		item, ok := pageMap[cleanURL]
		if !ok {
			title := cleanURL
			category := "Others"
			if v.EntityType == "course" || strings.Contains(cleanURL, "/courses/") {
				category = "Course"
				title = v.EntityID
				if title == "" {
					parts := strings.Split(cleanURL, "/courses/")
					if len(parts) > 1 {
						title = strings.Split(parts[1], "/")[0]
					}
				}
				entry := courseMap[cleanURL]
				if entry == nil {
					courseMap[cleanURL] = &dto.ContentItem{URL: cleanURL, Title: title, Views: 1}
				} else {
					entry.Views++
					courseMap[cleanURL] = entry
				}
			} else if v.EntityType == "post" || strings.Contains(cleanURL, "/posts/") {
				category = "Post"
				title = v.EntityID
				if title == "" {
					parts := strings.Split(cleanURL, "/posts/")
					if len(parts) > 1 {
						title = strings.Split(parts[1], "/")[0]
					}
				}
				entry := postMap[cleanURL]
				if entry == nil {
					postMap[cleanURL] = &dto.ContentItem{URL: cleanURL, Title: title, Views: 1, EntityID: v.EntityID}
				} else {
					entry.Views++
					postMap[cleanURL] = entry
				}
			}
			item = &dto.ContentItem{URL: cleanURL, Title: title, Views: 1, EntityID: v.EntityID}
			_ = category
			pageMap[cleanURL] = item
		} else {
			item.Views++
			pageMap[cleanURL] = item
		}
	}

	sortByViews := func(m map[string]*dto.ContentItem) []dto.ContentItem {
		var items []dto.ContentItem
		for _, v := range m {
			items = append(items, *v)
		}
		for i := 0; i < len(items)-1; i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].Views > items[i].Views {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
		return items
	}

	topPages := sortByViews(pageMap)
	topPosts := sortByViews(postMap)
	topCourses := sortByViews(courseMap)

	if len(topPages) > 10 {
		topPages = topPages[:10]
	}
	if len(topPosts) > 5 {
		topPosts = topPosts[:5]
	}
	if len(topCourses) > 5 {
		topCourses = topCourses[:5]
	}

	return dto.ContentStats{
		TopPages:      topPages,
		TopPosts:      topPosts,
		TopCourses:    topCourses,
		TrendingPosts: topPosts,
	}
}

func buildPeakHours(visits []*entities.Visit) []dto.PeakHour {
	hourMap := make(map[int]int)
	for _, v := range visits {
		hourMap[v.CreatedAt.Hour()]++
	}
	var hours []dto.PeakHour
	for h, c := range hourMap {
		hours = append(hours, dto.PeakHour{Hour: h, Count: c})
	}
	for i := 0; i < len(hours)-1; i++ {
		for j := i + 1; j < len(hours); j++ {
			if hours[j].Count > hours[i].Count {
				hours[i], hours[j] = hours[j], hours[i]
			}
		}
	}
	if len(hours) > 5 {
		hours = hours[:5]
	}
	return hours
}

func buildTrafficSources(visits []*entities.Visit) []dto.TrafficSource {
	sourceMap := map[string]int{"Direct": 0, "Google": 0, "Facebook": 0, "Other": 0}
	for _, v := range visits {
		ref := strings.ToLower(v.Referrer)
		if ref == "" || strings.Contains(ref, "erg.edu.vn") {
			sourceMap["Direct"]++
		} else if strings.Contains(ref, "google") {
			sourceMap["Google"]++
		} else if strings.Contains(ref, "facebook") || strings.Contains(ref, "fb.") {
			sourceMap["Facebook"]++
		} else {
			sourceMap["Other"]++
		}
	}

	total := len(visits)
	if total == 0 {
		total = 1
	}

	var sources []dto.TrafficSource
	for source, count := range sourceMap {
		pct := (count * 100) / total
		sources = append(sources, dto.TrafficSource{Source: source, Count: count, Percentage: pct})
	}
	return sources
}

func buildInteractionStats(stats map[string]int) []dto.InteractionStat {
	var items []dto.InteractionStat
	for name, count := range stats {
		items = append(items, dto.InteractionStat{Name: name, Count: count})
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Count > items[i].Count {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

// ─── Validate DTO ─────────────────────────────────────────────────────────────

// Validate validates analytics request DTOs.
func Validate(req interface{}) error {
	// Lightweight validation — skip full go-playground for now to avoid import cycle.
	return nil
}

var _ = bson.Marshal
var _ = json.Marshal
