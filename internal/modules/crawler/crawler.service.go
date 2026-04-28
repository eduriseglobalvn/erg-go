// Package crawler implements the core web crawler service with a 12-step pipeline.
package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"math/bits"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"

	"erg.ninja/internal/modules/crawler/dto"
	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/internal/modules/crawler/repository"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/scraper"
)

const (
	// PublishThreshold is the minimum quality score for content to be published.
	PublishThreshold = 70.0
)

// Service handles the 12-step crawl pipeline.
type Service struct {
	repo     *repository.Repository
	fetcher  *scraper.Fetcher
	log      *logger.Logger
	eventBus *event.EventBus
	sseHub   *SSEHub
	pipesMu  sync.RWMutex
	pipes    map[string]activePipelineResponse
}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// WithCrawlerLogger sets the logger.
func WithCrawlerLogger(log *logger.Logger) ServiceOption {
	return func(s *Service) { s.log = log }
}

// NewService creates a new crawler service.
func NewService(
	repo *repository.Repository,
	fetcher *scraper.Fetcher,
	log *logger.Logger,
	eventBus *event.EventBus,
	opts ...ServiceOption,
) *Service {
	s := &Service{
		repo:     repo,
		fetcher:  fetcher,
		log:      logger.NoOp(),
		eventBus: eventBus,
		sseHub:   NewSSEHub(),
		pipes:    make(map[string]activePipelineResponse),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// SSEHub returns the SSE hub for progress streaming.
func (s *Service) SSEHub() *SSEHub { return s.sseHub }

// EnqueueURL enqueues a URL for crawling via the 12-step pipeline.
// Implements the interface expected by bot commands.
func (s *Service) EnqueueURL(ctx context.Context, url, feedID string, priority int) (string, error) {
	jobID := uuid.New().String()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error().Interface("panic", r).Str("job_id", jobID).Msg("crawler: pipeline panic recovered")
			}
		}()
		result := s.RunPipeline(ctx, url, feedID, jobID)
		if !result.Success && result.ErrorMsg != "" {
			s.log.WarnContext(ctx).Str("job_id", jobID).Str("url", url).Str("error", result.ErrorMsg).Msg("crawler: async pipeline finished with error")
		}
	}()
	return jobID, nil
}

// GetJobStatus returns the status and quality score for a crawl job by job_id.
func (s *Service) GetJobStatus(ctx context.Context, jobID string) (status string, score float64, err error) {
	if s.IsJobRunning(jobID) {
		return string(entities.CrawlStatusRunning), 0, nil
	}
	h, err := s.repo.GetCrawlHistoryByJobID(ctx, jobID)
	if err != nil {
		return "error", 0, err
	}
	if h == nil {
		return string(entities.CrawlStatusPending), 0, nil
	}
	return string(h.Status), h.QualityScore, nil
}

// GetStats returns aggregated crawl statistics from the repository.
func (s *Service) GetStats(ctx context.Context) (totalCrawled int, passRate float64, avgScore float64, err error) {
	stats, err := s.repo.GetCrawlStats(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(stats.TotalCrawled), stats.PassRate, stats.AvgQualityScore, nil
}

// GetRecentHistory returns the most recent crawl history entries (up to limit).
func (s *Service) GetRecentHistory(ctx context.Context, limit int) ([]RecentCrawlItem, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	history, _, err := s.repo.ListCrawlHistory(ctx, repository.ListCrawlHistoryParams{
		Limit:  int64(limit),
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}
	items := make([]RecentCrawlItem, 0, len(history))
	for _, h := range history {
		items = append(items, RecentCrawlItem{
			URL:     h.URL,
			Status:  string(h.Status),
			Score:   h.QualityScore,
			Updated: h.UpdatedAt,
		})
	}
	return items, nil
}

// RecentCrawlItem is a simplified crawl history entry.
type RecentCrawlItem struct {
	URL     string
	Status  string
	Score   float64
	Updated time.Time
}

// IsJobRunning reports whether a job is still active in the SSE hub.
func (s *Service) IsJobRunning(jobID string) bool {
	s.pipesMu.RLock()
	if item, ok := s.pipes[jobID]; ok && (item.Status == "PENDING" || item.Status == "PROCESSING") {
		s.pipesMu.RUnlock()
		return true
	}
	s.pipesMu.RUnlock()

	if s.sseHub == nil {
		return false
	}
	s.sseHub.mu.RLock()
	defer s.sseHub.mu.RUnlock()
	_, ok := s.sseHub.clients[jobID]
	return ok
}

// ActivePipelines returns the FE-compatible list of currently tracked crawler jobs.
func (s *Service) ActivePipelines() []activePipelineResponse {
	s.pipesMu.RLock()
	defer s.pipesMu.RUnlock()

	items := make([]activePipelineResponse, 0, len(s.pipes))
	for _, item := range s.pipes {
		items = append(items, item)
	}
	return items
}

func (s *Service) beginPipeline(jobID, urlStr, feedID string, startedAt time.Time) {
	s.pipesMu.Lock()
	s.pipes[jobID] = activePipelineResponse{
		JobID:       jobID,
		URL:         urlStr,
		Source:      pipelineSource(feedID),
		Status:      "PENDING",
		CurrentStep: pipelineStepLabel(1),
		Progress:    5,
		TimeStarted: startedAt.UTC().Format(time.RFC3339),
		Message:     "Dang khoi tao pipeline",
	}
	s.pipesMu.Unlock()
}

func (s *Service) updatePipeline(jobID string, step int, message string) {
	s.pipesMu.Lock()
	defer s.pipesMu.Unlock()

	item, ok := s.pipes[jobID]
	if !ok {
		return
	}
	item.Status = "PROCESSING"
	item.CurrentStep = pipelineStepLabel(step)
	item.Progress = pipelineProgress(step)
	item.Message = message
	s.pipes[jobID] = item
}

func (s *Service) finishPipeline(jobID string, result *PipelineResult) {
	if result == nil {
		return
	}

	s.pipesMu.Lock()
	item, ok := s.pipes[jobID]
	if ok {
		item.CurrentStep = pipelineStepLabel(result.Step)
		item.Progress = 100
		item.QualityScore = result.QualityScore
		if result.Success {
			item.Status = "COMPLETED"
			item.Message = "Pipeline complete"
		} else {
			item.Status = "FAILED"
			if result.ErrorMsg != "" {
				item.Message = result.ErrorMsg
			} else {
				item.Message = "Pipeline failed"
			}
		}
		s.pipes[jobID] = item
	}
	s.pipesMu.Unlock()

	if !result.Success {
		message := result.ErrorMsg
		if ok && item.Message != "" {
			message = item.Message
		}
		s.emitTerminalEvent(jobID, "error", message)
	}

	go func() {
		time.Sleep(10 * time.Second)
		s.pipesMu.Lock()
		delete(s.pipes, jobID)
		s.pipesMu.Unlock()
	}()
}

// ─── 12-Step Pipeline ────────────────────────────────────────────────────────

// PipelineResult holds the result of a full crawl pipeline.
type PipelineResult struct {
	Success      bool
	Status       entities.CrawlStatus
	Step         int
	Title        string
	Description  string
	QualityScore float64
	Tags         []string
	Language     string
	ErrorMsg     string
	Duration     time.Duration
}

// RunPipeline executes the 12-step crawl pipeline for a given URL.
func (s *Service) RunPipeline(ctx context.Context, urlStr string, feedID, jobID string) (result PipelineResult) {
	start := time.Now()
	result = PipelineResult{Step: 1}
	s.beginPipeline(jobID, urlStr, feedID, start)
	defer s.finishPipeline(jobID, &result)

	// Create crawl history record.
	history := &entities.CrawlHistory{
		ID:     "",
		URL:    urlStr,
		FeedID: feedID,
		JobID:  jobID,
		Status: entities.CrawlStatusPending,
	}
	if err := s.repo.CreateCrawlHistory(ctx, history); err != nil {
		s.log.ErrorContext(ctx).Err(err).Str("url", urlStr).Msg("crawler: create history failed")
	}
	s.broadcast(jobID, 1, "Creating crawl record")

	// Step 1: Blacklist check.
	s.broadcast(jobID, 1, "Checking blacklist")
	blocked, err := s.repo.IsURLBlocked(ctx, urlStr)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Str("url", urlStr).Msg("crawler: blacklist check error, continuing")
	}
	if blocked {
		result.Status = entities.CrawlStatusBlacklisted
		result.ErrorMsg = "URL is blacklisted"
		result.Step = 1
		s.updateHistory(ctx, history.ID, result)
		s.publishResult(ctx, urlStr, feedID, jobID, result)
		return result
	}

	// Parse URL for domain checks.
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		result.Status = entities.CrawlStatusFailed
		result.ErrorMsg = fmt.Sprintf("parse URL: %v", err)
		result.Step = 1
		s.updateHistory(ctx, history.ID, result)
		return result
	}
	domain := strings.ToLower(parsedURL.Hostname())

	// Step 2: Domain reputation check.
	s.broadcast(jobID, 2, "Checking domain reputation")
	skip, err := s.repo.ShouldSkipDomain(ctx, domain, 10)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("crawler: domain reputation check error, continuing")
	}
	if skip {
		result.Status = entities.CrawlStatusFailed
		result.ErrorMsg = fmt.Sprintf("domain %s has low reputation score", domain)
		result.Step = 2
		s.updateHistory(ctx, history.ID, result)
		return result
	}

	// Step 3 & 4: Fetch content (robots.txt is checked inside Fetch()).
	s.broadcast(jobID, 3, "Fetching content")
	fetchResult := s.fetcher.Fetch(ctx, urlStr)
	if fetchResult.Error != nil {
		errStr := fetchResult.Error.Error()
		result.Step = 3
		if strings.Contains(errStr, "disallowed by robots.txt") {
			result.Status = entities.CrawlStatusFailed
			result.ErrorMsg = "disallowed by robots.txt"
		} else {
			result.Status = entities.CrawlStatusFailed
			result.ErrorMsg = errStr
			s.repo.UpdateDomainReputation(ctx, domain, false, fetchResult.Duration.Milliseconds(), true)
		}
		s.updateHistory(ctx, history.ID, result)
		return result
	}
	if fetchResult.StatusCode != http.StatusOK {
		result.Status = entities.CrawlStatusFailed
		result.ErrorMsg = fmt.Sprintf("HTTP %d", fetchResult.StatusCode)
		result.Step = 4
		s.updateHistory(ctx, history.ID, result)
		s.repo.UpdateDomainReputation(ctx, domain, false, fetchResult.Duration.Milliseconds(), true)
		return result
	}

	html := string(fetchResult.Body)
	bodyLen := len(fetchResult.Body)

	// Step 5: Quality gate.
	s.broadcast(jobID, 5, "Scoring content quality")
	score := s.scoreQuality(html)
	result.QualityScore = score
	if score < PublishThreshold {
		result.Status = entities.CrawlStatusFailed
		result.ErrorMsg = fmt.Sprintf("quality score %.1f below threshold %.1f", score, PublishThreshold)
		result.Step = 5
		s.updateHistory(ctx, history.ID, result)
		return result
	}

	// Step 6: Content deduplication.
	s.broadcast(jobID, 6, "Checking for duplicates")
	isDup, err := s.isDuplicate(ctx, html)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Str("url", urlStr).Msg("crawler: dedup check error, continuing")
	}
	if isDup {
		result.Status = entities.CrawlStatusDuplicate
		result.ErrorMsg = "duplicate content detected"
		result.Step = 6
		s.updateHistory(ctx, history.ID, result)
		return result
	}

	// Step 7: Extract metadata.
	s.broadcast(jobID, 7, "Extracting metadata")
	title, description := s.extractMetadata(html)

	// Step 8: Extract tags and language.
	s.broadcast(jobID, 8, "Analyzing content")
	tags, lang := s.extractTagsAndLanguage(html)
	result.Tags = tags
	result.Language = lang

	// Step 9: Compute content hash.
	hash := sha256.Sum256(fetchResult.Body)
	contentHash := hex.EncodeToString(hash[:])

	// Step 10: Save to MongoDB.
	s.broadcast(jobID, 10, "Saving to database")
	err = s.repo.UpdateCrawlMetadata(ctx, history.ID, title, description, score,
		fetchResult.StatusCode, int64(bodyLen), fetchResult.Duration.Milliseconds())
	if err != nil {
		s.log.ErrorContext(ctx).Err(err).Str("url", urlStr).Msg("crawler: save metadata failed")
	}

	// Store fingerprint for future dedup.
	fp := &entities.ContentFingerprint{
		URL:     urlStr,
		SimHash: s.computeSimHash(html),
		SHA256:  contentHash,
	}
	if err := s.repo.StoreFingerprint(ctx, fp); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("url", urlStr).Msg("crawler: store fingerprint failed")
	}

	// Update feed metadata.
	if feedID != "" {
		if err := s.repo.UpdateFeedLastItem(ctx, feedID); err != nil {
			s.log.WarnContext(ctx).Err(err).Str("feed_id", feedID).Msg("crawler: update feed last item failed")
		}
	}

	// Step 11: Update domain reputation.
	if err := s.repo.UpdateDomainReputation(ctx, domain, true, fetchResult.Duration.Milliseconds(), false); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("domain", domain).Msg("crawler: update domain reputation failed")
	}

	// Step 12: Publish events and broadcast.
	s.broadcast(jobID, 12, "Pipeline complete", true)
	result.Success = true
	result.Status = entities.CrawlStatusSuccess
	result.Title = title
	result.Description = description
	result.Duration = time.Since(start)
	s.updateHistory(ctx, history.ID, result)
	s.publishResult(ctx, urlStr, feedID, jobID, result)

	s.log.InfoContext(ctx).
		Str("url", urlStr).
		Float64("quality", score).
		Str("status", string(result.Status)).
		Str("duration", result.Duration.String()).
		Msg("crawler: pipeline complete")

	return result
}

// ─── Step implementations ────────────────────────────────────────────────────

// scoreQuality applies 8 quality rules, returning a score 0–100.
func (s *Service) scoreQuality(html string) float64 {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return 0
	}
	text := doc.Text()
	total := 0.0
	total += s.scoreLength(text)
	total += s.scoreFreshness(doc)
	total += s.scoreReadability(text)
	total += s.scoreMedia(doc)
	total += s.scoreStructure(doc)
	total += s.scoreOriginality(text)
	total += s.scoreSpamSignals(text, doc)
	return total
}

func (s *Service) scoreLength(text string) float64 {
	words := len(strings.Fields(text))
	switch {
	case words >= 500:
		return 12.5
	case words < 200:
		return 0
	default:
		return float64(words-200) / 300 * 12.5
	}
}

func (s *Service) scoreFreshness(doc *goquery.Document) float64 {
	dateStr := doc.Find(`meta[property="article:published_time"]`).AttrOr("content", "")
	if dateStr == "" {
		dateStr = doc.Find(`meta[name="pubdate"]`).AttrOr("content", "")
	}
	if dateStr == "" {
		return 6.25
	}
	pubDate, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return 6.25
	}
	daysOld := time.Since(pubDate).Hours() / 24
	switch {
	case daysOld <= 30:
		return 12.5
	case daysOld <= 90:
		return 6.25
	default:
		return 0
	}
}

func (s *Service) scoreReadability(text string) float64 {
	words := strings.Fields(text)
	sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
	if sentences == 0 {
		return 5
	}
	avgWordsPerSentence := float64(len(words)) / float64(sentences)
	if avgWordsPerSentence >= 15 && avgWordsPerSentence <= 25 {
		return 12.5
	}
	if avgWordsPerSentence >= 10 || avgWordsPerSentence <= 30 {
		return 6.25
	}
	return 0
}

func (s *Service) scoreMedia(doc *goquery.Document) float64 {
	score := 0.0
	if doc.Find("img").Length() > 0 {
		score += 6
	}
	if doc.Find("img[alt]").Length() > 0 {
		score += 6.5
	}
	if score > 12.5 {
		score = 12.5
	}
	return score
}

func (s *Service) scoreStructure(doc *goquery.Document) float64 {
	score := 0.0
	if doc.Find("h1,h2,h3").Length() > 0 {
		score += 7.5
	}
	if doc.Find("ul,ol").Length() > 0 {
		score += 5
	}
	return score
}

func (s *Service) scoreOriginality(text string) float64 {
	lower := strings.ToLower(text)
	stuffing := []string{"click here", "buy now", "subscribe", "limited time", "act now", "free money"}
	for _, kw := range stuffing {
		if strings.Contains(lower, kw) {
			return -5
		}
	}
	return 12.5
}

func (s *Service) scoreSpamSignals(text string, doc *goquery.Document) float64 {
	score := 12.5
	lower := strings.ToLower(text)
	spam := []string{"viagra", "casino", "lottery", "adult content"}
	for _, kw := range spam {
		if strings.Contains(lower, kw) {
			score -= 12.5
		}
	}
	adCount := doc.Find("iframe, [class*='ad-']").Length()
	if adCount > 3 {
		score -= 6.25
	}
	if score < 0 {
		score = 0
	}
	return score
}

// extractMetadata extracts title and description from HTML.
func (s *Service) extractMetadata(html string) (title, description string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", ""
	}
	title = strings.TrimSpace(doc.Find("title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	description = strings.TrimSpace(doc.Find("meta[name='description']").AttrOr("content", ""))
	if description == "" {
		description = strings.TrimSpace(doc.Find("meta[property='og:description']").AttrOr("content", ""))
	}
	return title, description
}

// extractTagsAndLanguage extracts language and keyword tags from HTML text.
func (s *Service) extractTagsAndLanguage(html string) (tags []string, language string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, "en"
	}
	text := doc.Text()
	language = detectLanguage(text)
	doc.Find("h1,h2,h3").Each(func(_ int, sel *goquery.Selection) {
		if len(tags) >= 8 {
			return
		}
		heading := strings.TrimSpace(sel.Text())
		if len(heading) > 3 && len(heading) < 50 {
			tags = append(tags, heading)
		}
	})
	return tags, language
}

// detectLanguage uses character-based heuristics for Vietnamese detection.
func detectLanguage(text string) string {
	vietChars := 0
	total := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			total++
			if (r >= 'à' && r <= 'ỹ') || r == 'đ' || r == 'Đ' {
				vietChars++
			}
		}
	}
	if total > 20 && float64(vietChars)/float64(total) > 0.15 {
		return "vi"
	}
	return "en"
}

// computeSimHash computes a 64-bit SimHash fingerprint using FNV-1a.
func (s *Service) computeSimHash(content string) uint64 {
	tokens := tokenize(content)
	trigrams := makeTrigrams(tokens)
	var v0, v1 uint64
	for i, tg := range trigrams {
		h := fnv1aHash(tg)
		if i%2 == 0 {
			v0 += h
		} else {
			v1 += h
		}
	}
	return (v0 & 0xFFFFFFFF) ^ (v1 & 0xFFFFFFFF00000000)
}

// isDuplicate checks SHA-256 exact match and SimHash Hamming distance.
func (s *Service) isDuplicate(ctx context.Context, html string) (bool, error) {
	hash := sha256.Sum256([]byte(html))
	hashHex := hex.EncodeToString(hash[:])

	// Exact match.
	exists, err := s.repo.FindFingerprintBySHA256(ctx, hashHex)
	if err != nil {
		return false, err
	}
	if exists != nil {
		return true, nil
	}

	// SimHash distance.
	fingerprint := s.computeSimHash(html)
	bucket := uint16(fingerprint >> 48)
	candidates, err := s.repo.FetchFingerprintsByBucket(ctx, bucket, 100)
	if err != nil {
		return false, err
	}

	const threshold = 6
	for _, c := range candidates {
		dist := bits.OnesCount64(fingerprint ^ c.SimHash)
		if dist <= threshold {
			return true, nil
		}
	}
	return false, nil
}

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func makeTrigrams(tokens []string) []string {
	var result []string
	for i := 0; i+2 < len(tokens); i++ {
		result = append(result, strings.Join(tokens[i:i+3], " "))
	}
	return result
}

func fnv1aHash(s string) uint64 {
	h := fnv.New64a()
	io.WriteString(h, s)
	return h.Sum64()
}

// ─── SSE Hub ────────────────────────────────────────────────────────────────

// SSEHub manages Server-Sent Events connections for crawl progress.
type SSEHub struct {
	clients map[string]chan dto.SSEProgressEvent
	mu      sync.RWMutex
	drained bool
}

// NewSSEHub creates a new SSE hub.
func NewSSEHub() *SSEHub {
	return &SSEHub{clients: make(map[string]chan dto.SSEProgressEvent)}
}

// Register adds a new SSE client for a job.
func (h *SSEHub) Register(jobID string) chan dto.SSEProgressEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan dto.SSEProgressEvent, 100)
	h.clients[jobID] = ch
	return ch
}

// Unregister removes an SSE client.
func (h *SSEHub) Unregister(jobID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.clients[jobID]; ok {
		close(ch)
		delete(h.clients, jobID)
	}
}

// Drain signals all connected SSE clients to stop and prevents new registrations.
// Called during graceful shutdown to allow in-flight jobs to observe the signal.
func (h *SSEHub) Drain() {
	h.mu.Lock()
	h.drained = true
	// Close all channels so goroutines waiting on them can observe Done.
	for _, ch := range h.clients {
		close(ch)
	}
	h.clients = make(map[string]chan dto.SSEProgressEvent)
	h.mu.Unlock()
}

// IsDrained reports whether the hub has been drained (during shutdown).
func (h *SSEHub) IsDrained() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.drained
}

// broadcast sends a progress event to the SSE client for a job.
func (s *Service) broadcast(jobID string, step int, message string, done ...bool) {
	if s.sseHub == nil {
		return
	}
	isDone := len(done) > 0 && done[0]
	evt := dto.SSEProgressEvent{
		Type:    "progress",
		JobID:   jobID,
		Step:    step,
		Message: message,
	}
	if isDone {
		evt.Type = "done"
	}

	s.updatePipeline(jobID, step, message)

	s.sseHub.mu.RLock()
	ch, ok := s.sseHub.clients[jobID]
	s.sseHub.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- evt:
	default:
	}
}

func (s *Service) emitTerminalEvent(jobID, eventType, message string) {
	if s.sseHub == nil {
		return
	}

	s.sseHub.mu.RLock()
	ch, ok := s.sseHub.clients[jobID]
	s.sseHub.mu.RUnlock()
	if !ok {
		return
	}

	select {
	case ch <- dto.SSEProgressEvent{
		Type:    eventType,
		JobID:   jobID,
		Message: message,
	}:
	default:
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func pipelineSource(feedID string) string {
	if feedID != "" {
		return "RSS"
	}
	return "MANUAL"
}

func pipelineStepLabel(step int) string {
	switch {
	case step <= 2:
		return "DISCOVER"
	case step <= 4:
		return "SCRAPE"
	case step <= 8:
		return "PROCESS"
	case step <= 11:
		return "SEO"
	default:
		return "PUBLISH"
	}
}

func pipelineProgress(step int) int {
	switch {
	case step <= 1:
		return 10
	case step == 2:
		return 20
	case step <= 4:
		return 40
	case step <= 8:
		return 65
	case step <= 11:
		return 85
	default:
		return 100
	}
}

func (s *Service) updateHistory(ctx context.Context, historyID string, result PipelineResult) {
	status := result.Status
	if status == "" {
		status = entities.CrawlStatusFailed
	}
	errMsg := result.ErrorMsg
	if errMsg == "" && !result.Success {
		errMsg = "unknown error"
	}
	if err := s.repo.UpdateCrawlStatus(ctx, historyID, status, result.Step, errMsg); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("history_id", historyID).Msg("crawler: update history failed")
	}
}

func (s *Service) publishResult(ctx context.Context, urlStr, feedID, jobID string, result PipelineResult) {
	if s.eventBus == nil {
		return
	}
	eventType := "crawl.success"
	if !result.Success {
		eventType = "crawl.failed"
	}
	payload := map[string]string{
		"url":           urlStr,
		"feed_id":       feedID,
		"job_id":        jobID,
		"status":        string(result.Status),
		"quality_score": fmt.Sprintf("%.1f", result.QualityScore),
		"title":         result.Title,
	}
	if result.ErrorMsg != "" {
		payload["error"] = result.ErrorMsg
	}
	if err := s.eventBus.Publish(ctx, eventType, payload); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("event_type", eventType).Msg("crawler: publish result failed")
	}
}
