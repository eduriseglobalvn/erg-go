// Package service contains the LMS management domain models.
package service

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	centerCollection               = "lms_centers"
	classCollection                = "lms_classes"
	studentCollection              = "lms_students"
	currentScopeCollection         = "lms_current_scopes"
	importPreviewCollection        = "lms_import_previews"
	importJobCollection            = "lms_import_jobs"
	subjectCollection              = "lms_subjects"
	levelCollection                = "lms_levels"
	topicCollection                = "lms_topics"
	questionCollection             = "lms_questions"
	quizCollection                 = "lms_quizzes"
	assignmentCollection           = "lms_assignments"
	attemptCollection              = "lms_attempts"
	discussionThreadCollection     = "lms_discussion_threads"
	discussionReplyCollection      = "lms_discussion_replies"
	discussionAttachmentCollection = "lms_discussion_attachments"
	announcementCollection         = "lms_announcements"
	internalDocumentCollection     = "lms_internal_documents"
	statusActive                   = "active"
	statusArchived                 = "archived"
	educationUnitTypeSystem        = "system"
	educationUnitTypeCenter        = "center"
	educationUnitTypeSchool        = "school"
	scopeLevelGlobal               = "global"
	scopeLevelSystem               = "system"
	scopeLevelCenter               = "center"
	scopeLevelClass                = "class"
	defaultStudentBatchSize        = int64(20)
	maxStudentBatchSize            = int64(100)
	defaultManagementPageSize      = int64(20)
)

type Center struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	TenantID      string        `bson:"tenant_id"`
	Type          string        `bson:"type,omitempty"`
	Name          string        `bson:"name"`
	Code          string        `bson:"code"`
	ParentID      bson.ObjectID `bson:"parent_id,omitempty"`
	AvatarURL     string        `bson:"avatar_url,omitempty"`
	Address       string        `bson:"address,omitempty"`
	Description   string        `bson:"description,omitempty"`
	Phone         string        `bson:"phone,omitempty"`
	Email         string        `bson:"email,omitempty"`
	Website       string        `bson:"website,omitempty"`
	Status        string        `bson:"status"`
	ManagerUserID string        `bson:"manager_user_id,omitempty"`
	CreatedAt     time.Time     `bson:"created_at"`
	UpdatedAt     time.Time     `bson:"updated_at"`
}

type Class struct {
	ID                bson.ObjectID `bson:"_id,omitempty"`
	TenantID          string        `bson:"tenant_id"`
	CenterID          bson.ObjectID `bson:"center_id"`
	Name              string        `bson:"name"`
	Grade             string        `bson:"grade"`
	AcademicYear      string        `bson:"academic_year,omitempty"`
	Status            string        `bson:"status"`
	HomeroomTeacherID string        `bson:"homeroom_teacher_id,omitempty"`
	CreatedAt         time.Time     `bson:"created_at"`
	UpdatedAt         time.Time     `bson:"updated_at"`
}

type Student struct {
	ID                 bson.ObjectID  `bson:"_id,omitempty"`
	TenantID           string         `bson:"tenant_id"`
	CenterID           bson.ObjectID  `bson:"center_id"`
	ClassID            bson.ObjectID  `bson:"class_id"`
	StudentCode        string         `bson:"student_code,omitempty"`
	FullName           string         `bson:"full_name"`
	Username           string         `bson:"username"`
	AuthUserID         string         `bson:"auth_user_id,omitempty"`
	Email              string         `bson:"email,omitempty"`
	Gender             string         `bson:"gender,omitempty"`
	Birthday           *time.Time     `bson:"birthday,omitempty"`
	Phone              string         `bson:"phone,omitempty"`
	Address            string         `bson:"address,omitempty"`
	ParentName         string         `bson:"parent_name,omitempty"`
	ParentPhone        string         `bson:"parent_phone,omitempty"`
	ParentEmail        string         `bson:"parent_email,omitempty"`
	ParentRelationship string         `bson:"parent_relationship,omitempty"`
	EnrollmentDate     *time.Time     `bson:"enrollment_date,omitempty"`
	Note               string         `bson:"note,omitempty"`
	Status             string         `bson:"status"`
	Metrics            StudentMetrics `bson:"metrics"`
	CreatedAt          time.Time      `bson:"created_at"`
	UpdatedAt          time.Time      `bson:"updated_at"`
}

type StudentMetrics struct {
	AverageScore         *float64   `bson:"average_score,omitempty"`
	CompletedAssignments int        `bson:"completed_assignments"`
	LastActivityAt       *time.Time `bson:"last_activity_at,omitempty"`
}

type CurrentScope struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TenantID  string        `bson:"tenant_id"`
	UserID    string        `bson:"user_id"`
	Level     string        `bson:"level"`
	CenterID  string        `bson:"center_id,omitempty"`
	ClassID   string        `bson:"class_id,omitempty"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type ImportPreview struct {
	ID            bson.ObjectID      `bson:"_id,omitempty"`
	TenantID      string             `bson:"tenant_id"`
	UserID        string             `bson:"user_id"`
	SpreadsheetID string             `bson:"spreadsheet_id"`
	SheetName     string             `bson:"sheet_name"`
	Range         string             `bson:"range"`
	Rows          []ParsedStudentRow `bson:"rows"`
	CreatedAt     time.Time          `bson:"created_at"`
}

type ParsedStudentRow struct {
	RowID              string     `bson:"row_id"`
	RowNumber          int        `bson:"row_number"`
	StudentCode        string     `bson:"student_code,omitempty"`
	FullName           string     `bson:"full_name"`
	ClassName          string     `bson:"class_name"`
	Email              string     `bson:"email,omitempty"`
	Gender             string     `bson:"gender,omitempty"`
	Birthday           *time.Time `bson:"birthday,omitempty"`
	Phone              string     `bson:"phone,omitempty"`
	Address            string     `bson:"address,omitempty"`
	ParentName         string     `bson:"parent_name,omitempty"`
	ParentPhone        string     `bson:"parent_phone,omitempty"`
	ParentEmail        string     `bson:"parent_email,omitempty"`
	ParentRelationship string     `bson:"parent_relationship,omitempty"`
	EnrollmentDate     *time.Time `bson:"enrollment_date,omitempty"`
	Note               string     `bson:"note,omitempty"`
	Status             string     `bson:"status"`
	Messages           []string   `bson:"messages"`
}

type ImportJob struct {
	ID            bson.ObjectID      `bson:"_id,omitempty"`
	TenantID      string             `bson:"tenant_id"`
	UserID        string             `bson:"user_id"`
	PreviewID     bson.ObjectID      `bson:"preview_id"`
	SpreadsheetID string             `bson:"spreadsheet_id"`
	SheetName     string             `bson:"sheet_name"`
	SheetRange    string             `bson:"sheet_range"`
	Status        string             `bson:"status"`
	Progress      int                `bson:"progress"`
	Created       int                `bson:"created"`
	Skipped       int                `bson:"skipped"`
	Duplicates    int                `bson:"duplicates"`
	Credentials   []ImportCredential `bson:"credentials"`
	Errors        []string           `bson:"errors"`
	CreatedAt     time.Time          `bson:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at"`
}

type ImportCredential struct {
	RowID     string `bson:"row_id"`
	RowNumber int    `bson:"row_number"`
	StudentID string `bson:"student_id"`
	Username  string `bson:"username"`
	Password  string `bson:"password"`
}

type ContentScope struct {
	Type     string `bson:"type" json:"type"`
	CenterID string `bson:"center_id,omitempty" json:"centerId,omitempty"`
}

type Subject struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TenantID  string        `bson:"tenant_id"`
	Scope     ContentScope  `bson:"scope"`
	Name      string        `bson:"name"`
	Code      string        `bson:"code"`
	Status    string        `bson:"status"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type Level struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TenantID  string        `bson:"tenant_id"`
	SubjectID bson.ObjectID `bson:"subject_id"`
	Name      string        `bson:"name"`
	Code      string        `bson:"code"`
	Order     int           `bson:"order"`
	Status    string        `bson:"status"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type Topic struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TenantID  string        `bson:"tenant_id"`
	LevelID   bson.ObjectID `bson:"level_id"`
	Name      string        `bson:"name"`
	Code      string        `bson:"code"`
	Order     int           `bson:"order"`
	Status    string        `bson:"status"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type QuestionChoice struct {
	ID      string `bson:"id" json:"id"`
	Label   string `bson:"label" json:"label"`
	Correct bool   `bson:"correct" json:"correct"`
}

type Question struct {
	ID         bson.ObjectID    `bson:"_id,omitempty"`
	TenantID   string           `bson:"tenant_id"`
	Scope      ContentScope     `bson:"scope"`
	SubjectID  bson.ObjectID    `bson:"subject_id"`
	LevelID    bson.ObjectID    `bson:"level_id"`
	TopicID    bson.ObjectID    `bson:"topic_id,omitempty"`
	Type       string           `bson:"type"`
	Stem       string           `bson:"stem"`
	Choices    []QuestionChoice `bson:"choices,omitempty"`
	Answer     any              `bson:"answer,omitempty"`
	Metadata   map[string]any   `bson:"metadata,omitempty"`
	Status     string           `bson:"status"`
	ArchivedAt *time.Time       `bson:"archived_at,omitempty"`
	CreatedAt  time.Time        `bson:"created_at"`
	UpdatedAt  time.Time        `bson:"updated_at"`
}

type Quiz struct {
	ID          bson.ObjectID   `bson:"_id,omitempty"`
	TenantID    string          `bson:"tenant_id"`
	Scope       ContentScope    `bson:"scope"`
	Title       string          `bson:"title"`
	Kind        string          `bson:"kind"`
	SubjectID   bson.ObjectID   `bson:"subject_id"`
	LevelID     bson.ObjectID   `bson:"level_id"`
	TopicIDs    []bson.ObjectID `bson:"topic_ids"`
	QuestionIDs []bson.ObjectID `bson:"question_ids"`
	Slides      []any           `bson:"slides,omitempty"`
	Settings    map[string]any  `bson:"settings,omitempty"`
	Result      map[string]any  `bson:"result,omitempty"`
	Theme       map[string]any  `bson:"theme,omitempty"`
	ThemeID     string          `bson:"theme_id,omitempty"`
	Status      string          `bson:"status"`
	Version     int             `bson:"version"`
	PackageHash string          `bson:"package_hash,omitempty"`
	PublishedAt *time.Time      `bson:"published_at,omitempty"`
	CreatedAt   time.Time       `bson:"created_at"`
	UpdatedAt   time.Time       `bson:"updated_at"`
}

type Assignment struct {
	ID            bson.ObjectID   `bson:"_id,omitempty"`
	TenantID      string          `bson:"tenant_id"`
	ClassID       bson.ObjectID   `bson:"class_id"`
	QuizID        bson.ObjectID   `bson:"quiz_id"`
	SubjectID     bson.ObjectID   `bson:"subject_id,omitempty"`
	StudentIDs    []bson.ObjectID `bson:"student_ids"`
	DueAt         *time.Time      `bson:"due_at,omitempty"`
	TeacherNote   string          `bson:"teacher_note,omitempty"`
	Status        string          `bson:"status"`
	AssignedBy    string          `bson:"assigned_by"`
	RecipientMode string          `bson:"recipient_mode"`
	CreatedAt     time.Time       `bson:"created_at"`
	UpdatedAt     time.Time       `bson:"updated_at"`
}

type AttemptAnswer struct {
	QuestionID   string         `bson:"question_id"`
	Answer       any            `bson:"answer"`
	ClientResult map[string]any `bson:"client_result,omitempty"`
	AnsweredAt   *time.Time     `bson:"answered_at,omitempty"`
}

type Attempt struct {
	ID           bson.ObjectID            `bson:"_id,omitempty"`
	TenantID     string                   `bson:"tenant_id"`
	AssignmentID bson.ObjectID            `bson:"assignment_id"`
	QuizID       bson.ObjectID            `bson:"quiz_id"`
	StudentID    bson.ObjectID            `bson:"student_id"`
	PackageID    string                   `bson:"package_id"`
	PackageHash  string                   `bson:"package_hash"`
	QuizVersion  string                   `bson:"quiz_version,omitempty"`
	Status       string                   `bson:"status"`
	Answers      map[string]AttemptAnswer `bson:"answers"`
	Events       []map[string]any         `bson:"events,omitempty"`
	Client       map[string]any           `bson:"client,omitempty"`
	Score        float64                  `bson:"score"`
	MaxScore     float64                  `bson:"max_score"`
	Percent      float64                  `bson:"percent"`
	Passed       bool                     `bson:"passed"`
	StartedAt    time.Time                `bson:"started_at"`
	SubmittedAt  *time.Time               `bson:"submitted_at,omitempty"`
	UpdatedAt    time.Time                `bson:"updated_at"`
}

type DiscussionThread struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	TenantID         string        `bson:"tenant_id"`
	ClassID          bson.ObjectID `bson:"class_id"`
	Title            string        `bson:"title"`
	Content          string        `bson:"content"`
	AssignmentID     string        `bson:"assignment_id,omitempty"`
	AttachmentIDs    []string      `bson:"attachment_ids,omitempty"`
	AuthorID         string        `bson:"author_id"`
	ReplyCount       int           `bson:"reply_count"`
	LatestActivityAt time.Time     `bson:"latest_activity_at"`
	CreatedAt        time.Time     `bson:"created_at"`
	UpdatedAt        time.Time     `bson:"updated_at"`
}

type DiscussionReply struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	TenantID      string        `bson:"tenant_id"`
	ThreadID      bson.ObjectID `bson:"thread_id"`
	ClassID       bson.ObjectID `bson:"class_id"`
	Content       string        `bson:"content"`
	AttachmentIDs []string      `bson:"attachment_ids,omitempty"`
	AuthorID      string        `bson:"author_id"`
	CreatedAt     time.Time     `bson:"created_at"`
}

type DiscussionAttachment struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TenantID  string        `bson:"tenant_id"`
	URL       string        `bson:"url"`
	Mime      string        `bson:"mime"`
	Size      int64         `bson:"size"`
	OwnerID   string        `bson:"owner_id"`
	CreatedAt time.Time     `bson:"created_at"`
}

type Announcement struct {
	ID         bson.ObjectID   `bson:"_id,omitempty"`
	TenantID   string          `bson:"tenant_id"`
	TargetType string          `bson:"target_type"`
	ClassIDs   []bson.ObjectID `bson:"class_ids,omitempty"`
	StudentIDs []bson.ObjectID `bson:"student_ids,omitempty"`
	Title      string          `bson:"title"`
	Content    string          `bson:"content"`
	Pinned     bool            `bson:"pinned"`
	AuthorID   string          `bson:"author_id"`
	CreatedAt  time.Time       `bson:"created_at"`
	UpdatedAt  time.Time       `bson:"updated_at"`
}

type InternalDocument struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TenantID  string        `bson:"tenant_id"`
	Type      string        `bson:"type"`
	Title     string        `bson:"title"`
	SubjectID string        `bson:"subject_id,omitempty"`
	FileID    string        `bson:"file_id,omitempty"`
	Content   string        `bson:"content,omitempty"`
	AuthorID  string        `bson:"author_id"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}
