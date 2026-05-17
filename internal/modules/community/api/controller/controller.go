package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	communitydto "erg.ninja/internal/modules/community/api/dto"
	communityservice "erg.ninja/internal/modules/community/application/service"
)

type Controller struct {
	svc *communityservice.Service
}

func NewController(svc *communityservice.Service) *Controller {
	return &Controller{svc: svc}
}

type CreateCommentRequest = communitydto.CreateCommentRequest
type CreatePostRequest = communitydto.CreatePostRequest
type CreateTopicRequest = communitydto.CreateTopicRequest
type FollowRequest = communitydto.FollowRequest
type ListPostsQuery = communitydto.ListPostsQuery
type SetReactionRequest = communitydto.SetReactionRequest

func (c *Controller) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/topics", c.ListTopics)
	rg.GET("/feed", c.ListPosts)
	rg.GET("/posts", c.ListPosts)
	rg.GET("/posts/:postId/comments", c.ListComments)
}

func (c *Controller) RegisterAuthenticatedRoutes(rg *gin.RouterGroup) {
	rg.POST("/topics", c.CreateTopic)
	rg.POST("/posts", c.CreatePost)
	rg.POST("/media/upload", c.UploadMedia)
	rg.POST("/posts/:postId/comments", c.CreateComment)
	rg.PUT("/reactions", c.SetReaction)
	rg.DELETE("/reactions", c.SetReaction)
	rg.POST("/follows", c.Follow)
	rg.DELETE("/follows", c.Unfollow)
}

func (c *Controller) ListTopics(ctx *gin.Context) {
	topics, err := c.svc.ListTopics(ctx.Request.Context(), middleware.GetUserID(ctx.Request.Context()))
	c.respond(ctx, topics, err, http.StatusOK)
}

func (c *Controller) CreateTopic(ctx *gin.Context) {
	var req CreateTopicRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	topic, err := c.svc.CreateTopic(ctx.Request.Context(), req, middleware.GetUserID(ctx.Request.Context()))
	c.respond(ctx, topic, err, http.StatusCreated)
}

func (c *Controller) ListPosts(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.Query("page"))
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	posts, total, page, limit, err := c.svc.ListPosts(ctx.Request.Context(), middleware.GetUserID(ctx.Request.Context()), ListPostsQuery{
		TopicID: ctx.Query("topicId"),
		Topic:   ctx.Query("topic"),
		Search:  ctx.Query("q"),
		Status:  ctx.Query("status"),
		Sort:    ctx.Query("sort"),
		Page:    page,
		Limit:   limit,
	})
	if err != nil {
		c.respond(ctx, nil, err, http.StatusOK)
		return
	}
	response.PaginatedGin(ctx, posts, total, page, limit)
}

func (c *Controller) CreatePost(ctx *gin.Context) {
	var req CreatePostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	post, err := c.svc.CreatePost(ctx.Request.Context(), req, middleware.GetUserID(ctx.Request.Context()))
	c.respond(ctx, post, err, http.StatusCreated)
}

func (c *Controller) UploadMedia(ctx *gin.Context) {
	const maxCommunityMediaBytes int64 = 250 << 20
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxCommunityMediaBytes+(1<<20))
	if err := ctx.Request.ParseMultipartForm(1 << 20); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(io.LimitReader(file, maxCommunityMediaBytes+1))
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if int64(len(buf)) > maxCommunityMediaBytes {
		response.BadRequestGin(ctx, errors.New("community media upload exceeds 250MB"))
		return
	}
	mime := fileHeader.Header.Get("Content-Type")
	if mime == "" {
		mime = http.DetectContentType(buf)
	}
	media, err := c.svc.UploadMedia(ctx.Request.Context(), buf, fileHeader.Filename, mime)
	c.respond(ctx, media, err, http.StatusCreated)
}

func (c *Controller) ListComments(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	comments, err := c.svc.ListComments(ctx.Request.Context(), ctx.Param("postId"), middleware.GetUserID(ctx.Request.Context()), limit)
	c.respond(ctx, comments, err, http.StatusOK)
}

func (c *Controller) CreateComment(ctx *gin.Context) {
	var req CreateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	comment, err := c.svc.CreateComment(ctx.Request.Context(), ctx.Param("postId"), req, middleware.GetUserID(ctx.Request.Context()))
	c.respond(ctx, comment, err, http.StatusCreated)
}

func (c *Controller) SetReaction(ctx *gin.Context) {
	var req SetReactionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if ctx.Request.Method == http.MethodDelete {
		req.Reaction = ""
	}
	summary, err := c.svc.SetReaction(ctx.Request.Context(), req, middleware.GetUserID(ctx.Request.Context()))
	c.respond(ctx, summary, err, http.StatusOK)
}

func (c *Controller) Follow(ctx *gin.Context) {
	var req FollowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	err := c.svc.SetFollow(ctx.Request.Context(), req, middleware.GetUserID(ctx.Request.Context()), true)
	c.respond(ctx, gin.H{"following": err == nil}, err, http.StatusOK)
}

func (c *Controller) Unfollow(ctx *gin.Context) {
	var req FollowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	err := c.svc.SetFollow(ctx.Request.Context(), req, middleware.GetUserID(ctx.Request.Context()), false)
	c.respond(ctx, gin.H{"following": false}, err, http.StatusOK)
}

func (c *Controller) respond(ctx *gin.Context, data any, err error, successStatus int) {
	if err == nil {
		if successStatus == http.StatusCreated {
			response.CreatedGin(ctx, data)
			return
		}
		response.SuccessGin(ctx, data)
		return
	}
	switch {
	case communityservice.IsStoreUnavailable(err):
		response.ErrorGin(ctx, http.StatusServiceUnavailable, "store_unavailable", err.Error())
	default:
		response.BadRequestGin(ctx, err)
	}
}
