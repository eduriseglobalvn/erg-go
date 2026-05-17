package postgrescore

import (
	"reflect"
	"strings"
	"testing"
)

func TestRelationshipSpecsCoverHardenedTasks(t *testing.T) {
	specs := RelationshipSpecs()
	if len(specs) == 0 {
		t.Fatal("RelationshipSpecs returned no specs")
	}

	seen := map[string]RelationshipSpec{}
	for _, spec := range specs {
		if spec.Name == "" || spec.SourceTable == "" || spec.SourceColumn == "" || spec.TargetTable == "" || spec.TargetColumn == "" {
			t.Fatalf("relationship spec has missing required fields: %#v", spec)
		}
		if _, ok := seen[spec.Name]; ok {
			t.Fatalf("duplicate relationship spec name %q", spec.Name)
		}
		seen[spec.Name] = spec
	}

	required := []string{
		"auth_session_user",
		"user_role_user",
		"user_role_role",
		"role_permission_role",
		"role_permission_permission",
		"profile_user",
		"certificate_user",
		"social_account_user",
		"course_progress_user",
		"work_shift_user",
		"post_category",
		"center_parent",
		"user_access_scope_user",
		"user_access_scope_center",
		"community_post_topic",
		"community_post_author",
		"community_media_post",
		"community_comment_post",
		"community_comment_author",
		"community_reaction_user",
		"community_follow_user",
		"community_reaction_target",
		"community_follow_target",
	}
	for _, name := range required {
		if _, ok := seen[name]; !ok {
			t.Fatalf("relationship spec %q is missing", name)
		}
	}
}

func TestDatabaseFKRelationshipTagsArePresent(t *testing.T) {
	cases := []struct {
		model any
		field string
	}{
		{AuthSession{}, "User"},
		{UserRole{}, "User"},
		{UserRole{}, "Role"},
		{RolePermission{}, "Role"},
		{RolePermission{}, "Permission"},
		{Profile{}, "User"},
		{Certificate{}, "User"},
		{SocialAccount{}, "User"},
		{CourseProgress{}, "User"},
		{WorkShift{}, "User"},
		{Post{}, "Category"},
		{Center{}, "Parent"},
		{UserAccessScope{}, "User"},
		{UserAccessScope{}, "Center"},
		{CommunityPost{}, "Topic"},
		{CommunityPost{}, "Author"},
		{CommunityMedia{}, "Post"},
		{CommunityComment{}, "Post"},
		{CommunityComment{}, "Author"},
		{CommunityReaction{}, "User"},
		{CommunityFollow{}, "User"},
	}
	for _, tc := range cases {
		t.Run(reflect.TypeOf(tc.model).Name()+"."+tc.field, func(t *testing.T) {
			field, ok := reflect.TypeOf(tc.model).FieldByName(tc.field)
			if !ok {
				t.Fatalf("field %s is missing", tc.field)
			}
			tag := field.Tag.Get("gorm")
			for _, want := range []string{"foreignKey:", "references:", "constraint:"} {
				if !strings.Contains(tag, want) {
					t.Fatalf("gorm tag %q does not contain %q", tag, want)
				}
			}
		})
	}
}
