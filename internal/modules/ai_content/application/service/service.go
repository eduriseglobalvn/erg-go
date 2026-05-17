package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	aicontentdto "erg.ninja/internal/modules/ai_content/api/dto"
	aicontentrepo "erg.ninja/internal/modules/ai_content/infrastructure/repository"
	"erg.ninja/pkg/ai"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/security/secretbox"
	"erg.ninja/pkg/storage"
)

const (
	TaskGeneratePost = "ai:generate_post"
)

var (
	ErrNoActiveAPIKey               = errors.New("no active AI API key found")
	ErrJobNotFound                  = errors.New("ai generation job not found")
	ErrAPIKeyNotFound               = errors.New("ai api key not found")
	ErrAIKeyEncryptionNotConfigured = errors.New("ai api key encryption is not configured")
)

// Service provides AI generation and refinement features.
type Service struct {
	repo  *aicontentrepo.Repository
	queue *queue.AsynqClient
	log   *logger.Logger
	ai    *ai.Client
	r2    *storage.R2Client
	image *HuggingFaceImageClient
	keys  *secretbox.Box
}

// NewService creates a new Service.
func NewService(repo *aicontentrepo.Repository, q *queue.AsynqClient, log *logger.Logger, aiClient *ai.Client, r2 *storage.R2Client, imageClient *HuggingFaceImageClient, keyBox *secretbox.Box) *Service {
	return &Service{
		repo:  repo,
		queue: q,
		log:   log,
		ai:    aiClient,
		r2:    r2,
		image: imageClient,
		keys:  keyBox,
	}
}

type APIKeyResponse = aicontentdto.APIKeyResponse
type APIKeysDashboard = aicontentdto.APIKeysDashboard
type ApiKey = aicontentdto.ApiKey
type ApiKeyStatus = aicontentdto.ApiKeyStatus
type ApiKeyType = aicontentdto.ApiKeyType
type CreateAPIKeyRequest = aicontentdto.CreateAPIKeyRequest
type GenerateRequest = aicontentdto.GenerateRequest
type JobPayload = aicontentdto.JobPayload
type ProviderType = aicontentdto.ProviderType
type RefineRequest = aicontentdto.RefineRequest

const (
	ApiKeyPrivate          = aicontentdto.ApiKeyPrivate
	ApiKeyShared           = aicontentdto.ApiKeyShared
	ApiStatusActive        = aicontentdto.ApiStatusActive
	ApiStatusError         = aicontentdto.ApiStatusError
	ApiStatusInactive      = aicontentdto.ApiStatusInactive
	ApiStatusQuotaExceeded = aicontentdto.ApiStatusQuotaExceeded
	ApiStatusRateLimited   = aicontentdto.ApiStatusRateLimited
	ProviderClaude         = aicontentdto.ProviderClaude
	ProviderGemini         = aicontentdto.ProviderGemini
	ProviderGroq           = aicontentdto.ProviderGroq
	ProviderOpenAI         = aicontentdto.ProviderOpenAI
)

// GeneratePost enqueues a background job to generate a post.
func (s *Service) GeneratePost(ctx context.Context, req *GenerateRequest, userID string) (string, error) {
	if _, _, _, err := s.activeAIClient(ctx); err != nil {
		if errors.Is(err, ErrNoActiveAPIKey) {
			return "", err
		}
		return "", fmt.Errorf("failed to check active AI provider: %w", err)
	}
	if s.queue == nil {
		return "", fmt.Errorf("ai generation queue is not configured")
	}
	if req == nil {
		return "", fmt.Errorf("generate request is required")
	}
	if strings.TrimSpace(req.Topic) == "" {
		return "", fmt.Errorf("topic is required")
	}
	if strings.TrimSpace(req.CategoryID) == "" {
		return "", fmt.Errorf("categoryId is required")
	}

	payload := JobPayload{
		Topic:      req.Topic,
		CategoryID: req.CategoryID,
		UserID:     userID,
	}

	taskID, err := s.queue.Enqueue(ctx, TaskGeneratePost, payload, queue.WithQueue("default"), queue.WithRetention(30*time.Minute))
	if err != nil {
		return "", fmt.Errorf("failed to enqueue generate post task: %w", err)
	}

	return taskID, nil
}

// RefineContent synchronously refines content utilizing AI.
func (s *Service) RefineContent(ctx context.Context, req *RefineRequest, userID string) (string, error) {
	client, provider, keyID, err := s.activeAIClient(ctx)
	if err != nil {
		return "", err
	}
	if req == nil {
		return "", fmt.Errorf("refine request is required")
	}
	source := strings.TrimSpace(req.Content)
	if source == "" {
		source = strings.TrimSpace(req.Text)
	}
	if source == "" {
		return "", fmt.Errorf("content or text is required")
	}

	s.log.Info().Str("userId", userID).Str("provider", provider).Str("key_id", keyID).Msg("Refining content with AI")

	refined, err := client.GenerateText(ctx, buildRefinePrompt(source, req.Instruction))
	if err != nil {
		return "", fmt.Errorf("ai refine content: %w", err)
	}
	s.markKeyUsed(ctx, keyID)
	return refined, nil
}

// GetJobStatus retrieves the status of a scheduled generation job.
func (s *Service) GetJobStatus(ctx context.Context, jobID string) (map[string]any, error) {
	if s.queue == nil {
		return nil, fmt.Errorf("ai generation queue is not configured")
	}
	cfg := s.queue.Config()
	redisOpt := asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Username: cfg.RedisUsername,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	if cfg.RedisTLS {
		redisOpt.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: cfg.RedisHost,
		}
	}
	inspector := asynq.NewInspector(redisOpt)
	defer inspector.Close()

	info, err := inspector.GetTaskInfo("default", jobID)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get task info: %w", err)
	}

	state := mapAsynqState(info.State)
	progress := progressForState(info.State)

	var resultData map[string]any
	if len(info.Result) > 0 {
		if err := json.Unmarshal(info.Result, &resultData); err != nil {
			resultData = map[string]any{
				"raw": string(info.Result),
			}
		}
	}

	resp := map[string]any{
		"id":       info.ID,
		"state":    state,
		"progress": progress,
		"error":    info.LastErr,
	}
	if len(resultData) > 0 {
		resp["data"] = resultData
		resp["result"] = resultData
	}

	return resp, nil
}

func (s *Service) GetProviderHealth(ctx context.Context) (map[string]any, error) {
	health := map[string]any{}
	if s.ai != nil {
		status := "unconfigured"
		message := fmt.Sprintf("%s provider is missing API key", s.ai.Provider())
		if s.ai.IsConfigured() {
			status = "healthy"
			message = fmt.Sprintf("%s provider is ready", s.ai.Provider())
		}
		health[s.ai.Provider()] = map[string]any{
			"status":  status,
			"message": message,
			"model":   s.ai.Model(),
			"source":  "config",
		}
	}

	if s.repo != nil {
		for _, provider := range []ProviderType{ProviderGroq, ProviderGemini} {
			key, err := s.repo.GetActiveKey(ctx, provider)
			if err != nil {
				return nil, fmt.Errorf("failed to check provider health: %w", err)
			}
			if key == nil {
				if _, exists := health[string(provider)]; !exists {
					health[string(provider)] = map[string]any{
						"status":  "unconfigured",
						"message": fmt.Sprintf("No active %s API key configured", provider),
						"source":  "database",
					}
				}
				continue
			}
			health[string(provider)] = map[string]any{
				"status":  "healthy",
				"message": fmt.Sprintf("%s provider has an active database API key", provider),
				"model":   key.Model,
				"source":  "database",
			}
		}
	}

	return health, nil
}

func (s *Service) ListKeys(ctx context.Context) ([]APIKeyResponse, error) {
	if s.repo == nil {
		return []APIKeyResponse{}, nil
	}
	keys, err := s.repo.ListKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("ai keys list: %w", err)
	}
	out := make([]APIKeyResponse, 0, len(keys))
	for i := range keys {
		out = append(out, apiKeyResponse(&keys[i]))
	}
	return out, nil
}

func (s *Service) KeysDashboard(ctx context.Context) (*APIKeysDashboard, error) {
	keys, err := s.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	dashboard := &APIKeysDashboard{
		TotalKeys:  len(keys),
		ByProvider: map[string]int{},
	}
	for i := range keys {
		key := keys[i]
		dashboard.ByProvider[string(key.Provider)]++
		dashboard.TotalUsage += key.UsageCount
		dashboard.MonthlyUsage += int64(key.TodayUsage)
		if key.Status == ApiStatusActive {
			dashboard.ActiveKeys++
		}
		if key.Selected {
			selected := key
			dashboard.SelectedKey = &selected
		}
	}
	return dashboard, nil
}

func (s *Service) CreateKey(ctx context.Context, req *CreateAPIKeyRequest, ownerID string) (*APIKeyResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("ai key repository is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	keyValue := strings.TrimSpace(req.Key)
	if keyValue == "" {
		return nil, fmt.Errorf("key is required")
	}
	provider := normalizeProvider(req.Provider)
	if err := validateSupportedProvider(provider); err != nil {
		return nil, err
	}
	keyType := req.Type
	if keyType == "" {
		keyType = ApiKeyShared
	}
	if keyType != ApiKeyShared && keyType != ApiKeyPrivate {
		return nil, fmt.Errorf("type must be shared or private")
	}

	now := time.Now().UTC()
	item := &ApiKey{
		Label:               strings.TrimSpace(req.Label),
		ProjectID:           strings.TrimSpace(req.ProjectID),
		Key:                 keyValue,
		Provider:            provider,
		Type:                keyType,
		Status:              ApiStatusActive,
		OwnerID:             ownerID,
		Selected:            req.Selected,
		Model:               defaultModelForProvider(provider, req.Model),
		MaxTokensPerRequest: defaultMaxTokens(req.MaxTokensPerRequest, provider),
		DefaultTemperature:  defaultTemperature(req.DefaultTemperature, provider),
		MaxDailyQuota:       defaultDailyQuota(req.MaxDailyQuota),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if item.Label == "" {
		item.Label = fmt.Sprintf("%s %s", strings.ToUpper(string(provider)), maskKey(keyValue))
	}

	if err := testAPIKey(ctx, item, s.log); err != nil {
		item.Status = ApiStatusError
		item.LastErrorMessage = err.Error()
		return nil, fmt.Errorf("api key test failed: %w", err)
	}
	item.LastTestedAt = &now
	if err := s.encryptKeyForStorage(item); err != nil {
		return nil, err
	}

	if err := s.repo.CreateKey(ctx, item); err != nil {
		return nil, fmt.Errorf("ai keys create: %w", err)
	}
	if item.Selected {
		selected, err := s.repo.SelectKey(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("ai keys select created key: %w", err)
		}
		item = selected
	}
	s.auditAIKeyEvent("created", item, ownerID, "")
	resp := apiKeyResponse(item)
	return &resp, nil
}

func (s *Service) DeleteKey(ctx context.Context, id string) error {
	if s.repo == nil {
		return fmt.Errorf("ai key repository is not configured")
	}
	ok, err := s.repo.DeleteKey(ctx, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("ai keys delete: %w", err)
	}
	if !ok {
		return ErrAPIKeyNotFound
	}
	s.auditAIKeyEvent("deleted", &ApiKey{ID: strings.TrimSpace(id)}, "", "")
	return nil
}

func (s *Service) TestKey(ctx context.Context, id string) (*APIKeyResponse, error) {
	key, err := s.loadKey(ctx, id)
	if err != nil {
		return nil, err
	}
	status := ApiStatusActive
	message := ""
	if err := testAPIKey(ctx, key, s.log); err != nil {
		status = ApiStatusError
		message = err.Error()
	}
	updated, err := s.repo.UpdateKeyStatus(ctx, key.ID, status, message)
	if err != nil {
		return nil, fmt.Errorf("ai keys update status: %w", err)
	}
	s.auditAIKeyEvent("tested", updated, "", message)
	resp := apiKeyResponse(updated)
	return &resp, nil
}

func (s *Service) SelectKey(ctx context.Context, id string) (*APIKeyResponse, error) {
	key, err := s.loadKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if key.Status != ApiStatusActive {
		if err := testAPIKey(ctx, key, s.log); err != nil {
			_, _ = s.repo.UpdateKeyStatus(ctx, key.ID, ApiStatusError, err.Error())
			return nil, fmt.Errorf("api key test failed: %w", err)
		}
	}
	selected, err := s.repo.SelectKey(ctx, key.ID)
	if err != nil {
		return nil, fmt.Errorf("ai keys select: %w", err)
	}
	if selected == nil {
		return nil, ErrJobNotFound
	}
	s.auditAIKeyEvent("selected", selected, "", "")
	resp := apiKeyResponse(selected)
	return &resp, nil
}

func (s *Service) ReactivateKey(ctx context.Context, id string) (*APIKeyResponse, error) {
	key, err := s.loadKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := testAPIKey(ctx, key, s.log); err != nil {
		updated, updateErr := s.repo.UpdateKeyStatus(ctx, key.ID, ApiStatusError, err.Error())
		if updateErr != nil {
			return nil, updateErr
		}
		s.auditAIKeyEvent("reactivate_failed", updated, "", err.Error())
		resp := apiKeyResponse(updated)
		return &resp, fmt.Errorf("api key test failed: %w", err)
	}
	updated, err := s.repo.UpdateKeyStatus(ctx, key.ID, ApiStatusActive, "")
	if err != nil {
		return nil, fmt.Errorf("ai keys reactivate: %w", err)
	}
	s.auditAIKeyEvent("reactivated", updated, "", "")
	resp := apiKeyResponse(updated)
	return &resp, nil
}

func (s *Service) MigratePlaintextKeys(ctx context.Context) (int, error) {
	if s.repo == nil || s.keys == nil {
		return 0, nil
	}
	keys, err := s.repo.ListKeys(ctx)
	if err != nil {
		return 0, fmt.Errorf("ai keys migration list: %w", err)
	}
	migrated := 0
	for i := range keys {
		key := &keys[i]
		if strings.TrimSpace(key.Key) == "" || strings.TrimSpace(key.EncryptedKey) != "" {
			continue
		}
		if err := s.prepareKeyForRuntime(ctx, key); err != nil {
			return migrated, err
		}
		migrated++
	}
	return migrated, nil
}

func (s *Service) activeAIClient(ctx context.Context) (*ai.Client, string, string, error) {
	if s.repo != nil {
		selected, err := s.repo.GetSelectedKey(ctx)
		if err != nil {
			return nil, "", "", err
		}
		if selected != nil {
			if err := s.prepareKeyForRuntime(ctx, selected); err != nil {
				return nil, "", "", err
			}
		}
		if selected != nil && strings.TrimSpace(selected.Key) != "" {
			client, err := ai.NewClient(aiConfigFromKey(selected), ai.WithGeminiLogger(s.log))
			if err != nil {
				return nil, "", "", err
			}
			if client.IsConfigured() {
				return client, string(selected.Provider), selected.ID, nil
			}
		}

		for _, provider := range []ProviderType{ProviderGroq, ProviderGemini} {
			key, err := s.repo.GetActiveKey(ctx, provider)
			if err != nil {
				return nil, "", "", err
			}
			if key == nil {
				continue
			}
			if err := s.prepareKeyForRuntime(ctx, key); err != nil {
				return nil, "", "", err
			}
			if strings.TrimSpace(key.Key) == "" {
				continue
			}
			client, err := ai.NewClient(aiConfigFromKey(key), ai.WithGeminiLogger(s.log))
			if err != nil {
				return nil, "", "", err
			}
			if client.IsConfigured() {
				return client, string(provider), key.ID, nil
			}
		}
	}

	if s.ai != nil && s.ai.IsConfigured() {
		return s.ai, s.ai.Provider(), "config", nil
	}

	return nil, "", "", ErrNoActiveAPIKey
}

func aiConfigFromKey(key *ApiKey) config.AiConfig {
	if key == nil {
		return config.AiConfig{}
	}
	model := strings.TrimSpace(key.Model)
	maxTokens := key.MaxTokensPerRequest
	temperature := key.DefaultTemperature
	switch key.Provider {
	case ProviderGroq:
		return config.AiConfig{
			Provider:                string(ProviderGroq),
			GroqAPIKey:              key.Key,
			GroqModel:               model,
			GroqMaxCompletionTokens: maxTokens,
			GroqTemperature:         temperature,
		}
	case ProviderGemini:
		return config.AiConfig{
			Provider:     string(ProviderGemini),
			GeminiAPIKey: key.Key,
			GeminiModel:  model,
		}
	default:
		return config.AiConfig{}
	}
}

func (s *Service) loadKey(ctx context.Context, id string) (*ApiKey, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("ai key repository is not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	key, err := s.repo.GetKeyByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ai keys get: %w", err)
	}
	if key == nil {
		return nil, ErrAPIKeyNotFound
	}
	if err := s.prepareKeyForRuntime(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Service) encryptKeyForStorage(key *ApiKey) error {
	if key == nil {
		return nil
	}
	plain := strings.TrimSpace(key.Key)
	if plain == "" {
		return fmt.Errorf("key is required")
	}
	if s.keys == nil {
		return ErrAIKeyEncryptionNotConfigured
	}
	ciphertext, nonce, err := s.keys.EncryptString(plain)
	if err != nil {
		return fmt.Errorf("ai keys encrypt: %w", err)
	}
	key.EncryptedKey = ciphertext
	key.KeyNonce = nonce
	key.KeyVersion = s.keys.Version()
	key.KeyFingerprint = s.keys.Fingerprint(plain)
	key.MaskedKeyPreview = maskKey(plain)
	key.Key = ""
	return nil
}

func (s *Service) prepareKeyForRuntime(ctx context.Context, key *ApiKey) error {
	if key == nil {
		return nil
	}
	if strings.TrimSpace(key.Key) != "" {
		plain := key.Key
		if strings.TrimSpace(key.EncryptedKey) == "" {
			if s.keys == nil {
				return ErrAIKeyEncryptionNotConfigured
			}
			if err := s.encryptKeyForStorage(key); err != nil {
				return err
			}
			if s.repo != nil {
				if err := s.repo.UpdateKeySecret(ctx, key); err != nil {
					return fmt.Errorf("ai keys migrate raw key: %w", err)
				}
			}
			key.Key = plain
		}
		return nil
	}
	if strings.TrimSpace(key.EncryptedKey) == "" {
		return ErrAIKeyEncryptionNotConfigured
	}
	if s.keys == nil {
		return ErrAIKeyEncryptionNotConfigured
	}
	plain, err := s.keys.DecryptString(key.EncryptedKey, key.KeyNonce, key.KeyVersion)
	if err != nil {
		return fmt.Errorf("ai keys decrypt: %w", err)
	}
	key.Key = plain
	if strings.TrimSpace(key.MaskedKeyPreview) == "" || strings.TrimSpace(key.KeyFingerprint) == "" {
		key.MaskedKeyPreview = maskKey(plain)
		key.KeyFingerprint = s.keys.Fingerprint(plain)
		if s.repo != nil {
			if err := s.repo.UpdateKeySecret(ctx, key); err != nil {
				return fmt.Errorf("ai keys backfill secret metadata: %w", err)
			}
			key.Key = plain
		}
	}
	return nil
}

func (s *Service) markKeyUsed(ctx context.Context, keyID string) {
	if s.repo == nil || keyID == "" || keyID == "config" {
		return
	}
	if err := s.repo.TouchKeyUsed(ctx, keyID); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("key_id", keyID).Msg("ai_content: failed to update API key usage")
	}
}

func (s *Service) auditAIKeyEvent(action string, key *ApiKey, ownerID string, message string) {
	if s == nil || s.log == nil || key == nil {
		return
	}
	s.log.Info().
		Str("action", action).
		Str("key_id", key.ID).
		Str("owner_id", strings.TrimSpace(ownerID)).
		Str("provider", string(key.Provider)).
		Str("fingerprint", key.KeyFingerprint).
		Str("status", string(key.Status)).
		Str("message", strings.TrimSpace(message)).
		Msg("ai_content: api key audit")
}

func testAPIKey(ctx context.Context, key *ApiKey, log *logger.Logger) error {
	if key == nil {
		return fmt.Errorf("key is required")
	}
	provider := normalizeProvider(key.Provider)
	if err := validateSupportedProvider(provider); err != nil {
		return err
	}
	testKey := *key
	testKey.Provider = provider
	testKey.Model = defaultModelForProvider(provider, testKey.Model)
	testKey.MaxTokensPerRequest = 128
	testKey.DefaultTemperature = defaultTemperature(testKey.DefaultTemperature, provider)

	testCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	client, err := ai.NewClient(aiConfigFromKey(&testKey), ai.WithGeminiLogger(log))
	if err != nil {
		return err
	}
	if !client.IsConfigured() {
		return fmt.Errorf("%s API key is empty", provider)
	}
	_, err = client.GenerateText(testCtx, "Write one short plain-text sentence confirming that the AI provider connection works. No markdown.")
	if err == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "empty content") {
		return err
	}

	testKey.MaxTokensPerRequest = 256
	client, retryErr := ai.NewClient(aiConfigFromKey(&testKey), ai.WithGeminiLogger(log))
	if retryErr != nil {
		return retryErr
	}
	_, retryErr = client.GenerateText(testCtx, "Generate a concise Vietnamese sentence about education. Return only the sentence.")
	if retryErr != nil {
		return retryErr
	}
	return nil
}

func validateSupportedProvider(provider ProviderType) error {
	switch provider {
	case ProviderGroq, ProviderGemini:
		return nil
	case ProviderOpenAI, ProviderClaude:
		return fmt.Errorf("%s provider is not wired for AI Content yet", provider)
	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
}

func normalizeProvider(provider ProviderType) ProviderType {
	return ProviderType(strings.ToLower(strings.TrimSpace(string(provider))))
}

func defaultModelForProvider(provider ProviderType, model string) string {
	model = strings.TrimSpace(model)
	if model != "" {
		return model
	}
	switch provider {
	case ProviderGroq:
		return "openai/gpt-oss-120b"
	case ProviderGemini:
		return "gemini-2.0-flash"
	default:
		return model
	}
}

func defaultMaxTokens(value int, provider ProviderType) int {
	if value > 0 {
		return value
	}
	if provider == ProviderGroq {
		return 4096
	}
	return 2048
}

func defaultTemperature(value float64, provider ProviderType) float64 {
	if value > 0 {
		return value
	}
	if provider == ProviderGroq {
		return 1
	}
	return 0.7
}

func defaultDailyQuota(value int) int {
	if value > 0 {
		return value
	}
	return 1000
}

func apiKeyResponse(key *ApiKey) APIKeyResponse {
	if key == nil {
		return APIKeyResponse{}
	}
	masked := strings.TrimSpace(key.MaskedKeyPreview)
	if masked == "" {
		masked = maskKey(key.Key)
	}
	return APIKeyResponse{
		ID:                  key.ID,
		Label:               key.Label,
		ProjectID:           key.ProjectID,
		MaskedKey:           masked,
		Key:                 masked,
		Provider:            key.Provider,
		Type:                key.Type,
		Status:              key.Status,
		Selected:            key.Selected,
		Model:               key.Model,
		MaxTokensPerRequest: key.MaxTokensPerRequest,
		DefaultTemperature:  key.DefaultTemperature,
		TodayUsage:          key.TodayUsage,
		MaxDailyQuota:       key.MaxDailyQuota,
		UsageCount:          key.UsageCount,
		LastUsedAt:          key.LastUsedAt,
		LastTestedAt:        key.LastTestedAt,
		CooldownUntil:       key.CooldownUntil,
		LastErrorMessage:    key.LastErrorMessage,
		CreatedAt:           key.CreatedAt,
		UpdatedAt:           key.UpdatedAt,
	}
}

func maskKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func buildRefinePrompt(content, instruction string) string {
	return fmt.Sprintf(`Refine the following content according to the instruction.

Instruction:
%s

Content:
%s

Return only the refined content. Keep the language Vietnamese unless the instruction asks otherwise.`, strings.TrimSpace(instruction), strings.TrimSpace(content))
}

func mapAsynqState(state asynq.TaskState) string {
	switch state {
	case asynq.TaskStateActive, asynq.TaskStateAggregating:
		return "active"
	case asynq.TaskStateCompleted:
		return "completed"
	case asynq.TaskStateArchived:
		return "failed"
	case asynq.TaskStateRetry:
		return "active"
	case asynq.TaskStatePending, asynq.TaskStateScheduled:
		return "waiting"
	default:
		return "waiting"
	}
}

func progressForState(state asynq.TaskState) int {
	switch state {
	case asynq.TaskStateCompleted:
		return 100
	case asynq.TaskStateActive, asynq.TaskStateAggregating:
		return 75
	case asynq.TaskStateRetry:
		return 55
	case asynq.TaskStateArchived:
		return 100
	case asynq.TaskStatePending, asynq.TaskStateScheduled:
		return 20
	default:
		return 10
	}
}
