package service

import (
	"context"

	"erg.ninja/internal/middleware"
	auditservice "erg.ninja/internal/modules/audit/application/service"
)

// SetAuditPublisher wires the audit sink used by service-level hooks.
func (s *Service) SetAuditPublisher(publisher auditservice.Publisher) {
	if publisher == nil {
		publisher = auditservice.NoopPublisher{}
	}
	s.audit = publisher
}

func (s *Service) publishAuditEvent(ctx context.Context, event auditservice.Event) {
	if s == nil || s.audit == nil {
		return
	}
	_ = s.audit.PublishAuditEvent(ctx, event)
}

func (s *Service) publishQuizPublishedAudit(ctx context.Context, tenantID string, quiz Quiz, version int, packageHash string) {
	actor := auditActorFromContext(ctx)
	s.publishAuditEvent(ctx, auditservice.BuildQuizEvent(auditservice.QuizEventInput{
		Action:      auditservice.ActionQuizPublished,
		TenantID:    tenantID,
		UserID:      actor.UserID,
		UserEmail:   actor.UserEmail,
		QuizID:      quiz.ID.Hex(),
		QuizVersion: version,
		PackageHash: packageHash,
		Context:     auditservice.ContextFieldsFromContext(ctx),
		Metadata: map[string]any{
			"subject_id": quiz.SubjectID.Hex(),
			"level_id":   quiz.LevelID.Hex(),
			"kind":       quiz.Kind,
		},
	}))
}

func (s *Service) publishQuizSubmittedAudit(ctx context.Context, tenantID string, attempt Attempt) {
	s.publishAuditEvent(ctx, auditservice.BuildQuizEvent(auditservice.QuizEventInput{
		Action:       auditservice.ActionQuizSubmitted,
		TenantID:     tenantID,
		UserID:       attempt.StudentID.Hex(),
		QuizID:       attempt.QuizID.Hex(),
		AttemptID:    attempt.ID.Hex(),
		AssignmentID: attempt.AssignmentID.Hex(),
		PackageHash:  attempt.PackageHash,
		AnswerCount:  len(attempt.Answers),
		Score:        attempt.Score,
		MaxScore:     attempt.MaxScore,
		Percent:      attempt.Percent,
		Passed:       attempt.Passed,
		Context:      auditservice.ContextFieldsFromContext(ctx),
	}))
}

func auditActorFromContext(ctx context.Context) auditservice.ActorFields {
	claims := middleware.GetClaims(ctx)
	if claims == nil {
		return auditservice.ActorFields{}
	}
	return auditservice.ActorFields{
		UserID:      claims.UserID,
		UserEmail:   claims.Email,
		Roles:       append([]string(nil), claims.Roles...),
		Permissions: append([]string(nil), claims.Permissions...),
	}
}
