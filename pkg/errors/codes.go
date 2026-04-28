// Package ergerr provides structured error codes, gRPC status mapping,
// and HTTP error response helpers for the erg.ninja platform.
package ergerr

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
)

// Code is a machine-readable error identifier.
type Code string

func (c Code) String() string { return string(c) }

// ── Domain: crawler ──────────────────────────────────────────────────────────

const (
	ErrCrawlerBlacklisted        Code = "CRAWLER_BLACKLISTED"
	ErrCrawlerRobotsDisallowed   Code = "CRAWLER_ROBOTS_DISALLOWED"
	ErrCrawlerQualityTooLow      Code = "CRAWLER_QUALITY_TOO_LOW"
	ErrCrawlerDuplicate          Code = "CRAWLER_DUPLICATE"
	ErrCrawlerTimeout            Code = "CRAWLER_TIMEOUT"
	ErrCrawlerRateLimited        Code = "CRAWLER_RATE_LIMITED"
	ErrCrawlerInvalidURL         Code = "CRAWLER_INVALID_URL"
	ErrCrawlerNotFound           Code = "CRAWLER_NOT_FOUND"
	ErrCrawlerServiceUnavailable Code = "CRAWLER_SERVICE_UNAVAILABLE"

	// ── Domain: bot ─────────────────────────────────────────────────────────────

	ErrBotCommandNotFound    Code = "BOT_COMMAND_NOT_FOUND"
	ErrBotConversationLimit  Code = "BOT_CONVERSATION_LIMIT"
	ErrBotUnauthorized       Code = "BOT_UNAUTHORIZED"
	ErrBotServiceUnavailable Code = "BOT_SERVICE_UNAVAILABLE"

	// ── Domain: trending ────────────────────────────────────────────────────────

	ErrTrendingNoData         Code = "TRENDING_NO_DATA"
	ErrTrendingStaleCache     Code = "TRENDING_STALE_CACHE"
	ErrTrendingSourceDisabled Code = "TRENDING_SOURCE_DISABLED"
	ErrTrendingRateLimited    Code = "TRENDING_RATELIMITED"

	// ── Domain: notification ────────────────────────────────────────────────────

	ErrNotificationChannelDisabled Code = "NOTIFICATION_CHANNEL_DISABLED"
	ErrNotificationDeliveryFailed  Code = "NOTIFICATION_DELIVERY_FAILED"
	ErrNotificationQueueFull       Code = "NOTIFICATION_QUEUE_FULL"
	ErrNotificationInvalidPayload  Code = "NOTIFICATION_INVALID_PAYLOAD"

	// ── Domain: auth ───────────────────────────────────────────────────────────

	ErrUnauthenticated Code = "UNAUTHENTICATED"
	ErrForbidden       Code = "FORBIDDEN"
	ErrTokenExpired    Code = "TOKEN_EXPIRED"
	ErrTokenInvalid    Code = "TOKEN_INVALID"
	ErrTenantMissing   Code = "TENANT_MISSING"
	ErrTenantNotFound  Code = "TENANT_NOT_FOUND"

	// ── Domain: internal ───────────────────────────────────────────────────────

	ErrInternal           Code = "INTERNAL_SERVER_ERROR"
	ErrServiceUnavailable Code = "SERVICE_UNAVAILABLE"
	ErrDatabaseError      Code = "DATABASE_ERROR"
	ErrCacheError         Code = "CACHE_ERROR"
	ErrQueueError         Code = "QUEUE_ERROR"

	// ── Domain: validation ─────────────────────────────────────────────────────

	ErrBadRequest          Code = "BAD_REQUEST"
	ErrNotFound            Code = "NOT_FOUND"
	ErrAlreadyExists       Code = "ALREADY_EXISTS"
	ErrConflict            Code = "CONFLICT"
	ErrUnprocessableEntity Code = "UNPROCESSABLE_ENTITY"
)

// ToGRPCCode maps an ergerr.Code to its gRPC codes.Code equivalent.
func (c Code) ToGRPCCode() codes.Code {
	switch c {
	case ErrCrawlerBlacklisted, ErrCrawlerRobotsDisallowed, ErrBotUnauthorized, ErrForbidden:
		return codes.PermissionDenied
	case ErrCrawlerDuplicate, ErrAlreadyExists:
		return codes.AlreadyExists
	case ErrCrawlerTimeout:
		return codes.DeadlineExceeded
	case ErrCrawlerRateLimited, ErrBotConversationLimit, ErrNotificationQueueFull, ErrTrendingRateLimited:
		return codes.ResourceExhausted
	case ErrCrawlerInvalidURL, ErrNotificationInvalidPayload, ErrBadRequest:
		return codes.InvalidArgument
	case ErrCrawlerNotFound, ErrBotCommandNotFound, ErrTrendingNoData, ErrTenantNotFound, ErrNotFound:
		return codes.NotFound
	case ErrCrawlerServiceUnavailable, ErrBotServiceUnavailable, ErrServiceUnavailable:
		return codes.Unavailable
	case ErrCrawlerQualityTooLow, ErrUnprocessableEntity:
		return codes.OutOfRange
	case ErrTrendingStaleCache:
		return codes.Aborted
	case ErrTrendingSourceDisabled, ErrNotificationChannelDisabled, ErrTenantMissing, ErrConflict:
		return codes.FailedPrecondition
	case ErrNotificationDeliveryFailed, ErrInternal, ErrDatabaseError, ErrCacheError, ErrQueueError:
		return codes.Internal
	case ErrUnauthenticated, ErrTokenExpired, ErrTokenInvalid:
		return codes.Unauthenticated
	default:
		return codes.Internal
	}
}

// ToGRPStatus returns a gRPC status.Status for this error code with optional details.
func (c Code) ToGRPStatus(msg string, opts ...Option) *status.Status {
	o := buildOpts(opts)
	st := status.New(c.ToGRPCCode(), msg)
	if o.details != nil {
		st, _ = st.WithDetails(o.details)
	}
	return st
}

// ToError returns an error wrapping this code with the given message.
func (c Code) ToError(msg string, opts ...Option) error {
	o := buildOpts(opts)
	return &E{Code: c, Message: msg, details: o.details, metadata: o.metadata}
}

// ── E — structured error type ───────────────────────────────────────────────

type E struct {
	Code     Code
	Message  string
	details  protoadapt.MessageV1
	metadata map[string]string
}

func (e *E) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func (e *E) Unwrap() error { return nil }
func (e *E) Is(target error) bool {
	if t, ok := target.(*E); ok {
		return e.Code == t.Code
	}
	return false
}
func (e *E) GRPCStatus() *status.Status {
	return e.Code.ToGRPStatus(e.Message, WithDetails(e.details), WithMetadata(e.metadata))
}
func (e *E) WithRequestID(id string) *E {
	cp := *e
	cp.metadata = mergeM(cp.metadata, map[string]string{"request_id": id})
	return &cp
}
func (e *E) WithTraceID(id string) *E {
	cp := *e
	cp.metadata = mergeM(cp.metadata, map[string]string{"trace_id": id})
	return &cp
}
func (e *E) WithRetryAfter(sec int) *E {
	cp := *e
	cp.metadata = mergeM(cp.metadata, map[string]string{"retry_after": fmt.Sprintf("%d", sec)})
	return &cp
}
func (e *E) ToResponse() *ErrorResponse {
	md := e.metadata
	if md == nil {
		md = map[string]string{}
	}
	return &ErrorResponse{
		Code:       e.Code,
		Message:    e.Message,
		RequestID:  md["request_id"],
		TraceID:    md["trace_id"],
		RetryAfter: parseRetryAfter(md["retry_after"]),
	}
}

func mergeM(base, add map[string]string) map[string]string {
	if base == nil {
		return add
	}
	out := make(map[string]string, len(base)+len(add))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range add {
		out[k] = v
	}
	return out
}

func parseRetryAfter(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ── Option helpers ───────────────────────────────────────────────────────────

type Option func(*opts)

type opts struct {
	details  protoadapt.MessageV1
	metadata map[string]string
}

func buildOpts(optionList []Option) opts {
	var o opts
	for _, fn := range optionList {
		fn(&o)
	}
	return o
}

func WithDetails(d protoadapt.MessageV1) Option { return func(o *opts) { o.details = d } }
func WithMetadata(m map[string]string) Option   { return func(o *opts) { o.metadata = m } }

// ── Constructor helpers ─────────────────────────────────────────────────────

func New(code Code, msg string) error { return &E{Code: code, Message: msg} }

func Errorf(code Code, format string, args ...any) error {
	return &E{Code: code, Message: fmt.Sprintf(format, args...)}
}

func Wrap(code Code, err error, msg string) error {
	if err == nil {
		return nil
	}
	return &E{Code: code, Message: fmt.Sprintf("%s: %v", msg, err)}
}

func WrapIf(code Code, err error, msg string) error {
	if err == nil {
		return nil
	}
	return Wrap(code, err, msg)
}

func Is(err error, code Code) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*E); ok && e.Code == code {
		return true
	}
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return Is(u.Unwrap(), code)
	}
	return false
}
