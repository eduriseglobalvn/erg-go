package service

import (
	"strings"
	"time"

	"erg.ninja/internal/modules/elearning/api/dto"
)

type studentContractSeed struct {
	assignments   []dto.StudentAssignmentItem
	attempts      map[string][]dto.StudentAttemptSummary
	scores        []dto.StudentScoreItem
	announcements []dto.StudentAnnouncementItem
	notifications []dto.StudentNotificationItem
	discussions   []dto.StudentDiscussionThreadItem
}

func studentContractSeedFor(actor StudentActor, now time.Time) studentContractSeed {
	studentID := strings.TrimSpace(actor.UserID)
	if studentID == "" {
		studentID = "student-seed"
	}
	openDueAt := now.Add(48 * time.Hour)
	submittedDueAt := now.Add(-24 * time.Hour)
	submittedAt := now.Add(-2 * time.Hour)
	updatedAt := now.Add(-90 * time.Minute)
	readAt := now.Add(-30 * time.Minute)

	assignments := []dto.StudentAssignmentItem{
		{
			ID:          "el-seed-assignment-open",
			StudentID:   studentID,
			Title:       "IC3 GS6 - Digital devices practice",
			QuizID:      "el-seed-quiz-devices",
			Status:      "open",
			DueAt:       &openDueAt,
			UpdatedAt:   now.Add(-45 * time.Minute),
			PackageURL:  "/api/lms/quizzes/el-seed-quiz-devices/package",
			TeacherNote: "Complete the warm-up quiz before the next live session.",
		},
		{
			ID:          "el-seed-assignment-submitted",
			StudentID:   studentID,
			Title:       "MOS Excel - Cell formulas checkpoint",
			QuizID:      "el-seed-quiz-excel",
			Status:      "submitted",
			DueAt:       &submittedDueAt,
			UpdatedAt:   updatedAt,
			PackageURL:  "/api/lms/quizzes/el-seed-quiz-excel/package",
			TeacherNote: "Review feedback and retry the practice file if needed.",
		},
	}

	attempts := map[string][]dto.StudentAttemptSummary{
		"el-seed-assignment-submitted": {
			{
				ID:           "el-seed-attempt-submitted",
				AssignmentID: "el-seed-assignment-submitted",
				StudentID:    studentID,
				Status:       "submitted",
				Score:        88,
				MaxScore:     100,
				Percent:      88,
				SubmittedAt:  &submittedAt,
			},
		},
	}

	return studentContractSeed{
		assignments: assignments,
		attempts:    attempts,
		scores: []dto.StudentScoreItem{
			{
				AssignmentID: "el-seed-assignment-submitted",
				StudentID:    studentID,
				BestScore:    88,
				MaxScore:     100,
				Percent:      88,
				UpdatedAt:    &updatedAt,
			},
		},
		announcements: []dto.StudentAnnouncementItem{
			{
				ID:        "el-seed-announcement-live-class",
				Title:     "IC3 live class schedule",
				Content:   "Join the digital literacy review room 10 minutes early.",
				Pinned:    true,
				CreatedAt: now.Add(-3 * time.Hour),
			},
		},
		notifications: []dto.StudentNotificationItem{
			{
				ID:        "el-seed-notification-due",
				StudentID: studentID,
				Title:     "Assignment due soon",
				Body:      "Digital devices practice is due in 48 hours.",
				Read:      false,
				CreatedAt: now.Add(-20 * time.Minute),
			},
			{
				ID:        "el-seed-notification-score",
				StudentID: studentID,
				Title:     "Score posted",
				Body:      "Your Excel checkpoint score is available.",
				Read:      true,
				ReadAt:    &readAt,
				CreatedAt: now.Add(-70 * time.Minute),
			},
		},
		discussions: []dto.StudentDiscussionThreadItem{
			{
				ID:               "el-seed-discussion-devices",
				StudentID:        studentID,
				Title:            "How do I identify input devices?",
				Content:          "Seed thread for the student discussion panel.",
				AssignmentID:     "el-seed-assignment-open",
				AuthorID:         studentID,
				ReplyCount:       1,
				LatestActivityAt: now.Add(-10 * time.Minute),
				CreatedAt:        now.Add(-25 * time.Minute),
			},
		},
	}
}

func studentDashboardSummary(seed studentContractSeed) dto.StudentDashboardSummary {
	summary := dto.StudentDashboardSummary{Discussions: len(seed.discussions)}
	for _, assignment := range seed.assignments {
		switch strings.ToLower(strings.TrimSpace(assignment.Status)) {
		case "submitted", "completed":
			summary.Submitted++
		default:
			summary.OpenAssignments++
		}
	}
	for _, notification := range seed.notifications {
		if !notification.Read {
			summary.Unread++
		}
	}
	return summary
}

func filterStudentAssignments(items []dto.StudentAssignmentItem, status string) []dto.StudentAssignmentItem {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return append([]dto.StudentAssignmentItem(nil), items...)
	}
	out := make([]dto.StudentAssignmentItem, 0, len(items))
	for _, item := range items {
		if strings.ToLower(item.Status) == status {
			out = append(out, item)
		}
	}
	return out
}

func findStudentAssignment(items []dto.StudentAssignmentItem, assignmentID string) (dto.StudentAssignmentItem, bool) {
	for _, item := range items {
		if item.ID == assignmentID {
			return item, true
		}
	}
	return dto.StudentAssignmentItem{}, false
}
