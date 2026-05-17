package dto

import "time"

type StudentSelfResponse struct {
	ID       string   `json:"id"`
	Roles    []string `json:"roles,omitempty"`
	TenantID string   `json:"tenantId"`
}

type StudentDashboardSummary struct {
	OpenAssignments int `json:"openAssignments"`
	Submitted       int `json:"submitted"`
	Unread          int `json:"unread"`
	Discussions     int `json:"discussions"`
}

type StudentDashboardResponse struct {
	Student       StudentSelfResponse       `json:"student"`
	Summary       StudentDashboardSummary   `json:"summary"`
	Assignments   []StudentAssignmentItem   `json:"assignments"`
	Scores        []StudentScoreItem        `json:"scores"`
	Announcements []StudentAnnouncementItem `json:"announcements"`
	Notifications []StudentNotificationItem `json:"notifications"`
	GeneratedAt   time.Time                 `json:"generatedAt"`
}

type StudentAssignmentItem struct {
	ID          string     `json:"id"`
	StudentID   string     `json:"studentId"`
	Title       string     `json:"title"`
	QuizID      string     `json:"quizId,omitempty"`
	Status      string     `json:"status"`
	DueAt       *time.Time `json:"dueAt,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	PackageURL  string     `json:"packageUrl,omitempty"`
	TeacherNote string     `json:"teacherNote,omitempty"`
}

type StudentAssignmentListResponse struct {
	Items []StudentAssignmentItem `json:"items"`
	Total int64                   `json:"total"`
}

type StudentAttemptSummary struct {
	ID           string     `json:"id"`
	AssignmentID string     `json:"assignmentId"`
	StudentID    string     `json:"studentId"`
	Status       string     `json:"status"`
	Score        float64    `json:"score"`
	MaxScore     float64    `json:"maxScore"`
	Percent      float64    `json:"percent"`
	SubmittedAt  *time.Time `json:"submittedAt,omitempty"`
}

type StudentAssignmentDetailResponse struct {
	Assignment StudentAssignmentItem   `json:"assignment"`
	Attempts   []StudentAttemptSummary `json:"attempts"`
}

type StudentScoreItem struct {
	AssignmentID string     `json:"assignmentId"`
	StudentID    string     `json:"studentId"`
	BestScore    float64    `json:"bestScore"`
	MaxScore     float64    `json:"maxScore"`
	Percent      float64    `json:"percent"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
}

type StudentScoreListResponse struct {
	Items []StudentScoreItem `json:"items"`
	Total int64              `json:"total"`
}

type StudentAnnouncementItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
}

type StudentAnnouncementListResponse struct {
	Items []StudentAnnouncementItem `json:"items"`
	Total int64                     `json:"total"`
}

type StudentNotificationItem struct {
	ID        string     `json:"id"`
	StudentID string     `json:"studentId"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Read      bool       `json:"read"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type StudentNotificationListResponse struct {
	Items       []StudentNotificationItem `json:"items"`
	Total       int64                     `json:"total"`
	UnreadCount int64                     `json:"unreadCount"`
}

type StudentNotificationReadResponse struct {
	ID        string    `json:"id"`
	StudentID string    `json:"studentId"`
	Status    string    `json:"status"`
	ReadAt    time.Time `json:"readAt"`
}

type StudentDiscussionThreadItem struct {
	ID               string    `json:"id"`
	StudentID        string    `json:"studentId"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	AssignmentID     string    `json:"assignmentId,omitempty"`
	AuthorID         string    `json:"authorId"`
	ReplyCount       int       `json:"replyCount"`
	LatestActivityAt time.Time `json:"latestActivityAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

type StudentDiscussionListResponse struct {
	Items []StudentDiscussionThreadItem `json:"items"`
	Total int64                         `json:"total"`
}

type CreateStudentDiscussionRequest struct {
	Title        string `json:"title" binding:"required"`
	Content      string `json:"content" binding:"required"`
	AssignmentID string `json:"assignmentId"`
}

type CreateStudentDiscussionReplyRequest struct {
	Content string `json:"content" binding:"required"`
}

type StudentDiscussionReplyResponse struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"threadId"`
	StudentID string    `json:"studentId"`
	Content   string    `json:"content"`
	AuthorID  string    `json:"authorId"`
	CreatedAt time.Time `json:"createdAt"`
}
