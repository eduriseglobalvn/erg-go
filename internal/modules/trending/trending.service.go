package trending

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	trendingcache "erg.ninja/internal/modules/trending/cache"
	"erg.ninja/internal/modules/trending/dto"
	"erg.ninja/internal/modules/trending/entities"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
)

type Service struct {
	repo      *Repository
	cache     *trendingcache.RedisCache
	log       *logger.Logger
	bus       *event.EventBus
	cfg       config.TrendingConfig
	lastError string
}

type RefreshResult struct {
	Topics      []*entities.TrendingTopic
	News        []*entities.NewsArticle
	Snapshots   []*entities.TrendingSnapshot
	GeneratedAt time.Time
}

func NewService(repo *Repository, cache *trendingcache.RedisCache, log *logger.Logger, bus *event.EventBus, cfg config.TrendingConfig) *Service {
	if log == nil {
		log = logger.NoOp()
	}
	return &Service{repo: repo, cache: cache, log: log, bus: bus, cfg: cfg}
}

func (s *Service) Refresh(ctx context.Context) (*RefreshResult, error) {
	seeds, err := s.repo.SeedCandidates(ctx, int64(maxInt(s.cfg.NewsLimit*5, s.cfg.TopicsLimit*5)))
	if err != nil {
		s.lastError = err.Error()
		return nil, fmt.Errorf("trending.Refresh seeds: %w", err)
	}

	generatedAt := time.Now().UTC()
	topics, articles := s.aggregateSeeds(seeds, generatedAt)
	if len(topics) > s.cfg.TopicsLimit {
		topics = topics[:s.cfg.TopicsLimit]
	}
	if len(articles) > s.cfg.NewsLimit {
		articles = articles[:s.cfg.NewsLimit]
	}

	if err := s.repo.ReplaceTopics(ctx, topics); err != nil {
		s.lastError = err.Error()
		return nil, fmt.Errorf("trending.Refresh topics: %w", err)
	}
	if err := s.repo.ReplaceNews(ctx, articles); err != nil {
		s.lastError = err.Error()
		return nil, fmt.Errorf("trending.Refresh news: %w", err)
	}

	snapshot := &entities.TrendingSnapshot{
		Topics:      topicNames(topics),
		TopicCount:  len(topics),
		GeneratedAt: generatedAt,
	}
	if err := s.repo.CreateSnapshot(ctx, snapshot); err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("trending: create snapshot failed")
	}

	if err := s.cache.SetJSON(ctx, trendingcache.TopicsKey(), topics); err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("trending: cache topics failed")
	}
	if err := s.cache.SetJSON(ctx, trendingcache.NewsKey(), articles); err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("trending: cache news failed")
	}
	if err := s.cache.StoreDiscoveryFeed(ctx, collectURLs(topics), int64(maxInt(s.cfg.FeedLimit, 100))); err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("trending: cache discovery feed failed")
	}

	for _, topic := range topics {
		if topic.Score >= s.cfg.MinHotScore && s.bus != nil {
			if err := s.bus.Publish(ctx, "trending.hot_topic", map[string]any{
				"topic":   topic.Topic,
				"score":   topic.Score,
				"volume":  topic.Volume,
				"source":  topic.Source,
				"refresh": generatedAt,
			}); err != nil {
				s.log.WarnContext(ctx).Err(err).Str("topic", topic.Topic).Msg("trending: publish hot_topic failed")
			}
		}
	}
	if s.bus != nil {
		if err := s.bus.Publish(ctx, "trending.daily", map[string]any{
			"date":        generatedAt.Format("2006-01-02"),
			"topic_count": len(topics),
			"topics":      topicNames(topics),
		}); err != nil {
			s.log.WarnContext(ctx).Err(err).Msg("trending: publish daily failed")
		}
	}

	s.lastError = ""
	return &RefreshResult{
		Topics:      topics,
		News:        articles,
		Snapshots:   []*entities.TrendingSnapshot{snapshot},
		GeneratedAt: generatedAt,
	}, nil
}

func (s *Service) ListTopics(ctx context.Context, limit int64) ([]dto.TopicResponse, error) {
	var topics []*entities.TrendingTopic
	found, err := s.cache.GetJSON(ctx, trendingcache.TopicsKey(), &topics)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("trending: topics cache read failed")
	}
	if !found {
		topics, err = s.repo.ListTopics(ctx, limit)
		if err != nil {
			return nil, err
		}
	}
	return topicsToDTO(topics, limit), nil
}

func (s *Service) GetTopic(ctx context.Context, slug string) (*dto.TopicResponse, error) {
	topic, err := s.repo.GetTopic(ctx, slugify(slug))
	if err != nil || topic == nil {
		return nil, err
	}
	resp := topicToDTO(topic)
	return &resp, nil
}

func (s *Service) ListNews(ctx context.Context, limit int64) ([]dto.NewsArticleResponse, error) {
	var articles []*entities.NewsArticle
	found, err := s.cache.GetJSON(ctx, trendingcache.NewsKey(), &articles)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("trending: news cache read failed")
	}
	if !found {
		articles, err = s.repo.ListNews(ctx, limit)
		if err != nil {
			return nil, err
		}
	}
	return newsToDTO(articles, limit), nil
}

func (s *Service) ListHistory(ctx context.Context, limit int64) ([]dto.SnapshotResponse, error) {
	snapshots, err := s.repo.ListSnapshots(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SnapshotResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, dto.SnapshotResponse{
			Topics:      snapshot.Topics,
			TopicCount:  snapshot.TopicCount,
			GeneratedAt: snapshot.GeneratedAt,
		})
	}
	return out, nil
}

func (s *Service) GetStats(ctx context.Context) (*dto.TrendingStats, error) {
	topics, err := s.repo.ListTopics(ctx, 1000)
	if err != nil {
		return nil, err
	}

	total := int64(len(topics))
	var active int64
	var totalScore float64
	today := time.Now().Truncate(24 * time.Hour)
	var discoveredToday int64
	topKeyword := ""
	var maxScore float64

	for _, t := range topics {
		if t.Score > 0 {
			active++
			totalScore += t.Score
		}
		if t.CreatedAt.After(today) {
			discoveredToday++
		}
		if t.Score > maxScore {
			maxScore = t.Score
			topKeyword = t.Topic
		}
	}

	avg := 0.0
	if active > 0 {
		avg = totalScore / float64(active)
	}

	return &dto.TrendingStats{
		TotalTopics:     total,
		ActiveTopics:    active,
		AvgScore:        avg,
		TopKeyword:      topKeyword,
		DiscoveredToday: discoveredToday,
	}, nil
}

func (s *Service) DiscoveryFeed(ctx context.Context, limit int64) ([]string, error) {
	urls, err := s.cache.GetDiscoveryFeed(ctx, limit)
	if err == nil && len(urls) > 0 {
		return urls, nil
	}
	topics, err := s.repo.ListTopics(ctx, limit)
	if err != nil {
		return nil, err
	}
	return collectURLs(topics), nil
}

func (s *Service) Ready(ctx context.Context) (map[string]any, error) {
	signals, err := s.repo.SourceSignals(ctx)
	if err != nil {
		return nil, err
	}
	lastRefresh, err := s.repo.LatestSnapshotTime(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":         "ready",
		"last_refresh":   lastRefresh,
		"source_signals": signals,
		"cache_ttl":      s.cfg.CacheTTL.String(),
		"last_error":     s.lastError,
	}, nil
}

func (s *Service) Sources(ctx context.Context) (map[string]any, error) {
	signals, err := s.repo.SourceSignals(ctx)
	if err != nil {
		return nil, err
	}
	status := "healthy"
	if signals["crawl_history_24h"] == 0 {
		status = "degraded"
	}
	return map[string]any{
		"google_trends": map[string]any{"status": status, "mode": "mongo-derived"},
		"news_api":      map[string]any{"status": status, "mode": "mongo-derived"},
		"signals":       signals,
	}, nil
}

func (s *Service) aggregateSeeds(seeds []seedDocument, generatedAt time.Time) ([]*entities.TrendingTopic, []*entities.NewsArticle) {
	topicsBySlug := map[string]*entities.TrendingTopic{}
	articles := make([]*entities.NewsArticle, 0, len(seeds))

	for _, seed := range seeds {
		names := pickTopicNames(seed)
		if len(names) == 0 {
			continue
		}

		for i, name := range names {
			slug := slugify(name)
			topic, ok := topicsBySlug[slug]
			if !ok {
				topic = &entities.TrendingTopic{
					Topic:           name,
					Slug:            slug,
					Source:          "mongo-aggregate",
					Keywords:        []string{},
					URLs:            []string{},
					Timeline:        []int{},
					LastRefreshedAt: generatedAt,
				}
				topicsBySlug[slug] = topic
			}
			topic.Score += scoreSeed(seed, i)
			topic.Volume += 100 + int(seed.QualityScore)
			topic.Keywords = appendUnique(topic.Keywords, names...)
			if seed.URL != "" {
				topic.URLs = appendUnique(topic.URLs, seed.URL)
			}
			topic.Timeline = append(topic.Timeline, int(generatedAt.Unix()))
		}

		articles = append(articles, &entities.NewsArticle{
			Topic:          names[0],
			Headline:       firstNonEmpty(seed.Title, seed.Description, names[0]),
			Source:         "crawler-history",
			URL:            seed.URL,
			PublishedAt:    zeroTo(seed.CreatedAt, generatedAt),
			RelevanceScore: scoreSeed(seed, 0),
		})
	}

	topics := make([]*entities.TrendingTopic, 0, len(topicsBySlug))
	for _, topic := range topicsBySlug {
		sort.Slice(topic.Timeline, func(i, j int) bool { return topic.Timeline[i] < topic.Timeline[j] })
		topics = append(topics, topic)
	}
	sort.Slice(topics, func(i, j int) bool {
		if topics[i].Score == topics[j].Score {
			return topics[i].Volume > topics[j].Volume
		}
		return topics[i].Score > topics[j].Score
	})
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].RelevanceScore > articles[j].RelevanceScore
	})
	return topics, articles
}

func topicsToDTO(items []*entities.TrendingTopic, limit int64) []dto.TopicResponse {
	if limit > 0 && int(limit) < len(items) {
		items = items[:limit]
	}
	out := make([]dto.TopicResponse, 0, len(items))
	for _, item := range items {
		out = append(out, topicToDTO(item))
	}
	return out
}

func topicToDTO(item *entities.TrendingTopic) dto.TopicResponse {
	return dto.TopicResponse{
		ID:              item.ID,
		Keyword:         item.Topic,
		TrendScore:      item.Score,
		Source:          item.Source,
		SearchVolume:    item.Volume,
		Keywords:        item.Keywords,
		URLs:            item.URLs,
		Timeline:        item.Timeline,
		DiscoveredAt:    item.CreatedAt,
		LastRefreshedAt: item.LastRefreshedAt,
		Status:          "active",
		Priority:        100,
	}
}

func newsToDTO(items []*entities.NewsArticle, limit int64) []dto.NewsArticleResponse {
	if limit > 0 && int(limit) < len(items) {
		items = items[:limit]
	}
	out := make([]dto.NewsArticleResponse, 0, len(items))
	for _, item := range items {
		out = append(out, dto.NewsArticleResponse{
			Topic:          item.Topic,
			Headline:       item.Headline,
			Source:         item.Source,
			URL:            item.URL,
			PublishedAt:    item.PublishedAt,
			RelevanceScore: item.RelevanceScore,
		})
	}
	return out
}

func pickTopicNames(seed seedDocument) []string {
	candidates := append([]string{}, seed.Tags...)
	candidates = append(candidates, tokenize(seed.Title)...)
	candidates = append(candidates, tokenize(seed.Description)...)
	if len(candidates) == 0 && seed.Title != "" {
		candidates = append(candidates, seed.Title)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		slug := slugify(candidate)
		if slug == "" || isStopWord(slug) {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, candidate)
		if len(out) == 4 {
			break
		}
	}
	return out
}

func tokenize(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) < 3 {
			continue
		}
		out = append(out, field)
	}
	return out
}

func slugify(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func isStopWord(token string) bool {
	switch token {
	case "the", "and", "for", "with", "dang", "nhung", "that", "this", "from", "into":
		return true
	default:
		return false
	}
}

func scoreSeed(seed seedDocument, topicRank int) float64 {
	score := seed.QualityScore
	if topicRank == 0 {
		score += 20
	}
	if !seed.CreatedAt.IsZero() {
		ageHours := time.Since(seed.CreatedAt).Hours()
		if ageHours < 24 {
			score += 15
		} else if ageHours < 72 {
			score += 7
		}
	}
	return score
}

func appendUnique(dst []string, values ...string) []string {
	seen := map[string]struct{}{}
	for _, item := range dst {
		seen[item] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

func collectURLs(topics []*entities.TrendingTopic) []string {
	var urls []string
	for _, topic := range topics {
		urls = appendUnique(urls, topic.URLs...)
	}
	return urls
}

func topicNames(topics []*entities.TrendingTopic) []string {
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		out = append(out, topic.Topic)
	}
	return out
}

func zeroTo(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
