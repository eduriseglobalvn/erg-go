package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"

	"erg.ninja/internal/modules/auth/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
)

type LoginAttemptListParams struct {
	TenantID string
	UserID   string
	Email    string
	IP       string
	Result   string
	Reason   string
	From     time.Time
	To       time.Time
	Page     int
	Limit    int
}

type FailedLoginAttemptCountParams struct {
	TenantID       string
	EmailHash      string
	IP             string
	Since          time.Time
	IncludeBlocked bool
	ResetOnSuccess bool
}

type FirewallRuleRecord struct {
	ID        string
	Entry     string
	RuleType  string
	Reason    string
	Source    string
	Active    bool
	ExpiresAt *time.Time
	RevokedAt *time.Time
	RevokedBy string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	FirewallRuleTypeBlock     = "block"
	FirewallRuleTypeAllowlist = "allowlist"
)

func (r *Repo) CreateLoginAttempt(ctx context.Context, attempt *entities.LoginAttempt) error {
	if attempt == nil {
		return nil
	}
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if attempt.ID.IsZero() {
		attempt.ID = bson.NewObjectID()
	}
	if attempt.CreatedAt.IsZero() {
		attempt.CreatedAt = now
	}
	record := &postgrescore.AuthLoginAttempt{
		ID:                 attempt.ID.Hex(),
		TenantID:           attempt.TenantID,
		UserID:             attempt.UserID,
		AttemptedEmail:     normalizeEmail(attempt.AttemptedEmail),
		AttemptedEmailHash: attempt.AttemptedEmailHash,
		IPAddress:          strings.TrimSpace(attempt.IPAddress),
		CountryCode:        strings.ToUpper(strings.TrimSpace(attempt.CountryCode)),
		CountryName:        strings.TrimSpace(attempt.CountryName),
		ContinentCode:      strings.ToUpper(strings.TrimSpace(attempt.ContinentCode)),
		UserAgent:          strings.TrimSpace(attempt.UserAgent),
		DeviceID:           strings.TrimSpace(attempt.DeviceID),
		DeviceName:         strings.TrimSpace(attempt.DeviceName),
		Result:             string(attempt.Result),
		Reason:             string(attempt.Reason),
		CreatedAt:          attempt.CreatedAt.UTC(),
	}

	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("auth.repository.createLoginAttempt: %w", err)
	}
	return nil
}

func (r *Repo) CountFailedLoginAttempts(ctx context.Context, params FailedLoginAttemptCountParams) (int64, error) {
	if err := r.ensureDB(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := r.db.WithContext(ctx).Model(&postgrescore.AuthLoginAttempt{})
	if params.TenantID != "" {
		query = query.Where("tenant_id = ?", params.TenantID)
	}
	if params.EmailHash != "" {
		query = query.Where("attempted_email_hash = ?", strings.TrimSpace(params.EmailHash))
	}
	if params.IP != "" {
		query = query.Where("ip_address = ?", strings.TrimSpace(params.IP))
	}
	if !params.Since.IsZero() {
		query = query.Where("created_at >= ?", params.Since.UTC())
	}
	if params.ResetOnSuccess {
		if lastSuccess, ok, err := r.latestSuccessfulLoginAt(ctx, params); err != nil {
			return 0, err
		} else if ok && lastSuccess.After(params.Since) {
			query = query.Where("created_at > ?", lastSuccess.UTC())
		}
	}
	if params.IncludeBlocked {
		query = query.Where("result IN ?", []string{string(entities.LoginAttemptFailed), string(entities.LoginAttemptBlocked)})
	} else {
		query = query.Where("result = ?", string(entities.LoginAttemptFailed))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("auth.repository.countFailedLoginAttempts: %w", err)
	}
	return total, nil
}

func (r *Repo) latestSuccessfulLoginAt(ctx context.Context, params FailedLoginAttemptCountParams) (time.Time, bool, error) {
	query := r.db.WithContext(ctx).Model(&postgrescore.AuthLoginAttempt{}).
		Select("created_at").
		Where("result = ?", string(entities.LoginAttemptSuccess))
	if params.TenantID != "" {
		query = query.Where("tenant_id = ?", params.TenantID)
	}
	if params.EmailHash != "" {
		query = query.Where("attempted_email_hash = ?", strings.TrimSpace(params.EmailHash))
	}
	if params.IP != "" {
		query = query.Where("ip_address = ?", strings.TrimSpace(params.IP))
	}
	if !params.Since.IsZero() {
		query = query.Where("created_at >= ?", params.Since.UTC())
	}

	var record postgrescore.AuthLoginAttempt
	err := query.Order("created_at DESC").Limit(1).Take(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("auth.repository.latestSuccessfulLoginAt: %w", err)
	}
	return record.CreatedAt, true, nil
}

func (r *Repo) ListLoginAttempts(ctx context.Context, params LoginAttemptListParams) ([]entities.LoginAttempt, int64, error) {
	if err := r.ensureDB(); err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	page := params.Page
	if page <= 0 {
		page = 1
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := r.db.WithContext(ctx).Model(&postgrescore.AuthLoginAttempt{})
	if params.TenantID != "" {
		query = query.Where("tenant_id = ?", params.TenantID)
	}
	if params.UserID != "" {
		query = query.Where("user_id = ?", params.UserID)
	}
	if params.Email != "" {
		query = query.Where("attempted_email = ?", normalizeEmail(params.Email))
	}
	if params.IP != "" {
		query = query.Where("ip_address = ?", strings.TrimSpace(params.IP))
	}
	if params.Result != "" {
		query = query.Where("result = ?", strings.TrimSpace(params.Result))
	}
	if params.Reason != "" {
		query = query.Where("reason = ?", strings.TrimSpace(params.Reason))
	}
	if !params.From.IsZero() {
		query = query.Where("created_at >= ?", params.From.UTC())
	}
	if !params.To.IsZero() {
		query = query.Where("created_at <= ?", params.To.UTC())
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("auth.repository.listLoginAttempts.count: %w", err)
	}

	var records []postgrescore.AuthLoginAttempt
	if err := query.
		Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&records).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, fmt.Errorf("auth.repository.listLoginAttempts: %w", err)
	}

	attempts := make([]entities.LoginAttempt, 0, len(records))
	for i := range records {
		attempts = append(attempts, mapLoginAttemptRecord(&records[i]))
	}
	return attempts, total, nil
}

func (r *Repo) UpsertFirewallRule(ctx context.Context, rule FirewallRuleRecord) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	entry := strings.TrimSpace(rule.Entry)
	ruleType := strings.TrimSpace(rule.RuleType)
	if entry == "" || ruleType == "" {
		return fmt.Errorf("auth.repository.upsertFirewallRule: entry and rule type are required")
	}

	now := time.Now().UTC()
	if rule.ID == "" {
		rule.ID = database.NewID()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = now
	}

	record := postgrescore.FirewallRule{
		ID:        rule.ID,
		Entry:     entry,
		RuleType:  ruleType,
		Reason:    strings.TrimSpace(rule.Reason),
		Source:    strings.TrimSpace(rule.Source),
		Active:    true,
		ExpiresAt: rule.ExpiresAt,
		CreatedBy: strings.TrimSpace(rule.CreatedBy),
		CreatedAt: rule.CreatedAt.UTC(),
		UpdatedAt: rule.UpdatedAt.UTC(),
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&postgrescore.FirewallRule{}).
			Where("rule_type = ? AND entry = ? AND active = ?", ruleType, entry, true).
			Updates(map[string]any{
				"active":     false,
				"revoked_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("auth.repository.upsertFirewallRule.revokeExisting: %w", err)
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("auth.repository.upsertFirewallRule.create: %w", err)
		}
		return nil
	})
}

func (r *Repo) RevokeFirewallRule(ctx context.Context, ruleType, entry, revokedBy string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&postgrescore.FirewallRule{}).
		Where("rule_type = ? AND entry = ? AND active = ?", strings.TrimSpace(ruleType), strings.TrimSpace(entry), true).
		Updates(map[string]any{
			"active":     false,
			"revoked_at": now,
			"revoked_by": strings.TrimSpace(revokedBy),
			"updated_at": now,
		}).Error
}

func (r *Repo) IsFirewallRuleActive(ctx context.Context, ruleType, entry string) (bool, error) {
	if err := r.ensureDB(); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int64
	err := r.activeFirewallRulesQuery(ctx, strings.TrimSpace(ruleType)).
		Where("entry = ?", strings.TrimSpace(entry)).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("auth.repository.isFirewallRuleActive: %w", err)
	}
	return count > 0, nil
}

func (r *Repo) ListActiveFirewallRules(ctx context.Context, ruleType string) ([]FirewallRuleRecord, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var records []postgrescore.FirewallRule
	if err := r.activeFirewallRulesQuery(ctx, strings.TrimSpace(ruleType)).
		Order("created_at DESC").
		Find(&records).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("auth.repository.listActiveFirewallRules: %w", err)
	}

	out := make([]FirewallRuleRecord, 0, len(records))
	for i := range records {
		out = append(out, mapFirewallRuleRecord(&records[i]))
	}
	return out, nil
}

func (r *Repo) activeFirewallRulesQuery(ctx context.Context, ruleType string) *gorm.DB {
	now := time.Now().UTC()
	query := r.db.WithContext(ctx).
		Model(&postgrescore.FirewallRule{}).
		Where("active = ? AND (expires_at IS NULL OR expires_at > ?)", true, now)
	if ruleType != "" {
		query = query.Where("rule_type = ?", ruleType)
	}
	return query
}

func mapLoginAttemptRecord(record *postgrescore.AuthLoginAttempt) entities.LoginAttempt {
	id, _ := database.ParseObjectID(record.ID)
	return entities.LoginAttempt{
		ID:                 id,
		TenantID:           record.TenantID,
		UserID:             record.UserID,
		AttemptedEmail:     record.AttemptedEmail,
		AttemptedEmailHash: record.AttemptedEmailHash,
		IPAddress:          record.IPAddress,
		CountryCode:        record.CountryCode,
		CountryName:        record.CountryName,
		ContinentCode:      record.ContinentCode,
		UserAgent:          record.UserAgent,
		DeviceID:           record.DeviceID,
		DeviceName:         record.DeviceName,
		Result:             entities.LoginAttemptResult(record.Result),
		Reason:             entities.LoginAttemptReason(record.Reason),
		CreatedAt:          record.CreatedAt,
	}
}

func mapFirewallRuleRecord(record *postgrescore.FirewallRule) FirewallRuleRecord {
	return FirewallRuleRecord{
		ID:        record.ID,
		Entry:     record.Entry,
		RuleType:  record.RuleType,
		Reason:    record.Reason,
		Source:    record.Source,
		Active:    record.Active,
		ExpiresAt: record.ExpiresAt,
		RevokedAt: record.RevokedAt,
		RevokedBy: record.RevokedBy,
		CreatedBy: record.CreatedBy,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}
