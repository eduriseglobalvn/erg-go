// Package service provides business logic for the audit module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/audit/api/dto"
	"erg.ninja/internal/modules/audit/domain/entity"
	"erg.ninja/internal/modules/audit/infrastructure/repository"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Service provides audit business logic.
type Service struct {
	repo *repository.Repository
	log  *logger.Logger
}

// NewService creates a new audit service.
func NewService(mongo *database.MongoClient, log *logger.Logger) *Service {
	return &Service{
		repo: repository.NewRepository(mongo, log),
		log:  log,
	}
}

// Repository returns the underlying repository.
func (s *Service) Repository() *repository.Repository {
	return s.repo
}

// ─── Log Operations ──────────────────────────────────────────────────────────

// LogAction records an audit event. Use this from other modules to log actions.
func (s *Service) LogAction(ctx context.Context, params LogParams) error {
	entry := &entities.AuditLog{
		ID:           bson.NewObjectID(),
		Action:       params.Action,
		ResourceType: params.ResourceType,
		ResourceID:   params.ResourceID,
		UserID:       params.UserID,
		UserEmail:    params.UserEmail,
		IPAddress:    params.IPAddress,
		UserAgent:    params.UserAgent,
		TenantID:     params.TenantID,
		Timestamp:    time.Now().UTC(),
	}

	if params.Changes != nil {
		if bytes, err := json.Marshal(params.Changes); err == nil {
			entry.Changes = string(bytes)
		}
	}
	if params.Metadata != nil {
		if bytes, err := json.Marshal(params.Metadata); err == nil {
			entry.Metadata = string(bytes)
		}
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		s.log.ErrorContext(ctx).Err(err).Str("action", params.Action).Msg("audit.LogAction failed")
		return fmt.Errorf("audit.LogAction: %w", err)
	}
	return nil
}

// LogParams holds the data for a single audit log entry.
type LogParams struct {
	Action       string
	ResourceType string
	ResourceID   string
	UserID       string
	UserEmail    string
	IPAddress    string
	UserAgent    string
	TenantID     string
	Changes      any
	Metadata     any
}

// GetLog retrieves a single audit log entry by ID.
func (s *Service) GetLog(ctx context.Context, id string) (*entities.AuditLog, error) {
	log, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("audit.GetLog: %w", err)
	}
	return log, nil
}

// ListLogs returns a paginated list of audit logs with optional filters.
func (s *Service) ListLogs(ctx context.Context, q dto.AuditLogQueryParams) ([]*entities.AuditLog, int64, error) {
	// Apply defaults.
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	repoQuery := repository.ListQuery{
		Page:         page,
		Limit:        limit,
		Action:       q.Action,
		UserID:       q.UserID,
		ResourceType: q.ResourceType,
	}

	// Parse date filters.
	if q.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, q.StartDate); err == nil {
			repoQuery.StartDate = &t
		}
	}
	if q.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, q.EndDate); err == nil {
			repoQuery.EndDate = &t
		}
	}

	logs, total, err := s.repo.List(ctx, repoQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.ListLogs: %w", err)
	}
	return logs, total, nil
}
