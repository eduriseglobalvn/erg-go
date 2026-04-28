package postgrescore

import (
	"time"

	"gorm.io/gorm"
)

// AuthUser stores the canonical relational user account.
type AuthUser struct {
	ID                  string         `gorm:"column:id;type:varchar(24);primaryKey"`
	TenantID            string         `gorm:"column:tenant_id;type:varchar(64);not null;index:idx_users_tenant_created,priority:1"`
	Email               string         `gorm:"column:email;type:varchar(255);not null;uniqueIndex:idx_users_tenant_email,priority:2"`
	PasswordHash        string         `gorm:"column:password_hash;type:text;not null"`
	FullName            string         `gorm:"column:full_name;type:varchar(255)"`
	AvatarURL           string         `gorm:"column:avatar_url;type:text"`
	Status              string         `gorm:"column:status;type:varchar(32);not null;index:idx_users_status_created,priority:1"`
	Provider            string         `gorm:"column:provider;type:varchar(32);not null"`
	ProviderID          string         `gorm:"column:provider_id;type:varchar(255)"`
	AccountType         string         `gorm:"column:account_type;type:varchar(32);not null;default:'erg';index"`
	GoogleSub           string         `gorm:"column:google_sub;type:varchar(255);index"`
	GoogleEmail         string         `gorm:"column:google_email;type:varchar(255);index"`
	GoogleEmailVerified bool           `gorm:"column:google_email_verified;not null;default:false"`
	LastLoginProvider   string         `gorm:"column:last_login_provider;type:varchar(32)"`
	Phone               string         `gorm:"column:phone;type:varchar(64)"`
	Bio                 string         `gorm:"column:bio;type:text"`
	Gender              string         `gorm:"column:gender;type:varchar(32)"`
	DateOfBirth         string         `gorm:"column:date_of_birth;type:varchar(64)"`
	Address             string         `gorm:"column:address;type:text"`
	City                string         `gorm:"column:city;type:varchar(128)"`
	District            string         `gorm:"column:district;type:varchar(128)"`
	JobTitle            string         `gorm:"column:job_title;type:varchar(255)"`
	Region              string         `gorm:"column:region;type:varchar(255)"`
	SocialLinksJSON     string         `gorm:"column:social_links_json;type:text"`
	ExtendedProfile     string         `gorm:"column:extended_profile;type:text"`
	IsProfileCompleted  bool           `gorm:"column:is_profile_completed;not null;default:false"`
	LastLoginAt         *time.Time     `gorm:"column:last_login_at"`
	LoginCount          int64          `gorm:"column:login_count;not null;default:0"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null;index:idx_users_tenant_created,priority:2;index:idx_users_status_created,priority:2"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AuthUser) TableName() string { return "users" }

// AuthSession stores refresh-token-backed sessions in PostgreSQL.
type AuthSession struct {
	ID               string     `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID           string     `gorm:"column:user_id;type:varchar(24);not null;index:idx_user_sessions_user,priority:1"`
	SessionID        string     `gorm:"column:session_id;type:varchar(128);not null;uniqueIndex:idx_user_sessions_session_tenant,priority:1"`
	IPAddress        string     `gorm:"column:ip_address;type:varchar(128)"`
	UserAgent        string     `gorm:"column:user_agent;type:text"`
	RefreshTokenHash string     `gorm:"column:refresh_token_hash;type:text;not null"`
	TenantID         string     `gorm:"column:tenant_id;type:varchar(64);not null;uniqueIndex:idx_user_sessions_session_tenant,priority:2;index:idx_user_sessions_tenant_expiry,priority:1"`
	LastActiveAt     time.Time  `gorm:"column:last_active_at;not null"`
	ExpiresAt        time.Time  `gorm:"column:expires_at;not null;index:idx_user_sessions_tenant_expiry,priority:2"`
	RevokedAt        *time.Time `gorm:"column:revoked_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null"`
}

func (AuthSession) TableName() string { return "user_sessions" }

// AuthPin stores one-time verification or password reset PINs.
type AuthPin struct {
	ID        string     `gorm:"column:id;type:varchar(24);primaryKey"`
	Email     string     `gorm:"column:email;type:varchar(255);not null;index:idx_auth_pins_lookup,priority:1"`
	Code      string     `gorm:"column:code;type:varchar(32);not null;index:idx_auth_pins_lookup,priority:2"`
	Purpose   string     `gorm:"column:purpose;type:varchar(64);not null;index:idx_auth_pins_lookup,priority:3"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null;index:idx_auth_pins_expiry"`
	UsedAt    *time.Time `gorm:"column:used_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (AuthPin) TableName() string { return "auth_pins" }

// ACPermission stores a granular permission.
type ACPermission struct {
	ID          string    `gorm:"column:id;type:varchar(24);primaryKey"`
	Name        string    `gorm:"column:name;type:varchar(255);not null;uniqueIndex"`
	GroupName   string    `gorm:"column:group_name;type:varchar(128);not null;index"`
	Label       string    `gorm:"column:label;type:varchar(255);not null"`
	Description string    `gorm:"column:description;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (ACPermission) TableName() string { return "permissions" }

// ACPermissionGroup stores metadata for UI grouping and permission categorization.
type ACPermissionGroup struct {
	ID        string    `gorm:"column:id;type:varchar(24);primaryKey"`
	Name      string    `gorm:"column:name;type:varchar(128);not null;uniqueIndex"`
	Label     string    `gorm:"column:label;type:varchar(255);not null"`
	Order     int       `gorm:"column:display_order;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (ACPermissionGroup) TableName() string { return "permission_groups" }

// ACRole stores named roles.
type ACRole struct {
	ID          string    `gorm:"column:id;type:varchar(24);primaryKey"`
	Name        string    `gorm:"column:name;type:varchar(128);not null;uniqueIndex"`
	Description string    `gorm:"column:description;type:text"`
	IsDefault   bool      `gorm:"column:is_default;not null;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (ACRole) TableName() string { return "roles" }

// ACUserPermissionOverride stores explicit GRANT/DENY overrides.
type ACUserPermissionOverride struct {
	ID         string     `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID     string     `gorm:"column:user_id;type:varchar(24);not null;index:idx_user_permissions_user,priority:1"`
	Permission string     `gorm:"column:permission;type:varchar(255);not null;index:idx_user_permissions_user,priority:2"`
	GrantType  string     `gorm:"column:grant_type;type:varchar(16);not null"`
	Reason     string     `gorm:"column:reason;type:text"`
	ExpiresAt  *time.Time `gorm:"column:expires_at"`
	CreatedBy  string     `gorm:"column:created_by;type:varchar(24)"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
}

func (ACUserPermissionOverride) TableName() string { return "user_permissions" }

// UserRole joins users and roles.
type UserRole struct {
	UserID    string    `gorm:"column:user_id;type:varchar(24);primaryKey"`
	RoleID    string    `gorm:"column:role_id;type:varchar(24);primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (UserRole) TableName() string { return "user_roles" }

// RolePermission joins roles and permissions.
type RolePermission struct {
	RoleID       string    `gorm:"column:role_id;type:varchar(24);primaryKey"`
	PermissionID string    `gorm:"column:permission_id;type:varchar(24);primaryKey"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

func (RolePermission) TableName() string { return "role_permissions" }

// PostCategory stores post categories.
type PostCategory struct {
	ID              string    `gorm:"column:id;type:varchar(24);primaryKey"`
	Name            string    `gorm:"column:name;type:varchar(255);not null"`
	Slug            string    `gorm:"column:slug;type:varchar(255);not null;uniqueIndex"`
	Description     string    `gorm:"column:description;type:text"`
	Icon            string    `gorm:"column:icon;type:varchar(255)"`
	MetaTitle       string    `gorm:"column:meta_title;type:varchar(255)"`
	MetaDescription string    `gorm:"column:meta_description;type:text"`
	Keywords        string    `gorm:"column:keywords;type:text"`
	IsHidden        bool      `gorm:"column:is_hidden;not null;default:false;index"`
	HiddenType      string    `gorm:"column:hidden_type;type:varchar(64)"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (PostCategory) TableName() string { return "post_categories" }

// Post stores blog/news post content.
type Post struct {
	ID               string         `gorm:"column:id;type:varchar(24);primaryKey"`
	Title            string         `gorm:"column:title;type:varchar(500);not null"`
	Slug             string         `gorm:"column:slug;type:varchar(500);not null;uniqueIndex"`
	Excerpt          string         `gorm:"column:excerpt;type:text"`
	Content          string         `gorm:"column:content;type:text"`
	MetaJSON         string         `gorm:"column:meta_json;type:text"`
	ThumbnailURL     string         `gorm:"column:thumbnail_url;type:text"`
	Status           string         `gorm:"column:status;type:varchar(32);not null;index:idx_posts_status_category_created,priority:1"`
	IsPublished      bool           `gorm:"column:is_published;not null;default:false;index"`
	PublishedAt      *time.Time     `gorm:"column:published_at"`
	CreatedByID      string         `gorm:"column:created_by_id;type:varchar(24)"`
	PublishedByID    string         `gorm:"column:published_by_id;type:varchar(24)"`
	AuthorID         string         `gorm:"column:author_id;type:varchar(24);index"`
	ViewCount        int64          `gorm:"column:view_count;not null;default:0"`
	CommentCount     int64          `gorm:"column:comment_count;not null;default:0"`
	CategoryID       string         `gorm:"column:category_id;type:varchar(24);not null;index:idx_posts_status_category_created,priority:2"`
	IsCreatedByAI    bool           `gorm:"column:is_created_by_ai;not null;default:false"`
	AIPrompt         string         `gorm:"column:ai_prompt;type:text"`
	AIJobID          string         `gorm:"column:ai_job_id;type:varchar(255);index"`
	MetaTitle        string         `gorm:"column:meta_title;type:varchar(500)"`
	MetaDescription  string         `gorm:"column:meta_description;type:text"`
	FocusKeyword     string         `gorm:"column:focus_keyword;type:varchar(255)"`
	Keywords         string         `gorm:"column:keywords;type:text"`
	CanonicalURL     string         `gorm:"column:canonical_url;type:text"`
	SchemaType       string         `gorm:"column:schema_type;type:varchar(64)"`
	SEOScore         int            `gorm:"column:seo_score;not null;default:0"`
	ReadabilityScore int            `gorm:"column:readability_score;not null;default:0"`
	KeywordDensity   float64        `gorm:"column:keyword_density;not null;default:0"`
	SchemaMarkupJSON string         `gorm:"column:schema_markup_json;type:text"`
	SchemaDataJSON   string         `gorm:"column:schema_data_json;type:text"`
	RobotsIndex      bool           `gorm:"column:robots_index;not null;default:true"`
	RobotsFollow     bool           `gorm:"column:robots_follow;not null;default:true"`
	RobotsAdvanced   string         `gorm:"column:robots_advanced;type:text"`
	OGTitle          string         `gorm:"column:og_title;type:varchar(500)"`
	OGDescription    string         `gorm:"column:og_description;type:text"`
	OGImage          string         `gorm:"column:og_image;type:text"`
	TwitterCard      string         `gorm:"column:twitter_card;type:varchar(255)"`
	BreadcrumbTitle  string         `gorm:"column:breadcrumb_title;type:varchar(255)"`
	FAQItemsJSON     string         `gorm:"column:faq_items_json;type:text"`
	HowToStepsJSON   string         `gorm:"column:how_to_steps_json;type:text"`
	IntroVideoJSON   string         `gorm:"column:intro_video_json;type:text"`
	TagsJSON         string         `gorm:"column:tags_json;type:text"`
	CreatedAt        time.Time      `gorm:"column:created_at;not null;index:idx_posts_status_category_created,priority:3"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Post) TableName() string { return "posts" }

// Page stores CMS page content.
type Page struct {
	ID              string     `gorm:"column:id;type:varchar(24);primaryKey"`
	TenantID        string     `gorm:"column:tenant_id;type:varchar(64);not null;uniqueIndex:idx_pages_tenant_domain_slug,priority:1;index:idx_pages_tenant_domain_updated,priority:1"`
	Domain          string     `gorm:"column:domain;type:varchar(255);not null;uniqueIndex:idx_pages_tenant_domain_slug,priority:2;index:idx_pages_tenant_domain_updated,priority:2"`
	Slug            string     `gorm:"column:slug;type:varchar(255);not null;uniqueIndex:idx_pages_tenant_domain_slug,priority:3"`
	Title           string     `gorm:"column:title;type:varchar(500);not null"`
	Content         string     `gorm:"column:content;type:text"`
	MetaTitle       string     `gorm:"column:meta_title;type:varchar(255)"`
	MetaDescription string     `gorm:"column:meta_description;type:text"`
	FAQJSON         string     `gorm:"column:faq_json;type:text"`
	Status          string     `gorm:"column:status;type:varchar(32);not null;index"`
	PublishedAt     *time.Time `gorm:"column:published_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;index:idx_pages_tenant_domain_updated,priority:3"`
}

func (Page) TableName() string { return "pages" }

// SystemConfig stores global operations settings.
type SystemConfig struct {
	ID          string    `gorm:"column:id;type:varchar(24);primaryKey"`
	Key         string    `gorm:"column:key;type:varchar(255);not null;uniqueIndex"`
	ValueJSON   string    `gorm:"column:value_json;type:text"`
	Description string    `gorm:"column:description;type:text"`
	UpdatedBy   string    `gorm:"column:updated_by;type:varchar(24)"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (SystemConfig) TableName() string { return "system_configs" }

// Profile stores public profile data migrated from the legacy Mongo profiles
// collection while also leaving room for teacher-specific metadata from
// erg-backend.
type Profile struct {
	ID                 string     `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID             string     `gorm:"column:user_id;type:varchar(24);not null;uniqueIndex"`
	FullName           string     `gorm:"column:full_name;type:varchar(255)"`
	Bio                string     `gorm:"column:bio;type:text"`
	Phone              string     `gorm:"column:phone;type:varchar(64)"`
	DateOfBirth        *time.Time `gorm:"column:date_of_birth"`
	Gender             string     `gorm:"column:gender;type:varchar(32)"`
	Address            string     `gorm:"column:address;type:text"`
	City               string     `gorm:"column:city;type:varchar(128)"`
	District           string     `gorm:"column:district;type:varchar(128)"`
	SocialLinksJSON    string     `gorm:"column:social_links_json;type:text"`
	AvatarURL          string     `gorm:"column:avatar_url;type:text"`
	IsProfileCompleted bool       `gorm:"column:is_profile_completed;not null;default:false"`
	TeachingPhilosophy string     `gorm:"column:teaching_philosophy;type:text"`
	SpecialtiesJSON    string     `gorm:"column:specialties_json;type:text"`
	Rating             float64    `gorm:"column:rating;not null;default:0"`
	InternalNote       string     `gorm:"column:internal_note;type:text"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null;index"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null;index"`
}

func (Profile) TableName() string { return "profiles" }

// Certificate stores supporting credentials for a user profile.
type Certificate struct {
	ID         string     `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID     string     `gorm:"column:user_id;type:varchar(24);not null;index"`
	Name       string     `gorm:"column:name;type:varchar(255);not null"`
	IssuedBy   string     `gorm:"column:issued_by;type:varchar(255)"`
	IssueDate  *time.Time `gorm:"column:issue_date"`
	ExpiryDate *time.Time `gorm:"column:expiry_date"`
	ImageURL   string     `gorm:"column:image_url;type:text"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null"`
}

func (Certificate) TableName() string { return "certificates" }

// SocialAccount stores external login/account bindings.
type SocialAccount struct {
	ID         string    `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID     string    `gorm:"column:user_id;type:varchar(24);not null;index"`
	Provider   string    `gorm:"column:provider;type:varchar(64);not null;uniqueIndex:idx_social_accounts_provider_user,priority:1"`
	ProviderID string    `gorm:"column:provider_id;type:varchar(255);not null;uniqueIndex:idx_social_accounts_provider_user,priority:2"`
	Email      string    `gorm:"column:email;type:varchar(255)"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

func (SocialAccount) TableName() string { return "social_accounts" }

// CourseProgress stores per-user lesson progress.
type CourseProgress struct {
	ID              string     `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID          string     `gorm:"column:user_id;type:varchar(24);not null;index:idx_course_progress_user_course,priority:1"`
	CourseID        string     `gorm:"column:course_id;type:varchar(24);not null;index:idx_course_progress_user_course,priority:2"`
	LessonID        string     `gorm:"column:lesson_id;type:varchar(24);not null;index"`
	IsCompleted     bool       `gorm:"column:is_completed;not null;default:false"`
	ProgressPercent float64    `gorm:"column:progress_percent;not null;default:0"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
	LastAccessedAt  *time.Time `gorm:"column:last_accessed_at"`
	Score           *float64   `gorm:"column:score"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (CourseProgress) TableName() string { return "course_progress" }

// WorkShift stores operations scheduling data migrated from erg-backend.
type WorkShift struct {
	ID              string     `gorm:"column:id;type:varchar(24);primaryKey"`
	UserID          string     `gorm:"column:user_id;type:varchar(24);not null;index"`
	SchoolID        string     `gorm:"column:school_id;type:varchar(24);index"`
	Room            string     `gorm:"column:room;type:varchar(255)"`
	TeachingSubject string     `gorm:"column:teaching_subject;type:varchar(255)"`
	StartTime       time.Time  `gorm:"column:start_time;not null;index"`
	EndTime         time.Time  `gorm:"column:end_time;not null"`
	Type            string     `gorm:"column:type;type:varchar(64);not null"`
	Status          string     `gorm:"column:status;type:varchar(64);not null;default:SCHEDULED"`
	Note            string     `gorm:"column:note;type:text"`
	Remuneration    float64    `gorm:"column:remuneration;not null;default:0"`
	ConfirmedBy     string     `gorm:"column:confirmed_by;type:varchar(24)"`
	ActualStartTime *time.Time `gorm:"column:actual_start_time"`
	ActualEndTime   *time.Time `gorm:"column:actual_end_time"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (WorkShift) TableName() string { return "work_shifts" }

// RecruitmentJob stores job postings in PostgreSQL.
type RecruitmentJob struct {
	ID               string     `gorm:"column:id;type:varchar(24);primaryKey"`
	TenantID         string     `gorm:"column:tenant_id;type:varchar(64);not null;index"`
	Slug             string     `gorm:"column:slug;type:varchar(255);not null;uniqueIndex"`
	Title            string     `gorm:"column:title;type:varchar(255);not null"`
	Status           string     `gorm:"column:status;type:varchar(32);not null;index"`
	IsHot            bool       `gorm:"column:is_hot;not null;default:false"`
	IsNew            bool       `gorm:"column:is_new;not null;default:false"`
	IsUrgent         bool       `gorm:"column:is_urgent;not null;default:false"`
	Salary           string     `gorm:"column:salary;type:varchar(255)"`
	SalaryMin        *float64   `gorm:"column:salary_min"`
	SalaryMax        *float64   `gorm:"column:salary_max"`
	SalaryCurrency   string     `gorm:"column:salary_currency;type:varchar(16)"`
	Quantity         int        `gorm:"column:quantity;not null;default:0"`
	ViewCount        int        `gorm:"column:view_count;not null;default:0"`
	WorkType         string     `gorm:"column:work_type;type:varchar(255)"`
	WorkSchedule     string     `gorm:"column:work_schedule;type:varchar(255)"`
	PostDate         string     `gorm:"column:post_date;type:varchar(64)"`
	Deadline         string     `gorm:"column:deadline;type:varchar(64)"`
	DeadlineDate     *time.Time `gorm:"column:deadline_date"`
	Location         string     `gorm:"column:location;type:varchar(255)"`
	StreetAddr       string     `gorm:"column:street_address;type:text"`
	City             string     `gorm:"column:city;type:varchar(128)"`
	Country          string     `gorm:"column:country;type:varchar(16)"`
	EmploymentType   string     `gorm:"column:employment_type;type:varchar(64)"`
	Summary          string     `gorm:"column:summary;type:text"`
	DescriptionJSON  string     `gorm:"column:description_json;type:text"`
	RequirementsJSON string     `gorm:"column:requirements_json;type:text"`
	BenefitsJSON     string     `gorm:"column:benefits_json;type:text"`
	IsActive         bool       `gorm:"column:is_active;not null;default:true;index"`
	CreatedBy        string     `gorm:"column:created_by;type:varchar(24)"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;index"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null"`
}

func (RecruitmentJob) TableName() string { return "jobs" }

// RecruitmentCandidate stores job applications in PostgreSQL.
type RecruitmentCandidate struct {
	ID           string    `gorm:"column:id;type:varchar(24);primaryKey"`
	JobID        string    `gorm:"column:job_id;type:varchar(24);index"`
	JobTitle     string    `gorm:"column:job_title;type:varchar(255)"`
	TenantID     string    `gorm:"column:tenant_id;type:varchar(64);not null;index"`
	FullName     string    `gorm:"column:full_name;type:varchar(255);not null"`
	Email        string    `gorm:"column:email;type:varchar(255);not null;index"`
	Phone        string    `gorm:"column:phone;type:varchar(64)"`
	CVURL        string    `gorm:"column:cv_url;type:text"`
	CoverLetter  string    `gorm:"column:cover_letter;type:text"`
	Note         string    `gorm:"column:note;type:text"`
	PublicNote   string    `gorm:"column:public_note;type:text"`
	ApplyType    string    `gorm:"column:apply_type;type:varchar(32);not null"`
	Status       string    `gorm:"column:status;type:varchar(32);not null;index"`
	TrackingCode string    `gorm:"column:tracking_code;type:varchar(128);not null;uniqueIndex"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;index"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (RecruitmentCandidate) TableName() string { return "candidates" }
