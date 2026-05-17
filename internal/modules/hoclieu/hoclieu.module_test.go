package hoclieu

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
)

func TestHocLieuPublicHomeDoesNotRequireAuth(t *testing.T) {
	router, _ := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/home", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestHocLieuResourcesRejectAnonymous(t *testing.T) {
	router, _ := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHocLieuResourcesRejectWrongPortal(t *testing.T) {
	router, validator, _ := testRouterWithValidator(t)
	token := hocLieuToken(t, validator, &auth.JWTClaims{
		UserID:      "teacher-1",
		Portal:      "lms",
		Permissions: []string{"hoclieu.resource.read"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHocLieuResourcesRejectMissingPermission(t *testing.T) {
	router, validator, _ := testRouterWithValidator(t)
	token := hocLieuToken(t, validator, &auth.JWTClaims{
		UserID: "teacher-1",
		Portal: "hoclieu",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHocLieuResourcesReturnSelectedFileTypeBadge(t *testing.T) {
	router, token := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources?programSlug=global-success&gradeId=7", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"selectedFileType":"PDF"`) || !strings.Contains(body, `"fileTypeBadge":"PPTX"`) {
		t.Fatalf("response does not expose selected file type badges: %s", body)
	}
}

func TestHocLieuRejectsInvalidSelectedFileType(t *testing.T) {
	router, token := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/hoclieu/resources", strings.NewReader(`{
		"title":"Bad file",
		"programSlug":"global-success",
		"subjectId":"tieng-anh",
		"categoryId":"textbook",
		"selectedFileType":"BAD"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHocLieuAdminContentModelIncludesExtensibleTaxonomy(t *testing.T) {
	router, token := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/admin/content-model", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"bookSeries"`, `"topics"`, `"designerPresets"`, `"fileTypes"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("content model missing %s: %s", fragment, body)
		}
	}
}

func TestHocLieuAdminCanListSubjectsSeparately(t *testing.T) {
	router, token := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/admin/taxonomy/subjects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"id":"giao-duc-stem"`, `"id":"ic3"`, `"id":"mos"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("subjects response missing %s: %s", fragment, body)
		}
	}
}

func TestHocLieuAdminCanCreateTopicAndPublishedResource(t *testing.T) {
	router, token := testRouter(t)
	topicRec := httptest.NewRecorder()
	topicReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/taxonomy/topics", strings.NewReader(`{
		"label":"Bai 2: Cay xanh quanh em",
		"subjectId":"giao-duc-stem",
		"gradeId":"1",
		"bookSeriesId":"stem-hanh-trinh-sang-tao",
		"categoryId":"slides"
	}`))
	topicReq.Header.Set("Authorization", "Bearer "+token)
	topicReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(topicRec, topicReq)
	if topicRec.Code != http.StatusCreated {
		t.Fatalf("topic status = %d, want 201: %s", topicRec.Code, topicRec.Body.String())
	}

	resourceRec := httptest.NewRecorder()
	resourceReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/resources", strings.NewReader(`{
		"title":"STEM 1 - Bai giang Cay xanh",
		"programSlug":"giao-duc-stem",
		"subjectId":"giao-duc-stem",
		"gradeId":"1",
		"categoryId":"slides",
		"documentTypeId":"slides",
		"bookSeriesId":"stem-hanh-trinh-sang-tao",
		"topicId":"bai-2-cay-xanh-quanh-em",
		"selectedFileType":"PPTX",
		"originalFileName":"stem-1-cay-xanh.pptx",
		"detectedMimeType":"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"status":"published",
		"visibility":"public",
		"lectureDesign":{"templateId":"hoclieu-lecture-grid","bannerTitle":"STEM 1","unitLabels":["Bai 2"]}
	}`))
	resourceReq.Header.Set("Authorization", "Bearer "+token)
	resourceReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resourceRec, resourceReq)
	if resourceRec.Code != http.StatusCreated {
		t.Fatalf("resource status = %d, want 201: %s", resourceRec.Code, resourceRec.Body.String())
	}
	if !strings.Contains(resourceRec.Body.String(), `"lectureDesign"`) || !strings.Contains(resourceRec.Body.String(), `"documentTypeId":"slides"`) {
		t.Fatalf("resource response missing authoring metadata: %s", resourceRec.Body.String())
	}
}

func TestHocLieuAdminResourceUploadFilterAndDeleteFlow(t *testing.T) {
	router, token := testRouter(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"title":            "STEM 1 - Upload Unit 12",
		"subjectId":        "giao-duc-stem",
		"gradeId":          "1",
		"categoryId":       "slides",
		"documentTypeId":   "slides",
		"bookSeriesId":     "stem-hanh-trinh-sang-tao",
		"topicId":          "unit-12-at-the-lake",
		"selectedFileType": "PPTX",
		"visibility":       "public",
		"status":           "published",
		"canDownload":      "true",
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "unit-12.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("pptx placeholder")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	uploadRec := httptest.NewRecorder()
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/resources/upload", &body)
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201: %s", uploadRec.Code, uploadRec.Body.String())
	}
	if !strings.Contains(uploadRec.Body.String(), `"resource"`) || !strings.Contains(uploadRec.Body.String(), `"asset"`) {
		t.Fatalf("upload response missing resource/asset contract: %s", uploadRec.Body.String())
	}
	var uploadBody struct {
		Data struct {
			Resource struct {
				ID string `json:"id"`
			} `json:"resource"`
		} `json:"data"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadBody); err != nil {
		t.Fatal(err)
	}
	if uploadBody.Data.Resource.ID == "" {
		t.Fatalf("upload response missing resource id: %s", uploadRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources?subjectId=giao-duc-stem&bookSeriesId=stem-hanh-trinh-sang-tao&topicId=unit-12-at-the-lake&documentTypeId=slides", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), "STEM 1 - Upload Unit 12") {
		t.Fatalf("list did not include uploaded resource: %s", listRec.Body.String())
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/hoclieu/admin/resources/"+uploadBody.Data.Resource.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestHocLieuAdminCanMoveTaxonomyNode(t *testing.T) {
	router, token := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/hoclieu/admin/taxonomy/topics/ta7-my-new-school", strings.NewReader(`{
		"parentId":"slides",
		"categoryId":"slides",
		"bookSeriesId":"stem-hanh-trinh-sang-tao",
		"sortOrder":99
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"parentId":"slides"`, `"categoryId":"slides"`, `"sortOrder":99`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("patch response missing %s: %s", fragment, body)
		}
	}
}

func TestHocLieuLaunchHidesUpstreamURLs(t *testing.T) {
	router, token := testRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/asset-global-success-7-lecture-pptx/launch", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "docs.google.com") || strings.Contains(body, "drive.google.com") {
		t.Fatalf("launch response leaked upstream url: %s", body)
	}
	if !strings.Contains(body, `"viewerTokenUrl"`) || !strings.Contains(body, `"streamUrl"`) {
		t.Fatalf("launch response missing same-domain viewer urls: %s", body)
	}
}

func testRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	router, _, token := testRouterWithValidator(t)
	return router, token
}

func testRouterWithValidator(t *testing.T) (*gin.Engine, *auth.JWTValidator, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token := hocLieuToken(t, validator, &auth.JWTClaims{
		UserID: "teacher-1",
		Portal: "hoclieu",
		Roles:  []string{"teacher"},
		Permissions: []string{
			"hoclieu.resource.read",
			"hoclieu.resource.create",
			"hoclieu.resource.update",
			"hoclieu.resource.delete",
			"hoclieu.asset.upload",
			"hoclieu.asset.manage",
			"hoclieu.asset.launch",
			"hoclieu.asset.download",
		},
	})
	router := gin.New()
	mod := NewModule(Deps{JWTValidator: validator})
	if err := mod.Setup(); err != nil {
		t.Fatal(err)
	}
	seedService(mod.svc)
	mod.RegisterRoutes(router)
	return router, validator, token
}

func hocLieuToken(t *testing.T, validator *auth.JWTValidator, claims *auth.JWTClaims) string {
	t.Helper()
	token, err := validator.GenerateHS256(claims, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
