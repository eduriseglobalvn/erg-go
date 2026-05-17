package hoclieu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHocLieuDownloadForbiddenWhenCanDownloadFalse(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/asset-global-success-7-student-book-pdf/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Hoclieu-Can-Download"); got != "false" {
		t.Fatalf("X-Hoclieu-Can-Download = %q, want false", got)
	}
	if got := rec.Header().Get("X-Hoclieu-Audit-Event"); got != "hoclieu.asset.download" {
		t.Fatalf("X-Hoclieu-Audit-Event = %q, want hoclieu.asset.download", got)
	}
}

func TestHocLieuStreamSupportsByteRangeForPDF(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/asset-global-success-7-student-book-pdf/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Range", "bytes=0-7")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", got)
	}
	if got := rec.Header().Get("Content-Range"); !strings.HasPrefix(got, "bytes 0-7/") {
		t.Fatalf("Content-Range = %q, want bytes 0-7/<size>", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/pdf") {
		t.Fatalf("Content-Type = %q, want application/pdf", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "8" {
		t.Fatalf("Content-Length = %q, want 8", got)
	}
	if got := rec.Header().Get("X-Hoclieu-Can-Download"); got != "false" {
		t.Fatalf("X-Hoclieu-Can-Download = %q, want false", got)
	}
	if got := rec.Header().Get("X-Hoclieu-Storage-Contract"); got != "placeholder" {
		t.Fatalf("X-Hoclieu-Storage-Contract = %q, want placeholder", got)
	}
	if got := rec.Header().Get("X-Hoclieu-Audit-Event"); got != "hoclieu.asset.stream" {
		t.Fatalf("X-Hoclieu-Audit-Event = %q, want hoclieu.asset.stream", got)
	}
	if got := rec.Body.String(); got != "%PDF-1.4" {
		t.Fatalf("body = %q, want first PDF bytes", got)
	}
}

func TestHocLieuPagesReturnDeterministicPDFAndPPTXDescriptors(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	cases := []struct {
		name      string
		assetID   string
		fileType  AssetFileType
		wantTitle []string
	}{
		{
			name:      "pdf",
			assetID:   "asset-global-success-7-student-book-pdf",
			fileType:  AssetFileTypePDF,
			wantTitle: []string{"Cover", "Unit 1"},
		},
		{
			name:      "pptx",
			assetID:   "asset-global-success-7-lecture-pptx",
			fileType:  AssetFileTypePPTX,
			wantTitle: []string{"Unit 1 - My new school", "Lesson 1 - GETTING STARTED", "A special day"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/"+tc.assetID+"/pages", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}

			var env struct {
				Data ViewerPagesResponseDTO `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
			}
			if env.Data.AssetID != tc.assetID {
				t.Fatalf("assetId = %q, want %q", env.Data.AssetID, tc.assetID)
			}
			if env.Data.SelectedFileType != tc.fileType {
				t.Fatalf("selectedFileType = %q, want %q", env.Data.SelectedFileType, tc.fileType)
			}
			if len(env.Data.Pages) != len(tc.wantTitle) {
				t.Fatalf("pages len = %d, want %d: %+v", len(env.Data.Pages), len(tc.wantTitle), env.Data.Pages)
			}
			for i, want := range tc.wantTitle {
				page := env.Data.Pages[i]
				if page.Index != i+1 {
					t.Fatalf("page[%d].index = %d, want %d", i, page.Index, i+1)
				}
				if page.Title != want {
					t.Fatalf("page[%d].title = %q, want %q", i, page.Title, want)
				}
				if page.Width <= 0 || page.Height <= 0 {
					t.Fatalf("page[%d] dimensions invalid: %+v", i, page)
				}
			}
		})
	}
}
