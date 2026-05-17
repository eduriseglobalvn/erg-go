package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var viProfanityWords = []string{"địt", "đụ", "lồn", "cặc", "buồi", "chó chết"}

func (s *Service) ListDiscussions(ctx context.Context, tenantID, classID, cursor string, limit int64) (DiscussionListResponseDTO, error) {
	items, total, next, err := s.repo.ListDiscussionThreads(ctx, tenantID, classID, cursor, limit)
	return DiscussionListResponseDTO{Items: discussionThreadsToDTO(items), Total: total, NextCursor: next}, err
}

func (s *Service) CreateDiscussion(ctx context.Context, tenantID string, actor Actor, classID string, req CreateDiscussionRequestDTO) (DiscussionThreadResponseDTO, error) {
	class, err := s.repo.GetClass(ctx, tenantID, classID)
	if err != nil {
		return DiscussionThreadResponseDTO{}, err
	}
	if class == nil {
		return DiscussionThreadResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && !s.canAccessClass(ctx, tenantID, actor, *class) {
		return DiscussionThreadResponseDTO{}, errScopeForbidden
	}
	classOID, _ := objectID(classID)
	thread := &DiscussionThread{TenantID: tenantID, ClassID: classOID, Title: req.Title, Content: sanitizeText(req.Content), AssignmentID: req.AssignmentID, AttachmentIDs: req.AttachmentIDs, AuthorID: actor.UserID}
	if err := s.repo.CreateDiscussionThread(ctx, thread); err != nil {
		return DiscussionThreadResponseDTO{}, err
	}
	return discussionThreadToDTO(*thread), nil
}

func (s *Service) CreateDiscussionReply(ctx context.Context, tenantID string, actor Actor, threadID string, req CreateDiscussionReplyRequestDTO) (DiscussionReplyResponseDTO, error) {
	thread, err := s.repo.GetDiscussionThread(ctx, tenantID, threadID)
	if err != nil {
		return DiscussionReplyResponseDTO{}, err
	}
	if thread == nil {
		return DiscussionReplyResponseDTO{}, errNotFound
	}
	class, err := s.repo.GetClass(ctx, tenantID, thread.ClassID.Hex())
	if err != nil {
		return DiscussionReplyResponseDTO{}, err
	}
	if class == nil || (!actor.canAccessGlobal() && !s.canAccessClass(ctx, tenantID, actor, *class)) {
		return DiscussionReplyResponseDTO{}, errScopeForbidden
	}
	reply := &DiscussionReply{TenantID: tenantID, ThreadID: thread.ID, ClassID: thread.ClassID, Content: sanitizeText(req.Content), AttachmentIDs: req.AttachmentIDs, AuthorID: actor.UserID}
	if err := s.repo.CreateDiscussionReply(ctx, reply); err != nil {
		return DiscussionReplyResponseDTO{}, err
	}
	return discussionReplyToDTO(*reply), nil
}

func (s *Service) CreateDiscussionAttachment(ctx context.Context, tenantID string, actor Actor, header *multipart.FileHeader) (AttachmentResponseDTO, error) {
	if header == nil {
		return AttachmentResponseDTO{}, fmt.Errorf("file required")
	}
	if !strings.HasPrefix(header.Header.Get("Content-Type"), "image/") {
		return AttachmentResponseDTO{}, fmt.Errorf("only image attachments are allowed")
	}
	if header.Size > 10<<20 {
		return AttachmentResponseDTO{}, fmt.Errorf("image too large")
	}
	item := &DiscussionAttachment{TenantID: tenantID, Mime: header.Header.Get("Content-Type"), Size: header.Size, OwnerID: actor.UserID}
	if err := s.repo.CreateDiscussionAttachment(ctx, item); err != nil {
		return AttachmentResponseDTO{}, err
	}
	item.URL = fmt.Sprintf("/api/lms/discussions/attachments/%s", item.ID.Hex())
	return AttachmentResponseDTO{ID: item.ID.Hex(), URL: item.URL, Mime: item.Mime, Size: item.Size}, nil
}

func (s *Service) ProfanityWords(lang string) ProfanityWordListResponseDTO {
	return ProfanityWordListResponseDTO{Words: viProfanityWords, Version: "vi-1"}
}

func (s *Service) ModerationCheck(text string) ModerationCheckResponseDTO {
	matches := matchedProfanity(text)
	return ModerationCheckResponseDTO{SanitizedText: sanitizeText(text), HasProfanity: len(matches) > 0, MatchedWords: matches}
}

func (s *Service) ListAnnouncements(ctx context.Context, tenantID string, targetType, classID, studentID, cursor string, limit int64) (AnnouncementListResponseDTO, error) {
	filter := bson.M{}
	if targetType != "" {
		filter["target_type"] = targetType
	}
	if classID != "" {
		oid, err := objectID(classID)
		if err != nil {
			return AnnouncementListResponseDTO{}, err
		}
		filter["class_ids"] = oid
	}
	if studentID != "" {
		oid, err := objectID(studentID)
		if err != nil {
			return AnnouncementListResponseDTO{}, err
		}
		filter["student_ids"] = oid
	}
	items, total, next, err := s.repo.ListAnnouncements(ctx, tenantID, filter, cursor, limit)
	return AnnouncementListResponseDTO{Items: announcementsToDTO(items), Total: total, NextCursor: next}, err
}

func (s *Service) CreateAnnouncement(ctx context.Context, tenantID string, actor Actor, req CreateAnnouncementRequestDTO) (AnnouncementResponseDTO, error) {
	item := &Announcement{TenantID: tenantID, TargetType: req.TargetType, ClassIDs: objectIDsOrNil(req.ClassIDs), StudentIDs: objectIDsOrNil(req.StudentIDs), Title: req.Title, Content: sanitizeText(req.Content), Pinned: req.Pinned, AuthorID: actor.UserID}
	if err := s.repo.CreateAnnouncement(ctx, item); err != nil {
		return AnnouncementResponseDTO{}, err
	}
	return announcementToDTO(*item), nil
}

func sanitizeText(text string) string {
	out := text
	for _, word := range viProfanityWords {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
		out = re.ReplaceAllString(out, "***")
	}
	return out
}

func matchedProfanity(text string) []string {
	matches := []string{}
	for _, word := range viProfanityWords {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
		if re.MatchString(text) {
			matches = append(matches, word)
		}
	}
	return matches
}

func discussionThreadsToDTO(items []DiscussionThread) []DiscussionThreadResponseDTO {
	out := make([]DiscussionThreadResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, discussionThreadToDTO(item))
	}
	return out
}

func discussionThreadToDTO(t DiscussionThread) DiscussionThreadResponseDTO {
	return DiscussionThreadResponseDTO{ID: t.ID.Hex(), ClassID: t.ClassID.Hex(), Title: t.Title, Content: t.Content, AssignmentID: t.AssignmentID, AttachmentIDs: t.AttachmentIDs, AuthorID: t.AuthorID, ReplyCount: t.ReplyCount, LatestActivityAt: t.LatestActivityAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func discussionReplyToDTO(r DiscussionReply) DiscussionReplyResponseDTO {
	return DiscussionReplyResponseDTO{ID: r.ID.Hex(), ThreadID: r.ThreadID.Hex(), ClassID: r.ClassID.Hex(), Content: r.Content, AttachmentIDs: r.AttachmentIDs, AuthorID: r.AuthorID, CreatedAt: r.CreatedAt}
}

func announcementsToDTO(items []Announcement) []AnnouncementResponseDTO {
	out := make([]AnnouncementResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, announcementToDTO(item))
	}
	return out
}

func announcementToDTO(a Announcement) AnnouncementResponseDTO {
	return AnnouncementResponseDTO{ID: a.ID.Hex(), TargetType: a.TargetType, ClassIDs: objectIDsToHex(a.ClassIDs), StudentIDs: objectIDsToHex(a.StudentIDs), Title: a.Title, Content: a.Content, Pinned: a.Pinned, AuthorID: a.AuthorID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}
