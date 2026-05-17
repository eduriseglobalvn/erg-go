package service

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/tenant"
)

func (c *Controller) ListQuestionBankCategories(ctx *gin.Context) {
	result, err := c.svc.ListQuestionBankCategories(
		ctx.Request.Context(),
		tenant.FromContext(ctx.Request.Context()),
		ctx.Query("subjectId"),
		ctx.Query("levelId"),
	)
	c.respond(ctx, result, err, http.StatusOK)
}
