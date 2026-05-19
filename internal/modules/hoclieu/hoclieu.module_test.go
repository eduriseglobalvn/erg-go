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
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources?subjectId=ic3-gs6&categoryId=ic3-gs6-level-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"selectedFileType":"PPTX"`) || !strings.Contains(body, `"fileTypeBadge":"PPTX"`) {
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
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/admin/taxonomies", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/admin/taxonomies/subjects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"id":"ic3-gs6"`, `"label":"IC3 GS6"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("subjects response missing %s: %s", fragment, body)
		}
	}
}

func TestHocLieuAdminSubjectsMatchContentModelSubjects(t *testing.T) {
	router, token := testRouter(t)

	contentModelRec := httptest.NewRecorder()
	contentModelReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/admin/taxonomies", nil)
	contentModelReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(contentModelRec, contentModelReq)
	if contentModelRec.Code != http.StatusOK {
		t.Fatalf("content-model status = %d, want 200: %s", contentModelRec.Code, contentModelRec.Body.String())
	}

	subjectsRec := httptest.NewRecorder()
	subjectsReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/admin/taxonomies/subjects", nil)
	subjectsReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(subjectsRec, subjectsReq)
	if subjectsRec.Code != http.StatusOK {
		t.Fatalf("subjects status = %d, want 200: %s", subjectsRec.Code, subjectsRec.Body.String())
	}

	var contentModelBody struct {
		Data struct {
			Subjects []TaxonomyOptionDTO `json:"subjects"`
		} `json:"data"`
	}
	if err := json.Unmarshal(contentModelRec.Body.Bytes(), &contentModelBody); err != nil {
		t.Fatalf("decode content-model: %v\n%s", err, contentModelRec.Body.String())
	}

	var subjectsBody struct {
		Data []TaxonomyOptionDTO `json:"data"`
	}
	if err := json.Unmarshal(subjectsRec.Body.Bytes(), &subjectsBody); err != nil {
		t.Fatalf("decode subjects: %v\n%s", err, subjectsRec.Body.String())
	}

	if len(contentModelBody.Data.Subjects) != len(subjectsBody.Data) {
		t.Fatalf("subjects length mismatch: content-model=%d taxonomy/subjects=%d", len(contentModelBody.Data.Subjects), len(subjectsBody.Data))
	}
	for index := range contentModelBody.Data.Subjects {
		if contentModelBody.Data.Subjects[index].ID != subjectsBody.Data[index].ID {
			t.Fatalf("subject mismatch at index %d: content-model=%s taxonomy/subjects=%s", index, contentModelBody.Data.Subjects[index].ID, subjectsBody.Data[index].ID)
		}
	}
}

func TestHocLieuAdminCanCreateTopicAndPublishedResource(t *testing.T) {
	router, token := testRouter(t)
	topicRec := httptest.NewRecorder()
	topicReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/taxonomies/topics", strings.NewReader(`{
		"label":"Chu de 4: Email co ban",
		"subjectId":"ic3-gs6",
		"gradeId":"6",
		"categoryId":"ic3-gs6-level-1",
		"parentId":"ic3-gs6-level-1"
	}`))
	topicReq.Header.Set("Authorization", "Bearer "+token)
	topicReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(topicRec, topicReq)
	if topicRec.Code != http.StatusCreated {
		t.Fatalf("topic status = %d, want 201: %s", topicRec.Code, topicRec.Body.String())
	}

	resourceRec := httptest.NewRecorder()
	resourceReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/resources", strings.NewReader(`{
		"title":"IC3 GS6 - Email co ban",
		"programSlug":"ic3-digital-literacy",
		"subjectId":"ic3-gs6",
		"gradeId":"6",
		"categoryId":"ic3-gs6-level-1",
		"documentTypeId":"ic3-gs6-level-1",
		"topicId":"chu-de-4-email-co-ban",
		"selectedFileType":"PPTX",
		"originalFileName":"ic3-gs6-email-co-ban.pptx",
		"detectedMimeType":"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"status":"published",
		"visibility":"public",
		"lectureDesign":{"templateId":"hoclieu-lecture-grid","bannerTitle":"IC3 GS6","unitLabels":["Chu de 4"]}
	}`))
	resourceReq.Header.Set("Authorization", "Bearer "+token)
	resourceReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resourceRec, resourceReq)
	if resourceRec.Code != http.StatusCreated {
		t.Fatalf("resource status = %d, want 201: %s", resourceRec.Code, resourceRec.Body.String())
	}
	if !strings.Contains(resourceRec.Body.String(), `"lectureDesign"`) || !strings.Contains(resourceRec.Body.String(), `"documentTypeId":"ic3-gs6-level-1"`) {
		t.Fatalf("resource response missing authoring metadata: %s", resourceRec.Body.String())
	}
}

func TestHocLieuAdminResourceUploadFilterAndDeleteFlow(t *testing.T) {
	router, token := testRouter(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"title":            "IC3 GS6 - Upload Excel nhap mon",
		"subjectId":        "ic3-gs6",
		"gradeId":          "7",
		"categoryId":       "ic3-gs6-level-2",
		"documentTypeId":   "ic3-gs6-level-2",
		"topicId":          "ic3-gs6-topic-level-2-excel",
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
	part, err := writer.CreateFormFile("file", "ic3-gs6-excel.pptx")
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
	listReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/resources?subjectId=ic3-gs6&topicId=ic3-gs6-topic-level-2-excel&documentTypeId=ic3-gs6-level-2", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), "IC3 GS6 - Upload Excel nhap mon") {
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
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/taxonomies/topics", strings.NewReader(`{
		"label":"Level 2 - Bài thực hành PowerPoint",
		"subjectId":"ic3-gs6",
		"gradeId":"7",
		"categoryId":"ic3-gs6-level-2",
		"parentId":"ic3-gs6-level-2"
	}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}
	var createBody struct {
		Data TaxonomyOptionDTO `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createBody); err != nil {
		t.Fatalf("decode create response: %v\n%s", err, createRec.Body.String())
	}
	if createBody.Data.ID == "" {
		t.Fatalf("create response missing taxonomy id: %s", createRec.Body.String())
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/hoclieu/admin/taxonomies/topics/"+createBody.Data.ID, strings.NewReader(`{
		"parentId":"ic3-gs6-level-3",
		"categoryId":"ic3-gs6-level-3",
		"sortOrder":99
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"parentId":"ic3-gs6-level-3"`, `"categoryId":"ic3-gs6-level-3"`, `"sortOrder":99`} {
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

func TestHocLieuLinkResourceLaunchesGoogleSlidesEmbed(t *testing.T) {
	router, token := testRouter(t)

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/hoclieu/admin/resources", strings.NewReader(`{
		"title":"Bai giang Google Slides",
		"slug":"bai-giang-google-slides",
		"thumbnailUrl":"https://docs.google.com/presentation/d/1dDhYpe7Ri4mDM3VR0LpDkalzpJDhDkMm/edit",
		"programSlug":"ic3-gs6",
		"subjectId":"ic3-gs6",
		"categoryId":"level-1",
		"sectionId":"bài-01--xác-định-phần-cứng-và-phần-mềm-phù-hợp",
		"topicId":"chủ-đề-1-:-công-dân-số",
		"documentTypeId":"level-1",
		"selectedFileType":"LINK",
		"status":"published",
		"visibility":"public",
		"canDownload":false
	}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}

	launchRec := httptest.NewRecorder()
	launchReq := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/asset-bai-giang-google-slides/launch", nil)
	launchReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(launchRec, launchReq)
	if launchRec.Code != http.StatusOK {
		t.Fatalf("launch status = %d, want 200: %s", launchRec.Code, launchRec.Body.String())
	}

	body := launchRec.Body.String()
	if !strings.Contains(body, `"launchMode":"google_slide_embed"`) {
		t.Fatalf("launch response missing google slide mode: %s", body)
	}
	if !strings.Contains(body, `"embedUrl":"https://docs.google.com/presentation/d/1dDhYpe7Ri4mDM3VR0LpDkalzpJDhDkMm/embed`) {
		t.Fatalf("launch response missing google slide embed url: %s", body)
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
