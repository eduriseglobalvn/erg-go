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
	"google.golang.org/api/idtoken"

	ac "erg.ninja/internal/modules/access_control/application/service"
	"erg.ninja/internal/modules/access_control/domain/policy"
	"erg.ninja/internal/modules/auth/api/request"
	"erg.ninja/internal/modules/auth/api/response"
	entities "erg.ninja/internal/modules/auth/domain/entity"
	"erg.ninja/internal/modules/auth/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/security/password"
	"erg.ninja/pkg/storage"
)

// Service errors surfaced to the HTTP layer.
var (
	ErrInvalidCredentials       = errors.New("auth.service: invalid credentials")
	ErrUserNotFound             = errors.New("auth.service: user not found")
	ErrEmailExists              = errors.New("auth.service: email already registered")
	ErrAccountLocked            = errors.New("auth.service: account locked")
	ErrEmailNotVerified         = errors.New("auth.service: email not verified")
	ErrGoogleBridgeForbidden    = errors.New("auth.service: google bridge request forbidden")
	ErrGoogleOAuthNotConfigured = errors.New("auth.service: google oauth client id is not configured")
	ErrGoogleIdentityInvalid    = errors.New("auth.service: invalid google identity")
	ErrTokenReuseDetected       = errors.New("auth.service: token reuse detected — all sessions revoked")
	ErrTooManyAttempts          = errors.New("auth.service: too many login attempts")
	ErrIPBlocked                = errors.New("auth.service: ip blocked")
	ErrGeoBlocked               = errors.New("auth.service: geo blocked")
	ErrInvalidPIN               = errors.New("auth.service: invalid or expired PIN")
	ErrInvalidToken             = errors.New("auth.service: invalid token")
	ErrSessionReplaced          = errors.New("auth.service: session replaced by another device")
	ErrInvalidOldPassword       = errors.New("auth.service: invalid old password")
	ErrStorageNotConfigured     = errors.New("auth.service: storage is not configured")
)

const defaultRootAdminEmail = "admin@erg.edu.vn"

const loginSessionPersistenceTimeout = 20 * time.Second

// AuthService is the top-level auth business logic.
type AuthService struct {
	repo          *repository.Repo
	provider      *auth.AuthServiceProvider
	log           *logger.Logger
	cfg           *config.Config
	redis         *cache.RedisClient
	ac            *ac.Service
	r2            *storage.R2Client
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
	R2     *storage.R2Client
}

// NewAuthService creates a new AuthService.
func NewAuthService(deps ServiceDeps) *AuthService {
	accessSecret := deps.Config.Auth.JWTSecret
	if accessSecret == "" {
		accessSecret = randomDevSecret()
	}
	refreshSecret := deps.Config.Auth.JWTRefreshSecret
	if refreshSecret == "" {
		refreshSecret = accessSecret + "-refresh"
	}
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
		r2:            deps.R2,
		smtpHost:      deps.Config.SMTP.Host,
		smtpPort:      deps.Config.SMTP.Port,
		smtpUser:      deps.Config.SMTP.Username,
		smtpPass:      deps.Config.SMTP.Password,
		smtpFrom:      deps.Config.SMTP.From,
		adminEmail:    deps.Config.Auth.AdminEmail,
		adminPassword: deps.Config.Auth.AdminPassword,
	}
}

func randomDevSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("auth.service: generate development JWT secret: %w", err))
	}
	return hex.EncodeToString(b)
}

// BootstrapAdmin ensures the configured super-admin account exists and is usable.
// Idempotent: existing admin accounts are repaired instead of being recreated.
func (s *AuthService) BootstrapAdmin() error {
	adminEmail := s.rootAdminEmail()
	if adminEmail == "" {
		return nil // not configured, skip
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tenantID := s.defaultTenantID()

	existing, err := s.repo.FindUserByEmail(ctx, adminEmail, tenantID)
	if err == nil && existing != nil {
		updates := map[string]any{}
		roles := ensureAdminRoles(existing.Roles)

		if existing.Status != entities.UserStatusActive {
			updates["status"] = string(entities.UserStatusActive)
		}
		if strings.TrimSpace(existing.Provider) == "" {
			updates["provider"] = "local"
		}
		if strings.TrimSpace(existing.AccountType) == "" {
			updates["account_type"] = "erg"
		}
		if strings.TrimSpace(existing.FullName) == "" {
			updates["full_name"] = "Super Administrator"
		}
		if !existing.IsProfileCompleted {
			updates["is_profile_completed"] = true
		}

		repaired := false
		if len(updates) > 0 {
			if err := s.repo.UpdateUserIdentityFields(ctx, existing.ID, updates); err != nil {
				return fmt.Errorf("auth.service.BootstrapAdmin: update admin identity: %w", err)
			}
			repaired = true
		}
		if !sameStringSet(existing.Roles, roles) {
			if err := s.repo.UpdateUserRoles(ctx, existing.ID, roles); err != nil {
				return fmt.Errorf("auth.service.BootstrapAdmin: update admin roles: %w", err)
			}
			repaired = true
		}
		if repaired {
			s.log.Warn().Str("email", adminEmail).Msg("auth.service: admin user metadata was incomplete and has been repaired")
		}
		return nil
	}
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return fmt.Errorf("auth.service.BootstrapAdmin: find admin: %w", err)
	}
	if strings.TrimSpace(s.adminPassword) == "" {
		return nil // only repair is possible without a configured password
	}

	// Create admin account
	passwordHash, err := s.hashPassword(s.adminPassword)
	if err != nil {
		return fmt.Errorf("auth.service.BootstrapAdmin: hash password: %w", err)
	}

	admin := &entities.User{
		Email:              adminEmail,
		PasswordHash:       passwordHash,
		FullName:           "Super Administrator",
		Status:             entities.UserStatusActive, // no PIN verification needed
		Provider:           "local",
		AccountType:        "erg",
		Roles:              ensureAdminRoles(nil),
		TenantID:           tenantID,
		IsProfileCompleted: true,
	}

	if err := s.repo.CreateUser(ctx, admin); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return nil // race condition — already created by another goroutine
		}
		return fmt.Errorf("auth.service.BootstrapAdmin: create user: %w", err)
	}

	s.log.Info().Str("email", adminEmail).Msg("auth.service: super-admin account bootstrapped")
	return nil
}

func ensureAdminRoles(roles []string) []string {
	roles = uniqueStrings(roles)
	seen := make(map[string]bool, len(roles)+4)
	for _, role := range roles {
		seen[role] = true
	}
	for _, required := range []string{"admin", "SUPER_ADMIN", policy.RoleSystemSuperAdmin, policy.RoleERGSuperAdmin} {
		if !seen[required] {
			roles = append(roles, required)
		}
	}
	return uniqueStrings(roles)
}

func sameStringSet(a, b []string) bool {
	a = uniqueStrings(a)
	b = uniqueStrings(b)
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, item := range a {
		seen[item] = true
	}
	for _, item := range b {
		if !seen[item] {
			return false
		}
	}
	return true
}

// defaultTenantID returns the configured default tenant ID.
func (s *AuthService) defaultTenantID() string {
	if s.cfg.Tenant.DefaultID != "" {
		return s.cfg.Tenant.DefaultID
	}
	return "default"
}

func (s *AuthService) isConfiguredAdminEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	return strings.EqualFold(email, s.rootAdminEmail()) || strings.EqualFold(email, defaultRootAdminEmail)
}

func (s *AuthService) rootAdminEmail() string {
	if configured := strings.TrimSpace(s.adminEmail); configured != "" {
		return strings.ToLower(configured)
	}
	return defaultRootAdminEmail
}

func (s *AuthService) repairConfiguredAdmin(ctx context.Context, user *entities.User) error {
	if user == nil || !s.isConfiguredAdminEmail(user.Email) {
		return nil
	}

	updates := map[string]any{}
	if !strings.EqualFold(string(user.Status), string(entities.UserStatusActive)) {
		updates["status"] = string(entities.UserStatusActive)
		user.Status = entities.UserStatusActive
	}
	if strings.TrimSpace(user.Provider) == "" {
		updates["provider"] = "local"
		user.Provider = "local"
	}
	if strings.TrimSpace(user.AccountType) == "" {
		updates["account_type"] = "erg"
		user.AccountType = "erg"
	}
	if strings.TrimSpace(user.FullName) == "" {
		updates["full_name"] = "Super Administrator"
		user.FullName = "Super Administrator"
	}
	if !user.IsProfileCompleted {
		updates["is_profile_completed"] = true
		user.IsProfileCompleted = true
	}
	if len(updates) > 0 {
		if err := s.repo.UpdateUserIdentityFields(ctx, user.ID, updates); err != nil {
			return err
		}
	}

	roles := ensureAdminRoles(user.Roles)
	if !sameStringSet(user.Roles, roles) {
		if err := s.repo.UpdateUserRoles(ctx, user.ID, roles); err != nil {
			return err
		}
		user.Roles = roles
	}
	return nil
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

	passwordHash, err := s.hashPassword(req.Password)
	if err != nil {
		return fmt.Errorf("auth.service.register: hash password: %w", err)
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
func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest, sec LoginSecurityContext) (*response.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	loginStarted := time.Now()
	stepStarted := loginStarted
	var precheckDur, findUserDur, adminRepairDur, passwordVerifyDur, successAuditScheduleDur, issueTokensDur time.Duration

	tenantID := s.defaultTenantID()
	sec = s.normalizeSecurityContext(sec)

	loginIdentifier := strings.TrimSpace(req.Email)
	loginEmail := canonicalLoginEmail(loginIdentifier)

	if reason, err := s.precheckLoginSecurity(ctx, tenantID, loginEmail, sec); err != nil {
		s.recordLoginAttempt(ctx, tenantID, "", loginEmail, entities.LoginAttemptBlocked, reason, sec)
		return nil, err
	}
	precheckDur = time.Since(stepStarted)
	stepStarted = time.Now()

	user, err := s.repo.FindUserByEmail(ctx, loginEmail, tenantID)
	findUserDur = time.Since(stepStarted)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			if thresholdErr := s.recordFailedLogin(ctx, tenantID, loginEmail, sec); thresholdErr != nil {
				s.recordLoginAttempt(ctx, tenantID, "", loginEmail, entities.LoginAttemptBlocked, entities.LoginAttemptReasonTooManyAttempts, sec)
				return nil, thresholdErr
			}
			s.recordLoginAttempt(ctx, tenantID, "", loginEmail, entities.LoginAttemptFailed, entities.LoginAttemptReasonInvalidCredentials, sec)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth.service.login: find user: %w", err)
	}

	if s.isConfiguredAdminEmail(user.Email) {
		stepStarted = time.Now()
		if err := s.repairConfiguredAdmin(ctx, user); err != nil {
			return nil, fmt.Errorf("auth.service.login: repair configured admin: %w", err)
		}
		adminRepairDur = time.Since(stepStarted)
	}

	if user.Status == entities.UserStatusBlocked || user.Status == entities.UserStatusBanned {
		s.recordLoginAttempt(ctx, tenantID, user.ID.Hex(), loginEmail, entities.LoginAttemptBlocked, entities.LoginAttemptReasonAccountLocked, sec)
		return nil, ErrAccountLocked
	}

	if user.Status == entities.UserStatusPending && user.Provider == "local" {
		s.recordLoginAttempt(ctx, tenantID, user.ID.Hex(), loginEmail, entities.LoginAttemptBlocked, entities.LoginAttemptReasonEmailNotVerified, sec)
		return nil, ErrEmailNotVerified
	}

	// Verify password
	stepStarted = time.Now()
	passwordOK, passwordNeedsRehash := s.verifyPassword(req.Password, user.PasswordHash)
	if !passwordOK {
		passwordVerifyDur = time.Since(stepStarted)
		if thresholdErr := s.recordFailedLogin(ctx, tenantID, loginEmail, sec); thresholdErr != nil {
			s.recordLoginAttempt(ctx, tenantID, user.ID.Hex(), loginEmail, entities.LoginAttemptBlocked, entities.LoginAttemptReasonTooManyAttempts, sec)
			return nil, thresholdErr
		}
		s.recordLoginAttempt(ctx, tenantID, user.ID.Hex(), loginEmail, entities.LoginAttemptFailed, entities.LoginAttemptReasonInvalidCredentials, sec)
		return nil, ErrInvalidCredentials
	}
	passwordVerifyDur = time.Since(stepStarted)
	if passwordNeedsRehash {
		s.rehashUserPasswordAsync(ctx, user, req.Password)
	}

	stepStarted = time.Now()
	authResponse, err := s.issueTokensForUser(ctx, user, sec, "erg")
	issueTokensDur = time.Since(stepStarted)
	if err != nil {
		return nil, err
	}
	stepStarted = time.Now()
	s.recordSuccessfulLoginAsync(ctx, tenantID, user.ID.Hex(), loginEmail, sec)
	successAuditScheduleDur = time.Since(stepStarted)
	s.log.DebugContext(ctx).
		Str("user_id", user.ID.Hex()).
		Dur("total", time.Since(loginStarted)).
		Dur("precheck", precheckDur).
		Dur("find_user", findUserDur).
		Dur("admin_repair", adminRepairDur).
		Dur("password_verify", passwordVerifyDur).
		Dur("issue_tokens", issueTokensDur).
		Dur("success_audit_schedule", successAuditScheduleDur).
		Msg("auth.service: login completed")
	return authResponse, nil
}

// GoogleLogin exchanges a Google ID token, or a trusted legacy bridge payload, for erg-go tokens.
func (s *AuthService) GoogleLogin(ctx context.Context, req *request.GoogleLoginRequest, sec LoginSecurityContext) (*response.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if strings.TrimSpace(req.IDToken) != "" {
		identity, err := s.verifyGoogleIDToken(ctx, req.IDToken)
		if err != nil {
			return nil, err
		}
		req.Email = identity.Email
		req.FullName = identity.FullName
		req.AvatarURL = identity.AvatarURL
		req.GoogleSub = identity.GoogleSub
		req.EmailVerified = identity.EmailVerified
	}

	email := strings.TrimSpace(req.Email)
	googleSub := strings.TrimSpace(req.GoogleSub)
	tenantID := s.defaultTenantID()
	sec = s.normalizeSecurityContext(sec)

	if reason, err := s.precheckLoginSecurity(ctx, tenantID, email, sec); err != nil {
		s.recordLoginAttempt(ctx, tenantID, "", email, entities.LoginAttemptBlocked, reason, sec)
		return nil, err
	}

	if email == "" || googleSub == "" || !req.EmailVerified {
		s.recordLoginAttempt(ctx, tenantID, "", email, entities.LoginAttemptFailed, entities.LoginAttemptReasonGoogleInvalid, sec)
		return nil, ErrGoogleIdentityInvalid
	}

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
		passwordHash, hashErr := s.hashPassword(generateSessionID())
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
			IsProfileCompleted:  isProfileComplete(strings.TrimSpace(req.FullName), ""),
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
		s.recordLoginAttempt(ctx, tenantID, user.ID.Hex(), email, entities.LoginAttemptBlocked, entities.LoginAttemptReasonAccountLocked, sec)
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
	nextProfileCompleted := isProfileComplete(user.FullName, user.Phone)
	updates["is_profile_completed"] = nextProfileCompleted
	user.IsProfileCompleted = nextProfileCompleted

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

	authResponse, err := s.issueTokensForUser(ctx, user, sec, "google")
	if err != nil {
		return nil, err
	}
	s.recordSuccessfulLoginAsync(ctx, tenantID, user.ID.Hex(), email, sec)
	return authResponse, nil
}

// issueTokensForUser creates a session and returns token pair.
func (s *AuthService) issueTokensForUser(ctx context.Context, user *entities.User, sec LoginSecurityContext, loginProvider string) (*response.AuthResponse, error) {
	issueStarted := time.Now()
	stepStarted := issueStarted
	var permissionsDur, issuePairDur, replaceSessionsDur, createSessionDur, touchLoginDur time.Duration

	sessionID := generateSessionID()
	sec = s.normalizeSecurityContext(sec)
	if sec.DeviceID == "" {
		sec.DeviceID = sessionID
	}

	roles := user.Roles
	perms := s.permissionsForToken(ctx, user)
	portals := portalsForToken(roles)
	accountType, accessLevel := classifyAccountAccess(user, portals, perms)
	permissionsDur = time.Since(stepStarted)

	stepStarted = time.Now()
	tokens, err := s.provider.IssuePair(
		sessionID,
		user.ID.Hex(),
		user.Email,
		roles,
		perms,
		auth.WithPortals(portals),
		auth.WithAccountAccess(accountType, accessLevel),
	)
	issuePairDur = time.Since(stepStarted)
	if err != nil {
		return nil, fmt.Errorf("auth.service.issueTokensForUser: issue pair: %w", err)
	}

	// Store refresh token hash for rotation detection
	rtHash := hashSHA256(tokens.RefreshToken)
	_ = rtHash // Just hash it for now, or remove if not needed here

	sessionCtx, sessionCancel := loginSessionPersistenceContext(ctx)
	defer sessionCancel()

	// Create session record
	refreshHash := hashSHA256(tokens.RefreshToken)
	session := &entities.UserSession{
		UserID:       user.ID,
		SessionID:    sessionID,
		DeviceID:     sec.DeviceID,
		DeviceName:   sec.DeviceName,
		IPAddress:    sec.IPAddress,
		UserAgent:    sec.UserAgent,
		RefreshToken: refreshHash,
		TenantID:     user.TenantID,
		ExpiresAt:    time.Now().Add(s.cfg.Auth.RefreshTokenTTL),
	}
	stepStarted = time.Now()
	if err := s.repo.CreateSession(sessionCtx, session); err != nil {
		return nil, fmt.Errorf("auth.service.issueTokensForUser: create session: %w", err)
	}
	createSessionDur = time.Since(stepStarted)

	stepStarted = time.Now()
	s.replaceExistingLoginSessionsAsync(sessionCtx, user, sessionID)
	replaceSessionsDur = time.Since(stepStarted)
	stepStarted = time.Now()
	s.touchSuccessfulLoginAsync(ctx, user, loginProvider)
	touchLoginDur = time.Since(stepStarted)
	s.log.DebugContext(sessionCtx).
		Str("user_id", user.ID.Hex()).
		Str("provider", loginProvider).
		Dur("total", time.Since(issueStarted)).
		Dur("effective_permissions", permissionsDur).
		Dur("issue_pair", issuePairDur).
		Dur("replace_sessions", replaceSessionsDur).
		Dur("create_session", createSessionDur).
		Dur("touch_login", touchLoginDur).
		Msg("auth.service: issue tokens completed")

	profile := response.NewProfileResponse(user)
	profile.AccountType = accountType
	profile.AccessLevel = accessLevel

	return &response.AuthResponse{
		User:         profile,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
		Session:      response.NewSessionDeviceDTO(session, true),
		Permissions:  perms,
		Portals:      portals,
		AccountType:  accountType,
		AccessLevel:  accessLevel,
	}, nil
}

func loginSessionPersistenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), loginSessionPersistenceTimeout)
}

func (s *AuthService) replaceExistingLoginSessionsAsync(parent context.Context, user *entities.User, keepSessionID string) {
	if user == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
		defer cancel()
		if err := s.replaceExistingLoginSessions(ctx, user, keepSessionID); err != nil {
			s.log.WarnContext(ctx).Err(err).Str("user_id", user.ID.Hex()).Msg("auth.service: failed to revoke previous login sessions")
		}
	}()
}

func (s *AuthService) replaceExistingLoginSessions(ctx context.Context, user *entities.User, keepSessionID string) error {
	sessions, err := s.repo.FindActiveSessions(ctx, user.ID, user.TenantID)
	if err != nil {
		return err
	}
	sessionIDs := make([]string, 0, len(sessions))
	for i := range sessions {
		if strings.TrimSpace(sessions[i].SessionID) != strings.TrimSpace(keepSessionID) {
			sessionIDs = append(sessionIDs, sessions[i].SessionID)
		}
	}
	_ = s.repo.InvalidateSessionCaches(ctx, user.ID.Hex(), sessionIDs)
	return s.repo.RevokeOtherActiveUserSessionsWithReason(ctx, user.ID, user.TenantID, keepSessionID, "replaced_by_new_login")
}

func (s *AuthService) recordSuccessfulLoginAsync(parent context.Context, tenantID, userID, email string, sec LoginSecurityContext) {
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
		defer cancel()
		s.resetFailedLoginCounters(ctx, tenantID, email, sec.IPAddress)
		s.recordLoginAttempt(ctx, tenantID, userID, email, entities.LoginAttemptSuccess, entities.LoginAttemptReasonSuccess, sec)
	}()
}

func (s *AuthService) touchSuccessfulLoginAsync(parent context.Context, user *entities.User, loginProvider string) {
	if user == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
		defer cancel()
		if err := s.repo.TouchSuccessfulLogin(ctx, user.ID, loginProvider); err != nil {
			s.log.WarnContext(ctx).Err(err).Str("user_id", user.ID.Hex()).Msg("auth.service: failed to update login metadata")
		}
	}()
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.LoginCount++
	user.LastLoginProvider = loginProvider
}

func (s *AuthService) permissionsForToken(ctx context.Context, user *entities.User) []string {
	if user == nil {
		return nil
	}
	if grantsAllPermissions(user.Roles) {
		return []string{"*"}
	}
	if s.ac == nil {
		return nil
	}
	eff, err := s.ac.GetEffectivePermissions(ctx, user.ID.Hex())
	if err != nil || eff == nil {
		if err != nil {
			s.log.WarnContext(ctx).Err(err).Str("user_id", user.ID.Hex()).Msg("auth.service: failed to resolve permissions for token")
		}
		return nil
	}
	return eff.EffectivePermissions
}

func grantsAllPermissions(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(role, "admin") || strings.EqualFold(role, "SUPER_ADMIN") || strings.EqualFold(role, policy.RoleSystemSuperAdmin) || strings.EqualFold(role, policy.RoleERGSuperAdmin) {
			return true
		}
	}
	return false
}

func canonicalLoginEmail(identifier string) string {
	identifier = strings.TrimSpace(strings.ToLower(identifier))
	if identifier == "" || strings.Contains(identifier, "@") {
		return identifier
	}
	return studentAuthEmail(identifier)
}

func studentAuthEmail(username string) string {
	return strings.TrimSpace(strings.ToLower(username)) + "@student.erg.edu.vn"
}

func portalsForToken(roles []string) []string {
	portals := policy.PortalsForRoles(roles)
	if len(portals) == 0 {
		return nil
	}
	out := make([]string, 0, len(portals))
	for _, portal := range portals {
		out = append(out, string(portal))
	}
	return out
}

func classifyAccountAccess(user *entities.User, portals []string, permissions []string) (string, string) {
	if user == nil {
		return "community", "community_only"
	}
	if grantsAllPermissions(user.Roles) || containsPortal(portals, "*") || containsPermission(permissions, "*") {
		return "admin", "full"
	}
	if containsPortal(portals, "cms") {
		return "staff", "cms"
	}
	if containsPortal(portals, "elearning") && !containsPortal(portals, "lms") && !containsPortal(portals, "hoclieu") {
		return "student", "elearning"
	}
	if containsPortal(portals, "lms") && containsPortal(portals, "hoclieu") {
		return "teacher", "full"
	}
	if containsPortal(portals, "lms") {
		return "teacher", "lms"
	}
	if containsPortal(portals, "hoclieu") {
		return "teacher", "hoclieu"
	}
	return "community", "community_only"
}

func containsPortal(portals []string, portal string) bool {
	for _, item := range portals {
		if strings.EqualFold(strings.TrimSpace(item), portal) {
			return true
		}
	}
	return false
}

func containsPermission(permissions []string, permission string) bool {
	for _, item := range permissions {
		if strings.EqualFold(strings.TrimSpace(item), permission) {
			return true
		}
	}
	return false
}

type googleIdentity struct {
	Email         string
	FullName      string
	AvatarURL     string
	GoogleSub     string
	EmailVerified bool
}

func (s *AuthService) verifyGoogleIDToken(ctx context.Context, rawToken string) (*googleIdentity, error) {
	clientIDs := s.googleClientIDs()
	if len(clientIDs) == 0 {
		return nil, ErrGoogleOAuthNotConfigured
	}

	var payload *idtoken.Payload
	var lastErr error
	for _, clientID := range clientIDs {
		verified, err := idtoken.Validate(ctx, rawToken, clientID)
		if err == nil {
			payload = verified
			break
		}
		lastErr = err
	}
	if payload == nil {
		if lastErr != nil {
			s.log.WarnContext(ctx).Err(lastErr).Msg("auth.service: google id token validation failed")
		}
		return nil, ErrGoogleIdentityInvalid
	}

	email, _ := payload.Claims["email"].(string)
	fullName, _ := payload.Claims["name"].(string)
	avatarURL, _ := payload.Claims["picture"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)

	if strings.TrimSpace(payload.Subject) == "" || strings.TrimSpace(email) == "" || !emailVerified {
		return nil, ErrGoogleIdentityInvalid
	}

	return &googleIdentity{
		Email:         strings.ToLower(strings.TrimSpace(email)),
		FullName:      strings.TrimSpace(fullName),
		AvatarURL:     strings.TrimSpace(avatarURL),
		GoogleSub:     strings.TrimSpace(payload.Subject),
		EmailVerified: emailVerified,
	}, nil
}

func (s *AuthService) googleClientIDs() []string {
	if s == nil || s.cfg == nil {
		return nil
	}
	values := make([]string, 0, 1+len(s.cfg.Auth.GoogleClientIDs))
	if clientID := strings.TrimSpace(s.cfg.Auth.GoogleClientID); clientID != "" {
		values = append(values, clientID)
	}
	for _, clientID := range s.cfg.Auth.GoogleClientIDs {
		if trimmed := strings.TrimSpace(clientID); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return uniqueStrings(values)
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
	session, err := s.repo.FindSessionByID(ctx, sessionID, user.TenantID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrSessionReplaced
		}
		return nil, fmt.Errorf("auth.service.refreshToken: find session: %w", err)
	}
	if session.IsRevoked() || time.Now().UTC().After(session.ExpiresAt) {
		return nil, ErrSessionReplaced
	}
	if session.RefreshToken != rtHash {
		_ = s.repo.RevokeAllUserSessionsWithReason(ctx, user.ID, user.TenantID, "refresh_token_reuse")
		s.log.WarnContext(ctx).Str("user_id", claims.UserID).Str("session_id", sessionID).Msg("auth.service: stale refresh token hash detected, all sessions revoked")
		return nil, ErrTokenReuseDetected
	}
	if strings.TrimSpace(req.DeviceID) != "" && session.DeviceID != "" && strings.TrimSpace(req.DeviceID) != session.DeviceID {
		return nil, ErrSessionReplaced
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

	portals := portalsForToken(roles)
	accountType, accessLevel := classifyAccountAccess(user, portals, perms)
	tokens, err := s.provider.IssuePair(
		sessionID,
		user.ID.Hex(),
		user.Email,
		roles,
		perms,
		auth.WithPortals(portals),
		auth.WithAccountAccess(accountType, accessLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("auth.service.refreshToken: issue pair: %w", err)
	}
	nextRefreshHash := hashSHA256(tokens.RefreshToken)
	if err := s.repo.RotateSessionRefreshToken(ctx, sessionID, user.TenantID, rtHash, nextRefreshHash, time.Now().Add(s.cfg.Auth.RefreshTokenTTL)); err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			_ = s.repo.RevokeAllUserSessionsWithReason(ctx, user.ID, user.TenantID, "refresh_token_race")
			return nil, ErrTokenReuseDetected
		}
		return nil, fmt.Errorf("auth.service.refreshToken: rotate refresh hash: %w", err)
	}
	_ = s.repo.InvalidateSessionCache(ctx, user.ID.Hex(), sessionID)

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
	newHash, err := s.hashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("auth.service.resetPassword: hash password: %w", err)
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

// ValidateActiveSession ensures a JWT session is still active before serving protected routes.
func (s *AuthService) ValidateActiveSession(ctx context.Context, userID string, sessionID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" {
		return ErrInvalidToken
	}

	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrInvalidToken
	}

	user, err := s.repo.FindUserByID(ctx, userObjectID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrInvalidToken
		}
		return fmt.Errorf("auth.service.validateActiveSession: find user: %w", err)
	}

	session, err := s.repo.FindSessionByID(ctx, sessionID, user.TenantID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return ErrSessionReplaced
		}
		return fmt.Errorf("auth.service.validateActiveSession: find session: %w", err)
	}
	if session.IsRevoked() || time.Now().UTC().After(session.ExpiresAt) {
		return ErrSessionReplaced
	}

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

	if s.isConfiguredAdminEmail(user.Email) {
		if err := s.repairConfiguredAdmin(ctx, user); err != nil {
			return nil, fmt.Errorf("auth.service.getProfile: repair configured admin: %w", err)
		}
	}

	perms := s.permissionsForToken(ctx, user)
	portals := portalsForToken(user.Roles)
	accountType, accessLevel := classifyAccountAccess(user, portals, perms)
	profile := response.NewProfileResponse(user)
	profile.AccountType = accountType
	profile.AccessLevel = accessLevel
	return &profile, nil
}

// AccountProfileUpdate is the subset of profile fields supported by the LMS auth compatibility API.
type AccountProfileUpdate struct {
	FullName    *string
	AvatarURL   *string
	Phone       *string
	Bio         *string
	Gender      *string
	DateOfBirth *string
	Address     *string
	City        *string
	District    *string
	JobTitle    *string
	Region      *string
	SocialLinks *map[string]string
}

// UpdateProfile updates the current user's profile and returns the refreshed profile.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req AccountProfileUpdate) (*response.ProfileResponse, error) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("auth.service.updateProfile: invalid user id: %w", err)
	}
	user, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.service.updateProfile: find user: %w", err)
	}

	updates := make(map[string]any)
	addStringUpdate(updates, "full_name", req.FullName)
	addStringUpdate(updates, "avatar_url", req.AvatarURL)
	addStringUpdate(updates, "phone", req.Phone)
	addStringUpdate(updates, "bio", req.Bio)
	addStringUpdate(updates, "gender", req.Gender)
	addStringUpdate(updates, "date_of_birth", req.DateOfBirth)
	addStringUpdate(updates, "address", req.Address)
	addStringUpdate(updates, "city", req.City)
	addStringUpdate(updates, "district", req.District)
	addStringUpdate(updates, "job_title", req.JobTitle)
	addStringUpdate(updates, "region", req.Region)
	if req.SocialLinks != nil {
		updates["social_links"] = *req.SocialLinks
	}
	if len(updates) == 0 {
		return s.GetProfile(ctx, userID)
	}
	nextFullName := user.FullName
	nextPhone := user.Phone
	if req.FullName != nil {
		nextFullName = strings.TrimSpace(*req.FullName)
	}
	if req.Phone != nil {
		nextPhone = strings.TrimSpace(*req.Phone)
	}
	updates["is_profile_completed"] = isProfileComplete(nextFullName, nextPhone)

	if err := s.repo.UpdateUserIdentityFields(ctx, id, updates); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.service.updateProfile: %w", err)
	}
	return s.GetProfile(ctx, userID)
}

// UploadAvatar stores a normalized avatar image in R2 and updates the user's profile.
func (s *AuthService) UploadAvatar(ctx context.Context, userID string, filename string, body []byte) (*response.ProfileResponse, error) {
	if s.r2 == nil {
		return nil, ErrStorageNotConfigured
	}
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("auth.service.uploadAvatar: invalid user id: %w", err)
	}
	if _, err := s.repo.FindUserByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.service.uploadAvatar: find user: %w", err)
	}

	avatarURL, err := s.r2.UploadAvatar(ctx, body, userID, filename)
	if err != nil {
		return nil, fmt.Errorf("auth.service.uploadAvatar: %w", err)
	}
	if err := s.repo.UpdateUserIdentityFields(ctx, id, map[string]any{"avatar_url": avatarURL}); err != nil {
		_ = s.r2.Delete(ctx, avatarURL)
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.service.uploadAvatar: update avatar url: %w", err)
	}
	return s.GetProfile(ctx, userID)
}

// ChangePassword validates the current password and replaces it with the new password.
func (s *AuthService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" || len(newPassword) < 8 {
		return fmt.Errorf("auth.service.changePassword: new password is too short")
	}
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("auth.service.changePassword: invalid user id: %w", err)
	}
	user, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("auth.service.changePassword: find user: %w", err)
	}
	passwordOK, _ := s.verifyPassword(oldPassword, user.PasswordHash)
	if !passwordOK {
		return ErrInvalidOldPassword
	}
	newHash, err := s.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("auth.service.changePassword: hash password: %w", err)
	}
	if err := s.repo.UpdatePasswordHash(ctx, user.Email, user.TenantID, newHash); err != nil {
		return fmt.Errorf("auth.service.changePassword: update password: %w", err)
	}
	return nil
}

func addStringUpdate(updates map[string]any, field string, value *string) {
	if value == nil {
		return
	}
	updates[field] = strings.TrimSpace(*value)
}

func isProfileComplete(fullName, phone string) bool {
	return strings.TrimSpace(fullName) != "" && strings.TrimSpace(phone) != ""
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

func (s *AuthService) passwordParams() password.Params {
	if s == nil || s.cfg == nil {
		return password.NormalizeParams(0, 0)
	}
	return password.NormalizeParams(s.cfg.Auth.Argon2Memory, s.cfg.Auth.Argon2Iterations)
}

func (s *AuthService) hashPassword(plain string) (string, error) {
	return password.Hash(plain, s.passwordParams())
}

func (s *AuthService) verifyPassword(plain, stored string) (bool, bool) {
	return password.Verify(plain, stored, s.passwordParams())
}

func (s *AuthService) rehashUserPassword(ctx context.Context, user *entities.User, plain string) error {
	if user == nil {
		return nil
	}
	hash, err := s.hashPassword(plain)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordHash(ctx, user.Email, user.TenantID, hash)
}

func (s *AuthService) rehashUserPasswordAsync(parent context.Context, user *entities.User, plain string) {
	if user == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
		defer cancel()
		if err := s.rehashUserPassword(ctx, user, plain); err != nil {
			if s.log != nil {
				s.log.WarnContext(ctx).Err(err).Str("user_id", user.ID.Hex()).Msg("auth.service: failed to upgrade password hash")
			}
		}
	}()
}
