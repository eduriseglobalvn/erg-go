package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRejectsInvalidEmailWithValidationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewController(nil, nil, nil, nil).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"bad","password":"secret123","fullName":"Vuong"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestLoginRejectsMissingPasswordWithValidationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewController(nil, nil, nil, nil).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"user@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}
