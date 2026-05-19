package dto

import "time"

type AssetFileType string

const (
	AssetFileTypePDF   AssetFileType = "PDF"
	AssetFileTypePPTX  AssetFileType = "PPTX"
	AssetFileTypeVideo AssetFileType = "VIDEO"
	AssetFileTypeAudio AssetFileType = "AUDIO"
	AssetFileTypeHTML5 AssetFileType = "HTML5"
	AssetFileTypeLink  AssetFileType = "LINK"
	AssetFileTypeQuiz  AssetFileType = "QUIZ"
	AssetFileTypeZIP   AssetFileType = "ZIP"
	AssetFileTypeDOCX  AssetFileType = "DOCX"
	AssetFileTypeXLSX  AssetFileType = "XLSX"
	AssetFileTypeImage AssetFileType = "IMAGE"
)

func (t AssetFileType) Valid() bool {
	switch t {
	case AssetFileTypePDF, AssetFileTypePPTX, AssetFileTypeVideo, AssetFileTypeAudio,
		AssetFileTypeHTML5, AssetFileTypeLink, AssetFileTypeQuiz, AssetFileTypeZIP,
		AssetFileTypeDOCX, AssetFileTypeXLSX, AssetFileTypeImage:
		return true
	default:
		return false
	}
}

type FileTypeWarningDTO struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type FileTypeAuditDTO struct {
	ChangedAt                *time.Time    `json:"changedAt,omitempty"`
	ChangedBy                string        `json:"changedBy,omitempty"`
	PreviousSelectedFileType AssetFileType `json:"previousSelectedFileType,omitempty"`
	SelectedFileType         AssetFileType `json:"selectedFileType"`
	Source                   string        `json:"source,omitempty"`
	OriginalFileName         string        `json:"originalFileName,omitempty"`
	DetectedMimeType         string        `json:"detectedMimeType,omitempty"`
	Warnings                 []string      `json:"warnings,omitempty"`
}

type LaunchMode string

const (
	LaunchModePDFReader       LaunchMode = "pdf_reader"
	LaunchModeEBookReader     LaunchMode = "ebook_reader"
	LaunchModeSlideImageProxy LaunchMode = "slide_image_proxy"
	LaunchModeGoogleSlide     LaunchMode = "google_slide_embed"
	LaunchModeVideoPlayer     LaunchMode = "video_player"
	LaunchModeAudioPlayer     LaunchMode = "audio_player"
	LaunchModeHTML5Embed      LaunchMode = "html5_embed"
	LaunchModeQuizRuntime     LaunchMode = "quiz_runtime"
	LaunchModeDownloadOnly    LaunchMode = "download_only"
	LaunchModeExternal        LaunchMode = "external"
)

func defaultLaunchMode(fileType AssetFileType) LaunchMode {
	switch fileType {
	case AssetFileTypePDF:
		return LaunchModePDFReader
	case AssetFileTypePPTX:
		return LaunchModeSlideImageProxy
	case AssetFileTypeVideo:
		return LaunchModeVideoPlayer
	case AssetFileTypeAudio:
		return LaunchModeAudioPlayer
	case AssetFileTypeHTML5:
		return LaunchModeHTML5Embed
	case AssetFileTypeQuiz:
		return LaunchModeQuizRuntime
	case AssetFileTypeLink:
		return LaunchModeExternal
	default:
		return LaunchModeDownloadOnly
	}
}

func DefaultLaunchMode(fileType AssetFileType) LaunchMode {
	return defaultLaunchMode(fileType)
}

type ProgramDTO struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	ShortName   string   `json:"shortName"`
	SubjectIDs  []string `json:"subjectIds"`
	GradeIDs    []string `json:"gradeIds"`
	Description string   `json:"description"`
}

type TaxonomyOptionDTO struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Slug         string            `json:"slug"`
	ParentID     string            `json:"parentId,omitempty"`
	SubjectID    string            `json:"subjectId,omitempty"`
	GradeID      string            `json:"gradeId,omitempty"`
	CategoryID   string            `json:"categoryId,omitempty"`
	BookSeriesID string            `json:"bookSeriesId,omitempty"`
	TopicID      string            `json:"topicId,omitempty"`
	LevelIDs     []string          `json:"levelIds,omitempty"`
	Description  string            `json:"description,omitempty"`
	SortOrder    int               `json:"sortOrder,omitempty"`
	Status       string            `json:"status,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Depth        int               `json:"depth,omitempty"`
}

type TaxonomyResponseDTO struct {
	Programs        []ProgramDTO               `json:"programs"`
	Grades          []TaxonomyOptionDTO        `json:"grades"`
	Subjects        []TaxonomyOptionDTO        `json:"subjects"`
	Categories      []TaxonomyOptionDTO        `json:"categories"`
	Sections        []TaxonomyOptionDTO        `json:"sections"`
	BookSeries      []TaxonomyOptionDTO        `json:"bookSeries"`
	Topics          []TaxonomyOptionDTO        `json:"topics"`
	FileTypes       []AssetFileType            `json:"fileTypes"`
	DesignerPresets []LectureDesignerPresetDTO `json:"designerPresets"`
}

type LectureDesignerPresetDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AccentColor string `json:"accentColor"`
	Layout      string `json:"layout"`
}

type HomeMetricDTO struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type HomeResponseDTO struct {
	Hero      HomeHeroDTO       `json:"hero"`
	Metrics   []HomeMetricDTO   `json:"metrics"`
	Programs  []ProgramDTO      `json:"programs"`
	Shortcuts []HomeShortcutDTO `json:"shortcuts"`
}

type HomeHeroDTO struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	PrimaryCTA   string `json:"primaryCta"`
	SecondaryCTA string `json:"secondaryCta"`
}

type HomeShortcutDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Href        string `json:"href"`
}

type ResourceCardDTO struct {
	ID               string               `json:"id"`
	Slug             string               `json:"slug"`
	Title            string               `json:"title"`
	Subtitle         string               `json:"subtitle,omitempty"`
	ThumbnailURL     string               `json:"thumbnailUrl,omitempty"`
	ProgramSlug      string               `json:"programSlug"`
	SubjectID        string               `json:"subjectId"`
	GradeID          string               `json:"gradeId,omitempty"`
	CategoryID       string               `json:"categoryId"`
	SectionID        string               `json:"sectionId,omitempty"`
	BookSeriesID     string               `json:"bookSeriesId,omitempty"`
	TopicID          string               `json:"topicId,omitempty"`
	LevelID          string               `json:"levelId,omitempty"`
	DocumentTypeID   string               `json:"documentTypeId,omitempty"`
	SelectedFileType AssetFileType        `json:"selectedFileType"`
	FileTypeBadge    string               `json:"fileTypeBadge"`
	LaunchMode       LaunchMode           `json:"launchMode"`
	OriginalFileName string               `json:"originalFileName,omitempty"`
	DetectedMimeType string               `json:"detectedMimeType,omitempty"`
	FileExtension    string               `json:"fileExtension,omitempty"`
	MetadataWarnings []FileTypeWarningDTO `json:"metadataWarnings,omitempty"`
	FileTypeAudit    *FileTypeAuditDTO    `json:"fileTypeAudit,omitempty"`
	PriceType        string               `json:"priceType"`
	AccessState      string               `json:"accessState"`
	Visibility       string               `json:"visibility,omitempty"`
	Status           string               `json:"status,omitempty"`
	CanDownload      bool                 `json:"canDownload"`
	UpdatedAt        time.Time            `json:"updatedAt"`
}

type ResourceDetailDTO struct {
	ResourceCardDTO
	Description   string            `json:"description,omitempty"`
	Tags          []string          `json:"tags"`
	Assets        []AssetDTO        `json:"assets"`
	Items         []ResourceItemDTO `json:"items,omitempty"`
	LectureDesign *LectureDesignDTO `json:"lectureDesign,omitempty"`
}

type ResourceItemDTO struct {
	ID          string `json:"id"`
	ResourceID  string `json:"resourceId"`
	AssetID     string `json:"assetId"`
	UnitTitle   string `json:"unitTitle"`
	LessonTitle string `json:"lessonTitle,omitempty"`
	SortOrder   int    `json:"sortOrder"`
	PageCount   int    `json:"pageCount,omitempty"`
	DurationSec int    `json:"durationSec,omitempty"`
}

type AssetDTO struct {
	ID               string               `json:"id"`
	ResourceID       string               `json:"resourceId"`
	Title            string               `json:"title"`
	SelectedFileType AssetFileType        `json:"selectedFileType"`
	FileTypeBadge    string               `json:"fileTypeBadge"`
	LaunchMode       LaunchMode           `json:"launchMode"`
	OriginalFileName string               `json:"originalFileName,omitempty"`
	DetectedMimeType string               `json:"detectedMimeType,omitempty"`
	FileExtension    string               `json:"fileExtension,omitempty"`
	MetadataWarnings []FileTypeWarningDTO `json:"metadataWarnings,omitempty"`
	FileTypeAudit    *FileTypeAuditDTO    `json:"fileTypeAudit,omitempty"`
	FileSizeBytes    int64                `json:"fileSizeBytes,omitempty"`
	StorageProvider  string               `json:"storageProvider"`
	StorageURL       string               `json:"storageUrl,omitempty"`
	Status           string               `json:"status"`
	CanDownload      bool                 `json:"canDownload"`
	UpdatedAt        time.Time            `json:"updatedAt"`
}

type LectureDesignDTO struct {
	TemplateID     string   `json:"templateId,omitempty"`
	BannerTitle    string   `json:"bannerTitle,omitempty"`
	BannerSubtitle string   `json:"bannerSubtitle,omitempty"`
	BackgroundURL  string   `json:"backgroundUrl,omitempty"`
	CoverURL       string   `json:"coverUrl,omitempty"`
	AccentColor    string   `json:"accentColor,omitempty"`
	SecondaryColor string   `json:"secondaryColor,omitempty"`
	LogoURL        string   `json:"logoUrl,omitempty"`
	ItemColumns    int      `json:"itemColumns,omitempty"`
	ShowDownload   bool     `json:"showDownload"`
	UnitLabels     []string `json:"unitLabels,omitempty"`
}

type LaunchResponseDTO struct {
	AssetID          string           `json:"assetId"`
	ResourceID       string           `json:"resourceId"`
	SelectedFileType AssetFileType    `json:"selectedFileType"`
	LaunchMode       LaunchMode       `json:"launchMode"`
	Title            string           `json:"title"`
	EmbedURL         string           `json:"embedUrl,omitempty"`
	ViewerTokenURL   string           `json:"viewerTokenUrl,omitempty"`
	StreamURL        string           `json:"streamUrl,omitempty"`
	ExpiresAt        time.Time        `json:"expiresAt"`
	CanDownload      bool             `json:"canDownload"`
	Watermark        *WatermarkDTO    `json:"watermark,omitempty"`
	Audit            AuditMetadataDTO `json:"audit"`
}

type WatermarkDTO struct {
	Text     string  `json:"text"`
	Opacity  float64 `json:"opacity"`
	Position string  `json:"position"`
}

type AuditMetadataDTO struct {
	Event   string    `json:"event"`
	AssetID string    `json:"assetId"`
	UserID  string    `json:"userId,omitempty"`
	At      time.Time `json:"at"`
}

type ViewerPagesResponseDTO struct {
	AssetID          string          `json:"assetId"`
	ResourceID       string          `json:"resourceId"`
	SelectedFileType AssetFileType   `json:"selectedFileType"`
	Pages            []ViewerPageDTO `json:"pages"`
}

type ViewerPageDTO struct {
	Index       int    `json:"index"`
	Title       string `json:"title"`
	ImageURL    string `json:"imageUrl,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	DurationSec int    `json:"durationSec,omitempty"`
}

type CreateResourceRequestDTO struct {
	Title            string            `json:"title" binding:"required"`
	Slug             string            `json:"slug"`
	Subtitle         string            `json:"subtitle"`
	Description      string            `json:"description"`
	ThumbnailURL     string            `json:"thumbnailUrl"`
	UpstreamURL      string            `json:"upstreamUrl"`
	ProgramSlug      string            `json:"programSlug" binding:"required"`
	SubjectID        string            `json:"subjectId" binding:"required"`
	GradeID          string            `json:"gradeId"`
	CategoryID       string            `json:"categoryId" binding:"required"`
	SectionID        string            `json:"sectionId"`
	BookSeriesID     string            `json:"bookSeriesId"`
	TopicID          string            `json:"topicId"`
	LevelID          string            `json:"levelId"`
	DocumentTypeID   string            `json:"documentTypeId"`
	SelectedFileType AssetFileType     `json:"selectedFileType" binding:"required"`
	OriginalFileName string            `json:"originalFileName"`
	DetectedMimeType string            `json:"detectedMimeType"`
	PriceType        string            `json:"priceType"`
	Visibility       string            `json:"visibility"`
	Status           string            `json:"status"`
	CanDownload      *bool             `json:"canDownload"`
	Tags             []string          `json:"tags"`
	Items            []ResourceItemDTO `json:"items"`
	LectureDesign    *LectureDesignDTO `json:"lectureDesign"`
	ActorID          string            `json:"-"`
}

type UpdateResourceRequestDTO struct {
	ProgramSlug      *string           `json:"programSlug"`
	SubjectID        *string           `json:"subjectId"`
	GradeID          *string           `json:"gradeId"`
	CategoryID       *string           `json:"categoryId"`
	SectionID        *string           `json:"sectionId"`
	Title            *string           `json:"title"`
	Subtitle         *string           `json:"subtitle"`
	Description      *string           `json:"description"`
	ThumbnailURL     *string           `json:"thumbnailUrl"`
	UpstreamURL      *string           `json:"upstreamUrl"`
	BookSeriesID     *string           `json:"bookSeriesId"`
	TopicID          *string           `json:"topicId"`
	LevelID          *string           `json:"levelId"`
	DocumentTypeID   *string           `json:"documentTypeId"`
	SelectedFileType *AssetFileType    `json:"selectedFileType"`
	OriginalFileName *string           `json:"originalFileName"`
	DetectedMimeType *string           `json:"detectedMimeType"`
	PriceType        *string           `json:"priceType"`
	Visibility       *string           `json:"visibility"`
	Status           *string           `json:"status"`
	CanDownload      *bool             `json:"canDownload"`
	Tags             []string          `json:"tags"`
	Items            []ResourceItemDTO `json:"items"`
	LectureDesign    *LectureDesignDTO `json:"lectureDesign"`
	ActorID          string            `json:"-"`
}

type CreateAssetRequestDTO struct {
	ResourceID       string        `json:"resourceId" binding:"required"`
	Title            string        `json:"title" binding:"required"`
	SelectedFileType AssetFileType `json:"selectedFileType" binding:"required"`
	OriginalFileName string        `json:"originalFileName"`
	DetectedMimeType string        `json:"detectedMimeType"`
	FileSizeBytes    int64         `json:"fileSizeBytes"`
	StorageProvider  string        `json:"storageProvider"`
	StorageURL       string        `json:"storageUrl"`
	UpstreamURL      string        `json:"upstreamUrl"`
	CanDownload      *bool         `json:"canDownload"`
	ActorID          string        `json:"-"`
}

type UpdateAssetRequestDTO struct {
	Title            *string        `json:"title"`
	SelectedFileType *AssetFileType `json:"selectedFileType"`
	OriginalFileName *string        `json:"originalFileName"`
	DetectedMimeType *string        `json:"detectedMimeType"`
	FileSizeBytes    *int64         `json:"fileSizeBytes"`
	StorageProvider  *string        `json:"storageProvider"`
	StorageURL       *string        `json:"storageUrl"`
	UpstreamURL      *string        `json:"upstreamUrl"`
	CanDownload      *bool          `json:"canDownload"`
	Status           *string        `json:"status"`
	ActorID          string         `json:"-"`
}

type CreateTaxonomyOptionRequestDTO struct {
	ID           string            `json:"id"`
	Label        string            `json:"label" binding:"required"`
	Slug         string            `json:"slug"`
	ParentID     string            `json:"parentId"`
	SubjectID    string            `json:"subjectId"`
	GradeID      string            `json:"gradeId"`
	CategoryID   string            `json:"categoryId"`
	BookSeriesID string            `json:"bookSeriesId"`
	TopicID      string            `json:"topicId"`
	LevelIDs     []string          `json:"levelIds"`
	Description  string            `json:"description"`
	SortOrder    int               `json:"sortOrder"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata"`
}

type UpdateTaxonomyOptionRequestDTO struct {
	Label        *string           `json:"label"`
	Slug         *string           `json:"slug"`
	ParentID     *string           `json:"parentId"`
	SubjectID    *string           `json:"subjectId"`
	GradeID      *string           `json:"gradeId"`
	CategoryID   *string           `json:"categoryId"`
	BookSeriesID *string           `json:"bookSeriesId"`
	TopicID      *string           `json:"topicId"`
	LevelIDs     []string          `json:"levelIds"`
	Description  *string           `json:"description"`
	SortOrder    *int              `json:"sortOrder"`
	Status       *string           `json:"status"`
	Metadata     map[string]string `json:"metadata"`
}

type UploadAssetRequestDTO struct {
	ResourceID       string        `json:"resourceId"`
	Title            string        `json:"title"`
	SelectedFileType AssetFileType `json:"selectedFileType"`
	CanDownload      bool          `json:"canDownload"`
	ActorID          string        `json:"-"`
}

type UploadResourceResponseDTO struct {
	Resource *ResourceDetailDTO `json:"resource"`
	Asset    *AssetDTO          `json:"asset"`
}

type TeacherDashboardNodeKind string

const (
	TeacherDashboardNodeKindFolder     TeacherDashboardNodeKind = "folder"
	TeacherDashboardNodeKindCategory   TeacherDashboardNodeKind = "category"
	TeacherDashboardNodeKindSection    TeacherDashboardNodeKind = "section"
	TeacherDashboardNodeKindBookSeries TeacherDashboardNodeKind = "book_series"
	TeacherDashboardNodeKindTopic      TeacherDashboardNodeKind = "topic"
	TeacherDashboardNodeKindResource   TeacherDashboardNodeKind = "resource"
	TeacherDashboardNodeKindLesson     TeacherDashboardNodeKind = "lesson"
)

type TeacherProgressEventType string

const (
	TeacherProgressEventOpen          TeacherProgressEventType = "open"
	TeacherProgressEventStartTeaching TeacherProgressEventType = "start_teaching"
	TeacherProgressEventMarkTaught    TeacherProgressEventType = "mark_taught"
	TeacherProgressEventComplete      TeacherProgressEventType = "complete"
)

func (t TeacherProgressEventType) Valid() bool {
	switch t {
	case TeacherProgressEventOpen, TeacherProgressEventStartTeaching, TeacherProgressEventMarkTaught, TeacherProgressEventComplete:
		return true
	default:
		return false
	}
}

type TeacherProgressSummaryDTO struct {
	ProgressRate float64 `json:"progressRate"`
	TaughtCount  int     `json:"taughtCount"`
	TotalCount   int     `json:"totalCount"`
	PendingCount int     `json:"pendingCount"`
}

type TeacherDashboardNodeDTO struct {
	ID            string                    `json:"id"`
	Label         string                    `json:"label"`
	Kind          TeacherDashboardNodeKind  `json:"kind"`
	ParentID      string                    `json:"parentId,omitempty"`
	SubjectID     string                    `json:"subjectId,omitempty"`
	SubjectLabel  string                    `json:"subjectLabel,omitempty"`
	Description   string                    `json:"description,omitempty"`
	ResourceID    string                    `json:"resourceId,omitempty"`
	ResourceType  string                    `json:"resourceType,omitempty"`
	ThumbnailURL  string                    `json:"thumbnailUrl,omitempty"`
	FileTypeBadge string                    `json:"fileTypeBadge,omitempty"`
	HasChildren   bool                      `json:"hasChildren"`
	Progress      TeacherProgressSummaryDTO `json:"progress"`
	LastOpenedAt  *time.Time                `json:"lastOpenedAt,omitempty"`
	UpdatedAt     *time.Time                `json:"updatedAt,omitempty"`
}

type TeacherDashboardBreadcrumbDTO struct {
	ID    string                   `json:"id"`
	Label string                   `json:"label"`
	Kind  TeacherDashboardNodeKind `json:"kind"`
}

type TeacherSubjectTreeDTO struct {
	SubjectID    string                          `json:"subjectId"`
	SubjectLabel string                          `json:"subjectLabel"`
	SchoolID     string                          `json:"schoolId"`
	AcademicYear string                          `json:"academicYear"`
	ParentID     string                          `json:"parentId,omitempty"`
	Breadcrumbs  []TeacherDashboardBreadcrumbDTO `json:"breadcrumbs"`
	Children     []TeacherDashboardNodeDTO       `json:"children"`
	Progress     TeacherProgressSummaryDTO       `json:"progress"`
}

type TeacherRecentOpenedItemDTO struct {
	ID            string                   `json:"id"`
	SubjectID     string                   `json:"subjectId"`
	SubjectLabel  string                   `json:"subjectLabel"`
	NodeID        string                   `json:"nodeId"`
	NodeLabel     string                   `json:"nodeLabel"`
	NodeKind      TeacherDashboardNodeKind `json:"nodeKind"`
	ResourceID    string                   `json:"resourceId,omitempty"`
	ResourceTitle string                   `json:"resourceTitle,omitempty"`
	ResourceType  string                   `json:"resourceType,omitempty"`
	OpenedAt      time.Time                `json:"openedAt"`
}

type TeacherProgressDetailItemDTO struct {
	ID           string                   `json:"id"`
	Label        string                   `json:"label"`
	Kind         TeacherDashboardNodeKind `json:"kind"`
	Status       string                   `json:"status"`
	ProgressRate float64                  `json:"progressRate"`
}

type TeacherProgressResponseDTO struct {
	SubjectID    string                         `json:"subjectId"`
	NodeID       string                         `json:"nodeId,omitempty"`
	SchoolID     string                         `json:"schoolId"`
	AcademicYear string                         `json:"academicYear"`
	Summary      TeacherProgressSummaryDTO      `json:"summary"`
	Items        []TeacherProgressDetailItemDTO `json:"items"`
}

type TeacherProgressEventDTO struct {
	ID           string                   `json:"id"`
	TeacherID    string                   `json:"teacherId"`
	SchoolID     string                   `json:"schoolId"`
	AcademicYear string                   `json:"academicYear"`
	SubjectID    string                   `json:"subjectId"`
	NodeID       string                   `json:"nodeId"`
	NodeKind     TeacherDashboardNodeKind `json:"nodeKind"`
	EventType    TeacherProgressEventType `json:"eventType"`
	ResourceID   string                   `json:"resourceId,omitempty"`
	OccurredAt   time.Time                `json:"occurredAt"`
}

type TrackTeacherProgressEventRequestDTO struct {
	SchoolID     string                   `json:"schoolId" binding:"required"`
	AcademicYear string                   `json:"academicYear" binding:"required"`
	SubjectID    string                   `json:"subjectId" binding:"required"`
	NodeID       string                   `json:"nodeId" binding:"required"`
	NodeKind     TeacherDashboardNodeKind `json:"nodeKind" binding:"required"`
	EventType    TeacherProgressEventType `json:"eventType" binding:"required"`
	ResourceID   string                   `json:"resourceId"`
	OccurredAt   *time.Time               `json:"occurredAt,omitempty"`
	TeacherID    string                   `json:"-"`
}
