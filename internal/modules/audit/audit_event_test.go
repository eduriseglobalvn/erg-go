package audit

import (
	"testing"

	auditservice "erg.ninja/internal/modules/audit/application/service"
)

func TestAuthzDeniedEventContainsReasonActionAndResource(t *testing.T) {
	event := auditservice.BuildAuthzDeniedEvent(auditservice.AuthzDeniedEventInput{
		UserID:       "user-1",
		Action:       auditservice.ActionAuthzDenied,
		ResourceType: "hoclieu_asset",
		ResourceID:   "asset-1",
		ReasonCode:   "MISSING_PERMISSION",
		Required:     "hoclieu.asset.launch",
	})

	if event.Action != auditservice.ActionAuthzDenied {
		t.Fatalf("action = %q, want %q", event.Action, auditservice.ActionAuthzDenied)
	}
	if event.ResourceType != "hoclieu_asset" || event.ResourceID != "asset-1" {
		t.Fatalf("resource = %s/%s, want hoclieu_asset/asset-1", event.ResourceType, event.ResourceID)
	}
	if event.ReasonCode != "MISSING_PERMISSION" {
		t.Fatalf("reason = %q, want MISSING_PERMISSION", event.ReasonCode)
	}
	if event.Metadata["required"] != "hoclieu.asset.launch" {
		t.Fatalf("required metadata = %v, want hoclieu.asset.launch", event.Metadata["required"])
	}
}
