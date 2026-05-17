package service

import (
	"context"
	"strings"
	"time"
)

const (
	ActionAuthzDenied       = "authz.denied"
	ActionRoleChanged       = "identity.role.changed"
	ActionPermissionChanged = "identity.permission.changed"
	ActionAssetLaunched     = "hoclieu.asset.launched"
	ActionAssetDownloaded   = "hoclieu.asset.downloaded"
	ActionAssetStreamed     = "hoclieu.asset.streamed"
	ActionQuizPublished     = "lms.quiz.published"
	ActionQuizSubmitted     = "lms.quiz.submitted"

	OutcomeDenied  = "denied"
	OutcomeSuccess = "success"
)

type contextFieldsKey struct{}

// ContextFields carries request-level observability data into audit events.
type ContextFields struct {
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
	Route     string `json:"route,omitempty"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// ActorFields identifies the subject that caused an audit event.
type ActorFields struct {
	UserID      string   `json:"user_id,omitempty"`
	UserEmail   string   `json:"user_email,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// Event is the lightweight audit contract other modules can publish.
type Event struct {
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	TenantID     string         `json:"tenant_id,omitempty"`
	Actor        ActorFields    `json:"actor,omitempty"`
	Outcome      string         `json:"outcome,omitempty"`
	ReasonCode   string         `json:"reason_code,omitempty"`
	Context      ContextFields  `json:"context,omitempty"`
	Changes      map[string]any `json:"changes,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at,omitempty"`
}

// Publisher is implemented by the audit service and by no-op/test publishers.
type Publisher interface {
	PublishAuditEvent(context.Context, Event) error
}

// PublisherFunc adapts a function to Publisher.
type PublisherFunc func(context.Context, Event) error

// PublishAuditEvent implements Publisher.
func (fn PublisherFunc) PublishAuditEvent(ctx context.Context, event Event) error {
	return fn(ctx, event)
}

// NoopPublisher lets modules install audit hooks before the real service is wired.
type NoopPublisher struct{}

// PublishAuditEvent implements Publisher.
func (NoopPublisher) PublishAuditEvent(context.Context, Event) error { return nil }

// WithContextFields stores audit observability fields on a context.
func WithContextFields(ctx context.Context, fields ContextFields) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextFieldsKey{}, fields)
}

// ContextFieldsFromContext extracts audit observability fields from a context.
func ContextFieldsFromContext(ctx context.Context) ContextFields {
	if ctx == nil {
		return ContextFields{}
	}
	fields, _ := ctx.Value(contextFieldsKey{}).(ContextFields)
	fields.RequestID = firstNonEmpty(fields.RequestID, stringContextValue(ctx, "request_id"))
	fields.TraceID = firstNonEmpty(fields.TraceID, stringContextValue(ctx, "trace_id"))
	fields.SpanID = firstNonEmpty(fields.SpanID, stringContextValue(ctx, "span_id"))
	return fields
}

// AuthzDeniedEventInput describes an authorization denial audit event.
type AuthzDeniedEventInput struct {
	TenantID     string
	UserID       string
	UserEmail    string
	Roles        []string
	Permissions  []string
	Action       string
	ResourceType string
	ResourceID   string
	ReasonCode   string
	Required     string
	Context      ContextFields
	Metadata     map[string]any
}

// BuildAuthzDeniedEvent creates the standard event for authorization denials.
func BuildAuthzDeniedEvent(input AuthzDeniedEventInput) Event {
	action := firstNonEmpty(input.Action, ActionAuthzDenied)
	resourceType := firstNonEmpty(input.ResourceType, "authorization")
	reasonCode := firstNonEmpty(input.ReasonCode, "AUTHZ_DENIED")
	metadata := cloneMetadata(input.Metadata)
	putString(metadata, "reason_code", reasonCode)
	putString(metadata, "required", input.Required)

	return Event{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   input.ResourceID,
		TenantID:     input.TenantID,
		Actor: ActorFields{
			UserID:      input.UserID,
			UserEmail:   input.UserEmail,
			Roles:       append([]string(nil), input.Roles...),
			Permissions: append([]string(nil), input.Permissions...),
		},
		Outcome:    OutcomeDenied,
		ReasonCode: reasonCode,
		Context:    input.Context,
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC(),
	}
}

// AccessChangeEventInput describes role or permission changes.
type AccessChangeEventInput struct {
	TenantID     string
	Actor        ActorFields
	TargetUserID string
	ChangeType   string
	Granted      []string
	Revoked      []string
	Context      ContextFields
	Metadata     map[string]any
}

// BuildAccessChangeEvent creates an audit event for role/permission changes.
func BuildAccessChangeEvent(input AccessChangeEventInput) Event {
	changeType := strings.ToLower(strings.TrimSpace(input.ChangeType))
	action := ActionPermissionChanged
	if changeType == "role" || changeType == "roles" {
		action = ActionRoleChanged
		changeType = "role"
	} else {
		changeType = "permission"
	}
	metadata := cloneMetadata(input.Metadata)
	metadata["change_type"] = changeType
	if len(input.Granted) > 0 {
		metadata["granted"] = append([]string(nil), input.Granted...)
	}
	if len(input.Revoked) > 0 {
		metadata["revoked"] = append([]string(nil), input.Revoked...)
	}

	return Event{
		Action:       action,
		ResourceType: "identity_access",
		ResourceID:   input.TargetUserID,
		TenantID:     input.TenantID,
		Actor:        cloneActor(input.Actor),
		Outcome:      OutcomeSuccess,
		Context:      input.Context,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
	}
}

// AssetEventInput describes asset launch/stream/download events.
type AssetEventInput struct {
	Action      string
	TenantID    string
	UserID      string
	UserEmail   string
	AssetID     string
	ResourceID  string
	FileType    string
	LaunchMode  string
	CanDownload bool
	Context     ContextFields
	Metadata    map[string]any
}

// BuildAssetEvent creates an audit event for protected asset access.
func BuildAssetEvent(input AssetEventInput) Event {
	action := firstNonEmpty(input.Action, ActionAssetLaunched)
	metadata := cloneMetadata(input.Metadata)
	putString(metadata, "resource_id", input.ResourceID)
	putString(metadata, "file_type", input.FileType)
	putString(metadata, "launch_mode", input.LaunchMode)
	metadata["can_download"] = input.CanDownload

	return Event{
		Action:       action,
		ResourceType: "hoclieu_asset",
		ResourceID:   input.AssetID,
		TenantID:     input.TenantID,
		Actor:        ActorFields{UserID: input.UserID, UserEmail: input.UserEmail},
		Outcome:      OutcomeSuccess,
		Context:      input.Context,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
	}
}

// QuizEventInput describes quiz publish/submit events.
type QuizEventInput struct {
	Action       string
	TenantID     string
	UserID       string
	UserEmail    string
	QuizID       string
	AttemptID    string
	AssignmentID string
	QuizVersion  int
	PackageHash  string
	AnswerCount  int
	Score        float64
	MaxScore     float64
	Percent      float64
	Passed       bool
	Context      ContextFields
	Metadata     map[string]any
}

// BuildQuizEvent creates an audit event for quiz publish/submit operations.
func BuildQuizEvent(input QuizEventInput) Event {
	action := firstNonEmpty(input.Action, ActionQuizSubmitted)
	resourceID := input.QuizID
	resourceType := "lms_quiz"
	if action == ActionQuizSubmitted && input.AttemptID != "" {
		resourceID = input.AttemptID
		resourceType = "lms_attempt"
	}
	metadata := cloneMetadata(input.Metadata)
	putString(metadata, "quiz_id", input.QuizID)
	putString(metadata, "assignment_id", input.AssignmentID)
	putString(metadata, "package_hash", input.PackageHash)
	if input.AttemptID != "" {
		metadata["attempt_id"] = input.AttemptID
	}
	if input.QuizVersion > 0 {
		metadata["quiz_version"] = input.QuizVersion
	}
	if input.AnswerCount >= 0 {
		metadata["answer_count"] = input.AnswerCount
	}
	if input.MaxScore > 0 {
		metadata["score"] = input.Score
		metadata["max_score"] = input.MaxScore
		metadata["percent"] = input.Percent
		metadata["passed"] = input.Passed
	}

	return Event{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		TenantID:     input.TenantID,
		Actor:        ActorFields{UserID: input.UserID, UserEmail: input.UserEmail},
		Outcome:      OutcomeSuccess,
		Context:      input.Context,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
	}
}

// StorageMetadata returns a metadata payload enriched with observability fields.
func (e Event) StorageMetadata() map[string]any {
	metadata := cloneMetadata(e.Metadata)
	putString(metadata, "outcome", e.Outcome)
	putString(metadata, "reason_code", e.ReasonCode)
	if !contextFieldsEmpty(e.Context) {
		metadata["context"] = e.Context
		putString(metadata, "request_id", e.Context.RequestID)
		putString(metadata, "trace_id", e.Context.TraceID)
		putString(metadata, "span_id", e.Context.SpanID)
	}
	return metadata
}

// PublishAuditEvent publishes an Event through the real audit service.
func (s *Service) PublishAuditEvent(ctx context.Context, event Event) error {
	return s.LogAction(ctx, LogParams{
		Action:       event.Action,
		ResourceType: event.ResourceType,
		ResourceID:   event.ResourceID,
		UserID:       event.Actor.UserID,
		UserEmail:    event.Actor.UserEmail,
		IPAddress:    event.Context.IPAddress,
		UserAgent:    event.Context.UserAgent,
		TenantID:     event.TenantID,
		Changes:      event.Changes,
		Metadata:     event.StorageMetadata(),
	})
}

func cloneActor(actor ActorFields) ActorFields {
	actor.Roles = append([]string(nil), actor.Roles...)
	actor.Permissions = append([]string(nil), actor.Permissions...)
	return actor
}

func cloneMetadata(input map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range input {
		out[k] = v
	}
	return out
}

func putString(metadata map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		metadata[key] = value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func contextFieldsEmpty(fields ContextFields) bool {
	return fields == ContextFields{}
}

func stringContextValue(ctx context.Context, key string) string {
	if value, ok := ctx.Value(key).(string); ok {
		return value
	}
	return ""
}
