package dto

import "time"

const (
	TargetTypePost    = "post"
	TargetTypeComment = "comment"
	TargetTypeTopic   = "topic"
	TargetTypeUser    = "user"

	ReactionLike  = "like"
	ReactionLove  = "love"
	ReactionCare  = "care"
	ReactionHaha  = "haha"
	ReactionWow   = "wow"
	ReactionSad   = "sad"
	ReactionAngry = "angry"
)

type TopicDTO struct {
	ID            string     `json:"id"`
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	GroupName     string     `json:"groupName"`
	Icon          string     `json:"icon,omitempty"`
	Color         string     `json:"color,omitempty"`
	SortOrder     int        `json:"sortOrder"`
	IsFeatured    bool       `json:"isFeatured"`
	ThreadCount   int64      `json:"threadCount"`
	PostCount     int64      `json:"postCount"`
	FollowerCount int64      `json:"followerCount"`
	LastPostID    string     `json:"lastPostId,omitempty"`
	LastPostAt    *time.Time `json:"lastPostAt,omitempty"`
	IsFollowing   bool       `json:"isFollowing"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type AuthorDTO struct {
	ID        string `json:"id"`
	FullName  string `json:"fullName"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	Role      string `json:"role,omitempty"`
	Verified  bool   `json:"verified"`
}

type MediaDTO struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	StorageKey   string `json:"storageKey,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	OriginalName string `json:"originalName,omitempty"`
	MimeType     string `json:"mimeType,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	DurationSec  int    `json:"durationSec,omitempty"`
	SortOrder    int    `json:"sortOrder"`
}

type UploadMediaResponse struct {
	Media MediaDTO `json:"media"`
}

type ReactionSummaryDTO struct {
	Like  int64 `json:"like"`
	Love  int64 `json:"love"`
	Care  int64 `json:"care"`
	Haha  int64 `json:"haha"`
	Wow   int64 `json:"wow"`
	Sad   int64 `json:"sad"`
	Angry int64 `json:"angry"`
	Total int64 `json:"total"`
}

type PostDTO struct {
	ID             string             `json:"id"`
	TopicID        string             `json:"topicId"`
	TopicSlug      string             `json:"topicSlug,omitempty"`
	TopicName      string             `json:"topicName,omitempty"`
	Author         AuthorDTO          `json:"author"`
	Title          string             `json:"title,omitempty"`
	Content        string             `json:"content"`
	PostType       string             `json:"postType"`
	Status         string             `json:"status"`
	Visibility     string             `json:"visibility"`
	Tags           []string           `json:"tags"`
	IsPinned       bool               `json:"isPinned"`
	IsLocked       bool               `json:"isLocked"`
	ViewCount      int64              `json:"viewCount"`
	CommentCount   int64              `json:"commentCount"`
	ReactionCount  int64              `json:"reactionCount"`
	ShareCount     int64              `json:"shareCount"`
	ViewerReaction string             `json:"viewerReaction,omitempty"`
	Reactions      ReactionSummaryDTO `json:"reactions"`
	Media          []MediaDTO         `json:"media"`
	LastActivityAt time.Time          `json:"lastActivityAt"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

type CommentDTO struct {
	ID             string             `json:"id"`
	PostID         string             `json:"postId"`
	ParentID       string             `json:"parentId,omitempty"`
	RootID         string             `json:"rootId,omitempty"`
	Author         AuthorDTO          `json:"author"`
	Content        string             `json:"content"`
	Depth          int                `json:"depth"`
	ReplyCount     int64              `json:"replyCount"`
	ReactionCount  int64              `json:"reactionCount"`
	ViewerReaction string             `json:"viewerReaction,omitempty"`
	Reactions      ReactionSummaryDTO `json:"reactions"`
	Replies        []CommentDTO       `json:"replies,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

type ListPostsQuery struct {
	TopicID string
	Topic   string
	Search  string
	Status  string
	Sort    string
	Page    int
	Limit   int
}

type CreateTopicRequest struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	GroupName   string `json:"groupName"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
}

type CreatePostRequest struct {
	TopicID    string     `json:"topicId"`
	TopicSlug  string     `json:"topicSlug"`
	Title      string     `json:"title"`
	Content    string     `json:"content" binding:"required"`
	PostType   string     `json:"postType"`
	Status     string     `json:"status"`
	Visibility string     `json:"visibility"`
	Tags       []string   `json:"tags"`
	Media      []MediaDTO `json:"media"`
}

type CreateCommentRequest struct {
	Content  string `json:"content" binding:"required"`
	ParentID string `json:"parentId"`
}

type SetReactionRequest struct {
	TargetType string `json:"targetType" binding:"required"`
	TargetID   string `json:"targetId" binding:"required"`
	Reaction   string `json:"reaction"`
}

type FollowRequest struct {
	TargetType string `json:"targetType" binding:"required"`
	TargetID   string `json:"targetId" binding:"required"`
}
