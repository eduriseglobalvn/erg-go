package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) CreateAssignment(ctx context.Context, tenantID string, actor Actor, req CreateAssignmentRequestDTO) (AssignmentBatchResponseDTO, error) {
	class, err := s.repo.GetClass(ctx, tenantID, req.ClassID)
	if err != nil {
		return AssignmentBatchResponseDTO{}, err
	}
	if class == nil {
		return AssignmentBatchResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && class.HomeroomTeacherID != actor.UserID && !s.canAccessCenter(ctx, tenantID, actor, class.CenterID.Hex()) {
		return AssignmentBatchResponseDTO{}, errScopeForbidden
	}
	recipients := objectIDsOrNil(req.StudentIDs)
	if req.RecipientMode == "all" {
		students, _, _, err := s.repo.ListStudents(ctx, tenantID, StudentListRequestDTO{ClassID: req.ClassID, Limit: 100}, nil, nil)
		if err != nil {
			return AssignmentBatchResponseDTO{}, err
		}
		recipients = make([]bson.ObjectID, 0, len(students))
		for _, student := range students {
			recipients = append(recipients, student.ID)
		}
	}
	if len(recipients) == 0 {
		return AssignmentBatchResponseDTO{}, errEmptyRecipient
	}
	ids := []string{}
	for _, quizID := range req.QuizIDs {
		quiz, err := s.repo.GetQuiz(ctx, tenantID, quizID)
		if err != nil {
			return AssignmentBatchResponseDTO{}, err
		}
		if quiz == nil {
			return AssignmentBatchResponseDTO{}, errNotFound
		}
		quizOID, _ := objectID(quizID)
		a := &Assignment{TenantID: tenantID, ClassID: class.ID, QuizID: quizOID, SubjectID: quiz.SubjectID, StudentIDs: recipients, DueAt: req.DueAt, TeacherNote: req.TeacherNote, AssignedBy: actor.UserID, RecipientMode: req.RecipientMode}
		if err := s.repo.CreateAssignment(ctx, a); err != nil {
			return AssignmentBatchResponseDTO{}, err
		}
		ids = append(ids, a.ID.Hex())
	}
	return AssignmentBatchResponseDTO{AssignmentIDs: ids, RecipientCount: len(recipients)}, nil
}

func (s *Service) ClassAssignments(ctx context.Context, tenantID, classID, status, subjectID string) (ClassAssignmentListResponseDTO, error) {
	classOID, err := objectID(classID)
	if err != nil {
		return ClassAssignmentListResponseDTO{}, err
	}
	filter := bson.M{"class_id": classOID}
	if status != "" {
		filter["status"] = status
	}
	if subjectID != "" {
		oid, err := objectID(subjectID)
		if err != nil {
			return ClassAssignmentListResponseDTO{}, err
		}
		filter["subject_id"] = oid
	}
	items, err := s.repo.ListAssignments(ctx, tenantID, filter)
	return ClassAssignmentListResponseDTO{Items: assignmentsToDTO(items)}, err
}

func (s *Service) AssignmentProgress(ctx context.Context, tenantID, assignmentID string) (AssignmentProgressResponseDTO, error) {
	assignment, err := s.repo.GetAssignment(ctx, tenantID, assignmentID)
	if err != nil {
		return AssignmentProgressResponseDTO{}, err
	}
	if assignment == nil {
		return AssignmentProgressResponseDTO{}, errNotFound
	}
	attempts, err := s.repo.ListAttempts(ctx, tenantID, bson.M{"assignment_id": assignment.ID})
	if err != nil {
		return AssignmentProgressResponseDTO{}, err
	}
	progress := AssignmentProgressResponseDTO{NotStarted: len(assignment.StudentIDs)}
	seen := map[string]bool{}
	for _, attempt := range attempts {
		studentKey := attempt.StudentID.Hex()
		if seen[studentKey] {
			continue
		}
		seen[studentKey] = true
		progress.NotStarted--
		if attempt.Status == "submitted" {
			progress.Submitted++
		} else {
			progress.InProgress++
		}
	}
	return progress, nil
}

func (s *Service) StudentAssignments(ctx context.Context, tenantID string, actor Actor, status string) (StudentAssignmentListResponseDTO, error) {
	studentID, err := objectID(actor.UserID)
	if err != nil {
		return StudentAssignmentListResponseDTO{Items: []AssignmentResponseDTO{}}, nil
	}
	filter := bson.M{"student_ids": studentID}
	if status != "" {
		filter["status"] = status
	}
	items, err := s.repo.ListAssignments(ctx, tenantID, filter)
	return StudentAssignmentListResponseDTO{Items: assignmentsToDTO(items)}, err
}

func (s *Service) StudentScores(ctx context.Context, tenantID string, actor Actor, subjectID string) (StudentScoreListResponseDTO, error) {
	studentID, err := objectID(actor.UserID)
	if err != nil {
		return StudentScoreListResponseDTO{Items: []StudentScoreItemDTO{}}, nil
	}
	assignments, err := s.repo.ListAssignments(ctx, tenantID, bson.M{"student_ids": studentID})
	if err != nil {
		return StudentScoreListResponseDTO{}, err
	}
	items := []StudentScoreItemDTO{}
	for _, assignment := range assignments {
		if subjectID != "" && assignment.SubjectID.Hex() != subjectID {
			continue
		}
		attempts, err := s.repo.ListAttempts(ctx, tenantID, bson.M{"assignment_id": assignment.ID, "student_id": studentID, "status": "submitted"})
		if err != nil {
			return StudentScoreListResponseDTO{}, err
		}
		if len(attempts) == 0 {
			continue
		}
		sort.Slice(attempts, func(i, j int) bool { return attempts[i].UpdatedAt.After(attempts[j].UpdatedAt) })
		best := attempts[0].Score
		for _, a := range attempts {
			if a.Score > best {
				best = a.Score
			}
		}
		top := attempts
		if len(top) > 3 {
			top = top[:3]
		}
		items = append(items, StudentScoreItemDTO{Assignment: assignmentToDTO(assignment), BestScore: best, RecentAttemptsTop3: attemptsToDTO(top)})
	}
	return StudentScoreListResponseDTO{Items: items}, nil
}

func (s *Service) StartAttempt(ctx context.Context, tenantID string, actor Actor, req StartAttemptRequestDTO) (AttemptResponseDTO, error) {
	assignment, err := s.repo.GetAssignment(ctx, tenantID, req.AssignmentID)
	if err != nil {
		return AttemptResponseDTO{}, err
	}
	if assignment == nil {
		return AttemptResponseDTO{}, errNotFound
	}
	studentID, err := objectID(actor.UserID)
	if err != nil {
		return AttemptResponseDTO{}, errScopeForbidden
	}
	if !assignmentIncludesStudent(*assignment, studentID) {
		return AttemptResponseDTO{}, errScopeForbidden
	}
	assignmentID, _ := objectID(req.AssignmentID)
	quizID, err := objectID(req.QuizID)
	if err != nil {
		return AttemptResponseDTO{}, err
	}
	if active, _ := s.repo.FindActiveAttempt(ctx, tenantID, assignmentID, quizID, studentID); active != nil {
		return attemptToDTO(*active), nil
	}
	attempt := &Attempt{TenantID: tenantID, AssignmentID: assignmentID, QuizID: quizID, StudentID: studentID, PackageID: req.PackageID, PackageHash: req.PackageHash, MaxScore: 100}
	if err := s.repo.CreateAttempt(ctx, attempt); err != nil {
		if active, _ := s.repo.FindActiveAttempt(ctx, tenantID, assignmentID, quizID, studentID); active != nil {
			return attemptToDTO(*active), nil
		}
		return AttemptResponseDTO{}, err
	}
	return attemptToDTO(*attempt), nil
}

func (s *Service) SaveAnswer(ctx context.Context, tenantID, attemptID, questionID string, req SaveAnswerRequestDTO) (AnswerResultResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AnswerResultResponseDTO{}, err
	}
	if attempt == nil {
		return AnswerResultResponseDTO{}, errNotFound
	}
	if attempt.Status == "submitted" {
		return AnswerResultResponseDTO{}, errAttemptDone
	}
	updated, err := s.repo.SaveAttemptAnswer(ctx, tenantID, attemptID, questionID, AttemptAnswer{
		QuestionID:   questionID,
		Answer:       req.Answer,
		ClientResult: req.ClientResult,
		AnsweredAt:   req.AnsweredAt,
	})
	if err != nil {
		return AnswerResultResponseDTO{}, err
	}
	if updated == nil {
		return AnswerResultResponseDTO{}, errNotFound
	}
	if updated.Status == "submitted" {
		return AnswerResultResponseDTO{}, errAttemptDone
	}
	return AnswerResultResponseDTO{AttemptID: attemptID, QuestionID: questionID, Saved: true}, nil
}

func (s *Service) SubmitAttempt(ctx context.Context, tenantID, attemptID string, req SubmitAttemptRequestDTO) (AttemptSubmitResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AttemptSubmitResponseDTO{}, err
	}
	if attempt == nil {
		return AttemptSubmitResponseDTO{}, errNotFound
	}
	if attempt.Status == "submitted" {
		return attemptSubmitResult(*attempt), nil
	}
	answers := map[string]AttemptAnswer{}
	for qid, answer := range req.Answers {
		answers[qid] = AttemptAnswer{QuestionID: qid, Answer: answer}
	}
	submittedAt := time.Now().UTC()
	if req.SubmittedAt != nil {
		submittedAt = *req.SubmittedAt
	}
	updated, didSubmit, err := s.repo.SubmitAttempt(ctx, tenantID, attemptID, answers, submittedAt)
	if err != nil {
		return AttemptSubmitResponseDTO{}, err
	}
	if updated == nil {
		return AttemptSubmitResponseDTO{}, errNotFound
	}
	if !didSubmit && updated.Status == "submitted" {
		return attemptSubmitResult(*updated), nil
	}
	scored, err := s.scoreSubmittedAttempt(ctx, tenantID, attemptID, *updated)
	if err != nil {
		return AttemptSubmitResponseDTO{}, err
	}
	s.publishQuizSubmittedAudit(ctx, tenantID, *scored)
	return attemptSubmitResult(*scored), nil
}

func (s *Service) scoreSubmittedAttempt(ctx context.Context, tenantID, attemptID string, attempt Attempt) (*Attempt, error) {
	maxScore := 100.0
	score := float64(len(attempt.Answers))
	if score > maxScore {
		score = maxScore
	}
	percent := score / maxScore * 100
	return s.repo.UpdateAttemptScore(ctx, tenantID, attemptID, score, maxScore, percent, percent >= 50)
}

func attemptSubmitResult(attempt Attempt) AttemptSubmitResponseDTO {
	if attempt.MaxScore > 0 && (attempt.Score > 0 || len(attempt.Answers) == 0) {
		return AttemptSubmitResponseDTO{Score: attempt.Score, MaxScore: attempt.MaxScore, Percent: attempt.Percent, Passed: attempt.Passed}
	}
	maxScore := 100.0
	score := float64(len(attempt.Answers))
	if score > maxScore {
		score = maxScore
	}
	percent := score / maxScore * 100
	return AttemptSubmitResponseDTO{Score: score, MaxScore: maxScore, Percent: percent, Passed: percent >= 50}
}

func (s *Service) SyncAttempt(ctx context.Context, tenantID, attemptID string, req AttemptSyncRequestDTO) (AttemptSyncResponseDTO, error) {
	attempt, err := s.repo.GetAttempt(ctx, tenantID, attemptID)
	if err != nil {
		return AttemptSyncResponseDTO{}, err
	}
	if attempt == nil {
		return AttemptSyncResponseDTO{}, errNotFound
	}
	if attempt.PackageHash != req.PackageHash {
		return AttemptSyncResponseDTO{}, errHashMismatch
	}
	update := bson.M{"quiz_version": req.QuizVersion, "events": req.Events, "client": req.Client}
	if status, ok := req.Attempt["status"].(string); ok && status != "" && attempt.Status != "submitted" {
		update["status"] = status
	}
	updated, err := s.repo.UpdateAttempt(ctx, tenantID, attemptID, update)
	if err != nil {
		return AttemptSyncResponseDTO{}, err
	}
	return AttemptSyncResponseDTO{Status: "synced", ServerAttempt: attemptToDTO(*updated)}, nil
}

func (s *Service) QuizStudentProgress(ctx context.Context, tenantID, quizID, classID, status string) (QuizStudentProgressResponseDTO, error) {
	quizOID, err := objectID(quizID)
	if err != nil {
		return QuizStudentProgressResponseDTO{}, err
	}
	filter := bson.M{"quiz_id": quizOID}
	if classID != "" {
		classOID, err := objectID(classID)
		if err != nil {
			return QuizStudentProgressResponseDTO{}, err
		}
		filter["class_id"] = classOID
	}
	assignments, err := s.repo.ListAssignments(ctx, tenantID, filter)
	if err != nil {
		return QuizStudentProgressResponseDTO{}, err
	}
	items := []QuizStudentProgressItemDTO{}
	for _, assignment := range assignments {
		for _, studentID := range assignment.StudentIDs {
			attempts, _ := s.repo.ListAttempts(ctx, tenantID, bson.M{"assignment_id": assignment.ID, "student_id": studentID})
			item := QuizStudentProgressItemDTO{StudentID: studentID.Hex(), AssignmentID: assignment.ID.Hex(), Status: "not_started"}
			for _, attempt := range attempts {
				item.AttemptCount++
				if attempt.Status == "submitted" {
					item.Status = "submitted"
					if attempt.Score > item.BestScore {
						item.BestScore = attempt.Score
					}
				} else if item.Status == "not_started" {
					item.Status = "in_progress"
				}
			}
			if status == "" || item.Status == status {
				items = append(items, item)
			}
		}
	}
	return QuizStudentProgressResponseDTO{Items: items}, nil
}

func assignmentsToDTO(items []Assignment) []AssignmentResponseDTO {
	out := make([]AssignmentResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, assignmentToDTO(item))
	}
	return out
}

func assignmentToDTO(a Assignment) AssignmentResponseDTO {
	subjectID := ""
	if a.SubjectID != bson.NilObjectID {
		subjectID = a.SubjectID.Hex()
	}
	return AssignmentResponseDTO{ID: a.ID.Hex(), ClassID: a.ClassID.Hex(), QuizID: a.QuizID.Hex(), SubjectID: subjectID, StudentIDs: objectIDsToHex(a.StudentIDs), DueAt: a.DueAt, TeacherNote: a.TeacherNote, Status: a.Status, RecipientMode: a.RecipientMode, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}

func attemptsToDTO(items []Attempt) []AttemptResponseDTO {
	out := make([]AttemptResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, attemptToDTO(item))
	}
	return out
}

func attemptToDTO(a Attempt) AttemptResponseDTO {
	return AttemptResponseDTO{ID: a.ID.Hex(), AssignmentID: a.AssignmentID.Hex(), QuizID: a.QuizID.Hex(), StudentID: a.StudentID.Hex(), PackageID: a.PackageID, PackageHash: a.PackageHash, Status: a.Status, Answers: a.Answers, Score: a.Score, MaxScore: a.MaxScore, Percent: a.Percent, Passed: a.Passed, StartedAt: a.StartedAt, SubmittedAt: a.SubmittedAt, UpdatedAt: a.UpdatedAt}
}

func ensureRuntimeUse(_ fmt.Stringer) {}
