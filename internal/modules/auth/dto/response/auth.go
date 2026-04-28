package response

// AuthResponse is the DTO returned on successful login / register.
type AuthResponse struct {
	User         ProfileResponse `json:"user"`
	AccessToken  string          `json:"accessToken"`
	RefreshToken string          `json:"refreshToken"`
	ExpiresIn    int64           `json:"expiresIn"`
	TokenType    string          `json:"tokenType"`
}

// TokenResponse is the DTO for token refresh responses.
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	TokenType    string `json:"tokenType"`
}
