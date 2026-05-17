package repository

import "testing"

func TestCommunityReactionTargetAllowlist(t *testing.T) {
	for _, value := range []string{TargetTypePost, TargetTypeComment} {
		if !validTargetType(value) {
			t.Fatalf("target type %q should be allowed for reactions", value)
		}
	}
	for _, value := range []string{"", TargetTypeTopic, TargetTypeUser, "lesson", "post;drop"} {
		if validTargetType(value) {
			t.Fatalf("target type %q should not be allowed for reactions", value)
		}
	}
}

func TestCommunityReactionAllowlist(t *testing.T) {
	for _, value := range []string{ReactionLike, ReactionLove, ReactionCare, ReactionHaha, ReactionWow, ReactionSad, ReactionAngry} {
		if !validReaction(value) {
			t.Fatalf("reaction %q should be allowed", value)
		}
	}
	for _, value := range []string{"", "LIKE", "delete", "like;drop"} {
		if validReaction(value) {
			t.Fatalf("reaction %q should not be allowed", value)
		}
	}
}
