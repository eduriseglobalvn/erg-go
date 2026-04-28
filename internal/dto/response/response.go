// Package response provides a unified HTTP response API for erg-go.
// All handlers should use these functions instead of raw json.Encode.
package response

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Response is the canonical JSON envelope for all API responses,
// precisely matching the NestJS TransformInterceptor output for FE parity.
type Response struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       any    `json:"data,omitempty"`
	Errors     any    `json:"errors"`
	Timestamp  string `json:"timestamp"`
	Path       string `json:"path,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// Meta contains pagination and summary metadata.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// PaginatedData combines items and meta for consistent FE consumption.
type PaginatedData struct {
	Data       any   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int64 `json:"totalPages"`
}

// ErrDetail encodes error information in the response.
type ErrDetail struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Details []any  `json:"details,omitempty"`
}

// FieldError represents a single field validation error.
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Actual  string `json:"actual_tag"`
	Message string `json:"message"`
}

// getRequestID extracts or generates a request ID.
func getRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return uuid.New().String()
}

// getRequestIDGin extracts or generates a request ID from Gin context.
func getRequestIDGin(c *gin.Context) string {
	if id := c.GetHeader("X-Request-ID"); id != "" {
		return id
	}
	return uuid.New().String()
}

// getTimestamp returns an RFC3339 timestamp string.
func getTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// write sends a Response with the given status code.
func write(w http.ResponseWriter, r *http.Request, status int, data any, meta *Meta, errDetail *ErrDetail) {
	reqID := getRequestID(r)
	msg := "Success"
	var errs any = nil

	if errDetail != nil {
		msg = errDetail.Message
		if len(errDetail.Details) > 0 {
			errs = errDetail.Details
		} else {
			errs = errDetail.Code
		}
	}

	// Handle pagination wrapping for FE compatibility
	finalData := data
	if meta != nil {
		finalData = PaginatedData{
			Data:       data,
			Total:      meta.Total,
			Page:       meta.Page,
			Limit:      meta.Limit,
			TotalPages: meta.TotalPages,
		}
	}

	resp := Response{
		StatusCode: status,
		Message:    msg,
		Data:       finalData,
		Errors:     errs,
		Timestamp:  getTimestamp(),
		Path:       r.URL.Path,
		RequestID:  reqID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", reqID)
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(resp)
}

// Success sends a 200 OK with data.
func Success(w http.ResponseWriter, r *http.Request, data any) {
	write(w, r, http.StatusOK, data, nil, nil)
}

// OK sends a 200 OK with data (alias for Success).
func OK(w http.ResponseWriter, r *http.Request, data any) {
	write(w, r, http.StatusOK, data, nil, nil)
}

// OKMeta sends a 200 OK with data and pagination metadata.
func OKMeta(w http.ResponseWriter, r *http.Request, data any, meta *Meta) {
	write(w, r, http.StatusOK, data, meta, nil)
}

// Created sends a 201 Created with data.
func Created(w http.ResponseWriter, r *http.Request, data any) {
	write(w, r, http.StatusCreated, data, nil, nil)
}

// Paginated sends a 200 OK with paginated data.
func Paginated(w http.ResponseWriter, r *http.Request, data any, total int64, page, limit int) {
	meta := &Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: (total + int64(limit) - 1) / int64(limit),
	}
	write(w, r, http.StatusOK, data, meta, nil)
}

// BadRequest sends a 400 Bad Request with optional error detail.
func BadRequest(w http.ResponseWriter, r *http.Request, err error) {
	write(w, r, http.StatusBadRequest, nil, nil, newErrDetail(ErrCodeBadRequest, err))
}

// Unauthorized sends a 401 Unauthorized.
func Unauthorized(w http.ResponseWriter, r *http.Request) {
	write(w, r, http.StatusUnauthorized, nil, nil, &ErrDetail{
		Code:    ErrCodeUnauthorized,
		Message: "unauthorized",
	})
}

// Forbidden sends a 403 Forbidden.
func Forbidden(w http.ResponseWriter, r *http.Request) {
	write(w, r, http.StatusForbidden, nil, nil, &ErrDetail{
		Code:    ErrCodeForbidden,
		Message: "forbidden",
	})
}

// NotFound sends a 404 Not Found with a message.
func NotFound(w http.ResponseWriter, r *http.Request, msg string) {
	write(w, r, http.StatusNotFound, nil, nil, &ErrDetail{
		Code:    ErrCodeNotFound,
		Message: msg,
	})
}

// InternalError sends a 500 Internal Server Error with error detail.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	msg := "internal server error"
	if err != nil {
		msg = err.Error()
	}
	write(w, r, http.StatusInternalServerError, nil, nil, &ErrDetail{
		Code:    ErrCodeInternal,
		Message: msg,
	})
}

// ValidationError sends a 422 Unprocessable Entity with per-field validation details.
func ValidationError(w http.ResponseWriter, r *http.Request, err error) {
	details := []any{}
	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ve {
			details = append(details, FieldError{
				Field:   fe.Field(),
				Tag:     fe.Tag(),
				Actual:  fe.ActualTag(),
				Message: msgForTag(fe.Tag(), fe.Field()),
			})
		}
	}
	write(w, r, http.StatusUnprocessableEntity, nil, nil, &ErrDetail{
		Code:    ErrCodeValidation,
		Message: "validation failed",
		Details: details,
	})
}

// newErrDetail builds an ErrDetail from an error using the canonical code map.
func newErrDetail(code string, err error) *ErrDetail {
	if err == nil {
		return &ErrDetail{Code: code, Message: "error"}
	}
	return &ErrDetail{Code: code, Message: err.Error()}
}

// msgForTag returns a human-readable message for a validation tag.
func msgForTag(tag, field string) string {
	switch tag {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return field + " is too short"
	case "max":
		return field + " is too long"
	case "gte":
		return field + " must be greater than or equal to the minimum"
	case "lte":
		return field + " must be less than or equal to the maximum"
	case "uuid":
		return field + " must be a valid UUID"
	case "url":
		return field + " must be a valid URL"
	default:
		return field + " failed validation: " + tag
	}
}

// Error sends an error response with the given HTTP status, canonical code, and message.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	write(w, r, status, nil, nil, &ErrDetail{Code: code, Message: message})
}

// ─── Gin helpers ─────────────────────────────────────────────────────────────

// WriteGin sends a Response with the given status code via Gin.
func WriteGin(c *gin.Context, status int, data any, meta *Meta, errDetail *ErrDetail) {
	reqID := getRequestIDGin(c)
	msg := "Success"
	var errs any = nil

	if errDetail != nil {
		msg = errDetail.Message
		if len(errDetail.Details) > 0 {
			errs = errDetail.Details
		} else {
			errs = errDetail.Code
		}
	}

	// Handle pagination wrapping for FE compatibility
	finalData := data
	if meta != nil {
		finalData = PaginatedData{
			Data:       data,
			Total:      meta.Total,
			Page:       meta.Page,
			Limit:      meta.Limit,
			TotalPages: meta.TotalPages,
		}
	}

	resp := Response{
		StatusCode: status,
		Message:    msg,
		Data:       finalData,
		Errors:     errs,
		Timestamp:  getTimestamp(),
		Path:       c.Request.URL.Path,
		RequestID:  reqID,
	}

	c.Header("X-Request-ID", reqID)
	c.JSON(status, resp)
}

func SuccessGin(c *gin.Context, data any) {
	WriteGin(c, http.StatusOK, data, nil, nil)
}

func OKGin(c *gin.Context, data any) {
	WriteGin(c, http.StatusOK, data, nil, nil)
}

func OKMetaGin(c *gin.Context, data any, meta *Meta) {
	WriteGin(c, http.StatusOK, data, meta, nil)
}

func CreatedGin(c *gin.Context, data any) {
	WriteGin(c, http.StatusCreated, data, nil, nil)
}

func PaginatedGin(c *gin.Context, data any, total int64, page, limit int) {
	meta := &Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: (total + int64(limit) - 1) / int64(limit),
	}
	WriteGin(c, http.StatusOK, data, meta, nil)
}

func BadRequestGin(c *gin.Context, err error) {
	WriteGin(c, http.StatusBadRequest, nil, nil, newErrDetail(ErrCodeBadRequest, err))
}

func UnauthorizedGin(c *gin.Context) {
	WriteGin(c, http.StatusUnauthorized, nil, nil, &ErrDetail{
		Code:    ErrCodeUnauthorized,
		Message: "unauthorized",
	})
}

func ForbiddenGin(c *gin.Context) {
	WriteGin(c, http.StatusForbidden, nil, nil, &ErrDetail{
		Code:    ErrCodeForbidden,
		Message: "forbidden",
	})
}

func NotFoundGin(c *gin.Context, msg string) {
	WriteGin(c, http.StatusNotFound, nil, nil, &ErrDetail{
		Code:    ErrCodeNotFound,
		Message: msg,
	})
}

func InternalErrorGin(c *gin.Context, err error) {
	msg := "internal server error"
	if err != nil {
		msg = err.Error()
	}
	WriteGin(c, http.StatusInternalServerError, nil, nil, &ErrDetail{
		Code:    ErrCodeInternal,
		Message: msg,
	})
}

func ValidationErrorGin(c *gin.Context, err error) {
	details := []any{}
	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ve {
			details = append(details, FieldError{
				Field:   fe.Field(),
				Tag:     fe.Tag(),
				Actual:  fe.ActualTag(),
				Message: msgForTag(fe.Tag(), fe.Field()),
			})
		}
	}
	WriteGin(c, http.StatusUnprocessableEntity, nil, nil, &ErrDetail{
		Code:    ErrCodeValidation,
		Message: "validation failed",
		Details: details,
	})
}

func ErrorGin(c *gin.Context, status int, code, message string) {
	WriteGin(c, status, nil, nil, &ErrDetail{Code: code, Message: message})
}
