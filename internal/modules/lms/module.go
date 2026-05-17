package lms

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	authrepo "erg.ninja/internal/modules/auth/infrastructure/repository"
	lmscontroller "erg.ninja/internal/modules/lms/api/controller"
	lmsservice "erg.ninja/internal/modules/lms/application/service"
	lmsrepository "erg.ninja/internal/modules/lms/infrastructure/repository"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

type Deps struct {
	Mongo        *database.MongoClient
	GORMClient   *database.GORMPostgresClient
	Redis        *cache.RedisClient
	Log          *logger.Logger
	Cfg          *config.Config
	JWTValidator *auth.JWTValidator
}

type Module struct {
	deps Deps
	repo *lmsrepository.Repository
	svc  *lmsservice.Service
	ctrl *lmscontroller.Controller
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func (m *Module) Name() string { return "lms" }

func (m *Module) Setup() error {
	m.repo = lmsrepository.NewRepository(m.deps.Mongo)
	var sheetsClient *storage.GoogleSheetsClient
	if m.deps.Cfg != nil && m.deps.Cfg.GDrive.CredentialJSON != "" {
		client, err := storage.NewGoogleSheetsClient(context.Background(), m.deps.Cfg.GDrive.CredentialJSON)
		if err != nil {
			if m.deps.Log != nil {
				m.deps.Log.Warn().Err(err).Msg("lms: google sheets client disabled")
			}
		} else {
			sheetsClient = client
		}
	}
	var accountRepo *authrepo.Repo
	var accessOption lmsservice.ServiceOption
	if m.deps.GORMClient != nil {
		accountRepo = authrepo.NewRepo(authrepo.RepoDeps{GORM: m.deps.GORMClient, Redis: m.deps.Redis})
		if err := m.deps.GORMClient.DB().AutoMigrate(&postgrescore.UserAccessScope{}); err != nil {
			return err
		}
		accessOption = lmsservice.WithAccessManagementRepository(lmsservice.NewAccessManagementRepository(m.deps.GORMClient.DB()))
	}
	m.svc = lmsservice.NewService(m.repo, sheetsClient, lmsservice.WithAuthRepository(accountRepo), accessOption)
	if m.deps.Cfg != nil && m.deps.Cfg.Lifecycle.LMSSeedOnStartup {
		if err := m.svc.SeedDefaultEducationUnits(context.Background(), lmsDefaultTenantID(m.deps.Cfg)); err != nil {
			return err
		}
	}
	m.ctrl = lmscontroller.NewController(m.svc)
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("lms: module setup")
	}
	return nil
}

func lmsDefaultTenantID(cfg *config.Config) string {
	if cfg != nil && cfg.Tenant.DefaultID != "" {
		return cfg.Tenant.DefaultID
	}
	return "default"
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	base := r.Group("/api/lms")
	base.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	base.Use(middleware.RequirePortal("lms"))
	base.Use(requireLMSRoutePermission())
	m.ctrl.RegisterRoutes(base)
}

func requireLMSRoutePermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		permission := lmsRoutePermissions[c.Request.Method+" "+c.FullPath()]
		if permission == "" {
			c.Next()
			return
		}
		middleware.RequireAccessPermission(permission)(c)
	}
}

var lmsRoutePermissions = map[string]string{
	"GET /api/lms/scopes/me":                               "lms.scope.read",
	"PUT /api/lms/scopes/current":                          "lms.scope.update",
	"GET /api/lms/access-management/options":               "lms.scope.read",
	"GET /api/lms/access-management/users":                 "lms.scope.read",
	"GET /api/lms/access-management/users/:userId/access":  "lms.scope.read",
	"POST /api/lms/access-management/preview":              "lms.scope.read",
	"PUT /api/lms/access-management/users/:userId/access":  "lms.scope.update",
	"GET /api/lms/dashboard/overview":                      "lms.report.read",
	"GET /api/lms/dashboard/interventions":                 "lms.report.read",
	"GET /api/lms/education-units":                         "lms.unit.read",
	"POST /api/lms/education-units":                        "lms.unit.create",
	"GET /api/lms/education-units/:id/classes":             "lms.class.read",
	"GET /api/lms/education-units/:id":                     "lms.unit.read",
	"PATCH /api/lms/education-units/:id":                   "lms.unit.update",
	"GET /api/lms/centers":                                 "lms.unit.read",
	"POST /api/lms/centers":                                "lms.unit.create",
	"PATCH /api/lms/centers/:centerId":                     "lms.unit.update",
	"GET /api/lms/classes":                                 "lms.class.read",
	"POST /api/lms/classes":                                "lms.class.create",
	"PATCH /api/lms/classes/:classId":                      "lms.class.update",
	"GET /api/lms/classes/:classId/reports":                "lms.report.read",
	"GET /api/lms/classes/:classId/students":               "lms.class.read",
	"GET /api/lms/classes/:classId":                        "lms.class.read",
	"POST /api/lms/classes/:classId/students/bulk-move":    "lms.class.update",
	"GET /api/lms/classes/:classId/assignments":            "lms.assignment.read",
	"GET /api/lms/classes/:classId/discussions":            "lms.class.read",
	"POST /api/lms/classes/:classId/discussions":           "lms.class.update",
	"GET /api/lms/students":                                "lms.class.read",
	"GET /api/lms/students/:studentId/journey":             "lms.report.read",
	"GET /api/lms/students/:studentId":                     "lms.class.read",
	"POST /api/lms/students":                               "lms.class.update",
	"POST /api/lms/students/bulk-accounts":                 "lms.student.import",
	"PATCH /api/lms/students/:studentId":                   "lms.class.update",
	"GET /api/lms/students/me/assignments":                 "lms.assignment.read",
	"GET /api/lms/students/me/scores":                      "lms.report.read",
	"POST /api/lms/imports/google-sheet/tabs":              "lms.unit.read",
	"POST /api/lms/imports/google-sheet/preview":           "lms.unit.read",
	"POST /api/lms/imports/google-sheet/commit":            "lms.unit.update",
	"GET /api/lms/imports/:jobId":                          "lms.unit.read",
	"POST /api/lms/imports/:jobId/writeback":               "lms.unit.update",
	"GET /api/lms/question-bank/subjects":                  "lms.question.read",
	"GET /api/lms/question-bank/categories":                "lms.question.read",
	"GET /api/lms/question-bank/questions":                 "lms.question.read",
	"POST /api/lms/question-bank/questions":                "lms.question.create",
	"PATCH /api/lms/question-bank/questions/:questionId":   "lms.question.update",
	"GET /api/lms/quiz-bank":                               "lms.quiz.read",
	"GET /api/lms/subjects":                                "lms.question.read",
	"GET /api/lms/subjects/:subjectId/levels":              "lms.question.read",
	"GET /api/lms/levels/:levelId/topics":                  "lms.question.read",
	"GET /api/lms/questions":                               "lms.question.read",
	"POST /api/lms/questions":                              "lms.question.create",
	"POST /api/lms/questions/random-pick":                  "lms.question.read",
	"PATCH /api/lms/questions/:questionId":                 "lms.question.update",
	"DELETE /api/lms/questions/:questionId":                "lms.question.delete",
	"GET /api/lms/quizzes":                                 "lms.quiz.read",
	"POST /api/lms/quizzes":                                "lms.quiz.create",
	"POST /api/lms/quizzes/from-questions":                 "lms.quiz.create",
	"POST /api/lms/quizzes/random":                         "lms.quiz.create",
	"GET /api/lms/quizzes/:quizId/students":                "lms.report.read",
	"GET /api/lms/quizzes/:quizId":                         "lms.quiz.read",
	"PATCH /api/lms/quizzes/:quizId":                       "lms.quiz.update",
	"PUT /api/lms/quizzes/:quizId":                         "lms.quiz.update",
	"POST /api/lms/quizzes/:quizId/publish":                "lms.quiz.update",
	"GET /api/lms/quizzes/:quizId/package":                 "lms.quiz.read",
	"GET /api/lms/assignments/active":                      "lms.assignment.read",
	"POST /api/lms/assignments/deliveries":                 "lms.assignment.create",
	"POST /api/lms/assignments":                            "lms.assignment.create",
	"GET /api/lms/assignments/:assignmentId/progress":      "lms.assignment.read",
	"POST /api/lms/attempts":                               "lms.assignment.submit",
	"PATCH /api/lms/attempts/:attemptId/draft":             "lms.assignment.submit",
	"PUT /api/lms/attempts/:attemptId/answers/:questionId": "lms.assignment.submit",
	"POST /api/lms/attempts/:attemptId/answers":            "lms.assignment.submit",
	"POST /api/lms/attempts/:attemptId/submit":             "lms.assignment.submit",
	"POST /api/lms/attempts/:attemptId/sync":               "lms.assignment.submit",
	"POST /api/lms/discussions/:threadId/replies":          "lms.class.update",
	"POST /api/lms/discussions/attachments":                "lms.class.update",
	"GET /api/lms/moderation/profanity-words":              "lms.class.read",
	"POST /api/lms/moderation/check":                       "lms.class.read",
	"GET /api/lms/announcements":                           "lms.class.read",
	"POST /api/lms/announcements":                          "lms.class.update",
	"GET /api/lms/reports/classroom":                       "lms.report.read",
	"GET /api/lms/reports/students/:studentId":             "lms.report.read",
	"GET /api/lms/reports/assignments/:assignmentId":       "lms.report.read",
	"GET /api/lms/reports/export":                          "lms.report.read",
	"GET /api/lms/internal-documents":                      "lms.unit.read",
	"POST /api/lms/internal-documents":                     "lms.unit.create",
}

func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("lms: module stopped")
	}
	return nil
}
