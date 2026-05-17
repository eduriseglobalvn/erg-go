package response

import (
	"strings"
	"time"

	authentities "erg.ninja/internal/modules/auth/domain/entity"
	"erg.ninja/internal/modules/users/domain/entity"
)

// UserListResponse is the DTO for paginated user lists (GET /users).
type UserListResponse struct {
	Users []UserItemResponse `json:"users"`
	Meta  *Meta              `json:"meta,omitempty"`
}

// UserItemResponse is a compact user item for list views.
type UserItemResponse struct {
	ID                 string   `json:"id"`
	Email              string   `json:"email"`
	FullName           string   `json:"fullName"`
	AvatarURL          string   `json:"avatar_url,omitempty"`
	Phone              string   `json:"phone,omitempty"`
	Status             string   `json:"status"`
	Provider           string   `json:"provider"`
	AccountType        string   `json:"accountType,omitempty"`
	LastLoginProvider  string   `json:"lastLoginProvider,omitempty"`
	Roles              []string `json:"roles"`
	IsProfileCompleted bool     `json:"isProfileCompleted"`
	LastLoginAt        string   `json:"last_login_at,omitempty"`
	CreatedAt          string   `json:"createdAt"`
}

// NewUserItemResponse builds a compact user item from a User entity.
func NewUserItemResponse(u *authentities.User) UserItemResponse {
	r := UserItemResponse{
		ID:                 u.ID.Hex(),
		Email:              u.Email,
		FullName:           u.FullName,
		AvatarURL:          u.AvatarURL,
		Phone:              u.Phone,
		Status:             string(u.Status),
		Provider:           u.Provider,
		AccountType:        u.AccountType,
		LastLoginProvider:  u.LastLoginProvider,
		Roles:              u.Roles,
		IsProfileCompleted: u.IsProfileCompleted && strings.TrimSpace(u.FullName) != "" && strings.TrimSpace(u.Phone) != "",
		CreatedAt:          u.CreatedAt.Format(time.RFC3339),
	}
	if u.LastLoginAt != nil {
		r.LastLoginAt = u.LastLoginAt.Format(time.RFC3339)
	}
	return r
}

// Meta contains pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// UserDetailResponse is the full admin user detail (GET /users/:id).
type UserDetailResponse struct {
	ID                  string            `json:"id"`
	Email               string            `json:"email"`
	FullName            string            `json:"fullName"`
	AvatarURL           string            `json:"avatar_url,omitempty"`
	Phone               string            `json:"phone,omitempty"`
	Bio                 string            `json:"bio,omitempty"`
	Gender              string            `json:"gender,omitempty"`
	DateOfBirth         string            `json:"date_of_birth,omitempty"`
	Address             string            `json:"address,omitempty"`
	City                string            `json:"city,omitempty"`
	District            string            `json:"district,omitempty"`
	JobTitle            string            `json:"job_title,omitempty"`
	Region              string            `json:"region,omitempty"`
	SocialLinks         map[string]string `json:"social_links,omitempty"`
	Status              string            `json:"status"`
	Provider            string            `json:"provider"`
	AccountType         string            `json:"accountType,omitempty"`
	GoogleSub           string            `json:"googleSub,omitempty"`
	GoogleEmail         string            `json:"googleEmail,omitempty"`
	GoogleEmailVerified bool              `json:"googleEmailVerified,omitempty"`
	LastLoginProvider   string            `json:"lastLoginProvider,omitempty"`
	Roles               []string          `json:"roles"`
	IsProfileCompleted  bool              `json:"isProfileCompleted"`
	LastLoginAt         string            `json:"last_login_at,omitempty"`
	LoginCount          int64             `json:"login_count,omitempty"`
	TenantID            string            `json:"tenant_id,omitempty"`
	CreatedAt           string            `json:"createdAt"`
	UpdatedAt           string            `json:"updatedAt"`
}

// NewUserDetailResponse builds a full admin user detail from a User entity.
func NewUserDetailResponse(u *authentities.User) UserDetailResponse {
	r := UserDetailResponse{
		ID:                  u.ID.Hex(),
		Email:               u.Email,
		FullName:            u.FullName,
		AvatarURL:           u.AvatarURL,
		Phone:               u.Phone,
		Bio:                 u.Bio,
		Gender:              u.Gender,
		DateOfBirth:         u.DateOfBirth,
		Address:             u.Address,
		City:                u.City,
		District:            u.District,
		JobTitle:            u.JobTitle,
		Region:              u.Region,
		SocialLinks:         u.SocialLinks,
		Status:              string(u.Status),
		Provider:            u.Provider,
		AccountType:         u.AccountType,
		GoogleSub:           u.GoogleSub,
		GoogleEmail:         u.GoogleEmail,
		GoogleEmailVerified: u.GoogleEmailVerified,
		LastLoginProvider:   u.LastLoginProvider,
		Roles:               u.Roles,
		IsProfileCompleted:  u.IsProfileCompleted && strings.TrimSpace(u.FullName) != "" && strings.TrimSpace(u.Phone) != "",
		LoginCount:          u.LoginCount,
		TenantID:            u.TenantID,
		CreatedAt:           u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           u.UpdatedAt.Format(time.RFC3339),
	}
	if u.LastLoginAt != nil {
		r.LastLoginAt = u.LastLoginAt.Format(time.RFC3339)
	}
	return r
}

// ActivityResponse is the DTO for GET /users/:id/activity.
type ActivityResponse struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	Action       string         `json:"action"`
	TargetUserID string         `json:"target_user_id,omitempty"`
	IPAddress    string         `json:"ip_address"`
	Description  string         `json:"description"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    string         `json:"createdAt"`
}

// NewActivityResponse builds an ActivityResponse from a UserActivity entity.
func NewActivityResponse(a *entity.UserActivity) ActivityResponse {
	r := ActivityResponse{
		ID:          a.ID.Hex(),
		UserID:      a.UserID.Hex(),
		Action:      string(a.Action),
		IPAddress:   a.IPAddress,
		Description: a.Description,
		Metadata:    a.Metadata,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
	if a.TargetUserID != nil {
		r.TargetUserID = a.TargetUserID.Hex()
	}
	return r
}

// BulkOperationResponse is the DTO for bulk operations.
type BulkOperationResponse struct {
	ModifiedCount int64 `json:"modified_count"`
}

// UserCreatedResponse is the DTO returned after admin creates a user.
type UserCreatedResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Status   string `json:"status"`
}

// OnboardingResponse is the DTO for POST /users/onboarding.
type OnboardingResponse struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	FullName           string `json:"fullName"`
	AvatarURL          string `json:"avatar_url,omitempty"`
	IsProfileCompleted bool   `json:"isProfileCompleted"`
}

// MessageResponse is a generic success message wrapper.
type MessageResponse struct {
	Message string `json:"message"`
}
