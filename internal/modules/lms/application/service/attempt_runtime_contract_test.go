package service

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestActorOwnsAttemptRequiresCurrentStudent(t *testing.T) {
	studentID := bson.NewObjectID()
	otherID := bson.NewObjectID()
	attempt := Attempt{StudentID: studentID}

	if !actorOwnsAttempt(Actor{UserID: studentID.Hex()}, attempt) {
		t.Fatal("expected current student to own attempt")
	}
	if actorOwnsAttempt(Actor{UserID: otherID.Hex()}, attempt) {
		t.Fatal("expected other student to be rejected")
	}
}

func TestSubmitAttemptSelfAccessRejectsOtherStudent(t *testing.T) {
	studentID := bson.NewObjectID()
	attempt := Attempt{StudentID: studentID}

	err := (&Service{}).ensureActorCanAccessAttempt(Actor{UserID: bson.NewObjectID().Hex()}, attempt)
	if err != errScopeForbidden {
		t.Fatalf("err = %v, want errScopeForbidden", err)
	}
}

func TestAssignmentIncludesStudent(t *testing.T) {
	studentID := bson.NewObjectID()
	assignment := Assignment{StudentIDs: []bson.ObjectID{studentID}}

	if !assignmentIncludesStudent(assignment, studentID) {
		t.Fatal("expected assignment to include student")
	}
	if assignmentIncludesStudent(assignment, bson.NewObjectID()) {
		t.Fatal("expected unknown student to be excluded")
	}
}

func TestAttemptSubmitResultReturnsStoredSubmittedScore(t *testing.T) {
	result := attemptSubmitResult(Attempt{
		Status:   "submitted",
		Score:    88,
		MaxScore: 100,
		Percent:  88,
		Passed:   true,
	})

	if result.Score != 88 || result.Percent != 88 || !result.Passed {
		t.Fatalf("unexpected submitted result: %+v", result)
	}
}
