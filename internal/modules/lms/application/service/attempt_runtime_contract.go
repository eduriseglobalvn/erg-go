package service

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func assignmentIncludesStudent(assignment Assignment, studentID bson.ObjectID) bool {
	for _, id := range assignment.StudentIDs {
		if id == studentID {
			return true
		}
	}
	return false
}

func actorOwnsAttempt(actor Actor, attempt Attempt) bool {
	return actor.UserID != "" && actor.UserID == attempt.StudentID.Hex()
}

func (s *Service) ensureActorCanAccessAttempt(actor Actor, attempt Attempt) error {
	if actorOwnsAttempt(actor, attempt) {
		return nil
	}
	return errScopeForbidden
}

func (s *Service) SaveAnswerForActor(ctx context.Context, tenantID string, actor Actor, attemptID, questionID string, req SaveAnswerRequestDTO) (AnswerResultResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AnswerResultResponseDTO{}, err
	}
	if attempt == nil {
		return AnswerResultResponseDTO{}, errNotFound
	}
	if err := s.ensureActorCanAccessAttempt(actor, *attempt); err != nil {
		return AnswerResultResponseDTO{}, err
	}
	return s.SaveAnswer(ctx, tenantID, attemptID, questionID, req)
}

func (s *Service) SaveAttemptDraft(ctx context.Context, tenantID string, actor Actor, attemptID string, req AttemptDraftRequestDTO) (AttemptResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AttemptResponseDTO{}, err
	}
	if attempt == nil {
		return AttemptResponseDTO{}, errNotFound
	}
	if err := s.ensureActorCanAccessAttempt(actor, *attempt); err != nil {
		return AttemptResponseDTO{}, err
	}
	if attempt.Status == "submitted" {
		return AttemptResponseDTO{}, errAttemptDone
	}
	if req.PackageHash != "" && attempt.PackageHash != req.PackageHash {
		return AttemptResponseDTO{}, errHashMismatch
	}
	updated, err := s.repo.SaveAttemptDraft(ctx, tenantID, attemptID, req)
	if err != nil {
		return AttemptResponseDTO{}, err
	}
	if updated == nil {
		return AttemptResponseDTO{}, errNotFound
	}
	if updated.Status == "submitted" {
		return AttemptResponseDTO{}, errAttemptDone
	}
	return attemptToDTO(*updated), nil
}

func (s *Service) SubmitAttemptForActor(ctx context.Context, tenantID string, actor Actor, attemptID string, req SubmitAttemptRequestDTO) (AttemptSubmitResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AttemptSubmitResponseDTO{}, err
	}
	if attempt == nil {
		return AttemptSubmitResponseDTO{}, errNotFound
	}
	if err := s.ensureActorCanAccessAttempt(actor, *attempt); err != nil {
		return AttemptSubmitResponseDTO{}, err
	}
	if attempt.Status == "submitted" {
		return attemptSubmitResult(*attempt), nil
	}
	return s.SubmitAttempt(ctx, tenantID, attemptID, req)
}

func (s *Service) SyncAttemptForActor(ctx context.Context, tenantID string, actor Actor, attemptID string, req AttemptSyncRequestDTO) (AttemptSyncResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AttemptSyncResponseDTO{}, err
	}
	if attempt == nil {
		return AttemptSyncResponseDTO{}, errNotFound
	}
	if err := s.ensureActorCanAccessAttempt(actor, *attempt); err != nil {
		return AttemptSyncResponseDTO{}, err
	}
	return s.SyncAttempt(ctx, tenantID, attemptID, req)
}
