package lms

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestControllerRegistersFECompatibilityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ctrl := NewController(nil)
	ctrl.RegisterRoutes(router.Group("/api/lms"))

	want := map[string]bool{
		"POST /api/lms/classes/:classId/students/bulk-move": false,
		"POST /api/lms/questions/random-pick":               false,
		"POST /api/lms/quizzes/from-questions":              false,
		"POST /api/lms/quizzes/random":                      false,
		"PUT /api/lms/quizzes/:quizId":                      false,
		"POST /api/lms/attempts/:attemptId/answers":         false,
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
