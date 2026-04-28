package ai_content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
)

const (
	TaskGeneratePost = "ai:generate_post"
)

var (
	ErrNoActiveAPIKey = errors.New("no active AI API key found")
	ErrJobNotFound    = errors.New("ai generation job not found")
)

// Service provides AI generation and refinement features.
type Service struct {
	repo  *Repository
	queue *queue.AsynqClient
	log   *logger.Logger
}

// NewService creates a new Service.
func NewService(repo *Repository, q *queue.AsynqClient, log *logger.Logger) *Service {
	return &Service{
		repo:  repo,
		queue: q,
		log:   log,
	}
}

// GeneratePost enqueues a background job to generate a post.
func (s *Service) GeneratePost(ctx context.Context, req *GenerateRequest, userID string) (string, error) {
	key, err := s.repo.GetActiveKey(ctx, ProviderGemini)
	if err != nil {
		return "", fmt.Errorf("failed to check active AI API key: %w", err)
	}
	if key == nil {
		return "", ErrNoActiveAPIKey
	}

	payload := JobPayload{
		Topic:      req.Topic,
		CategoryID: req.CategoryID,
		UserID:     userID,
	}

	taskID, err := s.queue.Enqueue(ctx, TaskGeneratePost, payload, queue.WithQueue("default"))
	if err != nil {
		return "", fmt.Errorf("failed to enqueue generate post task: %w", err)
	}

	return taskID, nil
}

// RefineContent synchronously refines content utilizing AI.
func (s *Service) RefineContent(ctx context.Context, req *RefineRequest, userID string) (string, error) {
	key, err := s.repo.GetActiveKey(ctx, ProviderGemini)
	if err != nil {
		return "", fmt.Errorf("failed to check active AI API key: %w", err)
	}
	if key == nil {
		return "", ErrNoActiveAPIKey
	}

	s.log.Info().Str("userId", userID).Str("key_id", key.ID).Msg("Refining content with AI")

	// Placeholder logic for calling the actual LLM API.
	refined := fmt.Sprintf("Refined content for instruction '%s'", req.Instruction)

	return refined, nil
}

// GetJobStatus retrieves the status of a scheduled generation job.
func (s *Service) GetJobStatus(ctx context.Context, jobID string) (map[string]any, error) {
	cfg := s.queue.Config()
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
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
	key, err := s.repo.GetActiveKey(ctx, ProviderGemini)
	if err != nil {
		return nil, fmt.Errorf("failed to check provider health: %w", err)
	}

	status := "healthy"
	message := "Gemini provider is ready"
	if key == nil {
		status = "unconfigured"
		message = "No active Gemini API key configured"
	}

	return map[string]any{
		"google_gemini": map[string]any{
			"status":  status,
			"message": message,
		},
	}, nil
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
