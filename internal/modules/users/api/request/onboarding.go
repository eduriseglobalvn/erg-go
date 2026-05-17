package request

// OnboardingRequest is the DTO for POST /users/onboarding.
// Avatar is handled as a multipart file field in the controller.
type OnboardingRequest struct {
	FullName    string `json:"fullName" validate:"required,min=2,max=100"`
	Phone       string `json:"phone" validate:"omitempty,max=20"`
	Bio         string `json:"bio" validate:"omitempty,max=500"`
	Gender      string `json:"gender" validate:"omitempty,oneof=male female other"`
	DateOfBirth string `json:"date_of_birth" validate:"omitempty"`
	Address     string `json:"address" validate:"omitempty,max=255"`
	City        string `json:"city" validate:"omitempty,max=100"`
	District    string `json:"district" validate:"omitempty,max=100"`
	JobTitle    string `json:"job_title" validate:"omitempty,max=100"`
	Region      string `json:"region" validate:"omitempty,max=100"`
}

// ChangePasswordRequest is the DTO for PUT /users/me/password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

// UpdateProfileRequest is the DTO for PATCH /users/me.
type UpdateProfileRequest struct {
	FullName    *string            `json:"full_name,omitempty" validate:"omitempty,min=2,max=100"`
	Phone       *string            `json:"phone,omitempty" validate:"omitempty,max=20"`
	Bio         *string            `json:"bio,omitempty" validate:"omitempty,max=500"`
	Gender      *string            `json:"gender,omitempty" validate:"omitempty,oneof=male female other"`
	DateOfBirth *string            `json:"date_of_birth,omitempty"`
	Address     *string            `json:"address,omitempty" validate:"omitempty,max=255"`
	City        *string            `json:"city,omitempty" validate:"omitempty,max=100"`
	District    *string            `json:"district,omitempty" validate:"omitempty,max=100"`
	JobTitle    *string            `json:"job_title,omitempty" validate:"omitempty,max=100"`
	Region      *string            `json:"region,omitempty" validate:"omitempty,max=100"`
	SocialLinks *map[string]string `json:"social_links,omitempty"`
	AvatarURL   *string            `json:"avatar_url,omitempty" validate:"omitempty,url"`
}

// AdminUpdateUserStatusRequest is the DTO for PUT /users/:id/status.
type AdminUpdateUserStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=ACTIVE PENDING BANNED BLOCKED"`
}

// AdminCreateUserRequest is the DTO for POST /users (admin).
type AdminCreateUserRequest struct {
	Email    string   `json:"email" validate:"required,email"`
	Password string   `json:"password" validate:"required,min=8"`
	FullName string   `json:"fullName" validate:"required,min=2,max=100"`
	Roles    []string `json:"roles" validate:"omitempty"`
	Phone    string   `json:"phone" validate:"omitempty"`
}

// BulkStatusRequest is the DTO for POST /users/bulk-status.
type BulkStatusRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1"`
	Status  string   `json:"status" validate:"required,oneof=ACTIVE BANNED BLOCKED"`
}

// BulkDeleteRequest is the DTO for POST /users/bulk-delete.
type BulkDeleteRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1"`
}

// AdminAssignRolesRequest is the DTO for POST /users/:id/roles.
type AdminAssignRolesRequest struct {
	Roles []string `json:"roles" validate:"required,min=1"`
}

// ListUsersQuery holds query parameters for GET /users.
type ListUsersQuery struct {
	Page   int    `json:"page"`
	Limit  int    `json:"limit"`
	Search string `json:"search"`
	Status string `json:"status"`
	Role   string `json:"role"`
}
