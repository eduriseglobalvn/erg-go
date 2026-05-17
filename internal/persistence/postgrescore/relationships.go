package postgrescore

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// RelationshipEnforcement describes where a persisted reference is enforced.
type RelationshipEnforcement string

const (
	EnforcementDatabaseFK        RelationshipEnforcement = "database_fk"
	EnforcementServiceValidation RelationshipEnforcement = "service_validation"
	EnforcementDocumentedOnly    RelationshipEnforcement = "documented_only"
)

// RelationshipSpec documents and powers integrity checks for persisted references.
type RelationshipSpec struct {
	Name         string
	SourceTable  string
	SourceColumn string
	TargetTable  string
	TargetColumn string
	Required     bool
	Cardinality  string
	DeletePolicy string
	Enforcement  RelationshipEnforcement
	Notes        string
}

// RelationshipIntegrityFinding reports orphaned or empty references.
type RelationshipIntegrityFinding struct {
	Relationship string
	SourceTable  string
	SourceColumn string
	TargetTable  string
	InvalidCount int64
	SampleIDs    []string
}

// RelationshipIntegrityReport summarizes orphan checks across persisted models.
type RelationshipIntegrityReport struct {
	Findings []RelationshipIntegrityFinding
}

func (r RelationshipIntegrityReport) HasFindings() bool {
	return len(r.Findings) > 0
}

// RelationshipSpecs is the canonical relationship matrix for PostgreSQL models.
func RelationshipSpecs() []RelationshipSpec {
	specs := []RelationshipSpec{
		{Name: "auth_session_user", SourceTable: "user_sessions", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Ephemeral sessions are deleted with the owning user."},
		{Name: "user_role_user", SourceTable: "user_roles", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Join row cannot outlive user."},
		{Name: "user_role_role", SourceTable: "user_roles", SourceColumn: "role_id", TargetTable: "roles", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Join row cannot outlive role."},
		{Name: "role_permission_role", SourceTable: "role_permissions", SourceColumn: "role_id", TargetTable: "roles", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Join row cannot outlive role."},
		{Name: "role_permission_permission", SourceTable: "role_permissions", SourceColumn: "permission_id", TargetTable: "permissions", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Join row cannot outlive permission."},
		{Name: "profile_user", SourceTable: "profiles", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "one-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Profile is owned by user."},
		{Name: "certificate_user", SourceTable: "certificates", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Certificate belongs to user profile."},
		{Name: "social_account_user", SourceTable: "social_accounts", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "External account binding belongs to user."},
		{Name: "course_progress_user", SourceTable: "course_progress", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Progress is per user."},
		{Name: "work_shift_user", SourceTable: "work_shifts", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "restrict", Enforcement: EnforcementDatabaseFK, Notes: "Scheduling/audit records should not disappear silently."},
		{Name: "post_category", SourceTable: "posts", SourceColumn: "category_id", TargetTable: "post_categories", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "restrict", Enforcement: EnforcementDatabaseFK, Notes: "Post category is required for publishing and filtering."},
		{Name: "center_parent", SourceTable: "centers", SourceColumn: "parent_id", TargetTable: "centers", TargetColumn: "id", Required: false, Cardinality: "self-reference", DeletePolicy: "set null", Enforcement: EnforcementDatabaseFK, Notes: "Root centers have no parent."},
		{Name: "user_access_scope_user", SourceTable: "user_access_scopes", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Access scope belongs to user."},
		{Name: "user_access_scope_center", SourceTable: "user_access_scopes", SourceColumn: "center_id", TargetTable: "centers", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Access scope belongs to center."},
		{Name: "community_post_topic", SourceTable: "community_posts", SourceColumn: "topic_id", TargetTable: "community_topics", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "restrict", Enforcement: EnforcementDatabaseFK, Notes: "Posts require an existing topic."},
		{Name: "community_post_author", SourceTable: "community_posts", SourceColumn: "author_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "restrict", Enforcement: EnforcementDatabaseFK, Notes: "Posts preserve author attribution."},
		{Name: "community_media_post", SourceTable: "community_media", SourceColumn: "post_id", TargetTable: "community_posts", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Media is owned by post."},
		{Name: "community_comment_post", SourceTable: "community_comments", SourceColumn: "post_id", TargetTable: "community_posts", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Comments are owned by post."},
		{Name: "community_comment_author", SourceTable: "community_comments", SourceColumn: "author_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "restrict", Enforcement: EnforcementDatabaseFK, Notes: "Comments preserve author attribution."},
		{Name: "community_reaction_user", SourceTable: "community_reactions", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Reaction belongs to user."},
		{Name: "community_follow_user", SourceTable: "community_follows", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: true, Cardinality: "many-to-one", DeletePolicy: "cascade", Enforcement: EnforcementDatabaseFK, Notes: "Follow belongs to user."},
		{Name: "community_reaction_target", SourceTable: "community_reactions", SourceColumn: "target_id", TargetTable: "community_posts/community_comments", TargetColumn: "id", Required: true, Cardinality: "polymorphic", DeletePolicy: "service managed", Enforcement: EnforcementServiceValidation, Notes: "Target type allowlist and existence are validated in community repository."},
		{Name: "community_follow_target", SourceTable: "community_follows", SourceColumn: "target_id", TargetTable: "community_topics/users", TargetColumn: "id", Required: true, Cardinality: "polymorphic", DeletePolicy: "service managed", Enforcement: EnforcementServiceValidation, Notes: "Target type allowlist and existence are validated in community repository."},
		{Name: "auth_login_attempt_user", SourceTable: "auth_login_attempts", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id", Required: false, Cardinality: "many-to-one", DeletePolicy: "audit retained", Enforcement: EnforcementDocumentedOnly, Notes: "Attempts may be anonymous or retained after user lifecycle changes."},
		{Name: "recruitment_candidate_job", SourceTable: "candidates", SourceColumn: "job_id", TargetTable: "jobs", TargetColumn: "id", Required: false, Cardinality: "many-to-one", DeletePolicy: "snapshot retained", Enforcement: EnforcementDocumentedOnly, Notes: "Candidate keeps job title snapshot; legacy records may not have job_id."},
	}
	out := make([]RelationshipSpec, len(specs))
	copy(out, specs)
	return out
}

// CheckRelationshipIntegrity scans documented PostgreSQL relationships for orphan references.
func CheckRelationshipIntegrity(ctx context.Context, db *gorm.DB) (RelationshipIntegrityReport, error) {
	if db == nil {
		return RelationshipIntegrityReport{}, fmt.Errorf("postgrescore.CheckRelationshipIntegrity: db is nil")
	}
	report := RelationshipIntegrityReport{}
	for _, spec := range RelationshipSpecs() {
		if spec.Enforcement == EnforcementServiceValidation || strings.Contains(spec.TargetTable, "/") {
			continue
		}
		count, err := countInvalidRelationshipRows(ctx, db, spec)
		if err != nil {
			return report, err
		}
		if count == 0 {
			continue
		}
		samples, err := sampleInvalidRelationshipIDs(ctx, db, spec, 10)
		if err != nil {
			return report, err
		}
		report.Findings = append(report.Findings, RelationshipIntegrityFinding{
			Relationship: spec.Name,
			SourceTable:  spec.SourceTable,
			SourceColumn: spec.SourceColumn,
			TargetTable:  spec.TargetTable,
			InvalidCount: count,
			SampleIDs:    samples,
		})
	}
	return report, nil
}

func countInvalidRelationshipRows(ctx context.Context, db *gorm.DB, spec RelationshipSpec) (int64, error) {
	var count int64
	if err := db.WithContext(ctx).Raw(invalidRelationshipCountSQL(spec)).Scan(&count).Error; err != nil {
		return 0, fmt.Errorf("postgrescore.relationships.%s.count: %w", spec.Name, err)
	}
	return count, nil
}

func sampleInvalidRelationshipIDs(ctx context.Context, db *gorm.DB, spec RelationshipSpec, limit int) ([]string, error) {
	var ids []string
	if limit <= 0 {
		limit = 10
	}
	if err := db.WithContext(ctx).Raw(invalidRelationshipSampleSQL(spec), limit).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("postgrescore.relationships.%s.sample: %w", spec.Name, err)
	}
	return ids, nil
}

func invalidRelationshipCountSQL(spec RelationshipSpec) string {
	return fmt.Sprintf(
		"SELECT COUNT(*) FROM %s s LEFT JOIN %s t ON s.%s = t.%s WHERE %s",
		spec.SourceTable,
		spec.TargetTable,
		spec.SourceColumn,
		spec.TargetColumn,
		invalidRelationshipPredicate(spec),
	)
}

func invalidRelationshipSampleSQL(spec RelationshipSpec) string {
	return fmt.Sprintf(
		"SELECT s.id FROM %s s LEFT JOIN %s t ON s.%s = t.%s WHERE %s LIMIT ?",
		spec.SourceTable,
		spec.TargetTable,
		spec.SourceColumn,
		spec.TargetColumn,
		invalidRelationshipPredicate(spec),
	)
}

func invalidRelationshipPredicate(spec RelationshipSpec) string {
	missingTarget := fmt.Sprintf("t.%s IS NULL", spec.TargetColumn)
	if spec.Required {
		return missingTarget
	}
	return fmt.Sprintf("s.%s IS NOT NULL AND s.%s <> '' AND %s", spec.SourceColumn, spec.SourceColumn, missingTarget)
}
