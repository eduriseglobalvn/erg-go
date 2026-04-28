// Package repository provides PostgreSQL data access for the access_control module.
package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"erg.ninja/internal/modules/access_control/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository provides data access for roles, permissions, and overrides.
type Repository struct {
	db  *gorm.DB
	log *logger.Logger
}

// NewRepository creates a new access_control repository.
func NewRepository(gormClient *database.GORMPostgresClient, log *logger.Logger) *Repository {
	var db *gorm.DB
	if gormClient != nil {
		db = gormClient.DB()
	}
	return &Repository{db: db, log: log}
}

func (r *Repository) ensureDB() error {
	if r.db == nil {
		return fmt.Errorf("access_control.repo: postgres client unavailable")
	}
	return nil
}

// ─── Roles ─────────────────────────────────────────────────────────────────

// CreateRole inserts a new role.
func (r *Repository) CreateRole(ctx context.Context, role *entities.Role) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	now := time.Now().UTC()
	record := &postgrescore.ACRole{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsDefault:   role.IsDefault,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			if isDuplicateKey(err) {
				return fmt.Errorf("access_control.repo.CreateRole: role name already exists")
			}
			return err
		}
		return r.replaceRolePermissionsTx(ctx, tx, role.ID, role.Permissions)
	})
}

// GetRoleByID retrieves a role by its ID.
func (r *Repository) GetRoleByID(ctx context.Context, id string) (*entities.Role, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.ACRole
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("access_control.repo.GetRoleByID: %w", err)
	}
	return r.mapRole(ctx, &record)
}

// GetRoleByName retrieves a role by name.
func (r *Repository) GetRoleByName(ctx context.Context, name string) (*entities.Role, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.ACRole
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("access_control.repo.GetRoleByName: %w", err)
	}
	return r.mapRole(ctx, &record)
}

// ListRoles returns all roles.
func (r *Repository) ListRoles(ctx context.Context) ([]*entities.Role, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []postgrescore.ACRole
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("access_control.repo.ListRoles: %w", err)
	}
	roles := make([]*entities.Role, 0, len(records))
	for i := range records {
		role, err := r.mapRole(ctx, &records[i])
		if err != nil {
			return nil, fmt.Errorf("access_control.repo.ListRoles.map: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// UpdateRole updates an existing role.
func (r *Repository) UpdateRole(ctx context.Context, id string, updates map[string]any) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if perms, ok := updates["permissions"]; ok {
			if permNames, ok := perms.([]string); ok {
				if err := r.replaceRolePermissionsTx(ctx, tx, id, permNames); err != nil {
					return err
				}
			}
			delete(updates, "permissions")
		}
		if len(updates) == 0 {
			return nil
		}
		updates["updated_at"] = time.Now().UTC()
		if err := tx.WithContext(ctx).Model(&postgrescore.ACRole{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return nil
	})
}

// DeleteRole removes a role.
func (r *Repository) DeleteRole(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&postgrescore.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&postgrescore.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&postgrescore.ACRole{}).Error
	})
}

// ─── Permissions ───────────────────────────────────────────────────────────

// ListPermissions returns all permissions.
func (r *Repository) ListPermissions(ctx context.Context) ([]*entities.Permission, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []postgrescore.ACPermission
	if err := r.db.WithContext(ctx).Order("group_name ASC, name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("access_control.repo.ListPermissions: %w", err)
	}
	result := make([]*entities.Permission, 0, len(records))
	for _, record := range records {
		result = append(result, &entities.Permission{
			ID:    record.ID,
			Name:  record.Name,
			Group: record.GroupName,
			Label: record.Label,
			Desc:  record.Description,
		})
	}
	return result, nil
}

// GetPermissionsByGroup returns permissions grouped by their group.
func (r *Repository) GetPermissionsByGroup(ctx context.Context) (map[string][]*entities.Permission, error) {
	perms, err := r.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]*entities.Permission)
	for _, p := range perms {
		result[p.Group] = append(result[p.Group], p)
	}
	return result, nil
}

// ─── Permission Groups ─────────────────────────────────────────────────────

// ListPermissionGroups returns all permission groups ordered by display order.
func (r *Repository) ListPermissionGroups(ctx context.Context) ([]*entities.PermissionGroup, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []postgrescore.ACPermissionGroup
	if err := r.db.WithContext(ctx).Order("display_order ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("access_control.repo.ListPermissionGroups: %w", err)
	}
	result := make([]*entities.PermissionGroup, 0, len(records))
	for _, record := range records {
		result = append(result, &entities.PermissionGroup{
			ID:    record.ID,
			Name:  record.Name,
			Label: record.Label,
			Order: record.Order,
		})
	}
	return result, nil
}

// ─── User Permission Overrides ─────────────────────────────────────────────

// CreateOverride inserts a new permission override.
func (r *Repository) CreateOverride(ctx context.Context, override *entities.UserPermissionOverride) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	override.CreatedAt = time.Now().UTC()
	record := &postgrescore.ACUserPermissionOverride{
		ID:         override.ID,
		UserID:     override.UserID,
		Permission: override.Permission,
		GrantType:  override.GrantType,
		Reason:     override.Reason,
		ExpiresAt:  override.ExpiresAt,
		CreatedBy:  override.CreatedBy,
		CreatedAt:  override.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("access_control.repo.CreateOverride: %w", err)
	}
	return nil
}

// ListOverridesByUser returns all overrides for a given user.
func (r *Repository) ListOverridesByUser(ctx context.Context, userID string) ([]*entities.UserPermissionOverride, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []postgrescore.ACUserPermissionOverride
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("access_control.repo.ListOverridesByUser: %w", err)
	}
	result := make([]*entities.UserPermissionOverride, 0, len(records))
	for _, record := range records {
		result = append(result, &entities.UserPermissionOverride{
			ID:         record.ID,
			UserID:     record.UserID,
			Permission: record.Permission,
			GrantType:  record.GrantType,
			Reason:     record.Reason,
			ExpiresAt:  record.ExpiresAt,
			CreatedBy:  record.CreatedBy,
			CreatedAt:  record.CreatedAt,
		})
	}
	return result, nil
}

// GetOverrideByID retrieves a specific override by ID.
func (r *Repository) GetOverrideByID(ctx context.Context, id string) (*entities.UserPermissionOverride, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.ACUserPermissionOverride
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("access_control.repo.GetOverrideByID: %w", err)
	}
	return &entities.UserPermissionOverride{
		ID:         record.ID,
		UserID:     record.UserID,
		Permission: record.Permission,
		GrantType:  record.GrantType,
		Reason:     record.Reason,
		ExpiresAt:  record.ExpiresAt,
		CreatedBy:  record.CreatedBy,
		CreatedAt:  record.CreatedAt,
	}, nil
}

// DeleteOverride removes a permission override.
func (r *Repository) DeleteOverride(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&postgrescore.ACUserPermissionOverride{}).Error; err != nil {
		return fmt.Errorf("access_control.repo.DeleteOverride: %w", err)
	}
	return nil
}

// GetUserRoles reads role names from the relational user_roles join table.
func (r *Repository) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	type row struct {
		Name string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.name").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("access_control.repo.GetUserRoles: %w", err)
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Name != "" {
			result = append(result, row.Name)
		}
	}
	return uniqueStrings(result), nil
}

// SetUserRoles writes canonical role names to the user_roles join table.
func (r *Repository) SetUserRoles(ctx context.Context, userID string, roles []string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&postgrescore.AuthUser{}).Where("id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("access_control.repo.SetUserRoles: user not found")
		}
		return r.replaceUserRolesTx(ctx, tx, userID, roles)
	})
}

// SeedDefaultData reconciles default permissions, groups, and roles on every startup.
func (r *Repository) SeedDefaultData(ctx context.Context) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	defaultPerms := defaultPermissions()
	for _, perm := range defaultPerms {
		if err := r.upsertPermission(ctx, perm); err != nil {
			return fmt.Errorf("access_control.repo.SeedDefaultData.permission[%s]: %w", perm.Name, err)
		}
	}

	for _, group := range defaultPermissionGroups() {
		if err := r.upsertPermissionGroup(ctx, group); err != nil {
			return fmt.Errorf("access_control.repo.SeedDefaultData.group[%s]: %w", group.Name, err)
		}
	}

	allPermissionNames := make([]string, 0, len(defaultPerms))
	for _, perm := range defaultPerms {
		allPermissionNames = append(allPermissionNames, perm.Name)
	}

	defaultRoles := []entities.Role{
		{
			ID:          database.NewID(),
			Name:        "admin",
			Description: "Super Administrator",
			Permissions: allPermissionNames,
			IsDefault:   false,
		},
		{
			ID:          database.NewID(),
			Name:        "user",
			Description: "Standard user",
			Permissions: []string{"posts.read"},
			IsDefault:   true,
		},
		{
			ID:          database.NewID(),
			Name:        "editor",
			Description: "Post Editor",
			Permissions: []string{"posts.read", "posts.create", "posts.update", "posts.delete", "users.read"},
			IsDefault:   false,
		},
		{
			ID:          database.NewID(),
			Name:        "content_manager",
			Description: "Content Manager",
			Permissions: []string{
				"posts.read", "posts.create", "posts.update", "posts.delete", "posts.publish",
				"courses.read", "courses.create", "courses.update", "courses.delete", "courses.publish",
				"crawler.read", "crawler.create", "crawler.update", "crawler.delete", "crawler.execute", "crawler.manage",
				"seo.read", "seo.update", "seo.analyze", "seo.manage",
			},
			IsDefault: false,
		},
		{
			ID:          database.NewID(),
			Name:        "seo_specialist",
			Description: "SEO Specialist",
			Permissions: []string{"seo.read", "seo.update", "seo.analyze", "seo.manage", "analytics.read"},
			IsDefault:   false,
		},
		{
			ID:          database.NewID(),
			Name:        "hr_manager",
			Description: "HR Manager",
			Permissions: []string{"recruitment.read", "recruitment.create", "recruitment.update", "recruitment.delete", "users.read"},
			IsDefault:   false,
		},
		{
			ID:          database.NewID(),
			Name:        "viewer",
			Description: "Read-only Viewer",
			Permissions: readOnlyPermissions(defaultPerms),
			IsDefault:   false,
		},
	}

	for _, role := range defaultRoles {
		if err := r.upsertRole(ctx, role); err != nil {
			return fmt.Errorf("access_control.repo.SeedDefaultData.role[%s]: %w", role.Name, err)
		}
	}

	r.log.Info().
		Int("permissions", len(defaultPerms)).
		Int("roles", len(defaultRoles)).
		Msg("access_control: default data reconciled in postgres")
	return nil
}

func (r *Repository) upsertPermission(ctx context.Context, perm entities.Permission) error {
	now := time.Now().UTC()
	var record postgrescore.ACPermission
	err := r.db.WithContext(ctx).Where("name = ?", perm.Name).First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = postgrescore.ACPermission{
			ID:          perm.ID,
			Name:        perm.Name,
			GroupName:   perm.Group,
			Label:       perm.Label,
			Description: perm.Desc,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return r.db.WithContext(ctx).Create(&record).Error
	}
	return r.db.WithContext(ctx).Model(&postgrescore.ACPermission{}).Where("id = ?", record.ID).Updates(map[string]any{
		"group_name":  perm.Group,
		"label":       perm.Label,
		"description": perm.Desc,
		"updated_at":  now,
	}).Error
}

func (r *Repository) upsertPermissionGroup(ctx context.Context, group entities.PermissionGroup) error {
	now := time.Now().UTC()
	var record postgrescore.ACPermissionGroup
	err := r.db.WithContext(ctx).Where("name = ?", group.Name).First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = postgrescore.ACPermissionGroup{
			ID:        group.ID,
			Name:      group.Name,
			Label:     group.Label,
			Order:     group.Order,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return r.db.WithContext(ctx).Create(&record).Error
	}
	return r.db.WithContext(ctx).Model(&postgrescore.ACPermissionGroup{}).Where("id = ?", record.ID).Updates(map[string]any{
		"label":         group.Label,
		"display_order": group.Order,
		"updated_at":    now,
	}).Error
}

func (r *Repository) upsertRole(ctx context.Context, role entities.Role) error {
	now := time.Now().UTC()
	var record postgrescore.ACRole
	err := r.db.WithContext(ctx).Where("name = ?", role.Name).First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = postgrescore.ACRole{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			IsDefault:   role.IsDefault,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
			return err
		}
	} else {
		if err := r.db.WithContext(ctx).Model(&postgrescore.ACRole{}).Where("id = ?", record.ID).Updates(map[string]any{
			"description": role.Description,
			"is_default":  role.IsDefault,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
	}
	return r.replaceRolePermissionsTx(ctx, r.db.WithContext(ctx), record.ID, role.Permissions)
}

func (r *Repository) replaceRolePermissionsTx(ctx context.Context, tx *gorm.DB, roleID string, permissionNames []string) error {
	if err := tx.WithContext(ctx).Where("role_id = ?", roleID).Delete(&postgrescore.RolePermission{}).Error; err != nil {
		return err
	}
	permissionNames = uniqueStrings(permissionNames)
	if len(permissionNames) == 0 {
		return nil
	}
	var perms []postgrescore.ACPermission
	if err := tx.WithContext(ctx).Where("name IN ?", permissionNames).Find(&perms).Error; err != nil {
		return err
	}
	permIDByName := make(map[string]string, len(perms))
	for _, perm := range perms {
		permIDByName[perm.Name] = perm.ID
	}
	now := time.Now().UTC()
	joins := make([]postgrescore.RolePermission, 0, len(permissionNames))
	for _, permissionName := range permissionNames {
		permissionID := permIDByName[permissionName]
		if permissionID == "" {
			continue
		}
		joins = append(joins, postgrescore.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
			CreatedAt:    now,
		})
	}
	if len(joins) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&joins).Error
}

func (r *Repository) replaceUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roles []string) error {
	if err := tx.WithContext(ctx).Where("user_id = ?", userID).Delete(&postgrescore.UserRole{}).Error; err != nil {
		return err
	}
	roles = uniqueStrings(roles)
	if len(roles) == 0 {
		return nil
	}
	var roleRecords []postgrescore.ACRole
	if err := tx.WithContext(ctx).Where("name IN ?", roles).Find(&roleRecords).Error; err != nil {
		return err
	}
	roleIDByName := make(map[string]string, len(roleRecords))
	for _, role := range roleRecords {
		roleIDByName[role.Name] = role.ID
	}
	now := time.Now().UTC()
	joins := make([]postgrescore.UserRole, 0, len(roles))
	for _, roleName := range roles {
		roleID := roleIDByName[roleName]
		if roleID == "" {
			continue
		}
		joins = append(joins, postgrescore.UserRole{
			UserID:    userID,
			RoleID:    roleID,
			CreatedAt: now,
		})
	}
	if len(joins) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&joins).Error
}

func (r *Repository) mapRole(ctx context.Context, record *postgrescore.ACRole) (*entities.Role, error) {
	permissionNames, err := r.loadRolePermissionNames(ctx, record.ID)
	if err != nil {
		return nil, err
	}
	return &entities.Role{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		Permissions: permissionNames,
		IsDefault:   record.IsDefault,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}, nil
}

func (r *Repository) loadRolePermissionNames(ctx context.Context, roleID string) ([]string, error) {
	type row struct {
		Name string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("permissions").
		Select("permissions.name").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).
		Order("permissions.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Name != "" {
			result = append(result, row.Name)
		}
	}
	return result, nil
}

func defaultPermissions() []entities.Permission {
	return []entities.Permission{
		{ID: database.NewID(), Name: "users.read", Group: "users", Label: "View users"},
		{ID: database.NewID(), Name: "users.create", Group: "users", Label: "Create users"},
		{ID: database.NewID(), Name: "users.update", Group: "users", Label: "Update users"},
		{ID: database.NewID(), Name: "users.delete", Group: "users", Label: "Delete users"},
		{ID: database.NewID(), Name: "users.manage", Group: "users", Label: "Manage users"},
		{ID: database.NewID(), Name: "roles.read", Group: "roles", Label: "View roles"},
		{ID: database.NewID(), Name: "roles.create", Group: "roles", Label: "Create roles"},
		{ID: database.NewID(), Name: "roles.update", Group: "roles", Label: "Update roles"},
		{ID: database.NewID(), Name: "roles.delete", Group: "roles", Label: "Delete roles"},
		{ID: database.NewID(), Name: "roles.assign", Group: "roles", Label: "Assign roles"},
		{ID: database.NewID(), Name: "roles.manage", Group: "roles", Label: "Manage role workflows"},
		{ID: database.NewID(), Name: "posts.read", Group: "posts", Label: "View posts"},
		{ID: database.NewID(), Name: "posts.create", Group: "posts", Label: "Create posts"},
		{ID: database.NewID(), Name: "posts.update", Group: "posts", Label: "Update posts"},
		{ID: database.NewID(), Name: "posts.delete", Group: "posts", Label: "Delete posts"},
		{ID: database.NewID(), Name: "posts.publish", Group: "posts", Label: "Publish posts"},
		{ID: database.NewID(), Name: "posts.manage_hidden", Group: "posts", Label: "Manage hidden posts"},
		{ID: database.NewID(), Name: "system.logs", Group: "system", Label: "View system logs"},
		{ID: database.NewID(), Name: "system.settings", Group: "system", Label: "Manage system settings"},
		{ID: database.NewID(), Name: "system.manage", Group: "system", Label: "Manage system operations"},
		{ID: database.NewID(), Name: "system.monitor", Group: "system", Label: "Monitor system queues"},
		{ID: database.NewID(), Name: "courses.read", Group: "courses", Label: "View courses"},
		{ID: database.NewID(), Name: "courses.create", Group: "courses", Label: "Create courses"},
		{ID: database.NewID(), Name: "courses.update", Group: "courses", Label: "Update courses"},
		{ID: database.NewID(), Name: "courses.delete", Group: "courses", Label: "Delete courses"},
		{ID: database.NewID(), Name: "courses.publish", Group: "courses", Label: "Publish courses"},
		{ID: database.NewID(), Name: "crawler.read", Group: "crawler", Label: "View crawler jobs"},
		{ID: database.NewID(), Name: "crawler.create", Group: "crawler", Label: "Create crawler jobs"},
		{ID: database.NewID(), Name: "crawler.update", Group: "crawler", Label: "Update crawler jobs"},
		{ID: database.NewID(), Name: "crawler.delete", Group: "crawler", Label: "Delete crawler jobs"},
		{ID: database.NewID(), Name: "crawler.execute", Group: "crawler", Label: "Execute crawler jobs"},
		{ID: database.NewID(), Name: "crawler.manage", Group: "crawler", Label: "Manage crawler"},
		{ID: database.NewID(), Name: "crawler.auto_crawl", Group: "crawler", Label: "Run auto crawl"},
		{ID: database.NewID(), Name: "seo.read", Group: "seo", Label: "View SEO data"},
		{ID: database.NewID(), Name: "seo.update", Group: "seo", Label: "Update SEO data"},
		{ID: database.NewID(), Name: "seo.analyze", Group: "seo", Label: "Analyze SEO"},
		{ID: database.NewID(), Name: "seo.manage", Group: "seo", Label: "Manage SEO"},
		{ID: database.NewID(), Name: "analytics.read", Group: "analytics", Label: "View analytics"},
		{ID: database.NewID(), Name: "analytics.export", Group: "analytics", Label: "Export analytics"},
		{ID: database.NewID(), Name: "notifications.read", Group: "notifications", Label: "View notifications"},
		{ID: database.NewID(), Name: "notifications.manage", Group: "notifications", Label: "Manage notifications"},
		{ID: database.NewID(), Name: "menus.read", Group: "menus", Label: "View menus"},
		{ID: database.NewID(), Name: "menus.create", Group: "menus", Label: "Create menus"},
		{ID: database.NewID(), Name: "menus.update", Group: "menus", Label: "Update menus"},
		{ID: database.NewID(), Name: "menus.delete", Group: "menus", Label: "Delete menus"},
		{ID: database.NewID(), Name: "pages.read", Group: "pages", Label: "View pages"},
		{ID: database.NewID(), Name: "pages.create", Group: "pages", Label: "Create pages"},
		{ID: database.NewID(), Name: "pages.update", Group: "pages", Label: "Update pages"},
		{ID: database.NewID(), Name: "pages.delete", Group: "pages", Label: "Delete pages"},
		{ID: database.NewID(), Name: "recruitment.read", Group: "recruitment", Label: "View recruitment"},
		{ID: database.NewID(), Name: "recruitment.create", Group: "recruitment", Label: "Create recruitment"},
		{ID: database.NewID(), Name: "recruitment.update", Group: "recruitment", Label: "Update recruitment"},
		{ID: database.NewID(), Name: "recruitment.delete", Group: "recruitment", Label: "Delete recruitment"},
		{ID: database.NewID(), Name: "settings.read", Group: "settings", Label: "View settings"},
		{ID: database.NewID(), Name: "settings.update", Group: "settings", Label: "Update settings"},
		{ID: database.NewID(), Name: "api-keys.read", Group: "api-keys", Label: "View API keys"},
		{ID: database.NewID(), Name: "api-keys.create", Group: "api-keys", Label: "Create API keys"},
		{ID: database.NewID(), Name: "api-keys.delete", Group: "api-keys", Label: "Delete API keys"},
		{ID: database.NewID(), Name: "audit.read", Group: "audit", Label: "View audit logs"},
		{ID: database.NewID(), Name: "reviews.read", Group: "reviews", Label: "View reviews"},
		{ID: database.NewID(), Name: "reviews.manage", Group: "reviews", Label: "Manage reviews"},
		{ID: database.NewID(), Name: "categories.read", Group: "categories", Label: "View categories"},
		{ID: database.NewID(), Name: "tags.read", Group: "tags", Label: "View tags"},
		{ID: database.NewID(), Name: "media.read", Group: "media", Label: "View media"},
	}
}

func defaultPermissionGroups() []entities.PermissionGroup {
	return []entities.PermissionGroup{
		{ID: database.NewID(), Name: "users", Label: "Users", Order: 1},
		{ID: database.NewID(), Name: "roles", Label: "Roles & Permissions", Order: 2},
		{ID: database.NewID(), Name: "posts", Label: "Posts", Order: 3},
		{ID: database.NewID(), Name: "courses", Label: "Courses", Order: 4},
		{ID: database.NewID(), Name: "crawler", Label: "Crawler", Order: 5},
		{ID: database.NewID(), Name: "seo", Label: "SEO", Order: 6},
		{ID: database.NewID(), Name: "analytics", Label: "Analytics", Order: 7},
		{ID: database.NewID(), Name: "notifications", Label: "Notifications", Order: 8},
		{ID: database.NewID(), Name: "menus", Label: "Menus", Order: 9},
		{ID: database.NewID(), Name: "pages", Label: "Pages", Order: 10},
		{ID: database.NewID(), Name: "recruitment", Label: "Recruitment", Order: 11},
		{ID: database.NewID(), Name: "settings", Label: "Settings", Order: 12},
		{ID: database.NewID(), Name: "api-keys", Label: "API Keys", Order: 13},
		{ID: database.NewID(), Name: "audit", Label: "Audit", Order: 14},
		{ID: database.NewID(), Name: "reviews", Label: "Reviews", Order: 15},
		{ID: database.NewID(), Name: "system", Label: "System", Order: 16},
		{ID: database.NewID(), Name: "categories", Label: "Categories", Order: 17},
		{ID: database.NewID(), Name: "tags", Label: "Tags", Order: 18},
		{ID: database.NewID(), Name: "media", Label: "Media", Order: 19},
	}
}

func readOnlyPermissions(perms []entities.Permission) []string {
	result := make([]string, 0, len(perms))
	for _, perm := range perms {
		if strings.HasSuffix(perm.Name, ".read") {
			result = append(result, perm.Name)
		}
	}
	sort.Strings(result)
	return result
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
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
	sort.Strings(result)
	return result
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
