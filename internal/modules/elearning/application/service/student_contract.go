package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"erg.ninja/internal/modules/elearning/api/dto"
)

var ErrStudentContractNotFound = errors.New("ELEARNING_STUDENT_CONTRACT_NOT_FOUND")

type StudentActor struct {
	UserID string
	Roles  []string
}

func (s *Service) StudentDashboard(ctx context.Context, tenantID string, actor StudentActor) (dto.StudentDashboardResponse, error) {
	now := time.Now().UTC()
	seed := studentContractSeedFor(actor, now)
	return dto.StudentDashboardResponse{
		Student:       studentSelf(tenantID, actor),
		Summary:       studentDashboardSummary(seed),
		Assignments:   seed.assignments,
		Scores:        seed.scores,
		Announcements: seed.announcements,
		Notifications: seed.notifications,
		GeneratedAt:   now,
	}, nil
}

func (s *Service) StudentAssignments(ctx context.Context, tenantID string, actor StudentActor, status string) (dto.StudentAssignmentListResponse, error) {
	seed := studentContractSeedFor(actor, time.Now().UTC())
	items := filterStudentAssignments(seed.assignments, status)
	return dto.StudentAssignmentListResponse{Items: items, Total: int64(len(items))}, nil
}

func (s *Service) StudentAssignmentDetail(ctx context.Context, tenantID string, actor StudentActor, assignmentID string) (dto.StudentAssignmentDetailResponse, error) {
	assignmentID = strings.TrimSpace(assignmentID)
	if assignmentID == "" {
		return dto.StudentAssignmentDetailResponse{}, ErrStudentContractNotFound
	}
	now := time.Now().UTC()
	seed := studentContractSeedFor(actor, now)
	assignment, ok := findStudentAssignment(seed.assignments, assignmentID)
	if !ok {
		return dto.StudentAssignmentDetailResponse{}, ErrStudentContractNotFound
	}
	return dto.StudentAssignmentDetailResponse{
		Assignment: assignment,
		Attempts:   append([]dto.StudentAttemptSummary(nil), seed.attempts[assignmentID]...),
	}, nil
}

func (s *Service) StudentScores(ctx context.Context, tenantID string, actor StudentActor, subjectID string) (dto.StudentScoreListResponse, error) {
	seed := studentContractSeedFor(actor, time.Now().UTC())
	return dto.StudentScoreListResponse{Items: seed.scores, Total: int64(len(seed.scores))}, nil
}

func (s *Service) StudentAnnouncements(ctx context.Context, tenantID string, actor StudentActor) (dto.StudentAnnouncementListResponse, error) {
	seed := studentContractSeedFor(actor, time.Now().UTC())
	return dto.StudentAnnouncementListResponse{Items: seed.announcements, Total: int64(len(seed.announcements))}, nil
}

func (s *Service) StudentNotifications(ctx context.Context, tenantID string, actor StudentActor) (dto.StudentNotificationListResponse, error) {
	seed := studentContractSeedFor(actor, time.Now().UTC())
	unread := int64(0)
	for _, notification := range seed.notifications {
		if !notification.Read {
			unread++
		}
	}
	return dto.StudentNotificationListResponse{Items: seed.notifications, Total: int64(len(seed.notifications)), UnreadCount: unread}, nil
}

func (s *Service) MarkStudentNotificationRead(ctx context.Context, tenantID string, actor StudentActor, notificationID string) (dto.StudentNotificationReadResponse, error) {
	notificationID = strings.TrimSpace(notificationID)
	if notificationID == "" {
		return dto.StudentNotificationReadResponse{}, ErrStudentContractNotFound
	}
	now := time.Now().UTC()
	return dto.StudentNotificationReadResponse{ID: notificationID, StudentID: actor.UserID, Status: "read", ReadAt: now}, nil
}

func (s *Service) StudentDiscussions(ctx context.Context, tenantID string, actor StudentActor) (dto.StudentDiscussionListResponse, error) {
	seed := studentContractSeedFor(actor, time.Now().UTC())
	return dto.StudentDiscussionListResponse{Items: seed.discussions, Total: int64(len(seed.discussions))}, nil
}

func (s *Service) CreateStudentDiscussion(ctx context.Context, tenantID string, actor StudentActor, req dto.CreateStudentDiscussionRequest) (dto.StudentDiscussionThreadItem, error) {
	now := time.Now().UTC()
	id := fmt.Sprintf("thread-%d", now.UnixNano())
	return dto.StudentDiscussionThreadItem{
		ID:               id,
		StudentID:        actor.UserID,
		Title:            req.Title,
		Content:          req.Content,
		AssignmentID:     req.AssignmentID,
		AuthorID:         actor.UserID,
		LatestActivityAt: now,
		CreatedAt:        now,
	}, nil
}

func (s *Service) CreateStudentDiscussionReply(ctx context.Context, tenantID string, actor StudentActor, threadID string, req dto.CreateStudentDiscussionReplyRequest) (dto.StudentDiscussionReplyResponse, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return dto.StudentDiscussionReplyResponse{}, ErrStudentContractNotFound
	}
	now := time.Now().UTC()
	return dto.StudentDiscussionReplyResponse{
		ID:        fmt.Sprintf("reply-%d", now.UnixNano()),
		ThreadID:  threadID,
		StudentID: actor.UserID,
		Content:   req.Content,
		AuthorID:  actor.UserID,
		CreatedAt: now,
	}, nil
}

func studentSelf(tenantID string, actor StudentActor) dto.StudentSelfResponse {
	return dto.StudentSelfResponse{ID: actor.UserID, Roles: actor.Roles, TenantID: tenantID}
}
