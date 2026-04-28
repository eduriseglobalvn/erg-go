package response

import "erg.ninja/internal/modules/auth/entities"

// ProfileResponse is the DTO returned by GET /auth/profile.
type ProfileResponse struct {
	ID                  string   `json:"id"`
	Email               string   `json:"email"`
	FullName            string   `json:"fullName"`
	AvatarURL           string   `json:"avatarUrl"`
	Status              string   `json:"status"`
	Provider            string   `json:"provider"`
	AccountType         string   `json:"accountType,omitempty"`
	GoogleSub           string   `json:"googleSub,omitempty"`
	GoogleEmail         string   `json:"googleEmail,omitempty"`
	GoogleEmailVerified bool     `json:"googleEmailVerified,omitempty"`
	LastLoginProvider   string   `json:"lastLoginProvider,omitempty"`
	Roles               []string `json:"roles"`
	CreatedAt           string   `json:"createdAt"`
}

// NewProfileResponse constructs a ProfileResponse from a User entity.
func NewProfileResponse(u *entities.User) ProfileResponse {
	return ProfileResponse{
		ID:                  u.ID.Hex(),
		Email:               u.Email,
		FullName:            u.FullName,
		AvatarURL:           u.AvatarURL,
		Status:              string(u.Status),
		Provider:            u.Provider,
		AccountType:         u.AccountType,
		GoogleSub:           u.GoogleSub,
		GoogleEmail:         u.GoogleEmail,
		GoogleEmailVerified: u.GoogleEmailVerified,
		LastLoginProvider:   u.LastLoginProvider,
		Roles:               u.Roles,
		CreatedAt:           u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
