package postgrescore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"

	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// BackfillAuthReport summarizes the Mongo -> PostgreSQL auth backfill.
type BackfillAuthReport struct {
	UsersSeen        int
	UsersCreated     int
	UsersUpdated     int
	UserIDsRekeyed   int
	SessionsSeen     int
	SessionsCreated  int
	SessionsUpdated  int
	SkippedUserRoles int
}

type legacyAuthUser struct {
	ID                  bson.ObjectID     `bson:"_id,omitempty"`
	Email               string            `bson:"email"`
	PasswordHash        string            `bson:"password_hash"`
	FullName            string            `bson:"full_name"`
	AvatarURL           string            `bson:"avatar_url"`
	Status              string            `bson:"status"`
	Provider            string            `bson:"provider"`
	ProviderID          string            `bson:"provider_id"`
	AccountType         string            `bson:"account_type,omitempty"`
	GoogleSub           string            `bson:"google_sub,omitempty"`
	GoogleEmail         string            `bson:"google_email,omitempty"`
	GoogleEmailVerified bool              `bson:"google_email_verified,omitempty"`
	LastLoginProvider   string            `bson:"last_login_provider,omitempty"`
	Roles               []string          `bson:"roles"`
	TenantID            string            `bson:"tenant_id"`
	Phone               string            `bson:"phone,omitempty"`
	Bio                 string            `bson:"bio,omitempty"`
	Gender              string            `bson:"gender,omitempty"`
	DateOfBirth         string            `bson:"date_of_birth,omitempty"`
	Address             string            `bson:"address,omitempty"`
	City                string            `bson:"city,omitempty"`
	District            string            `bson:"district,omitempty"`
	JobTitle            string            `bson:"job_title,omitempty"`
	Region              string            `bson:"region,omitempty"`
	SocialLinks         map[string]string `bson:"social_links,omitempty"`
	ExtendedProfile     string            `bson:"extended_profile,omitempty"`
	IsProfileCompleted  bool              `bson:"is_profile_completed,omitempty"`
	LastLoginAt         *time.Time        `bson:"last_login_at,omitempty"`
	LoginCount          int64             `bson:"login_count,omitempty"`
	CreatedAt           time.Time         `bson:"created_at"`
	UpdatedAt           time.Time         `bson:"updated_at"`
}

type legacyAuthSession struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	UserID           bson.ObjectID `bson:"user_id"`
	SessionID        string        `bson:"session_id"`
	IPAddress        string        `bson:"ip_address"`
	UserAgent        string        `bson:"user_agent"`
	RefreshTokenHash string        `bson:"refresh_token_hash"`
	TenantID         string        `bson:"tenant_id"`
	CreatedAt        time.Time     `bson:"created_at"`
	LastActive       time.Time     `bson:"last_active"`
	LastActiveAt     time.Time     `bson:"last_active_at"`
	ExpiresAt        time.Time     `bson:"expires_at"`
	RevokedAt        *time.Time    `bson:"revoked_at,omitempty"`
	IsActive         *bool         `bson:"is_active,omitempty"`
}

// BackfillLegacyAuthFromMongo reconciles legacy Mongo auth users/sessions into PostgreSQL.
// It is idempotent and safe to call on every startup.
func BackfillLegacyAuthFromMongo(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	log *logger.Logger,
	defaultTenantID string,
) (*BackfillAuthReport, error) {
	if db == nil || mongoClient == nil {
		return &BackfillAuthReport{}, nil
	}
	if log == nil {
		log = logger.NoOp()
	}
	if strings.TrimSpace(defaultTenantID) == "" {
		defaultTenantID = "default"
	}

	report := &BackfillAuthReport{}

	roleIDByName, err := loadRoleIDByName(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("postgrescore.backfill.loadRoles: %w", err)
	}

	aliasUserIDByLegacyID, err := backfillLegacyUsers(ctx, db, mongoClient, defaultTenantID, roleIDByName, report)
	if err != nil {
		return nil, err
	}
	if err := backfillLegacySessions(ctx, db, mongoClient, defaultTenantID, aliasUserIDByLegacyID, report); err != nil {
		return nil, err
	}

	log.Info().
		Int("users_seen", report.UsersSeen).
		Int("users_created", report.UsersCreated).
		Int("users_updated", report.UsersUpdated).
		Int("user_ids_rekeyed", report.UserIDsRekeyed).
		Int("sessions_seen", report.SessionsSeen).
		Int("sessions_created", report.SessionsCreated).
		Int("sessions_updated", report.SessionsUpdated).
		Int("skipped_user_roles", report.SkippedUserRoles).
		Msg("postgrescore: legacy auth backfill complete")

	return report, nil
}

func backfillLegacyUsers(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	defaultTenantID string,
	roleIDByName map[string]string,
	report *BackfillAuthReport,
) (map[string]string, error) {
	cursor, err := mongoClient.Collection("auth_users").Find(ctx, bson.M{}, mongoFindSort("updated_at", "_id"))
	if err != nil {
		return nil, fmt.Errorf("postgrescore.backfill.users.find: %w", err)
	}
	defer cursor.Close(ctx)

	var legacyUsers []legacyAuthUser
	for cursor.Next(ctx) {
		var legacy legacyAuthUser
		if err := cursor.Decode(&legacy); err != nil {
			return nil, fmt.Errorf("postgrescore.backfill.users.decode: %w", err)
		}
		report.UsersSeen++
		if legacy.ID.IsZero() || strings.TrimSpace(legacy.Email) == "" {
			continue
		}
		legacyUsers = append(legacyUsers, legacy)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("postgrescore.backfill.users.cursor: %w", err)
	}

	canonicalByEmailKey := make(map[string]legacyAuthUser)
	aliasUserIDByLegacyID := make(map[string]string, len(legacyUsers))
	for _, legacy := range legacyUsers {
		emailKey := userEmailKey(normalizedTenant(legacy.TenantID, defaultTenantID), legacy.Email)
		current, exists := canonicalByEmailKey[emailKey]
		if !exists || shouldReplaceCanonicalUser(current, legacy) {
			canonicalByEmailKey[emailKey] = legacy
		}
	}
	for _, legacy := range legacyUsers {
		emailKey := userEmailKey(normalizedTenant(legacy.TenantID, defaultTenantID), legacy.Email)
		canonical := canonicalByEmailKey[emailKey]
		aliasUserIDByLegacyID[legacy.ID.Hex()] = canonical.ID.Hex()
	}
	for _, canonical := range canonicalByEmailKey {
		if err := upsertCanonicalLegacyUser(ctx, db, defaultTenantID, roleIDByName, report, canonical); err != nil {
			return nil, err
		}
	}
	return aliasUserIDByLegacyID, nil
}

func upsertCanonicalLegacyUser(
	ctx context.Context,
	db *gorm.DB,
	defaultTenantID string,
	roleIDByName map[string]string,
	report *BackfillAuthReport,
	legacy legacyAuthUser,
) error {
	tenantID := normalizedTenant(legacy.TenantID, defaultTenantID)
	email := normalizeEmail(legacy.Email)
	now := time.Now().UTC()
	createdAt := legacy.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := legacy.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	socialLinksJSON := ""
	if len(legacy.SocialLinks) > 0 {
		raw, err := json.Marshal(legacy.SocialLinks)
		if err != nil {
			return fmt.Errorf("postgrescore.backfill.users.social_links[%s]: %w", email, err)
		}
		socialLinksJSON = string(raw)
	}

	record := &AuthUser{
		ID:                  legacy.ID.Hex(),
		TenantID:            tenantID,
		Email:               email,
		PasswordHash:        legacy.PasswordHash,
		FullName:            legacy.FullName,
		AvatarURL:           legacy.AvatarURL,
		Status:              defaultString(legacy.Status, "ACTIVE"),
		Provider:            defaultString(legacy.Provider, "local"),
		ProviderID:          legacy.ProviderID,
		AccountType:         deriveAccountType(legacy.AccountType, legacy.Provider, legacy.GoogleSub),
		GoogleSub:           legacy.GoogleSub,
		GoogleEmail:         defaultString(legacy.GoogleEmail, email),
		GoogleEmailVerified: legacy.GoogleEmailVerified,
		LastLoginProvider:   defaultString(legacy.LastLoginProvider, legacy.Provider),
		Phone:               legacy.Phone,
		Bio:                 legacy.Bio,
		Gender:              legacy.Gender,
		DateOfBirth:         legacy.DateOfBirth,
		Address:             legacy.Address,
		City:                legacy.City,
		District:            legacy.District,
		JobTitle:            legacy.JobTitle,
		Region:              legacy.Region,
		SocialLinksJSON:     socialLinksJSON,
		ExtendedProfile:     legacy.ExtendedProfile,
		IsProfileCompleted:  legacy.IsProfileCompleted,
		LastLoginAt:         legacy.LastLoginAt,
		LoginCount:          legacy.LoginCount,
		CreatedAt:           createdAt.UTC(),
		UpdatedAt:           updatedAt.UTC(),
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingByID AuthUser
		err := tx.Where("id = ?", record.ID).First(&existingByID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("postgrescore.backfill.users.findByID[%s]: %w", record.ID, err)
		}

		var existingByEmail AuthUser
		errEmail := tx.Where("tenant_id = ? AND email = ?", record.TenantID, record.Email).First(&existingByEmail).Error
		if errEmail != nil && !errors.Is(errEmail, gorm.ErrRecordNotFound) {
			return fmt.Errorf("postgrescore.backfill.users.findByEmail[%s]: %w", record.Email, errEmail)
		}

		if err == nil {
			if err := tx.Model(&AuthUser{}).Where("id = ?", record.ID).Updates(authUserUpdates(record)).Error; err != nil {
				return fmt.Errorf("postgrescore.backfill.users.update[%s]: %w", record.ID, err)
			}
			if err := replaceUserRolesTx(ctx, tx, record.ID, legacy.Roles, roleIDByName, report); err != nil {
				return err
			}
			report.UsersUpdated++
			return nil
		}

		if errEmail == nil && existingByEmail.ID != record.ID {
			if err := rekeyAuthUserTx(ctx, tx, existingByEmail.ID, record.ID); err != nil {
				return err
			}
			report.UserIDsRekeyed++
			if err := tx.Model(&AuthUser{}).Where("id = ?", record.ID).Updates(authUserUpdates(record)).Error; err != nil {
				return fmt.Errorf("postgrescore.backfill.users.rekeyUpdate[%s]: %w", record.ID, err)
			}
			if err := replaceUserRolesTx(ctx, tx, record.ID, legacy.Roles, roleIDByName, report); err != nil {
				return err
			}
			report.UsersUpdated++
			return nil
		}

		if err := tx.Create(record).Error; err != nil {
			return fmt.Errorf("postgrescore.backfill.users.create[%s]: %w", record.ID, err)
		}
		if err := replaceUserRolesTx(ctx, tx, record.ID, legacy.Roles, roleIDByName, report); err != nil {
			return err
		}
		report.UsersCreated++
		return nil
	})
}

func backfillLegacySessions(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	defaultTenantID string,
	aliasUserIDByLegacyID map[string]string,
	report *BackfillAuthReport,
) error {
	cursor, err := mongoClient.Collection("auth_sessions").Find(ctx, bson.M{}, mongoFindSort("created_at", "_id"))
	if err != nil {
		return fmt.Errorf("postgrescore.backfill.sessions.find: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var legacy legacyAuthSession
		if err := cursor.Decode(&legacy); err != nil {
			return fmt.Errorf("postgrescore.backfill.sessions.decode: %w", err)
		}
		report.SessionsSeen++
		if legacy.ID.IsZero() || legacy.UserID.IsZero() || strings.TrimSpace(legacy.SessionID) == "" {
			continue
		}
		if err := upsertLegacySession(ctx, db, defaultTenantID, aliasUserIDByLegacyID, report, legacy); err != nil {
			return err
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("postgrescore.backfill.sessions.cursor: %w", err)
	}
	return nil
}

func upsertLegacySession(
	ctx context.Context,
	db *gorm.DB,
	defaultTenantID string,
	aliasUserIDByLegacyID map[string]string,
	report *BackfillAuthReport,
	legacy legacyAuthSession,
) error {
	tenantID := normalizedTenant(legacy.TenantID, defaultTenantID)
	createdAt := legacy.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	lastActiveAt := legacy.LastActiveAt
	if lastActiveAt.IsZero() {
		lastActiveAt = legacy.LastActive
	}
	if lastActiveAt.IsZero() {
		lastActiveAt = createdAt
	}
	expiresAt := legacy.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = createdAt
	}
	record := &AuthSession{
		ID:               legacy.ID.Hex(),
		UserID:           canonicalUserID(aliasUserIDByLegacyID, legacy.UserID.Hex()),
		SessionID:        legacy.SessionID,
		IPAddress:        legacy.IPAddress,
		UserAgent:        legacy.UserAgent,
		RefreshTokenHash: legacy.RefreshTokenHash,
		TenantID:         tenantID,
		LastActiveAt:     lastActiveAt.UTC(),
		ExpiresAt:        expiresAt.UTC(),
		RevokedAt:        legacy.RevokedAt,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        maxTime(createdAt, lastActiveAt).UTC(),
	}
	if record.RefreshTokenHash == "" {
		record.RefreshTokenHash = legacy.SessionID
	}
	if legacy.IsActive != nil && !*legacy.IsActive && record.RevokedAt == nil {
		now := maxTime(record.UpdatedAt, time.Now().UTC())
		record.RevokedAt = &now
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&AuthUser{}).Where("id = ?", record.UserID).Count(&userCount).Error; err != nil {
			return fmt.Errorf("postgrescore.backfill.sessions.userExists[%s]: %w", record.UserID, err)
		}
		if userCount == 0 {
			return nil
		}

		var existing AuthSession
		err := tx.Where("session_id = ? AND tenant_id = ?", record.SessionID, record.TenantID).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("postgrescore.backfill.sessions.find[%s]: %w", record.SessionID, err)
		}
		if err == nil {
			if err := tx.Model(&AuthSession{}).
				Where("session_id = ? AND tenant_id = ?", record.SessionID, record.TenantID).
				Updates(authSessionUpdates(record)).Error; err != nil {
				return fmt.Errorf("postgrescore.backfill.sessions.update[%s]: %w", record.SessionID, err)
			}
			report.SessionsUpdated++
			return nil
		}
		if err := tx.Create(record).Error; err != nil {
			return fmt.Errorf("postgrescore.backfill.sessions.create[%s]: %w", record.SessionID, err)
		}
		report.SessionsCreated++
		return nil
	})
}

func replaceUserRolesTx(
	ctx context.Context,
	tx *gorm.DB,
	userID string,
	roles []string,
	roleIDByName map[string]string,
	report *BackfillAuthReport,
) error {
	if err := tx.WithContext(ctx).Where("user_id = ?", userID).Delete(&UserRole{}).Error; err != nil {
		return fmt.Errorf("postgrescore.backfill.user_roles.delete[%s]: %w", userID, err)
	}

	roleNames := uniqueStrings(roles)
	if len(roleNames) == 0 {
		return nil
	}
	now := time.Now().UTC()
	joins := make([]UserRole, 0, len(roleNames))
	for _, roleName := range roleNames {
		roleID := roleIDByName[roleName]
		if roleID == "" {
			report.SkippedUserRoles++
			continue
		}
		joins = append(joins, UserRole{
			UserID:    userID,
			RoleID:    roleID,
			CreatedAt: now,
		})
	}
	if len(joins) == 0 {
		return nil
	}
	if err := tx.WithContext(ctx).Create(&joins).Error; err != nil {
		return fmt.Errorf("postgrescore.backfill.user_roles.create[%s]: %w", userID, err)
	}
	return nil
}

func rekeyAuthUserTx(ctx context.Context, tx *gorm.DB, oldID, newID string) error {
	if oldID == "" || newID == "" || oldID == newID {
		return nil
	}

	var targetCount int64
	if err := tx.WithContext(ctx).Model(&AuthUser{}).Where("id = ?", newID).Count(&targetCount).Error; err != nil {
		return fmt.Errorf("postgrescore.backfill.rekey.targetCheck[%s]: %w", newID, err)
	}
	if targetCount > 0 {
		return nil
	}

	for _, table := range []struct {
		name   string
		column string
	}{
		{name: "user_roles", column: "user_id"},
		{name: "user_permissions", column: "user_id"},
		{name: "user_sessions", column: "user_id"},
	} {
		if err := tx.WithContext(ctx).Table(table.name).Where(table.column+" = ?", oldID).Update(table.column, newID).Error; err != nil {
			return fmt.Errorf("postgrescore.backfill.rekey.%s[%s->%s]: %w", table.name, oldID, newID, err)
		}
	}
	if err := tx.WithContext(ctx).Model(&AuthUser{}).Where("id = ?", oldID).Update("id", newID).Error; err != nil {
		return fmt.Errorf("postgrescore.backfill.rekey.users[%s->%s]: %w", oldID, newID, err)
	}
	return nil
}

func loadRoleIDByName(ctx context.Context, db *gorm.DB) (map[string]string, error) {
	var roles []ACRole
	if err := db.WithContext(ctx).Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(roles))
	for _, role := range roles {
		if role.Name != "" {
			result[role.Name] = role.ID
		}
	}
	return result, nil
}

func authUserUpdates(record *AuthUser) map[string]any {
	return map[string]any{
		"tenant_id":             record.TenantID,
		"email":                 record.Email,
		"password_hash":         record.PasswordHash,
		"full_name":             record.FullName,
		"avatar_url":            record.AvatarURL,
		"status":                record.Status,
		"provider":              record.Provider,
		"provider_id":           record.ProviderID,
		"account_type":          record.AccountType,
		"google_sub":            record.GoogleSub,
		"google_email":          record.GoogleEmail,
		"google_email_verified": record.GoogleEmailVerified,
		"last_login_provider":   record.LastLoginProvider,
		"phone":                 record.Phone,
		"bio":                   record.Bio,
		"gender":                record.Gender,
		"date_of_birth":         record.DateOfBirth,
		"address":               record.Address,
		"city":                  record.City,
		"district":              record.District,
		"job_title":             record.JobTitle,
		"region":                record.Region,
		"social_links_json":     record.SocialLinksJSON,
		"extended_profile":      record.ExtendedProfile,
		"is_profile_completed":  record.IsProfileCompleted,
		"last_login_at":         record.LastLoginAt,
		"login_count":           record.LoginCount,
		"created_at":            record.CreatedAt,
		"updated_at":            record.UpdatedAt,
	}
}

func authSessionUpdates(record *AuthSession) map[string]any {
	return map[string]any{
		"id":                 record.ID,
		"user_id":            record.UserID,
		"ip_address":         record.IPAddress,
		"user_agent":         record.UserAgent,
		"refresh_token_hash": record.RefreshTokenHash,
		"tenant_id":          record.TenantID,
		"last_active_at":     record.LastActiveAt,
		"expires_at":         record.ExpiresAt,
		"revoked_at":         record.RevokedAt,
		"created_at":         record.CreatedAt,
		"updated_at":         record.UpdatedAt,
	}
}

func normalizedTenant(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func deriveAccountType(current, provider, googleSub string) string {
	current = strings.TrimSpace(strings.ToLower(current))
	if current != "" {
		return current
	}

	provider = strings.TrimSpace(strings.ToLower(provider))
	if strings.TrimSpace(googleSub) != "" || provider == "google" {
		return "google"
	}

	return "erg"
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func mongoFindSort(fields ...string) *options.FindOptionsBuilder {
	sort := bson.D{}
	for _, field := range fields {
		sort = append(sort, bson.E{Key: field, Value: 1})
	}
	return options.Find().SetSort(sort)
}

func canonicalUserID(aliases map[string]string, legacyID string) string {
	if aliases != nil {
		if canonicalID := aliases[legacyID]; canonicalID != "" {
			return canonicalID
		}
	}
	return legacyID
}

func userEmailKey(tenantID, email string) string {
	return normalizedTenant(tenantID, "default") + "|" + normalizeEmail(email)
}

func shouldReplaceCanonicalUser(current, candidate legacyAuthUser) bool {
	currentUpdated := maxTime(current.UpdatedAt, current.CreatedAt)
	candidateUpdated := maxTime(candidate.UpdatedAt, candidate.CreatedAt)
	if candidateUpdated.After(currentUpdated) {
		return true
	}
	if currentUpdated.After(candidateUpdated) {
		return false
	}
	return candidate.ID.Hex() > current.ID.Hex()
}
