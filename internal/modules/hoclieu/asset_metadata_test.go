package hoclieu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHocLieuCreateResourcePDFReturnsBadgeAndMetadata(t *testing.T) {
	router, _, token := testRouterWithValidator(t)

	resource := createPDFResource(t, router, token)
	if resource.SelectedFileType != AssetFileTypePDF {
		t.Fatalf("selectedFileType = %q, want PDF", resource.SelectedFileType)
	}
	if resource.FileTypeBadge != "PDF" {
		t.Fatalf("fileTypeBadge = %q, want PDF", resource.FileTypeBadge)
	}
	if resource.LaunchMode != LaunchModePDFReader {
		t.Fatalf("launchMode = %q, want %q", resource.LaunchMode, LaunchModePDFReader)
	}
	if resource.FileExtension != ".pdf" || resource.DetectedMimeType != "application/pdf" {
		t.Fatalf("metadata not normalized: extension=%q mime=%q", resource.FileExtension, resource.DetectedMimeType)
	}
	if resource.FileTypeAudit == nil || resource.FileTypeAudit.ChangedBy != "teacher-1" || resource.FileTypeAudit.Source != "resource.create" {
		t.Fatalf("fileTypeAudit missing admin metadata: %+v", resource.FileTypeAudit)
	}
	if len(resource.Assets) != 1 || resource.Assets[0].FileTypeBadge != "PDF" {
		t.Fatalf("implicit asset missing PDF badge: %+v", resource.Assets)
	}
}

func TestHocLieuRejectsMismatchedFileTypeMetadata(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/hoclieu/assets", strings.NewReader(`{
		"resourceId":"res-global-success-7-student-book",
		"title":"Wrong extension",
		"selectedFileType":"PDF",
		"originalFileName":"wrong.pptx",
		"detectedMimeType":"application/pdf"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid asset metadata") {
		t.Fatalf("response should describe invalid metadata: %s", rec.Body.String())
	}
}

func TestHocLieuAllowsSoftMimeMismatchWithWarning(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/hoclieu/assets", strings.NewReader(`{
		"resourceId":"res-global-success-7-lecture-bank",
		"title":"OOXML container",
		"selectedFileType":"PPTX",
		"originalFileName":"lesson-container.pptx",
		"detectedMimeType":"application/zip"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data AssetDTO `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
	}
	asset := env.Data
	if asset.SelectedFileType != AssetFileTypePPTX || asset.FileTypeBadge != "PPTX" {
		t.Fatalf("asset badge mismatch: %+v", asset)
	}
	if len(asset.MetadataWarnings) != 1 || asset.MetadataWarnings[0].Code != "mime_metadata_warning" {
		t.Fatalf("expected soft mismatch warning, got %+v", asset.MetadataWarnings)
	}
	if asset.FileTypeAudit == nil || len(asset.FileTypeAudit.Warnings) != 1 {
		t.Fatalf("audit should carry warning code: %+v", asset.FileTypeAudit)
	}
}

func TestHocLieuResourceListAndDetailUseBackendBadge(t *testing.T) {
	router, _, token := testRouterWithValidator(t)
	created := createPDFResource(t, router, token)

	detailRec := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources/"+created.ID, nil)
	detailReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200: %s", detailRec.Code, detailRec.Body.String())
	}
	var detailEnv struct {
		Data ResourceDetailDTO `json:"data"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detailEnv); err != nil {
		t.Fatalf("decode detail: %v\n%s", err, detailRec.Body.String())
	}
	if detailEnv.Data.FileTypeBadge != "PDF" || detailEnv.Data.Assets[0].FileTypeBadge != "PDF" {
		t.Fatalf("detail should expose backend badge: %+v", detailEnv.Data)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources?query=ERG%2081%20PDF&fileType=PDF", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", listRec.Code, listRec.Body.String())
	}
	var listEnv struct {
		Data struct {
			Data []ResourceCardDTO `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listEnv); err != nil {
		t.Fatalf("decode list: %v\n%s", err, listRec.Body.String())
	}
	if len(listEnv.Data.Data) == 0 {
		t.Fatalf("created resource not present in list: %s", listRec.Body.String())
	}
	if got := listEnv.Data.Data[0].FileTypeBadge; got != "PDF" {
		t.Fatalf("list fileTypeBadge = %q, want PDF", got)
	}
}

func createPDFResource(t *testing.T, router http.Handler, token string) ResourceDetailDTO {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/hoclieu/resources", strings.NewReader(`{
		"title":"ERG 81 PDF Metadata Card",
		"slug":"erg-81-pdf-metadata-card",
		"programSlug":"ic3-digital-literacy",
		"subjectId":"ic3-gs6",
		"gradeId":"6",
		"categoryId":"ic3-gs6-level-1",
		"selectedFileType":"PDF",
		"originalFileName":"erg-81.pdf",
		"detectedMimeType":"application/pdf"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data ResourceDetailDTO `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
	}
	return env.Data
}
