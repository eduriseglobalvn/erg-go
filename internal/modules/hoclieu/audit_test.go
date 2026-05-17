package hoclieu

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	auditservice "erg.ninja/internal/modules/audit/application/service"
)

func TestAssetLaunchAuditEventDoesNotLeakUpstreamURL(t *testing.T) {
	svc := NewService()
	seedService(svc)
	var event auditservice.Event
	svc.SetAuditPublisher(auditservice.PublisherFunc(func(_ context.Context, published auditservice.Event) error {
		event = published
		return nil
	}))

	ctx := auditservice.WithContextFields(context.Background(), auditservice.ContextFields{RequestID: "req-asset-1"})
	_, err := svc.Launch(ctx, "asset-global-success-7-lecture-pptx", "teacher-1")
	if err != nil {
		t.Fatalf("launch: %v", err)
	}

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "docs.google.com") || strings.Contains(body, "drive.google.com") {
		t.Fatalf("audit event leaked upstream url: %s", body)
	}
	if event.Action != auditservice.ActionAssetLaunched {
		t.Fatalf("action = %q, want %q", event.Action, auditservice.ActionAssetLaunched)
	}
	if event.Context.RequestID != "req-asset-1" {
		t.Fatalf("request id = %q, want req-asset-1", event.Context.RequestID)
	}
}
