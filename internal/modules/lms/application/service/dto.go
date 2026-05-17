package service

import "time"

type ManagementScopeDTO struct {
	Level       string `json:"level"`
	Type        string `json:"type,omitempty"`
	UnitType    string `json:"unitType,omitempty"`
	CenterID    string `json:"centerId,omitempty"`
	CenterName  string `json:"centerName,omitempty"`
	ClassID     string `json:"classId,omitempty"`
	ClassName   string `json:"className,omitempty"`
	Badge       string `json:"badge,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

type ManagementScopeResponseDTO struct {
	CanAccessGlobalErg bool                 `json:"canAccessGlobalErg"`
	AssignedCenters    []CenterResponseDTO  `json:"assignedCenters"`
	AssignedClasses    []ClassResponseDTO   `json:"assignedClasses"`
	CurrentScope       ManagementScopeDTO   `json:"currentScope"`
	AvailableScopes    []ManagementScopeDTO `json:"availableScopes"`
}

type UpdateCurrentScopeRequestDTO struct {
	Level    string `json:"level" binding:"required"`
	CenterID string `json:"centerId"`
	ClassID  string `json:"classId"`
}

type CurrentScopeResponseDTO struct {
	CurrentScope ManagementScopeDTO `json:"currentScope"`
}

type CenterListRequestDTO struct {
	Keyword string
	Status  string
	Type    string
	Page    int64
	Limit   int64
}

type CreateCenterRequestDTO struct {
	Type          string `json:"type"`
	Name          string `json:"name" binding:"required"`
	Code          string `json:"code" binding:"required"`
	ParentID      string `json:"parentId,omitempty"`
	AvatarURL     string `json:"avatarUrl"`
	Address       string `json:"address"`
	Description   string `json:"description"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Website       string `json:"website"`
	ManagerUserID string `json:"managerUserId"`
}

type CreateEducationUnitRequestDTO struct {
	Type          string `json:"type" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Code          string `json:"code" binding:"required"`
	ParentID      string `json:"parentId,omitempty"`
	AvatarURL     string `json:"avatarUrl"`
	Address       string `json:"address"`
	Description   string `json:"description"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Website       string `json:"website"`
	ManagerUserID string `json:"managerUserId"`
}

type UpdateCenterRequestDTO struct {
	Type          *string `json:"type"`
	Name          *string `json:"name"`
	ParentID      *string `json:"parentId"`
	AvatarURL     *string `json:"avatarUrl"`
	Address       *string `json:"address"`
	Description   *string `json:"description"`
	Phone         *string `json:"phone"`
	Email         *string `json:"email"`
	Website       *string `json:"website"`
	Status        *string `json:"status"`
	ManagerUserID *string `json:"managerUserId"`
}

type UpdateEducationUnitRequestDTO = UpdateCenterRequestDTO

type CenterResponseDTO struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	ParentID      string    `json:"parentId,omitempty"`
	AvatarURL     string    `json:"avatarUrl,omitempty"`
	Address       string    `json:"address,omitempty"`
	Description   string    `json:"description,omitempty"`
	Phone         string    `json:"phone,omitempty"`
	Email         string    `json:"email,omitempty"`
	Website       string    `json:"website,omitempty"`
	Status        string    `json:"status"`
	ManagerUserID string    `json:"managerUserId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CenterListResponseDTO struct {
	Items []CenterResponseDTO `json:"items"`
	Total int64               `json:"total"`
}

type EducationUnitResponseDTO = CenterResponseDTO
type EducationUnitListResponseDTO = CenterListResponseDTO

type ClassListRequestDTO struct {
	CenterID     string
	UnitID       string
	Grade        string
	Keyword      string
	Status       string
	AcademicYear string
	Page         int64
	Limit        int64
}

type CreateClassRequestDTO struct {
	CenterID          string `json:"centerId" binding:"required"`
	Name              string `json:"name" binding:"required"`
	Grade             string `json:"grade" binding:"required"`
	AcademicYear      string `json:"academicYear"`
	HomeroomTeacherID string `json:"homeroomTeacherId"`
}

type UpdateClassRequestDTO struct {
	Name              *string `json:"name"`
	Grade             *string `json:"grade"`
	AcademicYear      *string `json:"academicYear"`
	Status            *string `json:"status"`
	HomeroomTeacherID *string `json:"homeroomTeacherId"`
}

type ClassResponseDTO struct {
	ID                string    `json:"id"`
	CenterID          string    `json:"centerId"`
	UnitID            string    `json:"unitId,omitempty"`
	CenterName        string    `json:"centerName,omitempty"`
	Name              string    `json:"name"`
	Grade             string    `json:"grade"`
	AcademicYear      string    `json:"academicYear,omitempty"`
	Status            string    `json:"status"`
	HomeroomTeacherID string    `json:"homeroomTeacherId,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type ClassListResponseDTO struct {
	Items []ClassResponseDTO `json:"items"`
	Total int64              `json:"total"`
}

type StudentListRequestDTO struct {
	CenterID  string
	ClassID   string
	Keyword   string
	Status    string
	Progress  string
	SubjectID string
	Cursor    string
	Limit     int64
}

type CreateStudentRequestDTO struct {
	FullName           string     `json:"fullName" binding:"required"`
	ClassID            string     `json:"classId" binding:"required"`
	StudentCode        string     `json:"studentCode"`
	Username           string     `json:"username"`
	Password           string     `json:"password"`
	Email              string     `json:"email"`
	Gender             string     `json:"gender"`
	Birthday           *time.Time `json:"birthday"`
	Phone              string     `json:"phone"`
	Address            string     `json:"address"`
	ParentName         string     `json:"parentName"`
	ParentPhone        string     `json:"parentPhone"`
	ParentEmail        string     `json:"parentEmail"`
	ParentRelationship string     `json:"parentRelationship"`
	EnrollmentDate     *time.Time `json:"enrollmentDate"`
	Note               string     `json:"note"`
}

type UpdateStudentRequestDTO struct {
	FullName           *string    `json:"fullName"`
	ClassID            *string    `json:"classId"`
	StudentCode        *string    `json:"studentCode"`
	Email              *string    `json:"email"`
	Gender             *string    `json:"gender"`
	Birthday           *time.Time `json:"birthday"`
	Phone              *string    `json:"phone"`
	Address            *string    `json:"address"`
	ParentName         *string    `json:"parentName"`
	ParentPhone        *string    `json:"parentPhone"`
	ParentEmail        *string    `json:"parentEmail"`
	ParentRelationship *string    `json:"parentRelationship"`
	EnrollmentDate     *time.Time `json:"enrollmentDate"`
	Note               *string    `json:"note"`
	Status             *string    `json:"status"`
}

type StudentListItemDTO struct {
	ID                   string     `json:"id"`
	StudentCode          string     `json:"studentCode,omitempty"`
	FullName             string     `json:"fullName"`
	Username             string     `json:"username"`
	Email                string     `json:"email,omitempty"`
	Gender               string     `json:"gender,omitempty"`
	Birthday             *time.Time `json:"birthday,omitempty"`
	Phone                string     `json:"phone,omitempty"`
	Address              string     `json:"address,omitempty"`
	ParentName           string     `json:"parentName,omitempty"`
	ParentPhone          string     `json:"parentPhone,omitempty"`
	ParentEmail          string     `json:"parentEmail,omitempty"`
	ParentRelationship   string     `json:"parentRelationship,omitempty"`
	EnrollmentDate       *time.Time `json:"enrollmentDate,omitempty"`
	Note                 string     `json:"note,omitempty"`
	CenterID             string     `json:"centerId"`
	CenterName           string     `json:"centerName"`
	CenterCode           string     `json:"centerCode,omitempty"`
	CenterType           string     `json:"centerType,omitempty"`
	ClassID              string     `json:"classId"`
	ClassName            string     `json:"className"`
	Grade                string     `json:"grade,omitempty"`
	AcademicYear         string     `json:"academicYear,omitempty"`
	Status               string     `json:"status"`
	AverageScore         *float64   `json:"averageScore,omitempty"`
	CompletedAssignments int        `json:"completedAssignments"`
	LastActivityAt       *time.Time `json:"lastActivityAt,omitempty"`
}

type StudentListResponseDTO struct {
	Items      []StudentListItemDTO `json:"items"`
	NextCursor string               `json:"nextCursor,omitempty"`
	Total      int64                `json:"total"`
}

type StudentResponseDTO struct {
	Student      StudentListItemDTO `json:"student"`
	TempPassword string             `json:"tempPassword,omitempty"`
}

type StudentDetailResponseDTO struct {
	Profile     StudentListItemDTO `json:"profile"`
	Classes     []ClassResponseDTO `json:"classes"`
	Metrics     StudentMetrics     `json:"metrics"`
	Assignments []any              `json:"assignments"`
	Journey     []any              `json:"journey"`
}

type BulkMoveStudentsRequestDTO struct {
	StudentIDs    []string `json:"studentIds" binding:"required"`
	TargetClassID string   `json:"targetClassId" binding:"required"`
}

type BulkActionFailedItemDTO struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BulkActionResponseDTO struct {
	SuccessCount int                       `json:"successCount"`
	FailedItems  []BulkActionFailedItemDTO `json:"failedItems"`
}

type BulkCreateStudentAccountRowDTO struct {
	RowID              string     `json:"rowId"`
	RowNumber          int        `json:"rowNumber"`
	Included           *bool      `json:"included,omitempty"`
	StudentCode        string     `json:"studentCode"`
	FullName           string     `json:"fullName" binding:"required"`
	ClassID            string     `json:"classId"`
	ClassName          string     `json:"className"`
	Username           string     `json:"username" binding:"required"`
	Password           string     `json:"password"`
	Email              string     `json:"email"`
	Gender             string     `json:"gender"`
	Birthday           *time.Time `json:"birthday"`
	Phone              string     `json:"phone"`
	Address            string     `json:"address"`
	ParentName         string     `json:"parentName"`
	ParentPhone        string     `json:"parentPhone"`
	ParentEmail        string     `json:"parentEmail"`
	ParentRelationship string     `json:"parentRelationship"`
	EnrollmentDate     *time.Time `json:"enrollmentDate"`
	Note               string     `json:"note"`
}

type BulkCreateStudentAccountsRequestDTO struct {
	CenterID string                           `json:"centerId" binding:"required"`
	ClassID  string                           `json:"classId"`
	Rows     []BulkCreateStudentAccountRowDTO `json:"rows" binding:"required"`
}

type BulkCreateStudentAccountsResponseDTO struct {
	Created     int                       `json:"created"`
	Skipped     int                       `json:"skipped"`
	Duplicates  int                       `json:"duplicates"`
	Credentials []ImportCredentialDTO     `json:"credentials"`
	Students    []StudentListItemDTO      `json:"students"`
	FailedItems []BulkActionFailedItemDTO `json:"failedItems"`
}

type GoogleSheetTabsRequestDTO struct {
	SheetURL string `json:"sheetUrl" binding:"required"`
}

type GoogleSheetTabDTO struct {
	SheetID int64  `json:"sheetId"`
	Title   string `json:"title"`
	Index   int64  `json:"index"`
}

type GoogleSheetTabsResponseDTO struct {
	SpreadsheetID string              `json:"spreadsheetId"`
	Tabs          []GoogleSheetTabDTO `json:"tabs"`
}

type GoogleSheetPreviewMappingDTO struct {
	RowNumber          string `json:"rowNumber"`
	FamilyName         string `json:"familyName"`
	GivenName          string `json:"givenName"`
	StudentCode        string `json:"studentCode"`
	FullName           string `json:"fullName"`
	ClassName          string `json:"className"`
	Email              string `json:"email"`
	Gender             string `json:"gender"`
	Birthday           string `json:"birthday"`
	Phone              string `json:"phone"`
	Address            string `json:"address"`
	ParentName         string `json:"parentName"`
	ParentPhone        string `json:"parentPhone"`
	ParentEmail        string `json:"parentEmail"`
	ParentRelationship string `json:"parentRelationship"`
	EnrollmentDate     string `json:"enrollmentDate"`
	Note               string `json:"note"`
}

type GoogleSheetPreviewRequestDTO struct {
	SheetURL       string                       `json:"sheetUrl" binding:"required"`
	SheetName      string                       `json:"sheetName" binding:"required"`
	Range          string                       `json:"range" binding:"required"`
	Mapping        GoogleSheetPreviewMappingDTO `json:"mapping"`
	UsernameColumn string                       `json:"usernameColumn"`
	PasswordColumn string                       `json:"passwordColumn"`
}

type ParsedStudentRowDTO struct {
	RowID              string     `json:"rowId"`
	RowNumber          int        `json:"rowNumber"`
	StudentCode        string     `json:"studentCode,omitempty"`
	FullName           string     `json:"fullName"`
	ClassName          string     `json:"className"`
	Email              string     `json:"email,omitempty"`
	Gender             string     `json:"gender,omitempty"`
	Birthday           *time.Time `json:"birthday,omitempty"`
	Phone              string     `json:"phone,omitempty"`
	Address            string     `json:"address,omitempty"`
	ParentName         string     `json:"parentName,omitempty"`
	ParentPhone        string     `json:"parentPhone,omitempty"`
	ParentEmail        string     `json:"parentEmail,omitempty"`
	ParentRelationship string     `json:"parentRelationship,omitempty"`
	EnrollmentDate     *time.Time `json:"enrollmentDate,omitempty"`
	Note               string     `json:"note,omitempty"`
	Status             string     `json:"status"`
	Messages           []string   `json:"messages"`
}

type GoogleSheetPreviewSummaryDTO struct {
	TotalRows    int `json:"totalRows"`
	ValidRows    int `json:"validRows"`
	WarningRows  int `json:"warningRows"`
	ErrorRows    int `json:"errorRows"`
	IncludedRows int `json:"includedRows"`
}

type GoogleSheetPreviewResponseDTO struct {
	PreviewID       string                       `json:"previewId"`
	Rows            []ParsedStudentRowDTO        `json:"rows"`
	DetectedClasses []string                     `json:"detectedClasses"`
	Errors          []string                     `json:"errors"`
	Warnings        []string                     `json:"warnings"`
	Summary         GoogleSheetPreviewSummaryDTO `json:"summary"`
}

type GoogleSheetCommitRowDTO struct {
	RowID              string     `json:"rowId" binding:"required"`
	Included           bool       `json:"included"`
	StudentCode        string     `json:"studentCode"`
	FullName           string     `json:"fullName"`
	ClassName          string     `json:"className"`
	Email              string     `json:"email"`
	Gender             string     `json:"gender"`
	Birthday           *time.Time `json:"birthday"`
	Phone              string     `json:"phone"`
	Address            string     `json:"address"`
	ParentName         string     `json:"parentName"`
	ParentPhone        string     `json:"parentPhone"`
	ParentEmail        string     `json:"parentEmail"`
	ParentRelationship string     `json:"parentRelationship"`
	EnrollmentDate     *time.Time `json:"enrollmentDate"`
	Note               string     `json:"note"`
}

type GoogleSheetCommitRequestDTO struct {
	PreviewID string                    `json:"previewId" binding:"required"`
	CenterID  string                    `json:"centerId" binding:"required"`
	ClassID   string                    `json:"classId"`
	Rows      []GoogleSheetCommitRowDTO `json:"rows" binding:"required"`
}

type ImportCredentialDTO struct {
	RowID     string `json:"rowId"`
	RowNumber int    `json:"rowNumber"`
	StudentID string `json:"studentId"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

type GoogleSheetCommitResponseDTO struct {
	JobID       string                `json:"jobId"`
	Created     int                   `json:"created"`
	Skipped     int                   `json:"skipped"`
	Duplicates  int                   `json:"duplicates"`
	Credentials []ImportCredentialDTO `json:"credentials"`
}

type ImportJobResponseDTO struct {
	JobID     string    `json:"jobId"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Created   int       `json:"created"`
	Skipped   int       `json:"skipped"`
	Errors    []string  `json:"errors"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SheetWritebackRequestDTO struct {
	UsernameColumn string `json:"usernameColumn" binding:"required"`
	PasswordColumn string `json:"passwordColumn" binding:"required"`
	WriteMode      string `json:"writeMode" binding:"required"`
}

type SheetWritebackResponseDTO struct {
	UpdatedRows    int        `json:"updatedRows"`
	DownloadURL    string     `json:"downloadUrl,omitempty"`
	SheetUpdatedAt *time.Time `json:"sheetUpdatedAt,omitempty"`
}

type ContentScopeDTO struct {
	Type     string `json:"type" binding:"required"`
	CenterID string `json:"centerId,omitempty"`
}

type SubjectResponseDTO struct {
	ID        string          `json:"id"`
	Scope     ContentScopeDTO `json:"scope"`
	Name      string          `json:"name"`
	Code      string          `json:"code"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type SubjectListResponseDTO struct {
	Items []SubjectResponseDTO `json:"items"`
}

type LevelResponseDTO struct {
	ID        string    `json:"id"`
	SubjectID string    `json:"subjectId"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Order     int       `json:"order"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type LevelListResponseDTO struct {
	Items []LevelResponseDTO `json:"items"`
}

type TopicResponseDTO struct {
	ID        string    `json:"id"`
	LevelID   string    `json:"levelId"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Order     int       `json:"order"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TopicListResponseDTO struct {
	Items []TopicResponseDTO `json:"items"`
}

type QuestionChoiceDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Correct bool   `json:"correct"`
}

type QuestionResponseDTO struct {
	ID        string              `json:"id"`
	Scope     ContentScopeDTO     `json:"scope"`
	SubjectID string              `json:"subjectId"`
	LevelID   string              `json:"levelId"`
	TopicID   string              `json:"topicId,omitempty"`
	Kind      string              `json:"kind"`
	Type      string              `json:"type"`
	Stem      string              `json:"stem"`
	Choices   []QuestionChoiceDTO `json:"choices,omitempty"`
	Answer    any                 `json:"answer,omitempty"`
	Metadata  map[string]any      `json:"metadata,omitempty"`
	Status    string              `json:"status"`
	CreatedAt time.Time           `json:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

type CreateQuestionRequestDTO struct {
	Scope     ContentScopeDTO     `json:"scope" binding:"required"`
	SubjectID string              `json:"subjectId" binding:"required"`
	LevelID   string              `json:"levelId" binding:"required"`
	TopicID   string              `json:"topicId"`
	Kind      string              `json:"kind"`
	Type      string              `json:"type"`
	Stem      string              `json:"stem" binding:"required"`
	Choices   []QuestionChoiceDTO `json:"choices"`
	Answer    any                 `json:"answer"`
	Metadata  map[string]any      `json:"metadata"`
}

type UpdateQuestionRequestDTO struct {
	Scope    *ContentScopeDTO    `json:"scope"`
	TopicID  *string             `json:"topicId"`
	Kind     *string             `json:"kind"`
	Type     *string             `json:"type"`
	Stem     *string             `json:"stem"`
	Choices  []QuestionChoiceDTO `json:"choices"`
	Answer   any                 `json:"answer"`
	Metadata map[string]any      `json:"metadata"`
}

type ArchiveQuestionRequestDTO struct {
	Reason string `json:"reason"`
}

type ArchiveQuestionResponseDTO struct {
	ArchivedAt time.Time `json:"archivedAt"`
}

type QuestionListResponseDTO struct {
	Items      []QuestionResponseDTO `json:"items"`
	NextCursor string                `json:"nextCursor,omitempty"`
	Total      int64                 `json:"total"`
}

type RandomPickQuestionsRequestDTO struct {
	SubjectID          string         `json:"subjectId" binding:"required"`
	LevelID            string         `json:"levelId" binding:"required"`
	TopicIDs           []string       `json:"topicIds"`
	Count              int64          `json:"count" binding:"required"`
	TypeMix            map[string]int `json:"typeMix"`
	ExcludeQuestionIDs []string       `json:"excludeQuestionIds"`
}

type RandomPickQuestionsResponseDTO struct {
	Questions []QuestionResponseDTO `json:"questions"`
	Seed      int64                 `json:"seed"`
}

type QuizResponseDTO struct {
	ID          string          `json:"id"`
	Scope       ContentScopeDTO `json:"scope"`
	Title       string          `json:"title"`
	Kind        string          `json:"kind"`
	SubjectID   string          `json:"subjectId"`
	LevelID     string          `json:"levelId"`
	TopicIDs    []string        `json:"topicIds"`
	QuestionIDs []string        `json:"questionIds"`
	Status      string          `json:"status"`
	Version     int             `json:"version"`
	PackageHash string          `json:"packageHash,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type QuizListResponseDTO struct {
	Items      []QuizResponseDTO `json:"items"`
	NextCursor string            `json:"nextCursor,omitempty"`
	Total      int64             `json:"total"`
}

type CreateQuizRequestDTO struct {
	Scope       ContentScopeDTO `json:"scope" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	Kind        string          `json:"kind" binding:"required"`
	SubjectID   string          `json:"subjectId" binding:"required"`
	LevelID     string          `json:"levelId" binding:"required"`
	TopicIDs    []string        `json:"topicIds"`
	QuestionIDs []string        `json:"questionIds"`
	Settings    map[string]any  `json:"settings"`
	ThemeID     string          `json:"themeId"`
}

type CreateQuizFromQuestionsRequestDTO struct {
	QuestionIDs []string       `json:"questionIds" binding:"required"`
	Title       string         `json:"title" binding:"required"`
	Kind        string         `json:"kind" binding:"required"`
	Settings    map[string]any `json:"settings"`
}

type RandomQuizTopicRuleDTO struct {
	TopicID string `json:"topicId"`
	Count   int64  `json:"count"`
}

type CreateRandomQuizRequestDTO struct {
	SubjectID  string                   `json:"subjectId" binding:"required"`
	LevelID    string                   `json:"levelId" binding:"required"`
	TopicRules []RandomQuizTopicRuleDTO `json:"topicRules"`
	Kind       string                   `json:"kind" binding:"required"`
	Settings   map[string]any           `json:"settings"`
}

type QuizDetailResponseDTO struct {
	Quiz     QuizResponseDTO `json:"quiz"`
	Slides   []any           `json:"slides"`
	Settings map[string]any  `json:"settings"`
	Result   map[string]any  `json:"result"`
	Theme    map[string]any  `json:"theme"`
}

type UpdateQuizRequestDTO struct {
	Slides   []any          `json:"slides"`
	Settings map[string]any `json:"settings"`
	Result   map[string]any `json:"result"`
	Theme    map[string]any `json:"theme"`
}

type PublishQuizRequestDTO struct {
	VersionNote string `json:"versionNote"`
}

type PublishQuizResponseDTO struct {
	Version     int    `json:"version"`
	PackageHash string `json:"packageHash"`
}

type QuizPackageResponseDTO struct {
	Version     int                   `json:"version"`
	PackageHash string                `json:"packageHash"`
	ContentHash string                `json:"contentHash"`
	Signature   string                `json:"signature,omitempty"`
	GradingMode string                `json:"gradingMode"`
	Quiz        QuizDetailResponseDTO `json:"quiz"`
}

type CreateAssignmentRequestDTO struct {
	ClassID       string     `json:"classId" binding:"required"`
	QuizIDs       []string   `json:"quizIds" binding:"required"`
	RecipientMode string     `json:"recipientMode" binding:"required"`
	StudentIDs    []string   `json:"studentIds"`
	DueAt         *time.Time `json:"dueAt"`
	TeacherNote   string     `json:"teacherNote"`
}

type AssignmentBatchResponseDTO struct {
	AssignmentIDs  []string `json:"assignmentIds"`
	RecipientCount int      `json:"recipientCount"`
}

type AssignmentResponseDTO struct {
	ID            string     `json:"id"`
	ClassID       string     `json:"classId"`
	QuizID        string     `json:"quizId"`
	SubjectID     string     `json:"subjectId,omitempty"`
	StudentIDs    []string   `json:"studentIds"`
	DueAt         *time.Time `json:"dueAt,omitempty"`
	TeacherNote   string     `json:"teacherNote,omitempty"`
	Status        string     `json:"status"`
	RecipientMode string     `json:"recipientMode"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type ClassAssignmentListResponseDTO struct {
	Items []AssignmentResponseDTO `json:"items"`
}

type AssignmentProgressResponseDTO struct {
	Submitted   int `json:"submitted"`
	InProgress  int `json:"inProgress"`
	NotStarted  int `json:"notStarted"`
	NeedsReview int `json:"needsReview"`
}

type StudentAssignmentListResponseDTO struct {
	Items []AssignmentResponseDTO `json:"items"`
}

type AttemptResponseDTO struct {
	ID           string                   `json:"id"`
	AssignmentID string                   `json:"assignmentId"`
	QuizID       string                   `json:"quizId"`
	StudentID    string                   `json:"studentId"`
	PackageID    string                   `json:"packageId"`
	PackageHash  string                   `json:"packageHash"`
	Status       string                   `json:"status"`
	Answers      map[string]AttemptAnswer `json:"answers"`
	Score        float64                  `json:"score"`
	MaxScore     float64                  `json:"maxScore"`
	Percent      float64                  `json:"percent"`
	Passed       bool                     `json:"passed"`
	StartedAt    time.Time                `json:"startedAt"`
	SubmittedAt  *time.Time               `json:"submittedAt,omitempty"`
	UpdatedAt    time.Time                `json:"updatedAt"`
}

type StartAttemptRequestDTO struct {
	AssignmentID string `json:"assignmentId" binding:"required"`
	QuizID       string `json:"quizId" binding:"required"`
	PackageID    string `json:"packageId" binding:"required"`
	PackageHash  string `json:"packageHash" binding:"required"`
}

type SaveAnswerRequestDTO struct {
	Answer       any            `json:"answer"`
	ClientResult map[string]any `json:"clientResult"`
	AnsweredAt   *time.Time     `json:"answeredAt"`
}

type AttemptDraftRequestDTO struct {
	Answers     map[string]any   `json:"answers"`
	PackageHash string           `json:"packageHash"`
	QuizVersion string           `json:"quizVersion"`
	Events      []map[string]any `json:"events"`
	Client      map[string]any   `json:"client"`
}

type AnswerResultResponseDTO struct {
	AttemptID  string `json:"attemptId"`
	QuestionID string `json:"questionId"`
	Saved      bool   `json:"saved"`
}

type SubmitAttemptRequestDTO struct {
	Answers     map[string]any `json:"answers"`
	SubmittedAt *time.Time     `json:"submittedAt"`
}

type AttemptSubmitResponseDTO struct {
	Score    float64 `json:"score"`
	MaxScore float64 `json:"maxScore"`
	Percent  float64 `json:"percent"`
	Passed   bool    `json:"passed"`
}

type AttemptSyncRequestDTO struct {
	PackageHash string           `json:"packageHash" binding:"required"`
	QuizVersion string           `json:"quizVersion"`
	Attempt     map[string]any   `json:"attempt"`
	Events      []map[string]any `json:"events"`
	Client      map[string]any   `json:"client"`
}

type AttemptSyncResponseDTO struct {
	Status        string             `json:"status"`
	ServerAttempt AttemptResponseDTO `json:"serverAttempt"`
	Conflicts     []string           `json:"conflicts,omitempty"`
}

type StudentScoreItemDTO struct {
	Assignment         AssignmentResponseDTO `json:"assignment"`
	BestScore          float64               `json:"bestScore"`
	RecentAttemptsTop3 []AttemptResponseDTO  `json:"recentAttemptsTop3"`
}

type StudentScoreListResponseDTO struct {
	Items []StudentScoreItemDTO `json:"items"`
}

type QuizStudentProgressItemDTO struct {
	StudentID    string  `json:"studentId"`
	AssignmentID string  `json:"assignmentId"`
	Status       string  `json:"status"`
	BestScore    float64 `json:"bestScore"`
	AttemptCount int     `json:"attemptCount"`
}

type QuizStudentProgressResponseDTO struct {
	Items []QuizStudentProgressItemDTO `json:"items"`
}

type CreateDiscussionRequestDTO struct {
	Title         string   `json:"title" binding:"required"`
	Content       string   `json:"content" binding:"required"`
	AssignmentID  string   `json:"assignmentId"`
	AttachmentIDs []string `json:"attachmentIds"`
}

type DiscussionThreadResponseDTO struct {
	ID               string    `json:"id"`
	ClassID          string    `json:"classId"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	AssignmentID     string    `json:"assignmentId,omitempty"`
	AttachmentIDs    []string  `json:"attachmentIds,omitempty"`
	AuthorID         string    `json:"authorId"`
	ReplyCount       int       `json:"replyCount"`
	LatestActivityAt time.Time `json:"latestActivityAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type DiscussionListResponseDTO struct {
	Items      []DiscussionThreadResponseDTO `json:"items"`
	NextCursor string                        `json:"nextCursor,omitempty"`
	Total      int64                         `json:"total"`
}

type CreateDiscussionReplyRequestDTO struct {
	Content       string   `json:"content" binding:"required"`
	AttachmentIDs []string `json:"attachmentIds"`
}

type DiscussionReplyResponseDTO struct {
	ID            string    `json:"id"`
	ThreadID      string    `json:"threadId"`
	ClassID       string    `json:"classId"`
	Content       string    `json:"content"`
	AttachmentIDs []string  `json:"attachmentIds,omitempty"`
	AuthorID      string    `json:"authorId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type AttachmentResponseDTO struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
}

type ProfanityWordListResponseDTO struct {
	Words   []string `json:"words"`
	Version string   `json:"version"`
}

type ModerationCheckRequestDTO struct {
	Text string `json:"text" binding:"required"`
}

type ModerationCheckResponseDTO struct {
	SanitizedText string   `json:"sanitizedText"`
	HasProfanity  bool     `json:"hasProfanity"`
	MatchedWords  []string `json:"matchedWords"`
}

type CreateAnnouncementRequestDTO struct {
	TargetType string   `json:"targetType" binding:"required"`
	ClassIDs   []string `json:"classIds"`
	StudentIDs []string `json:"studentIds"`
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	Pinned     bool     `json:"pinned"`
}

type AnnouncementResponseDTO struct {
	ID         string    `json:"id"`
	TargetType string    `json:"targetType"`
	ClassIDs   []string  `json:"classIds,omitempty"`
	StudentIDs []string  `json:"studentIds,omitempty"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Pinned     bool      `json:"pinned"`
	AuthorID   string    `json:"authorId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type AnnouncementListResponseDTO struct {
	Items      []AnnouncementResponseDTO `json:"items"`
	NextCursor string                    `json:"nextCursor,omitempty"`
	Total      int64                     `json:"total"`
}

type ClassroomReportResponseDTO struct {
	Summary             map[string]any          `json:"summary"`
	Classes             []ClassResponseDTO      `json:"classes"`
	Assignments         []AssignmentResponseDTO `json:"assignments"`
	StudentsNeedSupport []StudentListItemDTO    `json:"studentsNeedSupport"`
}

type DashboardScopeRequestDTO struct {
	ScopeType string
	UnitID    string
	CenterID  string
	ClassID   string
	Range     string
}

type DashboardScopeSummaryDTO struct {
	ScopeType    string `json:"scopeType"`
	UnitID       string `json:"unitId,omitempty"`
	CenterID     string `json:"centerId,omitempty"`
	ClassID      string `json:"classId,omitempty"`
	Range        string `json:"range,omitempty"`
	ClassCount   int    `json:"classCount"`
	StudentCount int    `json:"studentCount"`
}

type DashboardMetricsDTO struct {
	OpenAssignments      int     `json:"openAssignments"`
	NeedsSupport         int     `json:"needsSupport"`
	Completed            int     `json:"completed"`
	CompletedAssignments int     `json:"completedAssignments"`
	TotalAssignments     int     `json:"totalAssignments"`
	TotalStudents        int     `json:"totalStudents"`
	CompletionRate       float64 `json:"completionRate"`
}

type DashboardOverviewResponseDTO struct {
	Scope       DashboardScopeSummaryDTO `json:"scope"`
	Metrics     DashboardMetricsDTO      `json:"metrics"`
	GeneratedAt time.Time                `json:"generatedAt"`
}

type DashboardInterventionDTO struct {
	StudentID         string     `json:"studentId"`
	StudentName       string     `json:"studentName"`
	Username          string     `json:"username,omitempty"`
	UnitID            string     `json:"unitId,omitempty"`
	ClassID           string     `json:"classId"`
	ClassName         string     `json:"className"`
	AssignmentID      string     `json:"assignmentId,omitempty"`
	QuizID            string     `json:"quizId,omitempty"`
	Reason            string     `json:"reason"`
	Severity          string     `json:"severity"`
	RecommendedAction string     `json:"recommendedAction"`
	Score             *float64   `json:"score,omitempty"`
	LastActivityAt    *time.Time `json:"lastActivityAt,omitempty"`
}

type DashboardInterventionListResponseDTO struct {
	Scope DashboardScopeSummaryDTO   `json:"scope"`
	Items []DashboardInterventionDTO `json:"items"`
	Total int                        `json:"total"`
}

type ActiveAssignmentDTO struct {
	ID                string     `json:"id"`
	ClassID           string     `json:"classId"`
	ClassName         string     `json:"className,omitempty"`
	UnitID            string     `json:"unitId,omitempty"`
	QuizID            string     `json:"quizId"`
	SubjectID         string     `json:"subjectId,omitempty"`
	DueAt             *time.Time `json:"dueAt,omitempty"`
	TeacherNote       string     `json:"teacherNote,omitempty"`
	Status            string     `json:"status"`
	RecipientMode     string     `json:"recipientMode"`
	TotalStudents     int        `json:"totalStudents"`
	Submitted         int        `json:"submitted"`
	InProgress        int        `json:"inProgress"`
	NotStarted        int        `json:"notStarted"`
	NeedsReview       int        `json:"needsReview"`
	Overdue           int        `json:"overdue"`
	CompletionPercent float64    `json:"completionPercent"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ActiveAssignmentListResponseDTO struct {
	Scope DashboardScopeSummaryDTO `json:"scope"`
	Items []ActiveAssignmentDTO    `json:"items"`
	Total int                      `json:"total"`
}

type ClassReportResponseDTO struct {
	Class             ClassResponseDTO           `json:"class"`
	Summary           DashboardMetricsDTO        `json:"summary"`
	Students          []StudentListItemDTO       `json:"students"`
	ActiveAssignments []ActiveAssignmentDTO      `json:"activeAssignments"`
	Interventions     []DashboardInterventionDTO `json:"interventions"`
}

type AssignmentDeliveryRequestDTO struct {
	ClassID       string     `json:"classId" binding:"required"`
	QuizIDs       []string   `json:"quizIds" binding:"required"`
	RecipientMode string     `json:"recipientMode" binding:"required"`
	StudentIDs    []string   `json:"studentIds"`
	DueAt         *time.Time `json:"dueAt"`
	TeacherNote   string     `json:"teacherNote"`
}

type AssignmentDeliveryResponseDTO struct {
	AssignmentIDs  []string  `json:"assignmentIds"`
	RecipientCount int       `json:"recipientCount"`
	Status         string    `json:"status"`
	DeliveredAt    time.Time `json:"deliveredAt"`
}

type StudentJourneyResponseDTO struct {
	Strengths  []string         `json:"strengths"`
	FocusAreas []string         `json:"focusAreas"`
	Milestones []map[string]any `json:"milestones"`
	MentorNote string           `json:"mentorNote"`
}

type AssignmentReportResponseDTO struct {
	Completion        AssignmentProgressResponseDTO `json:"completion"`
	ScoreDistribution map[string]int                `json:"scoreDistribution"`
	LateSubmissions   int                           `json:"lateSubmissions"`
	NeedsReview       int                           `json:"needsReview"`
}

type ReportExportResponseDTO struct {
	DownloadURL string `json:"downloadUrl"`
}

type CreateInternalDocumentRequestDTO struct {
	Type      string `json:"type" binding:"required"`
	Title     string `json:"title" binding:"required"`
	SubjectID string `json:"subjectId"`
	FileID    string `json:"fileId"`
	Content   string `json:"content"`
}

type InternalDocumentResponseDTO struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	SubjectID string    `json:"subjectId,omitempty"`
	FileID    string    `json:"fileId,omitempty"`
	Content   string    `json:"content,omitempty"`
	AuthorID  string    `json:"authorId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type InternalDocumentListResponseDTO struct {
	Items []InternalDocumentResponseDTO `json:"items"`
	Total int64                         `json:"total"`
}
