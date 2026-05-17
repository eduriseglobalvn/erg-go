package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	platformctx "erg.ninja/internal/platform/context"
	"erg.ninja/internal/platform/exception"
)

type Meta struct {
	RequestID string `json:"request_id,omitempty"`
}

type Envelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
	Meta    Meta   `json:"meta"`
}

func OK(c *gin.Context, data any) {
	Write(c, http.StatusOK, "OK", "Success", data, nil)
}

func WriteError(c *gin.Context, err *exception.AppError) {
	if err == nil {
		err = exception.New("INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	}
	Write(c, err.Status(), err.Code, err.Message, nil, err.Details)
}

func Write(c *gin.Context, status int, code, message string, data, errors any) {
	c.JSON(status, Envelope{
		Success: status < http.StatusBadRequest,
		Code:    code,
		Message: message,
		Data:    data,
		Errors:  errors,
		Meta: Meta{
			RequestID: platformctx.RequestID(c.Request.Context()),
		},
	})
}
