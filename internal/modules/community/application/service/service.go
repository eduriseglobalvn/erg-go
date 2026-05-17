package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	communitydto "erg.ninja/internal/modules/community/api/dto"
	communityrepo "erg.ninja/internal/modules/community/infrastructure/repository"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

type Service struct {
	repo     *communityrepo.Repository
	log      *logger.Logger
	r2       *storage.R2Client
	tenantID string
}

func NewService(repo *communityrepo.Repository, log *logger.Logger, r2 *storage.R2Client, tenantID string) *Service {
	if tenantID == "" {
		tenantID = "default"
	}
	return &Service{repo: repo, log: log, r2: r2, tenantID: tenantID}
}

var ErrStoreUnavailable = communityrepo.ErrStoreUnavailable

func IsStoreUnavailable(err error) bool {
	return communityrepo.IsStoreUnavailable(err)
}

type CommentDTO = communitydto.CommentDTO
type CreateCommentRequest = communitydto.CreateCommentRequest
type CreatePostRequest = communitydto.CreatePostRequest
type CreateTopicRequest = communitydto.CreateTopicRequest
type FollowRequest = communitydto.FollowRequest
type ListPostsQuery = communitydto.ListPostsQuery
type MediaDTO = communitydto.MediaDTO
type PostDTO = communitydto.PostDTO
type ReactionSummaryDTO = communitydto.ReactionSummaryDTO
type SetReactionRequest = communitydto.SetReactionRequest
type TopicDTO = communitydto.TopicDTO
type UploadMediaResponse = communitydto.UploadMediaResponse

func (s *Service) Setup(ctx context.Context) error {
	if s.repo == nil {
		return ErrStoreUnavailable
	}
	if err := s.repo.SeedDefaultTopics(ctx, s.tenantID); err != nil {
		return err
	}
	return nil
}

func (s *Service) ListTopics(ctx context.Context, viewerID string) ([]TopicDTO, error) {
	topics, err := s.repo.ListTopics(ctx, s.tenantID, viewerID)
	if err != nil {
		if IsStoreUnavailable(err) {
			return fallbackTopicDTOs(s.tenantID), nil
		}
		return nil, err
	}
	return topics, nil
}

func (s *Service) CreateTopic(ctx context.Context, req CreateTopicRequest, actorID string) (*TopicDTO, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("community.CreateTopic: name is required")
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	topic, err := s.repo.CreateTopic(ctx, &postgrescore.CommunityTopic{
		TenantID: s.tenantID, Slug: slug, Name: name, Description: strings.TrimSpace(req.Description),
		GroupName: strings.TrimSpace(req.GroupName), Icon: strings.TrimSpace(req.Icon), Color: strings.TrimSpace(req.Color),
		CreatedBy: actorID,
	})
	if err != nil {
		return nil, err
	}
	dto := topicDTO(*topic, false)
	return &dto, nil
}

func (s *Service) ListPosts(ctx context.Context, viewerID string, q ListPostsQuery) ([]PostDTO, int64, int, int, error) {
	page, limit := normalizePage(q.Page, q.Limit)
	q.Page, q.Limit = page, limit
	posts, total, err := s.repo.ListPosts(ctx, s.tenantID, viewerID, q)
	if err != nil && IsStoreUnavailable(err) {
		return []PostDTO{}, 0, page, limit, nil
	}
	return posts, total, page, limit, err
}

func (s *Service) CreatePost(ctx context.Context, req CreatePostRequest, actorID string) (*PostDTO, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("community.CreatePost: content is required")
	}
	topicID := strings.TrimSpace(req.TopicID)
	if topicID == "" && strings.TrimSpace(req.TopicSlug) != "" {
		topics, err := s.repo.ListTopics(ctx, s.tenantID, actorID)
		if err != nil {
			return nil, err
		}
		for _, topic := range topics {
			if topic.Slug == req.TopicSlug {
				topicID = topic.ID
				break
			}
		}
	}
	if topicID == "" {
		return nil, fmt.Errorf("community.CreatePost: topicId is required")
	}
	row := &postgrescore.CommunityPost{
		TenantID: s.tenantID, TopicID: topicID, AuthorID: actorID, Title: strings.TrimSpace(req.Title),
		Content: strings.TrimSpace(req.Content), PostType: normalizePostType(req.PostType),
		Status: normalizePostStatus(req.Status), Visibility: normalizeVisibility(req.Visibility), TagsJSON: marshalStrings(req.Tags),
	}
	media := make([]postgrescore.CommunityMedia, 0, len(req.Media))
	for i, item := range req.Media {
		mediaType := strings.TrimSpace(item.Type)
		if mediaType != "image" && mediaType != "video" {
			return nil, fmt.Errorf("community.CreatePost: media type must be image or video")
		}
		if strings.TrimSpace(item.URL) == "" {
			return nil, fmt.Errorf("community.CreatePost: media url is required")
		}
		media = append(media, postgrescore.CommunityMedia{
			MediaType: mediaType, URL: strings.TrimSpace(item.URL), StorageKey: item.StorageKey,
			ThumbnailURL: item.ThumbnailURL, OriginalName: item.OriginalName, MimeType: item.MimeType,
			SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height, DurationSec: item.DurationSec, SortOrder: i,
		})
	}
	return s.repo.CreatePost(ctx, row, media)
}

func (s *Service) ListComments(ctx context.Context, postID, viewerID string, limit int) ([]CommentDTO, error) {
	if postID == "" {
		return nil, fmt.Errorf("community.ListComments: postId is required")
	}
	return s.repo.ListComments(ctx, s.tenantID, viewerID, postID, limit)
}

func (s *Service) CreateComment(ctx context.Context, postID string, req CreateCommentRequest, actorID string) (*CommentDTO, error) {
	if strings.TrimSpace(postID) == "" {
		return nil, fmt.Errorf("community.CreateComment: postId is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("community.CreateComment: content is required")
	}
	return s.repo.CreateComment(ctx, &postgrescore.CommunityComment{
		TenantID: s.tenantID, PostID: postID, ParentID: strings.TrimSpace(req.ParentID), AuthorID: actorID,
		Content: strings.TrimSpace(req.Content),
	})
}

func (s *Service) SetReaction(ctx context.Context, req SetReactionRequest, actorID string) (ReactionSummaryDTO, error) {
	return s.repo.SetReaction(ctx, s.tenantID, actorID, strings.TrimSpace(req.TargetType), strings.TrimSpace(req.TargetID), strings.TrimSpace(req.Reaction))
}

func (s *Service) SetFollow(ctx context.Context, req FollowRequest, actorID string, enabled bool) error {
	return s.repo.SetFollow(ctx, s.tenantID, actorID, strings.TrimSpace(req.TargetType), strings.TrimSpace(req.TargetID), enabled)
}

func (s *Service) UploadMedia(ctx context.Context, body []byte, filename, mime string) (*UploadMediaResponse, error) {
	if s.r2 == nil {
		return nil, fmt.Errorf("community.UploadMedia: storage is not configured")
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	mediaType := "image"
	var url string
	var err error
	if strings.HasPrefix(mime, "video/") {
		mediaType = "video"
		url, err = s.r2.UploadLearningAsset(ctx, body, "community/media", filename, mime)
	} else {
		url, err = s.r2.UploadRaw(ctx, body, "community/media", filename, mime)
	}
	if err != nil {
		return nil, fmt.Errorf("community.UploadMedia: %w", err)
	}
	return &UploadMediaResponse{Media: MediaDTO{
		Type: mediaType, URL: url, OriginalName: filename, MimeType: mime, SizeBytes: int64(len(body)),
	}}, nil
}

func normalizePostType(value string) string {
	switch strings.TrimSpace(value) {
	case "question", "review", "share", "case", "video", "resource":
		return value
	default:
		return "discussion"
	}
}

func normalizePostStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "open", "answered", "mentor-needed", "resource", "closed":
		return value
	default:
		return "open"
	}
}

func normalizeVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "public", "community", "private":
		return value
	default:
		return "community"
	}
}

func slugify(text string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	slug, _, _ := transform.String(t, text)
	slug = strings.ToLower(slug)
	var b strings.Builder
	prevDash := false
	for _, r := range slug {
		ok := unicode.IsLetter(r) || unicode.IsDigit(r)
		if ok {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "chu-de"
	}
	return out + "-" + bson.NewObjectID().Hex()[18:] + fmt.Sprintf("%d", time.Now().UTC().Unix()%1000)
}
