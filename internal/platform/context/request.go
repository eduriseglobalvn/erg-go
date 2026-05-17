package context

import stdctx "context"

type key string

const (
	requestIDKey key = "request_id"
	tenantIDKey  key = "tenant_id"
	userIDKey    key = "user_id"
)

func WithRequestID(ctx stdctx.Context, id string) stdctx.Context {
	return stdctx.WithValue(ctx, requestIDKey, id)
}

func WithTenantID(ctx stdctx.Context, id string) stdctx.Context {
	return stdctx.WithValue(ctx, tenantIDKey, id)
}

func WithUserID(ctx stdctx.Context, id string) stdctx.Context {
	return stdctx.WithValue(ctx, userIDKey, id)
}

func RequestID(ctx stdctx.Context) string {
	return value(ctx, requestIDKey)
}

func TenantID(ctx stdctx.Context) string {
	return value(ctx, tenantIDKey)
}

func UserID(ctx stdctx.Context) string {
	return value(ctx, userIDKey)
}

func value(ctx stdctx.Context, k key) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(k).(string); ok {
		return v
	}
	return ""
}
