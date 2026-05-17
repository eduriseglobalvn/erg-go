package elearning

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestControllerRegistersStudentContractRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ctrl := NewController(nil, nil)
	ctrl.RegisterStudentRoutes(router.Group("/api/elearning"))

	want := map[string]bool{
		"GET /api/elearning/dashboard":                           false,
		"GET /api/elearning/assignments":                         false,
		"GET /api/elearning/assignments/:assignmentId":           false,
		"GET /api/elearning/scores":                              false,
		"GET /api/elearning/announcements":                       false,
		"GET /api/elearning/notifications":                       false,
		"POST /api/elearning/notifications/:notificationId/read": false,
		"GET /api/elearning/discussions":                         false,
		"POST /api/elearning/discussions":                        false,
		"POST /api/elearning/discussions/:threadId/replies":      false,
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
