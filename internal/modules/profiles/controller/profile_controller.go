// Package controller handles HTTP requests for the profiles module.
package controller

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/profiles/dto"
	"erg.ninja/internal/modules/profiles/entities"
	"erg.ninja/internal/modules/profiles/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for profiles.
type Controller struct {
	svc          *service.Service
	jwtValidator *auth.JWTValidator
	log          *logger.Logger
}

// NewController creates a new profiles controller.
func NewController(svc *service.Service, jwtValidator *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, jwtValidator: jwtValidator, log: log}
}

// RegisterRoutes mounts the profiles REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/profiles")
	api.GET("/:userId", c.GetProfile)

	protected := api.Group("")
	protected.Use(c.authMiddleware())
	protected.GET("/me", c.GetMyProfile)
	protected.POST("/", c.CreateProfile)
	protected.PUT("/:userId", c.UpdateProfile)
	protected.DELETE("/:userId", c.DeleteProfile)
}

// ─── Auth Middleware ──────────────────────────────────────────────────────────

func (c *Controller) authMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.jwtValidator == nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		claims, err := c.jwtValidator.ValidateRequest(ctx.GetHeader("Authorization"))
		if err != nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		newCtx := contextWithClaims(ctx.Request.Context(), claims)
		ctx.Request = ctx.Request.WithContext(newCtx)
		ctx.Next()
	}
}

// contextKey is a custom type for context keys.
type contextKey string

const claimsCtxKey contextKey = "jwt_claims"

func contextWithClaims(ctx context.Context, claims *auth.JWTClaims) context.Context {
	return context.WithValue(ctx, claimsCtxKey, claims)
}

func getClaims(ctx context.Context) *auth.JWTClaims {
	if v := ctx.Value(claimsCtxKey); v != nil {
		return v.(*auth.JWTClaims)
	}
	return nil
}

func getUserIDFromCtx(ctx context.Context) string {
	if c := getClaims(ctx); c != nil {
		return c.UserID
	}
	return ""
}

func toProfileResponse(p *entities.Profile) dto.ProfileResponse {
	if p == nil {
		return dto.ProfileResponse{}
	}
	return dto.ProfileResponse{
		ID: p.ID, UserID: p.UserID, FullName: p.FullName,
		Bio: p.Bio, Phone: p.Phone, DateOfBirth: p.DateOfBirth,
		Gender: p.Gender, Address: p.Address, City: p.City,
		District: p.District, SocialLinks: p.SocialLinks,
		AvatarURL: p.AvatarURL, IsCompleted: p.IsCompleted,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// GetProfile handles GET /api/profiles/:userId (public).
// @Summary Get user profile
// @Description Returns a public user profile.
// @Tags Profiles
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/profiles/{userId} [get]
func (c *Controller) GetProfile(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	profile, err := c.svc.GetProfile(ctx.Request.Context(), userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("profiles: GetProfile failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if profile == nil {
		response.NotFoundGin(ctx, "profile not found")
		return
	}
	response.SuccessGin(ctx, toProfileResponse(profile))
}

// GetMyProfile handles GET /api/profiles/me.
// @Summary Get my profile
// @Description Returns the current user's profile.
// @Tags Profiles
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/profiles/me [get]
func (c *Controller) GetMyProfile(ctx *gin.Context) {
	userID := getUserIDFromCtx(ctx.Request.Context())
	if userID == "" {
		response.UnauthorizedGin(ctx)
		return
	}
	profile, err := c.svc.GetMyProfile(ctx.Request.Context(), userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("profiles: GetMyProfile failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if profile == nil {
		response.NotFoundGin(ctx, "profile not found")
		return
	}
	response.SuccessGin(ctx, toProfileResponse(profile))
}

// CreateProfile handles POST /api/profiles.
// @Summary Create profile
// @Description Creates a new user profile.
// @Tags Profiles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/profiles [post]
func (c *Controller) CreateProfile(ctx *gin.Context) {
	var req dto.CreateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	if req.UserID == "" {
		req.UserID = getUserIDFromCtx(ctx.Request.Context())
	}
	if req.FullName == "" {
		response.BadRequestGin(ctx, fmt.Errorf("full_name is required"))
		return
	}
	if req.UserID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("user_id is required"))
		return
	}
	profile, err := c.svc.CreateProfile(ctx.Request.Context(), req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("profiles: CreateProfile failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, toProfileResponse(profile))
}

// UpdateProfile handles PUT /api/profiles/:userId.
// @Summary Update profile
// @Description Updates a user profile.
// @Tags Profiles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/profiles/{userId} [put]
func (c *Controller) UpdateProfile(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	var req dto.UpdateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	profile, err := c.svc.UpdateProfile(ctx.Request.Context(), userID, req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("profiles: UpdateProfile failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if profile == nil {
		response.NotFoundGin(ctx, "profile not found")
		return
	}
	response.SuccessGin(ctx, toProfileResponse(profile))
}

// DeleteProfile handles DELETE /api/profiles/:userId.
// @Summary Delete profile
// @Description Deletes a user profile.
// @Tags Profiles
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/profiles/{userId} [delete]
func (c *Controller) DeleteProfile(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	if err := c.svc.DeleteProfile(ctx.Request.Context(), userID); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("profiles: DeleteProfile failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, map[string]any{"message": "profile deleted"})
}
