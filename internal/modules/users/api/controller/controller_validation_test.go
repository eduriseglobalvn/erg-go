package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
)

func TestUsersAdminStatusRejectsInvalidEnumBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token, err := validator.GenerateHS256(&auth.JWTClaims{
		UserID:    "507f1f77bcf86cd799439011",
		SessionID: "session-1",
		Roles:     []string{"admin"},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	New(nil, validator, nil).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPut, "/api/users/507f1f77bcf86cd799439012/status", bytes.NewBufferString(`{"status":"ROOT"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}
