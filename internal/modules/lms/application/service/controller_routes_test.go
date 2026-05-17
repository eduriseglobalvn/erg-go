package service

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
		"GET /api/lms/scopes/me":                             false,
		"PUT /api/lms/scopes/current":                        false,
		"GET /api/lms/education-units":                       false,
		"POST /api/lms/education-units":                      false,
		"GET /api/lms/education-units/:id":                   false,
		"PATCH /api/lms/education-units/:id":                 false,
		"GET /api/lms/education-units/:id/classes":           false,
		"POST /api/lms/classes":                              false,
		"GET /api/lms/classes/:classId":                      false,
		"GET /api/lms/dashboard/overview":                    false,
		"GET /api/lms/dashboard/interventions":               false,
		"GET /api/lms/assignments/active":                    false,
		"POST /api/lms/assignments/deliveries":               false,
		"GET /api/lms/classes/:classId/reports":              false,
		"GET /api/lms/classes/:classId/students":             false,
		"GET /api/lms/students/:studentId/journey":           false,
		"POST /api/lms/classes/:classId/students/bulk-move":  false,
		"GET /api/lms/question-bank/subjects":                false,
		"GET /api/lms/question-bank/categories":              false,
		"GET /api/lms/question-bank/questions":               false,
		"POST /api/lms/question-bank/questions":              false,
		"PATCH /api/lms/question-bank/questions/:questionId": false,
		"GET /api/lms/quiz-bank":                             false,
		"POST /api/lms/questions/random-pick":                false,
		"POST /api/lms/quizzes/from-questions":               false,
		"POST /api/lms/quizzes/random":                       false,
		"PUT /api/lms/quizzes/:quizId":                       false,
		"PATCH /api/lms/attempts/:attemptId/draft":           false,
		"POST /api/lms/attempts/:attemptId/answers":          false,
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
