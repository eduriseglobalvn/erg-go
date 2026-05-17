package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/elearning/api/dto"
	elearningservice "erg.ninja/internal/modules/elearning/application/service"
	"erg.ninja/pkg/tenant"
)

func (c *Controller) RegisterStudentRoutes(r *gin.RouterGroup) {
	r.GET("/dashboard", c.StudentDashboard)
	r.GET("/assignments", c.StudentAssignments)
	r.GET("/assignments/:assignmentId", c.StudentAssignmentDetail)
	r.GET("/scores", c.StudentScores)
	r.GET("/announcements", c.StudentAnnouncements)
	r.GET("/notifications", c.StudentNotifications)
	r.POST("/notifications/:notificationId/read", c.MarkStudentNotificationRead)
	r.GET("/discussions", c.StudentDiscussions)
	r.POST("/discussions", c.CreateStudentDiscussion)
	r.POST("/discussions/:threadId/replies", c.CreateStudentDiscussionReply)
}

func (c *Controller) StudentDashboard(ctx *gin.Context) {
	result, err := c.svc.StudentDashboard(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) StudentAssignments(ctx *gin.Context) {
	result, err := c.svc.StudentAssignments(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx), ctx.Query("status"))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) StudentAssignmentDetail(ctx *gin.Context) {
	result, err := c.svc.StudentAssignmentDetail(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx), ctx.Param("assignmentId"))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) StudentScores(ctx *gin.Context) {
	result, err := c.svc.StudentScores(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx), ctx.Query("subjectId"))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) StudentAnnouncements(ctx *gin.Context) {
	result, err := c.svc.StudentAnnouncements(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) StudentNotifications(ctx *gin.Context) {
	result, err := c.svc.StudentNotifications(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) MarkStudentNotificationRead(ctx *gin.Context) {
	result, err := c.svc.MarkStudentNotificationRead(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx), ctx.Param("notificationId"))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) StudentDiscussions(ctx *gin.Context) {
	result, err := c.svc.StudentDiscussions(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx))
	c.writeStudentResult(ctx, http.StatusOK, result, err)
}

func (c *Controller) CreateStudentDiscussion(ctx *gin.Context) {
	var req dto.CreateStudentDiscussionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateStudentDiscussion(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx), req)
	c.writeStudentResult(ctx, http.StatusCreated, result, err)
}

func (c *Controller) CreateStudentDiscussionReply(ctx *gin.Context) {
	var req dto.CreateStudentDiscussionReplyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateStudentDiscussionReply(ctx.Request.Context(), studentTenantFromGin(ctx), studentActorFromGin(ctx), ctx.Param("threadId"), req)
	c.writeStudentResult(ctx, http.StatusCreated, result, err)
}

func (c *Controller) writeStudentResult(ctx *gin.Context, status int, result any, err error) {
	if err != nil {
		if errors.Is(err, elearningservice.ErrStudentContractNotFound) {
			c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "ELEARNING_STUDENT_CONTRACT_FAILED", err.Error())
		return
	}
	c.json(ctx, status, result)
}

func studentActorFromGin(ctx *gin.Context) elearningservice.StudentActor {
	return elearningservice.StudentActor{UserID: middleware.GetUserID(ctx.Request.Context()), Roles: middleware.GetRoles(ctx.Request.Context())}
}

func studentTenantFromGin(ctx *gin.Context) string {
	if tenantID := tenant.FromContext(ctx.Request.Context()); tenantID != "" {
		return tenantID
	}
	return "default"
}
