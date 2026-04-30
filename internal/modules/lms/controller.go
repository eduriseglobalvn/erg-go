package lms

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/tenant"
)

type Controller struct {
	svc *Service
}

func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

func (c *Controller) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/scopes/me", c.GetScope)
	rg.PUT("/scopes/current", c.UpdateScope)
	rg.GET("/centers", c.ListCenters)
	rg.POST("/centers", c.CreateCenter)
	rg.PATCH("/centers/:centerId", c.UpdateCenter)
	rg.GET("/classes", c.ListClasses)
	rg.POST("/classes", c.CreateClass)
	rg.PATCH("/classes/:classId", c.UpdateClass)
	rg.POST("/classes/:classId/students/bulk-move", c.BulkMoveStudents)
	rg.GET("/classes/:classId/assignments", c.ClassAssignments)
	rg.GET("/classes/:classId/discussions", c.ListDiscussions)
	rg.POST("/classes/:classId/discussions", c.CreateDiscussion)
	rg.GET("/students", c.ListStudents)
	rg.GET("/students/:studentId", c.GetStudent)
	rg.POST("/students", c.CreateStudent)
	rg.PATCH("/students/:studentId", c.UpdateStudent)
	rg.GET("/students/me/assignments", c.StudentAssignments)
	rg.GET("/students/me/scores", c.StudentScores)
	rg.POST("/imports/google-sheet/tabs", c.GoogleSheetTabs)
	rg.POST("/imports/google-sheet/preview", c.PreviewGoogleSheet)
	rg.POST("/imports/google-sheet/commit", c.CommitGoogleSheetImport)
	rg.GET("/imports/:jobId", c.GetImportJob)
	rg.POST("/imports/:jobId/writeback", c.WritebackImportJob)
	rg.GET("/subjects", c.ListSubjects)
	rg.GET("/subjects/:subjectId/levels", c.ListLevels)
	rg.GET("/levels/:levelId/topics", c.ListTopics)
	rg.GET("/questions", c.ListQuestions)
	rg.POST("/questions", c.CreateQuestion)
	rg.POST("/questions/random-pick", c.RandomPickQuestions)
	rg.PATCH("/questions/:questionId", c.UpdateQuestion)
	rg.DELETE("/questions/:questionId", c.ArchiveQuestion)
	rg.GET("/quizzes", c.ListQuizzes)
	rg.POST("/quizzes", c.CreateQuiz)
	rg.POST("/quizzes/from-questions", c.CreateQuizFromQuestions)
	rg.POST("/quizzes/random", c.CreateRandomQuiz)
	rg.GET("/quizzes/:quizId/students", c.QuizStudentProgress)
	rg.GET("/quizzes/:quizId", c.GetQuizDetail)
	rg.PATCH("/quizzes/:quizId", c.UpdateQuiz)
	rg.PUT("/quizzes/:quizId", c.UpdateQuiz)
	rg.POST("/quizzes/:quizId/publish", c.PublishQuiz)
	rg.GET("/quizzes/:quizId/package", c.QuizPackage)
	rg.POST("/assignments", c.CreateAssignment)
	rg.GET("/assignments/:assignmentId/progress", c.AssignmentProgress)
	rg.POST("/attempts", c.StartAttempt)
	rg.PUT("/attempts/:attemptId/answers/:questionId", c.SaveAnswer)
	rg.POST("/attempts/:attemptId/answers", c.SaveAnswerCompat)
	rg.POST("/attempts/:attemptId/submit", c.SubmitAttempt)
	rg.POST("/attempts/:attemptId/sync", c.SyncAttempt)
	rg.POST("/discussions/:threadId/replies", c.CreateDiscussionReply)
	rg.POST("/discussions/attachments", c.CreateDiscussionAttachment)
	rg.GET("/moderation/profanity-words", c.ProfanityWords)
	rg.POST("/moderation/check", c.ModerationCheck)
	rg.GET("/announcements", c.ListAnnouncements)
	rg.POST("/announcements", c.CreateAnnouncement)
	rg.GET("/reports/classroom", c.ClassroomReport)
	rg.GET("/reports/students/:studentId", c.StudentJourney)
	rg.GET("/reports/assignments/:assignmentId", c.AssignmentReport)
	rg.GET("/reports/export", c.ExportReport)
	rg.GET("/internal-documents", c.ListInternalDocuments)
	rg.POST("/internal-documents", c.CreateInternalDocument)
}

func (c *Controller) GetScope(ctx *gin.Context) {
	result, err := c.svc.Scope(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) UpdateScope(ctx *gin.Context) {
	var req UpdateCurrentScopeRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.UpdateScope(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListCenters(ctx *gin.Context) {
	result, err := c.svc.ListCenters(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), CenterListRequestDTO{
		Keyword: ctx.Query("keyword"),
		Status:  ctx.Query("status"),
		Page:    queryInt(ctx, "page", 1),
		Limit:   queryInt(ctx, "limit", 20),
	})
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateCenter(ctx *gin.Context) {
	var req CreateCenterRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateCenter(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) UpdateCenter(ctx *gin.Context) {
	var req UpdateCenterRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.UpdateCenter(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("centerId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListClasses(ctx *gin.Context) {
	result, err := c.svc.ListClasses(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ClassListRequestDTO{
		CenterID: ctx.Query("centerId"),
		Grade:    ctx.Query("grade"),
		Keyword:  ctx.Query("keyword"),
		Status:   ctx.Query("status"),
		Page:     queryInt(ctx, "page", 1),
		Limit:    queryInt(ctx, "limit", 20),
	})
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateClass(ctx *gin.Context) {
	var req CreateClassRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateClass(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) UpdateClass(ctx *gin.Context) {
	var req UpdateClassRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.UpdateClass(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("classId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListStudents(ctx *gin.Context) {
	result, err := c.svc.ListStudents(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), StudentListRequestDTO{
		CenterID:  ctx.Query("centerId"),
		ClassID:   ctx.Query("classId"),
		Keyword:   ctx.Query("keyword"),
		Status:    ctx.Query("status"),
		Progress:  ctx.Query("progress"),
		SubjectID: ctx.Query("subjectId"),
		Cursor:    ctx.Query("cursor"),
		Limit:     queryInt(ctx, "limit", 20),
	})
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) GetStudent(ctx *gin.Context) {
	result, err := c.svc.GetStudent(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("studentId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateStudent(ctx *gin.Context) {
	var req CreateStudentRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateStudent(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) UpdateStudent(ctx *gin.Context) {
	var req UpdateStudentRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.UpdateStudent(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("studentId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) BulkMoveStudents(ctx *gin.Context) {
	var req BulkMoveStudentsRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.BulkMoveStudents(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("classId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListDiscussions(ctx *gin.Context) {
	result, err := c.svc.ListDiscussions(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("classId"), ctx.Query("cursor"), queryInt(ctx, "limit", 20))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateDiscussion(ctx *gin.Context) {
	var req CreateDiscussionRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateDiscussion(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("classId"), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) CreateDiscussionReply(ctx *gin.Context) {
	var req CreateDiscussionReplyRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateDiscussionReply(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("threadId"), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) CreateDiscussionAttachment(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateDiscussionAttachment(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), file)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) CreateAssignment(ctx *gin.Context) {
	var req CreateAssignmentRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateAssignment(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) ClassAssignments(ctx *gin.Context) {
	result, err := c.svc.ClassAssignments(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("classId"), ctx.Query("status"), ctx.Query("subjectId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) AssignmentProgress(ctx *gin.Context) {
	result, err := c.svc.AssignmentProgress(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("assignmentId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) StudentAssignments(ctx *gin.Context) {
	result, err := c.svc.StudentAssignments(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Query("status"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) StudentScores(ctx *gin.Context) {
	result, err := c.svc.StudentScores(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Query("subjectId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) StartAttempt(ctx *gin.Context) {
	var req StartAttemptRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.StartAttempt(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) SaveAnswer(ctx *gin.Context) {
	var req SaveAnswerRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.SaveAnswer(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("attemptId"), ctx.Param("questionId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) SaveAnswerCompat(ctx *gin.Context) {
	var req struct {
		QuestionID string `json:"questionId" binding:"required"`
		SaveAnswerRequestDTO
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.SaveAnswer(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("attemptId"), req.QuestionID, req.SaveAnswerRequestDTO)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) SubmitAttempt(ctx *gin.Context) {
	var req SubmitAttemptRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.SubmitAttempt(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("attemptId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) SyncAttempt(ctx *gin.Context) {
	var req AttemptSyncRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.SyncAttempt(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("attemptId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ProfanityWords(ctx *gin.Context) {
	response.OKGin(ctx, c.svc.ProfanityWords(ctx.Query("lang")))
}

func (c *Controller) ModerationCheck(ctx *gin.Context) {
	var req ModerationCheckRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	response.OKGin(ctx, c.svc.ModerationCheck(req.Text))
}

func (c *Controller) ListAnnouncements(ctx *gin.Context) {
	result, err := c.svc.ListAnnouncements(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Query("targetType"), ctx.Query("classId"), ctx.Query("studentId"), ctx.Query("cursor"), queryInt(ctx, "limit", 20))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateAnnouncement(ctx *gin.Context) {
	var req CreateAnnouncementRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateAnnouncement(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) ClassroomReport(ctx *gin.Context) {
	result, err := c.svc.ClassroomReport(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Query("centerId"), ctx.Query("classId"), ctx.Query("subjectId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) StudentJourney(ctx *gin.Context) {
	result, err := c.svc.StudentJourney(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("studentId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) AssignmentReport(ctx *gin.Context) {
	result, err := c.svc.AssignmentReport(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("assignmentId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ExportReport(ctx *gin.Context) {
	result, err := c.svc.ExportReport(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Query("type"), ctx.Query("centerId"), ctx.Query("classId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListInternalDocuments(ctx *gin.Context) {
	result, err := c.svc.ListInternalDocuments(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Query("type"), ctx.Query("keyword"), ctx.Query("subjectId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateInternalDocument(ctx *gin.Context) {
	var req CreateInternalDocumentRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateInternalDocument(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) GoogleSheetTabs(ctx *gin.Context) {
	var req GoogleSheetTabsRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.GoogleSheetTabs(ctx.Request.Context(), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) PreviewGoogleSheet(ctx *gin.Context) {
	var req GoogleSheetPreviewRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.PreviewGoogleSheet(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CommitGoogleSheetImport(ctx *gin.Context) {
	var req GoogleSheetCommitRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CommitGoogleSheetImport(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) GetImportJob(ctx *gin.Context) {
	result, err := c.svc.GetImportJob(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("jobId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) WritebackImportJob(ctx *gin.Context) {
	var req SheetWritebackRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.WritebackImportJob(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("jobId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListSubjects(ctx *gin.Context) {
	result, err := c.svc.ListSubjects(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Query("scopeType"), ctx.Query("centerId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListLevels(ctx *gin.Context) {
	result, err := c.svc.ListLevels(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("subjectId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListTopics(ctx *gin.Context) {
	result, err := c.svc.ListTopics(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("levelId"), ctx.Query("includeOther") == "true")
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListQuestions(ctx *gin.Context) {
	result, err := c.svc.ListQuestions(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), QuestionListRequestDTO{
		Scope:     contentScopeFromQuery(ctx),
		SubjectID: ctx.Query("subjectId"),
		LevelID:   ctx.Query("levelId"),
		TopicID:   ctx.Query("topicId"),
		Keyword:   ctx.Query("keyword"),
		Type:      ctx.Query("type"),
		Cursor:    ctx.Query("cursor"),
		Limit:     queryInt(ctx, "limit", 20),
	})
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateQuestion(ctx *gin.Context) {
	var req CreateQuestionRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateQuestion(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) UpdateQuestion(ctx *gin.Context) {
	var req UpdateQuestionRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.UpdateQuestion(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("questionId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ArchiveQuestion(ctx *gin.Context) {
	var req ArchiveQuestionRequestDTO
	_ = ctx.ShouldBindJSON(&req)
	result, err := c.svc.ArchiveQuestion(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), ctx.Param("questionId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) RandomPickQuestions(ctx *gin.Context) {
	var req RandomPickQuestionsRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.RandomPickQuestions(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) ListQuizzes(ctx *gin.Context) {
	result, err := c.svc.ListQuizzes(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), QuizListRequestDTO{
		Scope:     contentScopeFromQuery(ctx),
		SubjectID: ctx.Query("subjectId"),
		LevelID:   ctx.Query("levelId"),
		Kind:      ctx.Query("kind"),
		Keyword:   ctx.Query("keyword"),
		Cursor:    ctx.Query("cursor"),
		Limit:     queryInt(ctx, "limit", 20),
	})
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) CreateQuiz(ctx *gin.Context) {
	var req CreateQuizRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateQuiz(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) CreateQuizFromQuestions(ctx *gin.Context) {
	var req CreateQuizFromQuestionsRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateQuizFromQuestions(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) CreateRandomQuiz(ctx *gin.Context) {
	var req CreateRandomQuizRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.CreateRandomQuiz(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), actorFromContext(ctx), req)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) GetQuizDetail(ctx *gin.Context) {
	result, err := c.svc.GetQuizDetail(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("quizId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) UpdateQuiz(ctx *gin.Context) {
	var req UpdateQuizRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	result, err := c.svc.UpdateQuiz(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("quizId"), req)
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) PublishQuiz(ctx *gin.Context) {
	var req PublishQuizRequestDTO
	_ = ctx.ShouldBindJSON(&req)
	result, err := c.svc.PublishQuiz(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("quizId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) QuizPackage(ctx *gin.Context) {
	result, err := c.svc.QuizPackage(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("quizId"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) QuizStudentProgress(ctx *gin.Context) {
	result, err := c.svc.QuizStudentProgress(ctx.Request.Context(), tenant.FromContext(ctx.Request.Context()), ctx.Param("quizId"), ctx.Query("classId"), ctx.Query("status"))
	c.respond(ctx, result, err, http.StatusOK)
}

func (c *Controller) respond(ctx *gin.Context, data any, err error, successStatus int) {
	if err != nil {
		writeServiceError(func(status int, code, message string) {
			response.ErrorGin(ctx, status, code, message)
		}, err)
		return
	}
	response.WriteGin(ctx, successStatus, data, nil, nil)
}

func actorFromContext(ctx *gin.Context) Actor {
	return Actor{UserID: middleware.GetUserID(ctx.Request.Context()), Roles: middleware.GetRoles(ctx.Request.Context())}
}

func queryInt(ctx *gin.Context, key string, fallback int64) int64 {
	v, err := strconv.ParseInt(ctx.Query(key), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func contentScopeFromQuery(ctx *gin.Context) ContentScopeDTO {
	return ContentScopeDTO{Type: ctx.Query("scope"), CenterID: ctx.Query("centerId")}
}
