package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/auth/api/request"
	authr "erg.ninja/internal/modules/auth/api/response"
	"erg.ninja/internal/modules/auth/application/service"
	authrepo "erg.ninja/internal/modules/auth/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

// Controller holds the auth HTTP controller dependencies.
type Controller struct {
	svc       *service.AuthService
	validator *auth.JWTValidator
	log       *logger.Logger
	cfg       *config.Config
	val       *validator.Validate
}

// NewController creates a new auth controller.
func NewController(svc *service.AuthService, validator *auth.JWTValidator, log *logger.Logger, cfg *config.Config) *Controller {
	return &Controller{svc: svc, validator: validator, log: log, cfg: cfg, val: validatorv10()}
}

func validatorv10() *validator.Validate {
	return validator.New()
}

// RegisterRoutes mounts all auth routes onto the Gin router.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	c.registerRoutesAt(r.Group("/api/auth"), false)
	c.registerRoutesAt(r.Group("/api/lms/auth"), true)
}

func (c *Controller) registerRoutesAt(api *gin.RouterGroup, lmsCompat bool) {
	// Public routes
	api.POST("/register", c.handleRegister)
	api.POST("/login", c.handleLogin)
	api.POST("/refresh", c.handleRefresh)
	api.POST("/verify-pin", c.handleVerifyPIN)
	api.POST("/resend-pin", c.handleResendPIN)
	api.POST("/forgot-password", c.handleForgotPassword)
	api.POST("/reset-password", c.handleResetPassword)
	if lmsCompat {
		api.POST("/providers/google", c.handleGoogleLogin)
		api.POST("/providers/apple", c.handleProviderNotImplemented)
	} else {
		api.POST("/google/login", c.handleGoogleLogin)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(c.jwtMiddleware())
	protected.POST("/logout", c.handleLogout)
	protected.GET("/profile", c.handleProfile)
	protected.GET("/login-attempts", c.handleLoginAttempts)
	protected.GET("/security/ip-status", c.handleIPSecurityStatus)
	if lmsCompat {
		protected.GET("/sessions", c.handleSessionsCompat)
		protected.PUT("/accounts/:id/profile", c.handleAccountProfileNotImplemented)
		protected.POST("/accounts/:id/avatar", c.handleAccountAvatarUpload)
		protected.PUT("/accounts/:id/password", c.handleAccountPasswordNotImplemented)
	}
}

// jwtMiddleware extracts and validates the JWT from the Authorization header.
func (c *Controller) jwtMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.validator == nil {
			writeError(ctx, http.StatusUnauthorized, authr.CodeInvalidToken, "JWT validator is not configured")
			ctx.Abort()
			return
		}
		if auth.UsesCookieAuth(ctx.Request, ctrlAccessCookieName(c.cfg)) && auth.UnsafeMethod(ctx.Request.Method) {
			if err := auth.ValidateCSRF(ctx.Request); err != nil {
				writeError(ctx, http.StatusForbidden, authr.CodeInvalidToken, "missing or invalid CSRF token")
				ctx.Abort()
				return
			}
		}
		authHeader := auth.AuthorizationHeaderFromRequest(ctx.Request, ctrlAccessCookieName(c.cfg))
		if authHeader == "" {
			writeError(ctx, http.StatusUnauthorized, authr.CodeInvalidToken, "missing authorization header")
			ctx.Abort()
			return
		}

		claims, err := c.validator.ValidateRequest(authHeader)
		if err != nil {
			writeError(ctx, http.StatusUnauthorized, authr.CodeInvalidToken, "invalid or expired token")
			ctx.Abort()
			return
		}

		// Inject user ID into context
		newCtx := context.WithValue(ctx.Request.Context(), ctxKeyUserID, claims.UserID)
		newCtx = context.WithValue(newCtx, ctxKeySessionID, auth.SessionIDFromClaims(claims))
		newCtx = context.WithValue(newCtx, ctxKeyRoles, claims.Roles)
		if err := c.svc.ValidateActiveSession(newCtx, claims.UserID, auth.SessionIDFromClaims(claims)); err != nil {
			if errors.Is(err, service.ErrSessionReplaced) {
				writeError(ctx, http.StatusUnauthorized, authr.CodeAuthSessionReplaced, "session was replaced by a newer login")
			} else {
				writeError(ctx, http.StatusUnauthorized, authr.CodeInvalidToken, "session is no longer active")
			}
			ctx.Abort()
			return
		}
		ctx.Request = ctx.Request.WithContext(newCtx)
		ctx.Next()
	}
}

// context keys
type ctxKey string

const (
	ctxKeyUserID    ctxKey = "auth_user_id"
	ctxKeySessionID ctxKey = "auth_session_id"
	ctxKeyRoles     ctxKey = "auth_roles"
)

// userIDFromContext extracts the authenticated user ID from context.
func userIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUserID).(string); ok {
		return v
	}
	return ""
}

// sessionIDFromContext extracts the session ID from context.
func sessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeySessionID).(string); ok {
		return v
	}
	return ""
}

func rolesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(ctxKeyRoles).([]string); ok {
		return v
	}
	return nil
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// handleRegister registers a new user.
// @Summary Register
// @Description Register a new user account.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.RegisterRequest true "Registration Data"
// @Success 201 {object} map[string]string
// @Failure 400 {object} authr.ErrorResponse
// @Failure 409 {object} authr.ErrorResponse
// @Router /api/auth/register [post]
func (ctrl *Controller) handleRegister(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}
	if err := ctrl.val.Struct(req); err != nil {
		response.ValidationErrorGin(c, err)
		return
	}

	err := ctrl.svc.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrEmailExists) {
			writeError(c, http.StatusConflict, authr.CodeEmailExists, "email already registered")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: register")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "registration failed")
		return
	}

	writeJSON(c, http.StatusCreated, map[string]string{
		"message": "Registration successful. Please check your email for the verification PIN.",
	})
}

// handleLogin authenticates a user.
// @Summary Login
// @Description Authenticate user and return JWT tokens.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.LoginRequest true "Login Credentials"
// @Success 200 {object} authr.AuthResponse
// @Failure 401 {object} authr.ErrorResponse
// @Router /api/auth/login [post]
func (ctrl *Controller) handleLogin(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if err := ctrl.val.Struct(req); err != nil {
		response.ValidationErrorGin(c, err)
		return
	}

	ip := clientIP(c)
	sec := loginSecurityContext(c, ip, req.DeviceID, req.DeviceName, req.DeviceFingerprint)

	authResp, err := ctrl.svc.Login(c.Request.Context(), &req, sec)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidCredentials, "invalid email or password")
		case errors.Is(err, service.ErrTooManyAttempts):
			writeError(c, http.StatusTooManyRequests, authr.CodeAuthTooManyAttempts, "too many login attempts, please try again later")
		case errors.Is(err, service.ErrIPBlocked):
			writeError(c, http.StatusForbidden, authr.CodeAuthIPBlocked, "login from this IP is blocked")
		case errors.Is(err, service.ErrGeoBlocked):
			writeError(c, http.StatusForbidden, authr.CodeAuthGeoBlocked, "login from this region is not allowed")
		case errors.Is(err, service.ErrAccountLocked):
			writeError(c, http.StatusForbidden, authr.CodeAccountLocked, "account temporarily locked")
		case errors.Is(err, service.ErrEmailNotVerified):
			writeError(c, http.StatusForbidden, authr.CodeEmailNotVerified, "email not verified")
		default:
			ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: login")
			writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "login failed")
		}
		return
	}

	ctrl.setAuthCookies(c, authResp.AccessToken, authResp.RefreshToken, authResp.User)
	if ctrl.shouldSuppressTokens(c) {
		authResp = sanitizedAuthResponse(authResp)
	}
	writeJSON(c, http.StatusOK, authResp)
}

// handleGoogleLogin exchanges a trusted Google identity for erg-go tokens.
func (ctrl *Controller) handleGoogleLogin(c *gin.Context) {
	var req request.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.IDToken) == "" && !ctrl.svc.IsValidGoogleBridgeSecret(c.GetHeader("X-Auth-Bridge-Secret")) {
		writeError(c, http.StatusForbidden, authr.CodeInvalidToken, "google auth bridge is not allowed")
		return
	}

	ip := clientIP(c)
	sec := loginSecurityContext(c, ip, req.DeviceID, req.DeviceName, req.DeviceFingerprint)

	authResp, err := ctrl.svc.GoogleLogin(c.Request.Context(), &req, sec)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGoogleOAuthNotConfigured):
			writeError(c, http.StatusServiceUnavailable, authr.CodeInvalidRequest, "google oauth client id is not configured")
		case errors.Is(err, service.ErrGoogleIdentityInvalid):
			writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "google account is missing required verified identity data")
		case errors.Is(err, service.ErrTooManyAttempts):
			writeError(c, http.StatusTooManyRequests, authr.CodeAuthTooManyAttempts, "too many login attempts, please try again later")
		case errors.Is(err, service.ErrIPBlocked):
			writeError(c, http.StatusForbidden, authr.CodeAuthIPBlocked, "login from this IP is blocked")
		case errors.Is(err, service.ErrGeoBlocked):
			writeError(c, http.StatusForbidden, authr.CodeAuthGeoBlocked, "login from this region is not allowed")
		case errors.Is(err, service.ErrAccountLocked):
			writeError(c, http.StatusForbidden, authr.CodeAccountLocked, "account temporarily locked")
		default:
			ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: google login")
			writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "google login failed")
		}
		return
	}

	ctrl.setAuthCookies(c, authResp.AccessToken, authResp.RefreshToken, authResp.User)
	if ctrl.shouldSuppressTokens(c) {
		authResp = sanitizedAuthResponse(authResp)
	}
	writeJSON(c, http.StatusOK, authResp)
}

func (ctrl *Controller) handleProviderNotImplemented(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, authr.CodeInvalidRequest, "auth provider is not configured")
}

func (ctrl *Controller) handleSessionsCompat(c *gin.Context) {
	writeJSON(c, http.StatusOK, gin.H{
		"items": []gin.H{{
			"id":        sessionIDFromContext(c.Request.Context()),
			"userId":    userIDFromContext(c.Request.Context()),
			"roles":     rolesFromContext(c.Request.Context()),
			"isCurrent": true,
		}},
	})
}

type accountProfileCompatRequest struct {
	FullName         *string            `json:"fullName,omitempty"`
	FullNameSnake    *string            `json:"full_name,omitempty"`
	AvatarURL        *string            `json:"avatarUrl,omitempty"`
	AvatarURLSnake   *string            `json:"avatar_url,omitempty"`
	Phone            *string            `json:"phone,omitempty"`
	Bio              *string            `json:"bio,omitempty"`
	Gender           *string            `json:"gender,omitempty"`
	DateOfBirth      *string            `json:"dateOfBirth,omitempty"`
	DateOfBirthSnake *string            `json:"date_of_birth,omitempty"`
	Address          *string            `json:"address,omitempty"`
	City             *string            `json:"city,omitempty"`
	District         *string            `json:"district,omitempty"`
	JobTitle         *string            `json:"jobTitle,omitempty"`
	JobTitleSnake    *string            `json:"job_title,omitempty"`
	Region           *string            `json:"region,omitempty"`
	SocialLinks      *map[string]string `json:"socialLinks,omitempty"`
	SocialLinksSnake *map[string]string `json:"social_links,omitempty"`
}

func (r accountProfileCompatRequest) toServiceRequest() service.AccountProfileUpdate {
	return service.AccountProfileUpdate{
		FullName:    firstStringPtr(r.FullName, r.FullNameSnake),
		AvatarURL:   firstStringPtr(r.AvatarURL, r.AvatarURLSnake),
		Phone:       r.Phone,
		Bio:         r.Bio,
		Gender:      r.Gender,
		DateOfBirth: firstStringPtr(r.DateOfBirth, r.DateOfBirthSnake),
		Address:     r.Address,
		City:        r.City,
		District:    r.District,
		JobTitle:    firstStringPtr(r.JobTitle, r.JobTitleSnake),
		Region:      r.Region,
		SocialLinks: firstStringMapPtr(r.SocialLinks, r.SocialLinksSnake),
	}
}

func (ctrl *Controller) handleAccountProfileNotImplemented(c *gin.Context) {
	targetID := c.Param("id")
	currentID := userIDFromContext(c.Request.Context())
	if targetID != currentID && !hasAdminRole(rolesFromContext(c.Request.Context())) {
		writeError(c, http.StatusForbidden, authr.CodeInvalidToken, "cannot update another account")
		return
	}

	var req accountProfileCompatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	profile, err := ctrl.svc.UpdateProfile(c.Request.Context(), targetID, req.toServiceRequest())
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			writeError(c, http.StatusNotFound, authr.CodeUserNotFound, "user not found")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: update account profile")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "failed to update profile")
		return
	}
	writeJSON(c, http.StatusOK, profile)
}

func (ctrl *Controller) handleAccountAvatarUpload(c *gin.Context) {
	targetID := c.Param("id")
	currentID := userIDFromContext(c.Request.Context())
	if targetID != currentID && !hasAdminRole(rolesFromContext(c.Request.Context())) {
		writeError(c, http.StatusForbidden, authr.CodeInvalidToken, "cannot update another account avatar")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, storage.MaxRequestBytes(storage.UploadKindImage, storage.MaxImageSize))
	if err := c.Request.ParseMultipartForm(storage.MultipartMemoryLimit); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid multipart avatar upload")
		return
	}

	header, err := c.FormFile("file")
	if err != nil {
		header, err = c.FormFile("avatar")
	}
	if err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "avatar file is required")
		return
	}
	file, err := header.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "cannot open avatar file")
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "cannot read avatar file")
		return
	}
	profile, err := ctrl.svc.UploadAvatar(c.Request.Context(), targetID, header.Filename, body)
	if err != nil {
		if errors.Is(err, service.ErrStorageNotConfigured) {
			writeError(c, http.StatusServiceUnavailable, authr.CodeInternalError, "avatar storage is not configured")
			return
		}
		if errors.Is(err, service.ErrUserNotFound) {
			writeError(c, http.StatusNotFound, authr.CodeUserNotFound, "user not found")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: upload account avatar")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "failed to upload avatar")
		return
	}
	writeJSON(c, http.StatusOK, profile)
}

func (ctrl *Controller) handleAccountPasswordNotImplemented(c *gin.Context) {
	targetID := c.Param("id")
	currentID := userIDFromContext(c.Request.Context())
	if targetID != currentID {
		writeError(c, http.StatusForbidden, authr.CodeInvalidToken, "cannot change another account password")
		return
	}

	var req struct {
		OldPassword      string `json:"oldPassword"`
		OldPasswordSnake string `json:"old_password"`
		CurrentPassword  string `json:"currentPassword"`
		NewPassword      string `json:"newPassword"`
		NewPasswordSnake string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}
	oldPassword := firstString(req.OldPassword, req.OldPasswordSnake, req.CurrentPassword)
	newPassword := firstString(req.NewPassword, req.NewPasswordSnake)
	if oldPassword == "" || len(newPassword) < 8 {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "old password and new password are required")
		return
	}
	if err := ctrl.svc.ChangePassword(c.Request.Context(), currentID, oldPassword, newPassword); err != nil {
		if errors.Is(err, service.ErrInvalidOldPassword) {
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidCredentials, "current password is incorrect")
			return
		}
		if errors.Is(err, service.ErrUserNotFound) {
			writeError(c, http.StatusNotFound, authr.CodeUserNotFound, "user not found")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: change account password")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "failed to change password")
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"message": "password changed successfully"})
}

// handleLogout logouts a user.
// @Summary Logout
// @Description Invalidate the current user session.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Router /api/auth/logout [post]
func (ctrl *Controller) handleLogout(c *gin.Context) {
	userID := userIDFromContext(c.Request.Context())
	sessionID := sessionIDFromContext(c.Request.Context())

	if userID == "" || sessionID == "" {
		writeError(c, http.StatusUnauthorized, authr.CodeInvalidToken, "not authenticated")
		return
	}

	err := ctrl.svc.Logout(c.Request.Context(), sessionID, userID)
	if err != nil {
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: logout")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "logout failed")
		return
	}

	ctrl.clearAuthCookies(c)
	writeJSON(c, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// handleRefresh refreshes the access token.
// @Summary Refresh Token
// @Description Get a new access token using a refresh token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.RefreshRequest true "Refresh Token"
// @Success 200 {object} authr.TokenResponse
// @Router /api/auth/refresh [post]
func (ctrl *Controller) handleRefresh(c *gin.Context) {
	var req request.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = request.RefreshRequest{}
	}

	usesRefreshCookie := req.RefreshToken == "" && auth.UsesRefreshCookie(c.Request, ctrlRefreshCookieName(ctrl.cfg))
	if usesRefreshCookie && auth.UnsafeMethod(c.Request.Method) {
		if err := auth.ValidateCSRF(c.Request); err != nil {
			writeError(c, http.StatusForbidden, authr.CodeInvalidToken, "missing or invalid CSRF token")
			return
		}
	}
	if req.RefreshToken == "" {
		req.RefreshToken = auth.RefreshTokenFromRequest(c.Request, ctrlRefreshCookieName(ctrl.cfg))
	}

	if req.RefreshToken == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "refresh_token is required")
		return
	}

	tokens, err := ctrl.svc.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrTokenReuseDetected) {
			ctrl.clearAuthCookies(c)
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidToken,
				"token reuse detected. All sessions have been revoked for security.")
			return
		}
		if errors.Is(err, service.ErrSessionReplaced) {
			ctrl.clearAuthCookies(c)
			writeError(c, http.StatusUnauthorized, authr.CodeAuthSessionReplaced, "session was replaced by a newer login")
			return
		}
		if errors.Is(err, service.ErrInvalidToken) {
			ctrl.clearAuthCookies(c)
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidToken, "invalid or expired refresh token")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: refresh")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "token refresh failed")
		return
	}

	ctrl.setTokenCookies(c, tokens.AccessToken, tokens.RefreshToken)
	if ctrl.shouldSuppressTokens(c) {
		tokens = sanitizedTokenResponse(tokens)
	}
	writeJSON(c, http.StatusOK, tokens)
}

// handleVerifyPIN verifies a user's PIN.
// @Summary Verify PIN
// @Description Verify the one-time PIN sent to user email.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.VerifyPinRequest true "PIN Data"
// @Success 200 {object} map[string]string
// @Router /api/auth/verify-pin [post]
func (ctrl *Controller) handleVerifyPIN(c *gin.Context) {
	var req request.VerifyPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	if req.Email == "" || req.Code == "" || req.Purpose == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "email, code, and purpose are required")
		return
	}
	if len(req.Code) != 6 {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidPIN, "PIN code must be 6 digits")
		return
	}

	err := ctrl.svc.VerifyPIN(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPIN) {
			writeError(c, http.StatusBadRequest, authr.CodeInvalidPIN, "invalid, expired, or already used PIN")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: verifyPIN")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "PIN verification failed")
		return
	}

	writeJSON(c, http.StatusOK, map[string]string{"message": "PIN verified successfully"})
}

// handleResendPIN re-sends a verification PIN.
// @Summary Resend PIN
// @Description Re-send the one-time PIN to user email.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.ResendPinRequest true "Email & Purpose"
// @Success 200 {object} map[string]string
// @Router /api/auth/resend-pin [post]
func (ctrl *Controller) handleResendPIN(c *gin.Context) {
	var req request.ResendPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	if req.Email == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "email is required")
		return
	}

	// Always return 200 to prevent email enumeration
	_ = ctrl.svc.ResendPIN(c.Request.Context(), &req)

	writeJSON(c, http.StatusOK, map[string]string{
		"message": "If that email is registered, a new PIN has been sent.",
	})
}

// handleForgotPassword sends a password reset link or PIN.
// @Summary Forgot Password
// @Description Initiate the password recovery process.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.ForgotPasswordRequest true "Email"
// @Success 200 {object} map[string]string
// @Router /api/auth/forgot-password [post]
func (ctrl *Controller) handleForgotPassword(c *gin.Context) {
	var req request.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	if req.Email == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "email is required")
		return
	}

	// Always return 200 to prevent email enumeration attacks
	_ = ctrl.svc.ForgotPassword(c.Request.Context(), &req)

	writeJSON(c, http.StatusOK, map[string]string{
		"message": "If that email is registered, a reset PIN has been sent.",
	})
}

// handleResetPassword resets a user's password.
// @Summary Reset Password
// @Description Set a new password using a recovery token or PIN.
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body request.ResetPasswordRequest true "Reset Data"
// @Success 200 {object} map[string]string
// @Router /api/auth/reset-password [post]
func (ctrl *Controller) handleResetPassword(c *gin.Context) {
	var req request.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	if req.Email == "" || req.Code == "" || req.NewPassword == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "email, code, and new_password are required")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "new_password must be at least 8 characters")
		return
	}

	err := ctrl.svc.ResetPassword(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPIN) {
			writeError(c, http.StatusBadRequest, authr.CodeInvalidPIN, "invalid, expired, or already used PIN")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: resetPassword")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "password reset failed")
		return
	}

	writeJSON(c, http.StatusOK, map[string]string{"message": "Password reset successful. Please login."})
}

// handleProfile returns the current user profile.
// @Summary Get Profile
// @Description Fetch profile details for the authenticated user.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} authr.ProfileResponse
// @Failure 401 {object} authr.ErrorResponse
// @Router /api/auth/profile [get]
func (ctrl *Controller) handleProfile(c *gin.Context) {
	userID := userIDFromContext(c.Request.Context())
	if userID == "" {
		writeError(c, http.StatusUnauthorized, authr.CodeInvalidToken, "user not found in token")
		return
	}

	profile, err := ctrl.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			writeError(c, http.StatusNotFound, authr.CodeUserNotFound, "user not found")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: getProfile")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "failed to fetch profile")
		return
	}

	writeJSON(c, http.StatusOK, profile)
}

// ─── Utilities ───────────────────────────────────────────────────────────────

func (ctrl *Controller) handleLoginAttempts(c *gin.Context) {
	page := parseIntQuery(c, "page", 1)
	limit := parseIntQuery(c, "limit", 50)
	userID := userIDFromContext(c.Request.Context())
	params := authrepo.LoginAttemptListParams{
		TenantID: c.Query("tenantId"),
		UserID:   c.Query("userId"),
		Email:    c.Query("email"),
		IP:       c.Query("ip"),
		Result:   c.Query("result"),
		Reason:   c.Query("reason"),
		From:     parseTimeQuery(c, "from"),
		To:       parseTimeQuery(c, "to"),
		Page:     page,
		Limit:    limit,
	}
	if !hasAdminRole(rolesFromContext(c.Request.Context())) {
		params.UserID = userID
		params.Email = ""
		params.IP = ""
	}
	attempts, total, err := ctrl.svc.ListLoginAttempts(c.Request.Context(), params)
	if err != nil {
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: list login attempts")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "failed to list login attempts")
		return
	}
	response.PaginatedGin(c, attempts, total, page, limit)
}

func (ctrl *Controller) handleIPSecurityStatus(c *gin.Context) {
	ip := strings.TrimSpace(c.Query("ip"))
	if ip == "" {
		ip = clientIP(c)
	}
	sec := loginSecurityContext(c, ip, "", "", "")
	status, err := ctrl.svc.GetIPSecurityStatus(c.Request.Context(), c.Query("email"), sec)
	if err != nil {
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: ip security status")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "failed to read IP security status")
		return
	}
	writeJSON(c, http.StatusOK, status)
}

func clientIP(c *gin.Context) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		return value
	}
	return c.ClientIP()
}

func loginSecurityContext(c *gin.Context, ip, deviceID, deviceName, deviceFingerprint string) service.LoginSecurityContext {
	return service.LoginSecurityContext{
		IPAddress:         ip,
		UserAgent:         c.GetHeader("User-Agent"),
		DeviceID:          deviceID,
		DeviceName:        deviceName,
		DeviceFingerprint: deviceFingerprint,
		CountryCode:       firstHeader(c, "CF-IPCountry", "X-Vercel-IP-Country", "X-Geo-Country"),
		CountryName:       firstHeader(c, "X-Geo-Country-Name"),
		ContinentCode:     firstHeader(c, "CF-IPContinent", "X-Vercel-IP-Continent", "X-Geo-Continent"),
	}
}

func firstHeader(c *gin.Context, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(c.GetHeader(name)); value != "" {
			return value
		}
	}
	return ""
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstStringPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstStringMapPtr(values ...*map[string]string) *map[string]string {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func parseIntQuery(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func parseTimeQuery(c *gin.Context, key string) time.Time {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t
	}
	return time.Time{}
}

func hasAdminRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToUpper(strings.TrimSpace(role)) {
		case "ADMIN", "SUPER_ADMIN":
			return true
		}
	}
	return false
}

func writeError(c *gin.Context, status int, code, message string) {
	response.ErrorGin(c, status, code, message)
}

func writeJSON(c *gin.Context, status int, v interface{}) {
	response.WriteGin(c, status, v, nil, nil)
}
