package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"

	"erg.ninja/internal/persistence/postgrescore"
)

type accessManagementRepository struct {
	db *gorm.DB
}

type accessUserRecord struct {
	ID                 string
	Email              string
	FullName           string
	AvatarURL          string
	Phone              string
	Status             string
	AccountType        string
	IsProfileCompleted bool
	CreatedAt          time.Time
	Roles              []string
}

type userAccessRecord struct {
	ID       string
	UserID   string
	CenterID string
	Modules  []string
	Role     string
}

func newAccessManagementRepository(db *gorm.DB) *accessManagementRepository {
	if db == nil {
		return nil
	}
	return &accessManagementRepository{db: db}
}

func NewAccessManagementRepository(db *gorm.DB) *accessManagementRepository {
	return newAccessManagementRepository(db)
}

func (r *accessManagementRepository) listUsers(ctx context.Context, tenantID, search, status, role string, page, limit int) ([]accessUserRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("lms.access_management: postgres client unavailable")
	}
	query := r.db.WithContext(ctx).Model(&postgrescore.AuthUser{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		needle := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		query = query.Where("LOWER(email) LIKE ? OR LOWER(full_name) LIKE ?", needle, needle)
	}
	if role != "" {
		query = query.Joins("JOIN user_roles ON user_roles.user_id = users.id").
			Joins("JOIN roles ON roles.id = user_roles.role_id").
			Where("roles.name = ?", role)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("lms.access_management.listUsers.count: %w", err)
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var users []postgrescore.AuthUser
	if err := query.Order("created_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("lms.access_management.listUsers.find: %w", err)
	}
	roleMap, err := r.rolesForUsers(ctx, userIDs(users))
	if err != nil {
		return nil, 0, err
	}
	out := make([]accessUserRecord, 0, len(users))
	for _, user := range users {
		out = append(out, accessUserRecord{
			ID: user.ID, Email: user.Email, FullName: user.FullName, AvatarURL: user.AvatarURL,
			Phone: user.Phone, Status: user.Status, AccountType: user.AccountType, IsProfileCompleted: user.IsProfileCompleted,
			CreatedAt: user.CreatedAt, Roles: roleMap[user.ID],
		})
	}
	return out, total, nil
}

func (r *accessManagementRepository) getUser(ctx context.Context, tenantID, userID string) (*accessUserRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("lms.access_management: postgres client unavailable")
	}
	var user postgrescore.AuthUser
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lms.access_management.getUser: %w", err)
	}
	roles, err := r.rolesForUsers(ctx, []string{user.ID})
	if err != nil {
		return nil, err
	}
	return &accessUserRecord{
		ID: user.ID, Email: user.Email, FullName: user.FullName, AvatarURL: user.AvatarURL,
		Phone: user.Phone, Status: user.Status, AccountType: user.AccountType, IsProfileCompleted: user.IsProfileCompleted,
		CreatedAt: user.CreatedAt, Roles: roles[user.ID],
	}, nil
}

func (r *accessManagementRepository) listAccess(ctx context.Context, userID string) ([]userAccessRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("lms.access_management: postgres client unavailable")
	}
	var rows []postgrescore.UserAccessScope
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("lms.access_management.listAccess: %w", err)
	}
	out := make([]userAccessRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, userAccessRecord{
			ID: row.ID, UserID: row.UserID, CenterID: row.CenterID, Modules: parseJSONStringSlice(row.Modules), Role: row.Role,
		})
	}
	return out, nil
}

func (r *accessManagementRepository) replaceAccess(ctx context.Context, userID string, policies []userAccessRecord) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("lms.access_management: postgres client unavailable")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&postgrescore.UserAccessScope{}).Error; err != nil {
			return err
		}
		for _, policy := range policies {
			if policy.ID == "" {
				policy.ID = bson.NewObjectID().Hex()
			}
			modules, _ := json.Marshal(uniqueStrings(policy.Modules))
			row := postgrescore.UserAccessScope{
				ID: policy.ID, UserID: userID, CenterID: policy.CenterID, Modules: string(modules), Role: policy.Role,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *accessManagementRepository) rolesForUsers(ctx context.Context, ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	type roleRow struct {
		UserID string
		Name   string
	}
	var rows []roleRow
	if err := r.db.WithContext(ctx).Table("user_roles").
		Select("user_roles.user_id, roles.name").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id IN ?", ids).
		Order("roles.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lms.access_management.rolesForUsers: %w", err)
	}
	for _, row := range rows {
		out[row.UserID] = append(out[row.UserID], row.Name)
	}
	return out, nil
}

func userIDs(users []postgrescore.AuthUser) []string {
	out := make([]string, 0, len(users))
	for _, user := range users {
		out = append(out, user.ID)
	}
	return out
}

func parseJSONStringSlice(raw string) []string {
	var values []string
	_ = json.Unmarshal([]byte(raw), &values)
	return uniqueStrings(values)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
