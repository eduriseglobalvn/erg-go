package service

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/pkg/tenant"
)

func (c *Controller) SaveAttemptDraft(ctx *gin.Context) {
	var req AttemptDraftRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.SaveAttemptDraft(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("attemptId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}
