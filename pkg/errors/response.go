package ergerr

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

// ErrorResponse is the canonical JSON error shape returned by all HTTP handlers.
//
// Fields marked "omitempty" are omitted when empty to keep responses lean.
type ErrorResponse struct {
	// Code is the machine-readable error identifier (e.g. "CRAWLER_RATE_LIMITED").
	Code Code `json:"code"`
	// Message is a human-readable description. Never expose raw internal details.
	Message string `json:"message"`
	// Details contains additional context intended for client-side debugging
	// (e.g. "retry after 30s"). Never expose raw DB errors or stack traces.
	Details string `json:"details,omitempty"`
	// RequestID is the server-assigned request correlation ID.
	RequestID string `json:"request_id"`
	// RetryAfter is the number of seconds the client should wait before retrying.
	// Present only for rate-limit and service-unavailable errors.
	RetryAfter int `json:"retry_after,omitempty"`
	// TraceID is the distributed trace ID (OpenTelemetry).
	TraceID string `json:"trace_id,omitempty"`
}

// ToHTTPStatus maps an ergerr.Code to the equivalent HTTP status code.
//
// Mappings follow the gRPC ↔ HTTP compatibility guidelines from google.rpc error spec.
func (c Code) ToHTTPStatus() int {
	switch c.ToGRPCCode() {
	case codes.Canceled:
		return http.StatusRequestTimeout // 408
	case codes.Unknown:
		return http.StatusInternalServerError // 500
	case codes.InvalidArgument:
		return http.StatusBadRequest // 400
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout // 504
	case codes.NotFound:
		return http.StatusNotFound // 404
	case codes.AlreadyExists:
		return http.StatusConflict // 409
	case codes.PermissionDenied:
		return http.StatusForbidden // 403
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests // 429
	case codes.FailedPrecondition:
		return http.StatusUnprocessableEntity // 422
	case codes.Aborted:
		return http.StatusConflict // 409
	case codes.OutOfRange:
		return http.StatusUnprocessableEntity // 422
	case codes.Unimplemented:
		return http.StatusNotImplemented // 501
	case codes.Internal:
		return http.StatusInternalServerError // 500
	case codes.Unavailable:
		return http.StatusServiceUnavailable // 503
	case codes.DataLoss:
		return http.StatusInternalServerError // 500
	case codes.Unauthenticated:
		return http.StatusUnauthorized // 401
	default:
		return http.StatusInternalServerError // 500
	}
}

// HTTPStatusText returns a short description of the HTTP status for this code.
func (c Code) HTTPStatusText() string {
	return http.StatusText(c.ToHTTPStatus())
}

// NewResponse creates an ErrorResponse from a Code and message.
func NewResponse(code Code, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
	}
}

// NewResponseWithRequestID is like NewResponse but also sets the request ID.
func NewResponseWithRequestID(code Code, message, requestID string) *ErrorResponse {
	return &ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
}

// FromError extracts an ErrorResponse from any error.
// If err is nil, returns nil.
// If err is a *E, extracts its structured fields.
// Otherwise wraps the error as INTERNAL_SERVER_ERROR.
func FromError(err error) *ErrorResponse {
	if err == nil {
		return nil
	}

	if e, ok := err.(*E); ok {
		return &ErrorResponse{Code: e.Code, Message: e.Message, RequestID: e.metadata["request_id"], TraceID: e.metadata["trace_id"], RetryAfter: parseRetryAfter(e.metadata["retry_after"])}
	}

	// For plain errors, wrap as internal. Strip any sensitive detail.
	return &ErrorResponse{
		Code:    ErrInternal,
		Message: "An unexpected error occurred",
	}
}

// WithDetails returns a copy of r with the Details field set.
func (r *ErrorResponse) WithDetails(d string) *ErrorResponse {
	cp := *r
	cp.Details = d
	return &cp
}

// WithTraceID returns a copy of r with the TraceID field set.
func (r *ErrorResponse) WithTraceID(t string) *ErrorResponse {
	cp := *r
	cp.TraceID = t
	return &cp
}

// WithRetryAfter returns a copy of r with the RetryAfter field set.
func (r *ErrorResponse) WithRetryAfter(sec int) *ErrorResponse {
	cp := *r
	cp.RetryAfter = sec
	return &cp
}
