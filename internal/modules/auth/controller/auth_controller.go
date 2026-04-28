package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/auth/dto/request"
	authr "erg.ninja/internal/modules/auth/dto/response"
	"erg.ninja/internal/modules/auth/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

// Controller holds the auth HTTP controller dependencies.
type Controller struct {
	svc       *service.AuthService
	validator *auth.JWTValidator
	log       *logger.Logger
}

// NewController creates a new auth controller.
func NewController(svc *service.AuthService, validator *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, validator: validator, log: log}
}

// RegisterRoutes mounts all auth routes onto the Gin router.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/auth")
	// Public routes
	api.POST("/register", c.handleRegister)
	api.POST("/login", c.handleLogin)
	api.POST("/google/login", c.handleGoogleLogin)
	api.POST("/logout", c.handleLogout)
	api.POST("/refresh", c.handleRefresh)
	api.POST("/verify-pin", c.handleVerifyPIN)
	api.POST("/resend-pin", c.handleResendPIN)
	api.POST("/forgot-password", c.handleForgotPassword)
	api.POST("/reset-password", c.handleResetPassword)

	// Protected routes
	protected := api.Group("")
	protected.Use(c.jwtMiddleware())
	protected.GET("/profile", c.handleProfile)
}

// jwtMiddleware extracts and validates the JWT from the Authorization header.
func (c *Controller) jwtMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
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
		ctx.Request = ctx.Request.WithContext(newCtx)
		ctx.Next()
	}
}

// context keys
type ctxKey string

const (
	ctxKeyUserID    ctxKey = "auth_user_id"
	ctxKeySessionID ctxKey = "auth_session_id"
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

	if req.Email == "" || req.Password == "" || req.FullName == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "email, password, and full_name are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "password must be at least 8 characters")
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

	ip := clientIP(c)
	userAgent := c.GetHeader("User-Agent")

	authResp, err := ctrl.svc.Login(c.Request.Context(), &req, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidCredentials, "invalid email or password")
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

	writeJSON(c, http.StatusOK, authResp)
}

// handleGoogleLogin exchanges a trusted Google identity for erg-go tokens.
func (ctrl *Controller) handleGoogleLogin(c *gin.Context) {
	if !ctrl.svc.IsValidGoogleBridgeSecret(c.GetHeader("X-Auth-Bridge-Secret")) {
		writeError(c, http.StatusForbidden, authr.CodeInvalidToken, "google auth bridge is not allowed")
		return
	}

	var req request.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	ip := clientIP(c)
	userAgent := c.GetHeader("User-Agent")

	authResp, err := ctrl.svc.GoogleLogin(c.Request.Context(), &req, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGoogleIdentityInvalid):
			writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "google account is missing required verified identity data")
		case errors.Is(err, service.ErrAccountLocked):
			writeError(c, http.StatusForbidden, authr.CodeAccountLocked, "account temporarily locked")
		default:
			ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: google login")
			writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "google login failed")
		}
		return
	}

	writeJSON(c, http.StatusOK, authResp)
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
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "invalid JSON body")
		return
	}

	if req.RefreshToken == "" {
		writeError(c, http.StatusBadRequest, authr.CodeInvalidRequest, "refresh_token is required")
		return
	}

	tokens, err := ctrl.svc.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrTokenReuseDetected) {
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidToken,
				"token reuse detected. All sessions have been revoked for security.")
			return
		}
		if errors.Is(err, service.ErrInvalidToken) {
			writeError(c, http.StatusUnauthorized, authr.CodeInvalidToken, "invalid or expired refresh token")
			return
		}
		ctrl.log.ErrorContext(c.Request.Context()).Err(err).Msg("auth.controller: refresh")
		writeError(c, http.StatusInternalServerError, authr.CodeInternalError, "token refresh failed")
		return
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

func clientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		return xff
	}
	return c.ClientIP()
}

func writeError(c *gin.Context, status int, code, message string) {
	response.ErrorGin(c, status, code, message)
}

func writeJSON(c *gin.Context, status int, v interface{}) {
	response.WriteGin(c, status, v, nil, nil)
}
