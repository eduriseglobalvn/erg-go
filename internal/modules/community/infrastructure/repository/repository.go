package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

var ErrStoreUnavailable = errors.New("community: postgres store unavailable")
var ErrSchemaUnavailable = errors.New("community: postgres schema unavailable")

type Repository struct {
	db  *gorm.DB
	log *logger.Logger
}

func NewRepository(client *database.GORMPostgresClient, log *logger.Logger) *Repository {
	var db *gorm.DB
	if client != nil {
		db = client.DB()
	}
	return &Repository{db: db, log: log}
}

func (r *Repository) ensureDB() error {
	if r.db == nil {
		return ErrStoreUnavailable
	}
	return nil
}

func (r *Repository) SeedDefaultTopics(ctx context.Context, tenantID string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	defaults := []postgrescore.CommunityTopic{
		newTopic(tenantID, "bang-tin-giao-vien", "Bảng tin giáo viên", "Trao đổi chung, hỏi nhanh và thông báo cộng đồng.", "Đại sảnh giáo viên", "users", "#172554", 10, true),
		newTopic(tenantID, "review-bai-giang", "Review bài giảng", "Đăng lesson flow, slide, worksheet để mentor và đồng nghiệp góp ý.", "Chuyên môn & học liệu", "sparkles", "#0f766e", 20, true),
		newTopic(tenantID, "kho-chia-se", "Kho chia sẻ", "Rubric, checklist, template, hoạt động nhóm và tài nguyên tái sử dụng.", "Chuyên môn & học liệu", "file-text", "#7c3aed", 30, true),
		newTopic(tenantID, "video-tiet-day", "Video tiết dạy", "Recording, demo hoạt động lớp, phân tích góc máy và tương tác học sinh.", "Media & minh chứng", "video", "#be123c", 40, false),
		newTopic(tenantID, "tinh-huong-lop-hoc", "Tình huống lớp học", "Case quản lớp, xử lý hành vi, học sinh lệch trình độ và giữ nhịp tiết học.", "Hỏi đáp & xử lý tình huống", "message-circle", "#ea580c", 50, true),
		newTopic(tenantID, "danh-gia-kiem-tra", "Đánh giá & kiểm tra", "Đề kiểm tra, rubric, nhận xét học sinh và phản hồi phụ huynh.", "Hỏi đáp & xử lý tình huống", "check-circle", "#2563eb", 60, false),
	}
	for _, topic := range defaults {
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "group_name", "icon", "color", "sort_order", "is_featured", "updated_at"}),
		}).Create(&topic).Error; err != nil {
			return fmt.Errorf("community.seedTopics: %w", err)
		}
	}
	return nil
}

func (r *Repository) ListTopics(ctx context.Context, tenantID, viewerID string) ([]TopicDTO, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var rows []postgrescore.CommunityTopic
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("group_name ASC, sort_order ASC, name ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.ListTopics: %w", err)
	}
	followed, err := r.followedTargets(ctx, tenantID, viewerID, TargetTypeTopic)
	if err != nil {
		return nil, err
	}
	out := make([]TopicDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, topicDTO(row, followed[row.ID]))
	}
	return out, nil
}

func (r *Repository) CreateTopic(ctx context.Context, row *postgrescore.CommunityTopic) (*postgrescore.CommunityTopic, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if row.ID == "" {
		row.ID = bson.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	row.CreatedAt = now
	row.UpdatedAt = now
	if row.GroupName == "" {
		row.GroupName = "Chủ đề cộng đồng"
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, fmt.Errorf("community.CreateTopic: %w", err)
	}
	return row, nil
}

func (r *Repository) ListPosts(ctx context.Context, tenantID, viewerID string, q ListPostsQuery) ([]PostDTO, int64, error) {
	if err := r.ensureDB(); err != nil {
		return nil, 0, err
	}
	query := r.db.WithContext(ctx).Model(&postgrescore.CommunityPost{}).
		Where("community_posts.tenant_id = ? AND community_posts.deleted_at IS NULL", tenantID)
	if q.TopicID != "" {
		query = query.Where("community_posts.topic_id = ?", q.TopicID)
	}
	if q.Topic != "" {
		query = query.Joins("JOIN community_topics ON community_topics.id = community_posts.topic_id").
			Where("community_topics.tenant_id = ? AND community_topics.slug = ?", tenantID, q.Topic)
	}
	if q.Status != "" {
		query = query.Where("community_posts.status = ?", q.Status)
	}
	if q.Search != "" {
		needle := "%" + strings.ToLower(q.Search) + "%"
		query = query.Where("LOWER(community_posts.title) LIKE ? OR LOWER(community_posts.content) LIKE ?", needle, needle)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("community.ListPosts.Count: %w", err)
	}
	page, limit := normalizePage(q.Page, q.Limit)
	order := "community_posts.last_activity_at DESC"
	if q.Sort == "newest" {
		order = "community_posts.created_at DESC"
	}
	var rows []postgrescore.CommunityPost
	if err := query.Order("community_posts.is_pinned DESC").Order(order).
		Offset((page - 1) * limit).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("community.ListPosts.Find: %w", err)
	}
	posts, _, err := r.postsToDTO(ctx, tenantID, viewerID, rows, true)
	return posts, total, err
}

func (r *Repository) CreatePost(ctx context.Context, row *postgrescore.CommunityPost, media []postgrescore.CommunityMedia) (*PostDTO, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if row.ID == "" {
		row.ID = bson.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	row.CreatedAt = now
	row.UpdatedAt = now
	row.LastActivityAt = now
	if row.Status == "" {
		row.Status = "open"
	}
	if row.PostType == "" {
		row.PostType = "discussion"
	}
	if row.Visibility == "" {
		row.Visibility = "community"
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		for i := range media {
			media[i].ID = bson.NewObjectID().Hex()
			media[i].PostID = row.ID
			media[i].SortOrder = i
			media[i].CreatedAt = now
		}
		if len(media) > 0 {
			if err := tx.Create(&media).Error; err != nil {
				return err
			}
		}
		return tx.Model(&postgrescore.CommunityTopic{}).
			Where("id = ? AND tenant_id = ?", row.TopicID, row.TenantID).
			Updates(map[string]any{
				"thread_count": gorm.Expr("thread_count + 1"),
				"post_count":   gorm.Expr("post_count + 1"),
				"last_post_id": row.ID,
				"last_post_at": now,
				"updated_at":   now,
			}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("community.CreatePost: %w", err)
	}
	dto, err := r.GetPost(ctx, row.TenantID, row.AuthorID, row.ID)
	return dto, err
}

func (r *Repository) GetPost(ctx context.Context, tenantID, viewerID, postID string) (*PostDTO, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var row postgrescore.CommunityPost
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, postID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("community.GetPost: %w", err)
	}
	posts, _, err := r.postsToDTO(ctx, tenantID, viewerID, []postgrescore.CommunityPost{row}, true)
	if err != nil || len(posts) == 0 {
		return nil, err
	}
	return &posts[0], nil
}

func (r *Repository) CreateComment(ctx context.Context, row *postgrescore.CommunityComment) (*CommentDTO, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if row.ID == "" {
		row.ID = bson.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	row.CreatedAt = now
	row.UpdatedAt = now
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if row.ParentID != "" {
			var parent postgrescore.CommunityComment
			if err := tx.Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", row.TenantID, row.ParentID).First(&parent).Error; err != nil {
				return err
			}
			if parent.Depth >= 2 {
				return fmt.Errorf("community.CreateComment: max comment depth is 3")
			}
			row.Depth = parent.Depth + 1
			if parent.RootID != "" {
				row.RootID = parent.RootID
			} else {
				row.RootID = parent.ID
			}
			if err := tx.Model(&postgrescore.CommunityComment{}).Where("id = ?", parent.ID).
				Updates(map[string]any{"reply_count": gorm.Expr("reply_count + 1"), "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return tx.Model(&postgrescore.CommunityPost{}).Where("tenant_id = ? AND id = ?", row.TenantID, row.PostID).
			Updates(map[string]any{
				"comment_count":    gorm.Expr("comment_count + 1"),
				"last_activity_at": now,
				"updated_at":       now,
			}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("community.CreateComment: %w", err)
	}
	dto, err := r.commentToDTO(ctx, row.TenantID, row.AuthorID, *row)
	return &dto, err
}

func (r *Repository) ListComments(ctx context.Context, tenantID, viewerID, postID string, limit int) ([]CommentDTO, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []postgrescore.CommunityComment
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND post_id = ? AND deleted_at IS NULL", tenantID, postID).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.ListComments: %w", err)
	}
	return r.nestComments(ctx, tenantID, viewerID, rows)
}

func (r *Repository) SetReaction(ctx context.Context, tenantID, userID, targetType, targetID, reaction string) (ReactionSummaryDTO, error) {
	if err := r.ensureDB(); err != nil {
		return ReactionSummaryDTO{}, err
	}
	if !validTargetType(targetType) {
		return ReactionSummaryDTO{}, fmt.Errorf("community.SetReaction: invalid target type")
	}
	if strings.TrimSpace(targetID) == "" {
		return ReactionSummaryDTO{}, fmt.Errorf("community.SetReaction: target id is required")
	}
	if reaction != "" && !validReaction(reaction) {
		return ReactionSummaryDTO{}, fmt.Errorf("community.SetReaction: invalid reaction")
	}
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.ensureReactionTargetTx(tx, tenantID, targetType, targetID); err != nil {
			return err
		}
		var existing postgrescore.CommunityReaction
		err := tx.Where("tenant_id = ? AND target_type = ? AND target_id = ? AND user_id = ?", tenantID, targetType, targetID, userID).First(&existing).Error
		if reaction == "" {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			return r.incrementReactionCounterTx(tx, targetType, targetID, -1, now)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row := postgrescore.CommunityReaction{ID: bson.NewObjectID().Hex(), TenantID: tenantID, TargetType: targetType, TargetID: targetID, UserID: userID, Reaction: reaction, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			return r.incrementReactionCounterTx(tx, targetType, targetID, 1, now)
		}
		if err != nil {
			return err
		}
		return tx.Model(&postgrescore.CommunityReaction{}).Where("id = ?", existing.ID).Updates(map[string]any{"reaction": reaction, "updated_at": now}).Error
	})
	if err != nil {
		return ReactionSummaryDTO{}, fmt.Errorf("community.SetReaction: %w", err)
	}
	return r.reactionSummary(ctx, tenantID, targetType, []string{targetID})
}

func (r *Repository) SetFollow(ctx context.Context, tenantID, userID, targetType, targetID string, enabled bool) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if targetType != TargetTypeTopic && targetType != TargetTypeUser {
		return fmt.Errorf("community.SetFollow: invalid follow target")
	}
	if strings.TrimSpace(targetID) == "" {
		return fmt.Errorf("community.SetFollow: target id is required")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.ensureFollowTargetTx(tx, tenantID, targetType, targetID); err != nil {
			return err
		}
		if enabled {
			row := postgrescore.CommunityFollow{ID: bson.NewObjectID().Hex(), TenantID: tenantID, TargetType: targetType, TargetID: targetID, UserID: userID, CreatedAt: now}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 && targetType == TargetTypeTopic {
				return tx.Model(&postgrescore.CommunityTopic{}).Where("id = ?", targetID).Update("follower_count", gorm.Expr("follower_count + 1")).Error
			}
			return nil
		}
		res := tx.Where("tenant_id = ? AND target_type = ? AND target_id = ? AND user_id = ?", tenantID, targetType, targetID, userID).Delete(&postgrescore.CommunityFollow{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 && targetType == TargetTypeTopic {
			return tx.Model(&postgrescore.CommunityTopic{}).Where("id = ?", targetID).Update("follower_count", gorm.Expr("GREATEST(follower_count - 1, 0)")).Error
		}
		return nil
	})
}

func (r *Repository) postsToDTO(ctx context.Context, tenantID, viewerID string, rows []postgrescore.CommunityPost, includeMedia bool) ([]PostDTO, int64, error) {
	if len(rows) == 0 {
		return []PostDTO{}, 0, nil
	}
	ids := make([]string, 0, len(rows))
	userIDs := make([]string, 0, len(rows))
	topicIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		userIDs = append(userIDs, row.AuthorID)
		topicIDs = append(topicIDs, row.TopicID)
	}
	authors, err := r.authors(ctx, userIDs)
	if err != nil {
		return nil, 0, err
	}
	topics, err := r.topicMap(ctx, tenantID, topicIDs)
	if err != nil {
		return nil, 0, err
	}
	mediaByPost := map[string][]MediaDTO{}
	if includeMedia {
		mediaByPost, err = r.mediaForPosts(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
	}
	reactions, err := r.reactionSummaryMap(ctx, tenantID, TargetTypePost, ids)
	if err != nil {
		return nil, 0, err
	}
	viewerReactions, err := r.viewerReactions(ctx, tenantID, viewerID, TargetTypePost, ids)
	if err != nil {
		return nil, 0, err
	}
	out := make([]PostDTO, 0, len(rows))
	for _, row := range rows {
		topic := topics[row.TopicID]
		out = append(out, PostDTO{
			ID: row.ID, TopicID: row.TopicID, TopicSlug: topic.Slug, TopicName: topic.Name,
			Author: authors[row.AuthorID], Title: row.Title, Content: row.Content,
			PostType: row.PostType, Status: row.Status, Visibility: row.Visibility, Tags: unmarshalStrings(row.TagsJSON),
			IsPinned: row.IsPinned, IsLocked: row.IsLocked, ViewCount: row.ViewCount, CommentCount: row.CommentCount,
			ReactionCount: row.ReactionCount, ShareCount: row.ShareCount, ViewerReaction: viewerReactions[row.ID],
			Reactions: reactions[row.ID], Media: mediaByPost[row.ID], LastActivityAt: row.LastActivityAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return out, int64(len(out)), nil
}

func (r *Repository) authors(ctx context.Context, userIDs []string) (map[string]AuthorDTO, error) {
	unique := uniqueStrings(userIDs)
	result := map[string]AuthorDTO{}
	for _, id := range unique {
		result[id] = AuthorDTO{ID: id, FullName: "Thành viên ERG"}
	}
	if len(unique) == 0 {
		return result, nil
	}
	var users []postgrescore.AuthUser
	if err := r.db.WithContext(ctx).Where("id IN ?", unique).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("community.authors: %w", err)
	}
	for _, user := range users {
		name := user.FullName
		if name == "" {
			name = user.Email
		}
		result[user.ID] = AuthorDTO{ID: user.ID, FullName: name, AvatarURL: user.AvatarURL, Role: user.JobTitle, Verified: user.GoogleEmailVerified || user.Provider == "google"}
	}
	return result, nil
}

func (r *Repository) topicMap(ctx context.Context, tenantID string, topicIDs []string) (map[string]postgrescore.CommunityTopic, error) {
	ids := uniqueStrings(topicIDs)
	result := map[string]postgrescore.CommunityTopic{}
	if len(ids) == 0 {
		return result, nil
	}
	var rows []postgrescore.CommunityTopic
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.topicMap: %w", err)
	}
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func (r *Repository) mediaForPosts(ctx context.Context, postIDs []string) (map[string][]MediaDTO, error) {
	result := map[string][]MediaDTO{}
	var rows []postgrescore.CommunityMedia
	if err := r.db.WithContext(ctx).Where("post_id IN ?", postIDs).Order("post_id ASC, sort_order ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.mediaForPosts: %w", err)
	}
	for _, row := range rows {
		result[row.PostID] = append(result[row.PostID], MediaDTO{
			ID: row.ID, Type: row.MediaType, URL: row.URL, StorageKey: row.StorageKey, ThumbnailURL: row.ThumbnailURL,
			OriginalName: row.OriginalName, MimeType: row.MimeType, SizeBytes: row.SizeBytes, Width: row.Width, Height: row.Height,
			DurationSec: row.DurationSec, SortOrder: row.SortOrder,
		})
	}
	return result, nil
}

func (r *Repository) nestComments(ctx context.Context, tenantID, viewerID string, rows []postgrescore.CommunityComment) ([]CommentDTO, error) {
	flat := make(map[string]CommentDTO, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		dto, err := r.commentToDTO(ctx, tenantID, viewerID, row)
		if err != nil {
			return nil, err
		}
		flat[row.ID] = dto
		order = append(order, row.ID)
	}
	for _, id := range order {
		dto := flat[id]
		if dto.ParentID == "" {
			continue
		}
		parent := flat[dto.ParentID]
		parent.Replies = append(parent.Replies, dto)
		flat[dto.ParentID] = parent
	}
	out := []CommentDTO{}
	for _, id := range order {
		dto := flat[id]
		if dto.ParentID == "" {
			out = append(out, dto)
		}
	}
	return out, nil
}

func (r *Repository) commentToDTO(ctx context.Context, tenantID, viewerID string, row postgrescore.CommunityComment) (CommentDTO, error) {
	authors, err := r.authors(ctx, []string{row.AuthorID})
	if err != nil {
		return CommentDTO{}, err
	}
	reactions, err := r.reactionSummary(ctx, tenantID, TargetTypeComment, []string{row.ID})
	if err != nil {
		return CommentDTO{}, err
	}
	viewer, err := r.viewerReactions(ctx, tenantID, viewerID, TargetTypeComment, []string{row.ID})
	if err != nil {
		return CommentDTO{}, err
	}
	return CommentDTO{
		ID: row.ID, PostID: row.PostID, ParentID: row.ParentID, RootID: row.RootID, Author: authors[row.AuthorID],
		Content: row.Content, Depth: row.Depth, ReplyCount: row.ReplyCount, ReactionCount: row.ReactionCount,
		ViewerReaction: viewer[row.ID], Reactions: reactions, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *Repository) reactionSummary(ctx context.Context, tenantID, targetType string, targetIDs []string) (ReactionSummaryDTO, error) {
	m, err := r.reactionSummaryMap(ctx, tenantID, targetType, targetIDs)
	if err != nil {
		return ReactionSummaryDTO{}, err
	}
	if len(targetIDs) == 0 {
		return ReactionSummaryDTO{}, nil
	}
	return m[targetIDs[0]], nil
}

func (r *Repository) reactionSummaryMap(ctx context.Context, tenantID, targetType string, targetIDs []string) (map[string]ReactionSummaryDTO, error) {
	result := map[string]ReactionSummaryDTO{}
	for _, id := range targetIDs {
		result[id] = ReactionSummaryDTO{}
	}
	type row struct {
		TargetID, Reaction string
		Count              int64
	}
	var rows []row
	if len(targetIDs) == 0 {
		return result, nil
	}
	if err := r.db.WithContext(ctx).Table("community_reactions").
		Select("target_id, reaction, count(*) as count").
		Where("tenant_id = ? AND target_type = ? AND target_id IN ?", tenantID, targetType, targetIDs).
		Group("target_id, reaction").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.reactionSummary: %w", err)
	}
	for _, row := range rows {
		s := result[row.TargetID]
		switch row.Reaction {
		case ReactionLike:
			s.Like = row.Count
		case ReactionLove:
			s.Love = row.Count
		case ReactionCare:
			s.Care = row.Count
		case ReactionHaha:
			s.Haha = row.Count
		case ReactionWow:
			s.Wow = row.Count
		case ReactionSad:
			s.Sad = row.Count
		case ReactionAngry:
			s.Angry = row.Count
		}
		s.Total += row.Count
		result[row.TargetID] = s
	}
	return result, nil
}

func (r *Repository) viewerReactions(ctx context.Context, tenantID, viewerID, targetType string, targetIDs []string) (map[string]string, error) {
	result := map[string]string{}
	if viewerID == "" || len(targetIDs) == 0 {
		return result, nil
	}
	var rows []postgrescore.CommunityReaction
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND target_type = ? AND target_id IN ?", tenantID, viewerID, targetType, targetIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.viewerReactions: %w", err)
	}
	for _, row := range rows {
		result[row.TargetID] = row.Reaction
	}
	return result, nil
}

func (r *Repository) followedTargets(ctx context.Context, tenantID, viewerID, targetType string) (map[string]bool, error) {
	result := map[string]bool{}
	if viewerID == "" {
		return result, nil
	}
	var rows []postgrescore.CommunityFollow
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND target_type = ?", tenantID, viewerID, targetType).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("community.followedTargets: %w", err)
	}
	for _, row := range rows {
		result[row.TargetID] = true
	}
	return result, nil
}

func (r *Repository) incrementReactionCounterTx(tx *gorm.DB, targetType, targetID string, delta int, now time.Time) error {
	expr := gorm.Expr("GREATEST(reaction_count + ?, 0)", delta)
	switch targetType {
	case TargetTypePost:
		return tx.Model(&postgrescore.CommunityPost{}).Where("id = ?", targetID).Updates(map[string]any{"reaction_count": expr, "updated_at": now}).Error
	case TargetTypeComment:
		return tx.Model(&postgrescore.CommunityComment{}).Where("id = ?", targetID).Updates(map[string]any{"reaction_count": expr, "updated_at": now}).Error
	default:
		return nil
	}
}

func (r *Repository) ensureReactionTargetTx(tx *gorm.DB, tenantID, targetType, targetID string) error {
	switch targetType {
	case TargetTypePost:
		return ensureRowExists(tx.Model(&postgrescore.CommunityPost{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, targetID),
			"community.SetReaction: target post not found")
	case TargetTypeComment:
		return ensureRowExists(tx.Model(&postgrescore.CommunityComment{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, targetID),
			"community.SetReaction: target comment not found")
	default:
		return fmt.Errorf("community.SetReaction: invalid target type")
	}
}

func (r *Repository) ensureFollowTargetTx(tx *gorm.DB, tenantID, targetType, targetID string) error {
	switch targetType {
	case TargetTypeTopic:
		return ensureRowExists(tx.Model(&postgrescore.CommunityTopic{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, targetID),
			"community.SetFollow: target topic not found")
	case TargetTypeUser:
		return ensureRowExists(tx.Model(&postgrescore.AuthUser{}).
			Where("id = ? AND deleted_at IS NULL", targetID),
			"community.SetFollow: target user not found")
	default:
		return fmt.Errorf("community.SetFollow: invalid follow target")
	}
}

func ensureRowExists(query *gorm.DB, message string) error {
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New(message)
	}
	return nil
}

func newTopic(tenantID, slug, name, desc, group, icon, color string, order int, featured bool) postgrescore.CommunityTopic {
	now := time.Now().UTC()
	return postgrescore.CommunityTopic{ID: bson.NewObjectID().Hex(), TenantID: tenantID, Slug: slug, Name: name, Description: desc, GroupName: group, Icon: icon, Color: color, SortOrder: order, IsFeatured: featured, CreatedAt: now, UpdatedAt: now}
}

func fallbackTopicDTOs(tenantID string) []TopicDTO {
	topics := []postgrescore.CommunityTopic{
		newTopic(tenantID, "bang-tin-giao-vien", "Bảng tin giáo viên", "Trao đổi chung, hỏi nhanh và thông báo cộng đồng.", "Đại sảnh giáo viên", "users", "#172554", 10, true),
		newTopic(tenantID, "review-bai-giang", "Review bài giảng", "Đăng lesson flow, slide, worksheet để mentor và đồng nghiệp góp ý.", "Chuyên môn & học liệu", "sparkles", "#0f766e", 20, true),
		newTopic(tenantID, "kho-chia-se", "Kho chia sẻ", "Rubric, checklist, template, hoạt động nhóm và tài nguyên tái sử dụng.", "Chuyên môn & học liệu", "file-text", "#7c3aed", 30, true),
		newTopic(tenantID, "video-tiet-day", "Video tiết dạy", "Recording, demo hoạt động lớp, phân tích góc máy và tương tác học sinh.", "Media & minh chứng", "video", "#be123c", 40, false),
		newTopic(tenantID, "tinh-huong-lop-hoc", "Tình huống lớp học", "Case quản lớp, xử lý hành vi, học sinh lệch trình độ và giữ nhịp tiết học.", "Hỏi đáp & xử lý tình huống", "message-circle", "#ea580c", 50, true),
		newTopic(tenantID, "danh-gia-kiem-tra", "Đánh giá & kiểm tra", "Đề kiểm tra, rubric, nhận xét học sinh và phản hồi phụ huynh.", "Hỏi đáp & xử lý tình huống", "check-circle", "#2563eb", 60, false),
	}
	out := make([]TopicDTO, 0, len(topics))
	for _, topic := range topics {
		out = append(out, topicDTO(topic, false))
	}
	return out
}

func IsStoreUnavailable(err error) bool {
	return errors.Is(err, ErrStoreUnavailable) || errors.Is(err, ErrSchemaUnavailable) || isSchemaUnavailableError(err)
}

func isSchemaUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlstate 42p01") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such table")
}

func topicDTO(row postgrescore.CommunityTopic, followed bool) TopicDTO {
	return TopicDTO{ID: row.ID, Slug: row.Slug, Name: row.Name, Description: row.Description, GroupName: row.GroupName, Icon: row.Icon, Color: row.Color, SortOrder: row.SortOrder, IsFeatured: row.IsFeatured, ThreadCount: row.ThreadCount, PostCount: row.PostCount, FollowerCount: row.FollowerCount, LastPostID: row.LastPostID, LastPostAt: row.LastPostAt, IsFollowing: followed, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func validReaction(value string) bool {
	switch value {
	case ReactionLike, ReactionLove, ReactionCare, ReactionHaha, ReactionWow, ReactionSad, ReactionAngry:
		return true
	default:
		return false
	}
}

func validTargetType(value string) bool {
	return value == TargetTypePost || value == TargetTypeComment
}

func marshalStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	raw, _ := json.Marshal(values)
	return string(raw)
}

func unmarshalStrings(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
