package service

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestQuizPackageFromQuizIncludesVersionAndHash(t *testing.T) {
	quiz := Quiz{
		ID:          bson.NewObjectID(),
		SubjectID:   bson.NewObjectID(),
		LevelID:     bson.NewObjectID(),
		Title:       "Contract quiz",
		Kind:        "practice",
		Version:     3,
		PackageHash: "pkg-hash",
	}

	got := quizPackageFromQuiz(quiz)
	if got.Version != 3 {
		t.Fatalf("version = %d, want 3", got.Version)
	}
	if got.PackageHash != "pkg-hash" || got.ContentHash != "pkg-hash" {
		t.Fatalf("hash fields = (%q, %q), want pkg-hash", got.PackageHash, got.ContentHash)
	}
}

func TestQuizPackageFromQuizDerivesHashForDraft(t *testing.T) {
	quiz := Quiz{ID: bson.NewObjectID(), SubjectID: bson.NewObjectID(), LevelID: bson.NewObjectID(), Title: "Draft"}

	got := quizPackageFromQuiz(quiz)
	if got.PackageHash == "" || got.ContentHash == "" {
		t.Fatalf("expected draft package hash to be derived, got %+v", got)
	}
}
