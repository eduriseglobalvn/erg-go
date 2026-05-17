package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestControllerRegistersLMSAuthCompatibilityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ctrl := NewController(nil, nil, nil, nil)
	ctrl.RegisterRoutes(router)

	want := map[string]bool{
		"POST /api/lms/auth/login":                false,
		"POST /api/lms/auth/register":             false,
		"POST /api/lms/auth/logout":               false,
		"GET /api/lms/auth/profile":               false,
		"GET /api/lms/auth/sessions":              false,
		"PUT /api/lms/auth/accounts/:id/profile":  false,
		"POST /api/lms/auth/accounts/:id/avatar":  false,
		"PUT /api/lms/auth/accounts/:id/password": false,
		"POST /api/lms/auth/providers/google":     false,
		"POST /api/lms/auth/providers/apple":      false,
		"POST /api/auth/google/login":             false,
		"POST /api/auth/login":                    false,
		"GET /api/auth/profile":                   false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Fatalf("expected route %s to be registered", route)
		}
	}
}

func TestShouldSuppressTokensDoesNotHideBodyTokensForAllowedBrowserOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewController(nil, nil, nil, nil)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/lms/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3001")
	ctx.Request = req

	if ctrl.shouldSuppressTokens(ctx) {
		t.Fatal("browser-origin login responses must keep body tokens for the current API contract")
	}
}
