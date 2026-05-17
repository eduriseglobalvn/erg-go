package hoclieu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHocLieuLaunchReturnsSecureViewerContract(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/asset-global-success-7-lecture-pptx/launch", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var env struct {
		Data LaunchResponseDTO `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
	}
	launch := env.Data
	if launch.EmbedURL != "" {
		t.Fatalf("embedUrl should be empty for secure launch, got %q", launch.EmbedURL)
	}
	if strings.Contains(rec.Body.String(), "docs.google.com") || strings.Contains(rec.Body.String(), "drive.google.com") {
		t.Fatalf("launch response leaked upstream url: %s", rec.Body.String())
	}
	if !strings.HasPrefix(launch.ViewerTokenURL, "/api/hoclieu/assets/asset-global-success-7-lecture-pptx/pages?token=") {
		t.Fatalf("viewerTokenUrl = %q, want same-domain pages URL", launch.ViewerTokenURL)
	}
	if !strings.HasPrefix(launch.StreamURL, "/api/hoclieu/assets/asset-global-success-7-lecture-pptx/stream?token=") {
		t.Fatalf("streamUrl = %q, want same-domain stream URL", launch.StreamURL)
	}
	if !launch.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("expiresAt = %s, want a future TTL", launch.ExpiresAt)
	}
	if launch.CanDownload {
		t.Fatalf("canDownload = true, want false for protected PPTX seed")
	}
	if launch.Watermark == nil || launch.Watermark.Text == "" {
		t.Fatalf("watermark missing from launch response: %+v", launch.Watermark)
	}
	if launch.Audit.Event != "hoclieu.asset.launch" || launch.Audit.AssetID != launch.AssetID {
		t.Fatalf("audit metadata missing or mismatched: %+v", launch.Audit)
	}
}
