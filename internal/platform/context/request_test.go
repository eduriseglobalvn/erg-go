package context

import (
	stdctx "context"
	"testing"
)

func TestRequestMetadataRoundTrip(t *testing.T) {
	ctx := stdctx.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithTenantID(ctx, "tenant-a")
	ctx = WithUserID(ctx, "user-9")

	if got := RequestID(ctx); got != "req-123" {
		t.Fatalf("RequestID() = %q, want %q", got, "req-123")
	}
	if got := TenantID(ctx); got != "tenant-a" {
		t.Fatalf("TenantID() = %q, want %q", got, "tenant-a")
	}
	if got := UserID(ctx); got != "user-9" {
		t.Fatalf("UserID() = %q, want %q", got, "user-9")
	}
}

func TestRequestMetadataMissingValuesReturnEmptyString(t *testing.T) {
	ctx := stdctx.Background()

	if got := RequestID(ctx); got != "" {
		t.Fatalf("RequestID() = %q, want empty", got)
	}
	if got := TenantID(ctx); got != "" {
		t.Fatalf("TenantID() = %q, want empty", got)
	}
	if got := UserID(ctx); got != "" {
		t.Fatalf("UserID() = %q, want empty", got)
	}
}
