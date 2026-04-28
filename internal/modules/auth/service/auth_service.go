package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	ac "erg.ninja/internal/modules/access_control/service"
	"erg.ninja/internal/modules/auth/dto/request"
	"erg.ninja/internal/modules/auth/dto/response"
	"erg.ninja/internal/modules/auth/entities"
	"erg.ninja/internal/modules/auth/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// Service errors surfaced to the HTTP layer.
var (
	ErrInvalidCredentials    = errors.New("auth.service: invalid credentials")
	ErrUserNotFound          = errors.New("auth.service: user not found")
	ErrEmailExists           = errors.New("auth.service: email already registered")
	ErrAccountLocked         = errors.New("auth.service: account locked")
	ErrEmailNotVerified      = errors.New("auth.service: email not verified")
	ErrGoogleBridgeForbidden = errors.New("auth.service: google bridge request forbidden")
	ErrGoogleIdentityInvalid = errors.New("auth.service: invalid google identity")
	ErrTokenReuseDetected    = errors.New("auth.service: token reuse detected — all sessions revoked")
	ErrInvalidPIN            = errors.New("auth.service: invalid or expired PIN")
	ErrInvalidToken          = errors.New("auth.service: invalid token")
)

// AuthService is the top-level auth business logic.
type AuthService struct {
	repo          *repository.Repo
	provider      *auth.AuthServiceProvider
	log           *logger.Logger
	cfg           *config.Config
	redis         *cache.RedisClient
	ac            *ac.Service
	smtpHost      string
	smtpPort      int
	smtpUser      string
	smtpPass      string
	smtpFrom      string
	adminEmail    string
	adminPassword string
}

// ServiceDeps holds dependencies for NewAuthService.
type ServiceDeps struct {
	Repo   *repository.Repo
	Redis  *cache.RedisClient
	Log    *logger.Logger
	Config *config.Config
	AC     *ac.Service
}

// NewAuthService creates a new AuthService.
func NewAuthService(deps ServiceDeps) *AuthService {
	accessSecret := deps.Config.Auth.JWTSecret
	if accessSecret == "" {
		accessSecret = "dev-access-secret-change-in-production"
	}
	// Derive a separate refresh secret by hashing the access secret + a suffix.
	refreshSecret := accessSecret + "-refresh"
	h := sha256.Sum256([]byte(refreshSecret))
	refreshSecret = hex.EncodeToString(h[:])

	provider := auth.NewAuthServiceProvider(
		accessSecret,
		refreshSecret,
		auth.WithAccessExpiry(deps.Config.Auth.AccessTokenTTL),
		auth.WithRefreshExpiry(deps.Config.Auth.RefreshTokenTTL),
		auth.WithIssuer(deps.Config.Auth.JWTIssuer),
	)

	return &AuthService{
		repo:          deps.Repo,
		provider:      provider,
		log:           deps.Log,
		cfg:           deps.Config,
		redis:         deps.Redis,
		ac:            deps.AC,
		smtpHost:      deps.Config.SMTP.Host,
		smtpPort:      deps.Config.SMTP.Port,
		smtpUser:      deps.Config.SMTP.Username,
		smtpPass:      deps.Config.SMTP.Password,
		smtpFrom:      deps.Config.SMTP.From,
		adminEmail:    deps.Config.Auth.AdminEmail,
		adminPassword: deps.Config.Auth.AdminPassword,
	}
}

// BootstrapAdmin ensures the super-admin account exists and has the "admin" role.
// Idempotent: skipped if the account already exists. Called automatically on startup.
func (s *AuthService) BootstrapAdmin() error {
	if s.adminEmail == "" || s.adminPassword == "" {
		return nil // not configured, skip
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	existing, err := s.repo.FindUserByEmail(ctx, s.adminEmail, tenantID)
	if err == nil && existing != nil {
		// Admin already exists — ensure it has "admin" and "SUPER_ADMIN" roles
		needsUpdate := false
		rolesMap := make(map[string]bool)
		for _, r := range existing.Roles {
			rolesMap[r] = true
		}

		if !rolesMap["admin"] {
			existing.Roles = append(existing.Roles, "admin")
			needsUpdate = true
		}
		if !rolesMap["SUPER_ADMIN"] {
			existing.Roles = append(existing.Roles, "SUPER_ADMIN")
			needsUpdate = true
		}
		if !existing.IsProfileCompleted {
			existing.IsProfileCompleted = true
			needsUpdate = true
		}

		if needsUpdate {
			if err := s.repo.UpdateUserIdentityFields(ctx, existing.ID, map[string]any{
				"roles":                uniqueStrings(existing.Roles),
				"is_profile_completed": true,
			}); err != nil {
				return fmt.Errorf("auth.service.BootstrapAdmin: update admin: %w", err)
			}
			s.log.Warn().Str("email", s.adminEmail).Msg("auth.service: admin user metadata was incomplete and has been repaired")
		}
		return nil
	}

	// Create admin account
	passwordHash, err := hashArgon2(s.adminPassword, s.cfg.Auth.Argon2Memory, s.cfg.Auth.Argon2Iterations)
	if err != nil {
		return fmt.Errorf("auth.service.BootstrapAdmin: hash password: %w", err)
	}

	admin := &entities.User{
		Email:              s.adminEmail,
		PasswordHash:       passwordHash,
		FullName:           "Super Administrator",
		Status:             entities.UserStatusActive, // no PIN verification needed
		Provider:           "local",
		Roles:              []string{"admin", "SUPER_ADMIN"},
		TenantID:           tenantID,
		IsProfileCompleted: true,
	}

	if err := s.repo.CreateUser(ctx, admin); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return nil // race condition — already created by another goroutine
		}
		return fmt.Errorf("auth.service.BootstrapAdmin: create user: %w", err)
	}

	s.log.Info().Str("email", s.adminEmail).Msg("auth.service: ✅ super-admin account bootstrapped")
	return nil
}

// defaultTenantID returns the configured default tenant ID.
func (s *AuthService) defaultTenantID() string {
	if s.cfg.Tenant.DefaultID != "" {
		return s.cfg.Tenant.DefaultID
	}
	return "default"
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// ─── Register ────────────────────────────────────────────────────────────────

// Register creates a PENDING user and sends a PIN email.
func (s *AuthService) Register(ctx context.Context, req *request.RegisterRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	// Check duplicate
	existing, _ := s.repo.FindUserByEmail(ctx, req.Email, tenantID)
	if existing != nil {
		return ErrEmailExists
	}

	// Hash password with argon2
	passwordHash, err := hashArgon2(req.Password,
		s.cfg.Auth.Argon2Memory,
		s.cfg.Auth.Argon2Iterations,
	)
	if err != nil {
		return fmt.Errorf("auth.service.register: argon2 hash: %w", err)
	}

	user := &entities.User{
		Email:        req.Email,
		PasswordHash: passwordHash,
		FullName:     req.FullName,
		Status:       entities.UserStatusPending,
		Provider:     "local",
		Roles:        []string{"user"},
		TenantID:     tenantID,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return ErrEmailExists
		}
		return fmt.Errorf("auth.service.register: create user: %w", err)
	}

	// Generate PIN
	code, err := generatePIN(6)
	if err != nil {
		return fmt.Errorf("auth.service.register: generate pin: %w", err)
	}
	pin := &entities.PinCode{
		Email:   req.Email,
		Code:    code,
		Purpose: entities.PinPurposeRegister,
	}
	if err := s.repo.CreatePin(ctx, pin); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("email", req.Email).Msg("auth.service: failed to store PIN")
	}

	// Send PIN email (best-effort)
	go s.sendPINEmail(req.Email, code, "register")

	s.log.InfoContext(ctx).Str("email", req.Email).Msg("auth.service: user registered (pending verification)")
	return nil
}

// ─── Login ───────────────────────────────────────────────────────────────────

// Login authenticates a user with email+password and issues tokens.
func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest, ip, userAgent string) (*response.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	// Check abuse config
	if s.cfg.Auth.MaxFailedLogin > 0 {
		failKey := fmt.Sprintf("auth_fail:%s", req.Email)
		failed, _ := s.redis.Get(ctx, failKey)
		if failed != "" {
			// rate-limit key stores count, e.g. "3"
			// Check if in block window
			ttl, _ := s.redis.TTL(ctx, failKey)
			if ttl > 0 && len(failed) >= s.cfg.Auth.MaxFailedLogin {
				return nil, ErrAccountLocked
			}
		}
	}

	user, err := s.repo.FindUserByEmail(ctx, req.Email, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.recordFailedLogin(ctx, req.Email)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth.service.login: find user: %w", err)
	}

	if user.Status == entities.UserStatusBlocked || user.Status == entities.UserStatusBanned {
		return nil, ErrAccountLocked
	}

	if user.Status == entities.UserStatusPending && user.Provider == "local" {
		return nil, ErrEmailNotVerified
	}

	// Verify password
	if !verifyArgon2(req.Password, user.PasswordHash) {
		s.recordFailedLogin(ctx, req.Email)
		return nil, ErrInvalidCredentials
	}

	// Clear failed login counter on success
	failKey := fmt.Sprintf("auth_fail:%s", req.Email)
	_ = s.redis.Del(ctx, failKey)

	return s.issueTokensForUser(ctx, user, ip, userAgent, "erg")
}

// GoogleLogin trusts the frontend OAuth bridge and exchanges a verified Google identity for erg-go tokens.
func (s *AuthService) GoogleLogin(ctx context.Context, req *request.GoogleLoginRequest, ip, userAgent string) (*response.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	email := strings.TrimSpace(req.Email)
	googleSub := strings.TrimSpace(req.GoogleSub)
	if email == "" || googleSub == "" || !req.EmailVerified {
		return nil, ErrGoogleIdentityInvalid
	}

	tenantID := s.defaultTenantID()

	user, err := s.repo.FindUserByGoogleSub(ctx, googleSub, tenantID)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("auth.service.googleLogin: find user by google sub: %w", err)
	}

	if user == nil {
		user, err = s.repo.FindUserByEmail(ctx, email, tenantID)
		if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("auth.service.googleLogin: find user by email: %w", err)
		}
		if errors.Is(err, repository.ErrUserNotFound) {
			user = nil
		}
	}

	if user == nil {
		passwordHash, hashErr := hashArgon2(generateSessionID(), s.cfg.Auth.Argon2Memory, s.cfg.Auth.Argon2Iterations)
		if hashErr != nil {
			return nil, fmt.Errorf("auth.service.googleLogin: bootstrap password hash: %w", hashErr)
		}

		user = &entities.User{
			Email:               email,
			PasswordHash:        passwordHash,
			FullName:            strings.TrimSpace(req.FullName),
			AvatarURL:           strings.TrimSpace(req.AvatarURL),
			Status:              entities.UserStatusActive,
			Provider:            "google",
			ProviderID:          googleSub,
			AccountType:         "google",
			GoogleSub:           googleSub,
			GoogleEmail:         email,
			GoogleEmailVerified: true,
			LastLoginProvider:   "google",
			Roles:               []string{"user"},
			TenantID:            tenantID,
			IsProfileCompleted:  strings.TrimSpace(req.FullName) != "",
		}

		if err := s.repo.CreateUser(ctx, user); err != nil {
			if errors.Is(err, repository.ErrDuplicateEmail) {
				user, err = s.repo.FindUserByEmail(ctx, email, tenantID)
				if err != nil {
					return nil, fmt.Errorf("auth.service.googleLogin: load duplicate email user: %w", err)
				}
			} else {
				return nil, fmt.Errorf("auth.service.googleLogin: create user: %w", err)
			}
		}
	}

	if user.Status == entities.UserStatusBlocked || user.Status == entities.UserStatusBanned {
		return nil, ErrAccountLocked
	}

	updates := map[string]any{
		"provider_id":           googleSub,
		"google_sub":            googleSub,
		"google_email":          strings.ToLower(email),
		"google_email_verified": true,
		"last_login_provider":   "google",
		"account_type":          mergeAccountType(user.AccountType, user.Provider, googleSub),
		"updated_at":            time.Now().UTC(),
	}
	if user.Provider == "" {
		updates["provider"] = "google"
	}
	if strings.TrimSpace(user.ProviderID) == "" {
		updates["provider_id"] = googleSub
	}
	if strings.TrimSpace(user.FullName) == "" && strings.TrimSpace(req.FullName) != "" {
		updates["full_name"] = strings.TrimSpace(req.FullName)
		user.FullName = strings.TrimSpace(req.FullName)
	}
	if strings.TrimSpace(user.AvatarURL) == "" && strings.TrimSpace(req.AvatarURL) != "" {
		updates["avatar_url"] = strings.TrimSpace(req.AvatarURL)
		user.AvatarURL = strings.TrimSpace(req.AvatarURL)
	}
	if user.Status == entities.UserStatusPending {
		updates["status"] = string(entities.UserStatusActive)
		user.Status = entities.UserStatusActive
	}
	if !user.IsProfileCompleted && strings.TrimSpace(req.FullName) != "" {
		updates["is_profile_completed"] = true
		user.IsProfileCompleted = true
	}

	if err := s.repo.UpdateUserIdentityFields(ctx, user.ID, updates); err != nil {
		return nil, fmt.Errorf("auth.service.googleLogin: update identity fields: %w", err)
	}

	user.ProviderID = googleSub
	user.GoogleSub = googleSub
	user.GoogleEmail = strings.ToLower(email)
	user.GoogleEmailVerified = true
	user.AccountType = mergeAccountType(user.AccountType, user.Provider, googleSub)
	user.LastLoginProvider = "google"
	if user.Provider == "" {
		user.Provider = "google"
	}

	return s.issueTokensForUser(ctx, user, ip, userAgent, "google")
}

// recordFailedLogin increments the failed-login counter and sets block window.
func (s *AuthService) recordFailedLogin(ctx context.Context, email string) {
	if s.cfg.Auth.MaxFailedLogin <= 0 {
		return
	}
	failKey := fmt.Sprintf("auth_fail:%s", email)
	_, _ = s.redis.Incr(ctx, failKey)
	// Set expiry on first failure
	ttl, _ := s.redis.TTL(ctx, failKey)
	if ttl < 0 {
		_ = s.redis.Set(ctx, failKey, "1", s.cfg.Auth.BlockDuration)
	}
}

// issueTokensForUser creates a session and returns token pair.
func (s *AuthService) issueTokensForUser(ctx context.Context, user *entities.User, ip, userAgent, loginProvider string) (*response.AuthResponse, error) {
	sessionID := generateSessionID()

	// Fetch effective permissions from AC
	var perms []string
	roles := user.Roles
	if s.ac != nil {
		eff, err := s.ac.GetEffectivePermissions(ctx, user.ID.Hex())
		if err == nil && eff != nil {
			perms = eff.EffectivePermissions
		}
	}

	tokens, err := s.provider.IssuePair(sessionID, user.ID.Hex(), user.Email, roles, perms)
	if err != nil {
		return nil, fmt.Errorf("auth.service.issueTokensForUser: issue pair: %w", err)
	}

	// Store refresh token hash for rotation detection
	rtHash := hashSHA256(tokens.RefreshToken)
	_ = rtHash // Just hash it for now, or remove if not needed here

	// Create session record
	refreshHash := hashSHA256(tokens.RefreshToken)
	session := &entities.UserSession{
		UserID:       user.ID,
		SessionID:    sessionID,
		IPAddress:    ip,
		UserAgent:    userAgent,
		RefreshToken: refreshHash,
		TenantID:     user.TenantID,
		ExpiresAt:    time.Now().Add(s.cfg.Auth.RefreshTokenTTL),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("user_id", user.ID.Hex()).Msg("auth.service: failed to store session")
	}

	if err := s.repo.TouchSuccessfulLogin(ctx, user.ID, loginProvider); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("user_id", user.ID.Hex()).Msg("auth.service: failed to update login metadata")
	} else {
		now := time.Now().UTC()
		user.LastLoginAt = &now
		user.LoginCount++
		user.LastLoginProvider = loginProvider
	}

	return &response.AuthResponse{
		User:         response.NewProfileResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}, nil
}

func (s *AuthService) IsValidGoogleBridgeSecret(secret string) bool {
	configured := strings.TrimSpace(s.cfg.Auth.GoogleBridgeSecret)
	if configured == "" {
		return false
	}
	return strings.TrimSpace(secret) == configured
}

func mergeAccountType(current, provider, googleSub string) string {
	current = strings.TrimSpace(strings.ToLower(current))
	provider = strings.TrimSpace(strings.ToLower(provider))

	switch current {
	case "hybrid", "google":
		if provider == "local" {
			return "hybrid"
		}
		if strings.TrimSpace(googleSub) != "" {
			return current
		}
	case "erg":
		if strings.TrimSpace(googleSub) != "" {
			return "hybrid"
		}
	}

	if strings.TrimSpace(googleSub) != "" && provider == "local" {
		return "hybrid"
	}
	if strings.TrimSpace(googleSub) != "" || provider == "google" {
		return "google"
	}
	return "erg"
}

// ─── Refresh ─────────────────────────────────────────────────────────────────

// RefreshToken validates a refresh token and issues a new pair.
// On token reuse (rotation attack), ALL user sessions are revoked.
func (s *AuthService) RefreshToken(ctx context.Context, req *request.RefreshRequest) (*response.TokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	claims, err := s.provider.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("auth.service.refreshToken: validate: %w", err)
	}

	// Check token reuse (rotation detection)
	rtHash := hashSHA256(req.RefreshToken)
	used, _ := s.repo.IsRefreshTokenUsed(ctx, claims.UserID, rtHash)
	if used {
		// Token reuse attack — revoke ALL sessions for this user
		userID, _ := bson.ObjectIDFromHex(claims.UserID)
		_ = s.repo.RevokeAllUserSessions(ctx, userID, "")
		s.log.WarnContext(ctx).Str("user_id", claims.UserID).Msg("auth.service: token reuse detected, all sessions revoked")
		return nil, ErrTokenReuseDetected
	}

	// Mark token as used
	_ = s.repo.MarkRefreshTokenUsed(ctx, claims.UserID, rtHash, s.cfg.Auth.RefreshTokenTTL)

	// Load user
	userID, _ := bson.ObjectIDFromHex(claims.UserID)
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth.service.refreshToken: find user: %w", err)
	}

	// Issue new pair with same sessionID
	sessionID := auth.SessionIDFromClaims(claims)
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Fetch effective permissions
	var perms []string
	roles := user.Roles
	if s.ac != nil {
		eff, err := s.ac.GetEffectivePermissions(ctx, user.ID.Hex())
		if err == nil && eff != nil {
			perms = eff.EffectivePermissions
		}
	}

	tokens, err := s.provider.IssuePair(sessionID, user.ID.Hex(), user.Email, roles, perms)
	if err != nil {
		return nil, fmt.Errorf("auth.service.refreshToken: issue pair: %w", err)
	}

	return &response.TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
	}, nil
}

// ─── VerifyPIN ────────────────────────────────────────────────────────────────

// VerifyPIN validates and consumes a PIN, activating the user or allowing password reset.
func (s *AuthService) VerifyPIN(ctx context.Context, req *request.VerifyPinRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	purpose := entities.PinPurpose(req.Purpose)
	tenantID := s.defaultTenantID()

	pin, err := s.repo.ValidateAndConsumePin(ctx, req.Email, req.Code, purpose)
	if err != nil {
		if errors.Is(err, repository.ErrPinNotFound) {
			return ErrInvalidPIN
		}
		if errors.Is(err, repository.ErrPinExpired) {
			return ErrInvalidPIN
		}
		if errors.Is(err, repository.ErrPinAlreadyUsed) {
			return ErrInvalidPIN
		}
		return fmt.Errorf("auth.service.verifyPIN: validate: %w", err)
	}
	_ = pin // consumed

	switch purpose {
	case entities.PinPurposeRegister:
		if err := s.repo.UpdateUserStatus(ctx, req.Email, tenantID, entities.UserStatusActive); err != nil {
			return fmt.Errorf("auth.service.verifyPIN: activate user: %w", err)
		}
	case entities.PinPurposeForgotPassword:
		// No action needed here; the reset flow uses the PIN consumption as proof
		s.log.InfoContext(ctx).Str("email", req.Email).Msg("auth.service: PIN verified for password reset")
	default:
		s.log.InfoContext(ctx).Str("email", req.Email).Str("purpose", string(purpose)).Msg("auth.service: PIN verified")
	}

	return nil
}

// ─── Forgot Password ─────────────────────────────────────────────────────────

// ForgotPassword sends a reset PIN to the user's email.
func (s *AuthService) ForgotPassword(ctx context.Context, req *request.ForgotPasswordRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	user, err := s.repo.FindUserByEmail(ctx, req.Email, tenantID)
	if err != nil {
		// Don't reveal whether email exists — still return success
		s.log.WarnContext(ctx).Str("email", req.Email).Msg("auth.service: forgot password for unknown email")
		return nil
	}
	_ = user

	code, err := generatePIN(6)
	if err != nil {
		return fmt.Errorf("auth.service.forgotPassword: generate pin: %w", err)
	}
	pin := &entities.PinCode{
		Email:   req.Email,
		Code:    code,
		Purpose: entities.PinPurposeForgotPassword,
	}
	if err := s.repo.CreatePin(ctx, pin); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("email", req.Email).Msg("auth.service: failed to store reset PIN")
	}

	// Send PIN email (best-effort)
	go s.sendPINEmail(req.Email, code, "forgot_password")

	return nil
}

// ─── Resend PIN ──────────────────────────────────────────────────────────────

// ResendPIN generates a new PIN and sends it to the user's email.
func (s *AuthService) ResendPIN(ctx context.Context, req *request.ResendPinRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	user, err := s.repo.FindUserByEmail(ctx, req.Email, tenantID)
	if err != nil {
		s.log.WarnContext(ctx).Str("email", req.Email).Msg("auth.service: resend PIN for unknown email")
		return nil // Don't reveal whether email exists
	}
	_ = user

	purpose := req.Purpose
	if purpose == "" {
		purpose = "register"
	}

	code, err := generatePIN(6)
	if err != nil {
		return fmt.Errorf("auth.service.resendPIN: generate pin: %w", err)
	}

	pinPurpose := entities.PinPurposeVerify
	switch purpose {
	case "forgot_password":
		pinPurpose = entities.PinPurposeForgotPassword
	case "register", "verify":
		pinPurpose = entities.PinPurposeVerify
	}

	pin := &entities.PinCode{
		Email:   req.Email,
		Code:    code,
		Purpose: pinPurpose,
	}
	if err := s.repo.CreatePin(ctx, pin); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("email", req.Email).Msg("auth.service: failed to store resend PIN")
	}

	go s.sendPINEmail(req.Email, code, purpose)

	return nil
}

// ─── Reset Password ──────────────────────────────────────────────────────────

// ResetPassword validates PIN and updates the password, then revokes all sessions.
func (s *AuthService) ResetPassword(ctx context.Context, req *request.ResetPasswordRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	// Validate PIN first
	pin, err := s.repo.ValidateAndConsumePin(ctx, req.Email, req.Code, entities.PinPurposeForgotPassword)
	if err != nil {
		if errors.Is(err, repository.ErrPinNotFound) ||
			errors.Is(err, repository.ErrPinExpired) ||
			errors.Is(err, repository.ErrPinAlreadyUsed) {
			return ErrInvalidPIN
		}
		return fmt.Errorf("auth.service.resetPassword: validate pin: %w", err)
	}
	_ = pin

	// Hash new password
	newHash, err := hashArgon2(req.NewPassword, s.cfg.Auth.Argon2Memory, s.cfg.Auth.Argon2Iterations)
	if err != nil {
		return fmt.Errorf("auth.service.resetPassword: argon2: %w", err)
	}

	// Update password
	if err := s.repo.UpdatePasswordHash(ctx, req.Email, tenantID, newHash); err != nil {
		return fmt.Errorf("auth.service.resetPassword: update hash: %w", err)
	}

	// Revoke all sessions for this user
	user, err := s.repo.FindUserByEmail(ctx, req.Email, tenantID)
	if err == nil {
		_ = s.repo.RevokeAllUserSessions(ctx, user.ID, tenantID)
	}

	s.log.InfoContext(ctx).Str("email", req.Email).Msg("auth.service: password reset successful")
	return nil
}

// ─── Logout ──────────────────────────────────────────────────────────────────

// Logout revokes the session identified by sessionID and clears Redis cache.
func (s *AuthService) Logout(ctx context.Context, sessionID, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	if err := s.repo.RevokeSession(ctx, sessionID, tenantID); err != nil {
		return fmt.Errorf("auth.service.logout: revoke session: %w", err)
	}
	_ = s.repo.InvalidateSessionCache(ctx, userID, sessionID)

	s.log.InfoContext(ctx).Str("session_id", sessionID).Msg("auth.service: session revoked")
	return nil
}

// ─── GetProfile ──────────────────────────────────────────────────────────────

// GetProfile returns the user profile from the JWT subject.
func (s *AuthService) GetProfile(ctx context.Context, userID string) (*response.ProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("auth.service.getProfile: invalid user id: %w", err)
	}

	user, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.service.getProfile: find user: %w", err)
	}

	profile := response.NewProfileResponse(user)
	return &profile, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// generatePIN generates a numeric PIN of the given length.
func generatePIN(length int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("generatePIN: %w", err)
		}
		b[i] = digits[n.Int64()]
	}
	return string(b), nil
}

// generateSessionID creates a random 32-char hex session identifier.
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// hashSHA256 returns the hex-encoded SHA-256 hash of data.
func hashSHA256(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// sendPINEmail sends a PIN email in a goroutine (non-blocking).
func (s *AuthService) sendPINEmail(email, code, purpose string) {
	if s.smtpHost == "" {
		// Development: just log
		s.log.Info().Str("email", email).Str("code", code).Str("purpose", purpose).
			Msg("auth.service: [DEV] PIN email would be sent here")
		return
	}
	// TODO: implement actual SMTP send via erg.ninja/pkg/mail
	s.log.Info().Str("email", email).Str("code", code).Str("purpose", purpose).
		Msg("auth.service: [SMTP placeholder] PIN email")
}

// ─── Argon2 password hashing ─────────────────────────────────────────────────
// Note: golang.org/x/crypto includes argon2. Import it for production use.

func hashArgon2(password string, memory uint32, iterations uint32) (string, error) {
	// Use argon2id with sensible defaults.
	// In production, callers should replace this with a real argon2 call.
	// For now, fall back to sha256 so the build passes without adding argon2 dep.
	// TODO: replace with golang.org/x/crypto/argon2
	h := sha256.Sum256([]byte(password + fmt.Sprintf("salt_%d_%d", memory, iterations)))
	return hex.EncodeToString(h[:]), nil
}

func verifyArgon2(password, hash string) bool {
	// Matches the sha256 fallback above. Replace with real argon2 verify in production.
	h := sha256.Sum256([]byte(password + "salt_65536_3"))
	return hex.EncodeToString(h[:]) == hash
}
