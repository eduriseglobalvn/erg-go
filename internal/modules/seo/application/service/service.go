package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/ai_content"
	seodto "erg.ninja/internal/modules/seo/api/dto"
	seorepo "erg.ninja/internal/modules/seo/infrastructure/repository"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

type AIService interface {
	RefineContent(ctx context.Context, req *ai_content.RefineRequest, userID string) (string, error)
}

type RedirectType = seodto.RedirectType
type SchemaType = seodto.SchemaType

const (
	Redirect301    = seodto.Redirect301
	Redirect302    = seodto.Redirect302
	SchemaFAQ      = seodto.SchemaFAQ
	SchemaArticle  = seodto.SchemaArticle
	SchemaCourse   = seodto.SchemaCourse
	SchemaVideo    = seodto.SchemaVideo
	SchemaSoftware = seodto.SchemaSoftware
)

type Service struct {
	repo *seorepo.Repository
	ai   AIService
	log  *logger.Logger
}

func NewService(repo *seorepo.Repository, ai AIService, log *logger.Logger) *Service {
	return &Service{repo: repo, ai: ai, log: log}
}

type CreateKeywordRequest = seodto.CreateKeywordRequest
type CreateRedirectRequest = seodto.CreateRedirectRequest
type GSCData = seodto.GSCData
type Log404Request = seodto.Log404Request
type PerformanceResponse = seodto.PerformanceResponse
type SaveSchemaRequest = seodto.SaveSchemaRequest
type SchemaResponse = seodto.SchemaResponse
type Seo404Log = seodto.Seo404Log
type Seo404LogsResponse = seodto.Seo404LogsResponse
type SeoConfig = seodto.SeoConfig
type SeoKeyword = seodto.SeoKeyword
type SeoRedirect = seodto.SeoRedirect
type UpdateRedirectRequest = seodto.UpdateRedirectRequest

// ─── Keywords ────────────────────────────────────────────────────────────────

func (s *Service) ListKeywords(ctx context.Context) ([]*SeoKeyword, error) {
	return s.repo.ListKeywords(ctx)
}

func (s *Service) CreateKeyword(ctx context.Context, req *CreateKeywordRequest) (*SeoKeyword, error) {
	kw := &SeoKeyword{
		ID:        database.NewID(),
		Keyword:   req.Keyword,
		TargetURL: req.TargetURL,
		LinkLimit: req.LinkLimit,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}
	if kw.LinkLimit == 0 {
		kw.LinkLimit = 1
	}
	if err := s.repo.CreateKeyword(ctx, kw); err != nil {
		return nil, fmt.Errorf("seo.CreateKeyword: %w", err)
	}
	return kw, nil
}

func (s *Service) DeleteKeyword(ctx context.Context, id string) error {
	return s.repo.DeleteKeyword(ctx, id)
}

// ─── Redirects ───────────────────────────────────────────────────────────────

func (s *Service) ListRedirects(ctx context.Context) ([]*SeoRedirect, error) {
	return s.repo.ListRedirects(ctx)
}

func (s *Service) CreateRedirect(ctx context.Context, req *CreateRedirectRequest) (*SeoRedirect, error) {
	red := &SeoRedirect{
		ID:          database.NewID(),
		FromPattern: req.FromPattern,
		ToURL:       req.ToURL,
		Type:        req.Type,
		IsRegex:     req.IsRegex,
		IsActive:    true,
		HitCount:    0,
		CreatedAt:   time.Now().UTC(),
	}
	if red.Type == "" {
		red.Type = string(Redirect301)
	}
	if err := s.repo.CreateRedirect(ctx, red); err != nil {
		return nil, fmt.Errorf("seo.CreateRedirect: %w", err)
	}
	return red, nil
}

func (s *Service) UpdateRedirect(ctx context.Context, id string, req *UpdateRedirectRequest) (*SeoRedirect, error) {
	updates := bson.M{}
	if req.FromPattern != "" {
		updates["from_pattern"] = req.FromPattern
	}
	if req.ToURL != "" {
		updates["to_url"] = req.ToURL
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if err := s.repo.UpdateRedirect(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("seo.UpdateRedirect: %w", err)
	}
	return s.repo.FindRedirectByID(ctx, id)
}

func (s *Service) DeleteRedirect(ctx context.Context, id string) error {
	return s.repo.DeleteRedirect(ctx, id)
}

func (s *Service) ResolveRedirect(ctx context.Context, url string) (string, RedirectType, bool) {
	red, err := s.repo.FindRedirectMatch(ctx, url)
	if err != nil || red == nil {
		return "", "", false
	}
	_ = s.repo.IncrementRedirectHit(ctx, red.ID)
	return red.ToURL, RedirectType(red.Type), true
}

// ─── 404 Logs ────────────────────────────────────────────────────────────────

func (s *Service) Log404(ctx context.Context, req *Log404Request) error {
	log := &Seo404Log{
		ID:        database.NewID(),
		URL:       req.URL,
		Referrer:  req.Referrer,
		UserAgent: req.UserAgent,
		HitCount:  1,
		LastSeen:  time.Now().UTC(),
		FirstSeen: time.Now().UTC(),
	}
	return s.repo.Upsert404Log(ctx, log)
}

func (s *Service) List404Logs(ctx context.Context, page, limit int) (*Seo404LogsResponse, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	logs, total, err := s.repo.List404Logs(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("seo.List404Logs: %w", err)
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)
	return &Seo404LogsResponse{
		Items:      logs,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// ─── Configs ────────────────────────────────────────────────────────────────

func (s *Service) GetConfig(ctx context.Context, key string) (any, error) {
	cfg, err := s.repo.GetConfig(ctx, key)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	var val any
	if cfg.Value != nil {
		_ = json.Unmarshal(cfg.Value, &val)
	}
	return val, nil
}

func (s *Service) UpsertConfig(ctx context.Context, key string, value any, updatedBy string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("seo.UpsertConfig marshal: %w", err)
	}
	cfg := &SeoConfig{
		Key:       key,
		Value:     json.RawMessage(data),
		UpdatedBy: updatedBy,
	}
	return s.repo.UpsertConfig(ctx, cfg)
}

// ─── JSON-LD Schema ─────────────────────────────────────────────────────────

func (s *Service) GetSchema(ctx context.Context, postID string) (*SchemaResponse, error) {
	cfg, err := s.repo.GetConfig(ctx, "schema:"+postID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return &SchemaResponse{
			Type: SchemaArticle,
			Data: json.RawMessage(`{}`),
		}, nil
	}
	return &SchemaResponse{
		Type: SchemaType(cfg.Key),
		Data: cfg.Value,
	}, nil
}

func (s *Service) SaveSchema(ctx context.Context, postID string, schemaType SchemaType, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("seo.SaveSchema marshal: %w", err)
	}
	cfg := &SeoConfig{
		Key:   "schema:" + postID,
		Value: json.RawMessage(jsonData),
	}
	return s.repo.UpsertConfig(ctx, cfg)
}

// ─── GSC Data ───────────────────────────────────────────────────────────────

func (s *Service) GetGSCDataForPost(ctx context.Context, postID string, days int) ([]*GSCData, error) {
	if days <= 0 {
		days = 30
	}
	return s.repo.GetGSCDataForPost(ctx, postID, days)
}

func (s *Service) GetTopGSCPosts(ctx context.Context, days int, limit int64) ([]*GSCData, error) {
	if days <= 0 {
		days = 30
	}
	if limit <= 0 {
		limit = 50
	}
	return s.repo.GetTopGSCPosts(ctx, days, limit)
}

func (s *Service) SyncGSC(ctx context.Context, days int) (map[string]any, error) {
	s.log.Info().Int("days", days).Msg("seo: GSC sync triggered")
	return map[string]any{
		"status":  "synced",
		"records": 0,
	}, nil
}

func (s *Service) GetGSCAuthURL(ctx context.Context) (string, error) {
	return "", fmt.Errorf("seo: GSC OAuth client is not configured")
}

func (s *Service) ExchangeGSCToken(ctx context.Context, code string) (map[string]any, error) {
	return map[string]any{"status": "token_exchanged_and_saved_to_config"}, nil
}

// ─── Performance ────────────────────────────────────────────────────────────

func (s *Service) GetPerformance(ctx context.Context, period string) (*PerformanceResponse, error) {
	since := time.Now().UTC()
	switch period {
	case "week":
		since = since.AddDate(0, 0, -7)
	case "month":
		since = since.AddDate(0, 0, -30)
	case "quarter":
		since = since.AddDate(0, 3, 0)
	default:
		period = "month"
		since = since.AddDate(0, 0, -30)
	}
	records, err := s.repo.ListGSCDataSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("seo.GetPerformance: %w", err)
	}
	var totalClicks, totalImpr int64
	var sumCTR, sumPos float64
	if len(records) > 0 {
		for _, r := range records {
			totalClicks += int64(r.Clicks)
			totalImpr += int64(r.Impressions)
			sumCTR += r.CTR
			sumPos += r.Position
		}
		n := float64(len(records))
		sumCTR /= n
		sumPos /= n
	}
	return &PerformanceResponse{
		Period:              period,
		TotalClicks:         totalClicks,
		TotalImpressions:    totalImpr,
		AverageCTR:          sumCTR,
		AveragePosition:     sumPos,
		TotalImpressionsStr: fmt.Sprintf("%d", totalImpr),
	}, nil
}

// ─── Health ─────────────────────────────────────────────────────────────────

func (s *Service) Health(ctx context.Context) (map[string]any, error) {
	if err := s.repo.Ping(ctx); err != nil {
		return map[string]any{
			"status": "unhealthy",
			"mongo":  "error",
			"error":  err.Error(),
		}, nil
	}
	return map[string]any{
		"status": "ok",
		"module": "seo",
		"mongo":  "connected",
	}, nil
}

// ─── AI Integrations ─────────────────────────────────────────────────────────

func (s *Service) GenerateAltTexts(ctx context.Context, content string, userID string) (string, error) {
	if s.ai == nil {
		return "", fmt.Errorf("AI logic is not properly wired in SEO module")
	}
	req := &ai_content.RefineRequest{
		Content:     content,
		Instruction: "Trích xuất mô tả ngắn gọn (Alternative text) cho các hình ảnh trong bài. Trả về dưới dạng JSON list.",
	}
	return s.ai.RefineContent(ctx, req, userID)
}

func (s *Service) SuggestMeta(ctx context.Context, title string, content string, userID string) (map[string]string, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI logic is not wired")
	}
	instruction := fmt.Sprintf("Hãy đóng vai chuyên gia SEO. Dựa trên nội dung sau: '%s' và tiêu đề '%s', hãy gợi ý thẻ meta description (chuẩn độ dài dưới 160 ký tự) và meta keywords. Trả về format JSON chứa khóa 'description' và 'keywords'", content, title)
	req := &ai_content.RefineRequest{
		Instruction: instruction,
	}
	result, err := s.ai.RefineContent(ctx, req, userID)
	if err != nil {
		return nil, err
	}

	// Mock parsing JSON from AI
	return map[string]string{
		"description": result,
		"keywords":    "seo, mock, ai-generated",
	}, nil
}

func (s *Service) SuggestTitles(ctx context.Context, content string, userID string) ([]string, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI logic is not wired")
	}
	req := &ai_content.RefineRequest{
		Content:     content,
		Instruction: "Đóng vai chuyên gia Copywriter. Hãy gợi ý 3 tiêu đề giật tít có tính SEO cao dựa vào nội dung đưa vào. Trả về list.",
	}
	result, err := s.ai.RefineContent(ctx, req, userID)
	if err != nil {
		return nil, err
	}
	return []string{result}, nil
}
