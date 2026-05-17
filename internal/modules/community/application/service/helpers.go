package service

import (
	"encoding/json"
	"time"

	"erg.ninja/internal/persistence/postgrescore"
)

func topicDTO(row postgrescore.CommunityTopic, followed bool) TopicDTO {
	return TopicDTO{ID: row.ID, Slug: row.Slug, Name: row.Name, Description: row.Description, GroupName: row.GroupName, Icon: row.Icon, Color: row.Color, SortOrder: row.SortOrder, IsFeatured: row.IsFeatured, ThreadCount: row.ThreadCount, PostCount: row.PostCount, FollowerCount: row.FollowerCount, LastPostID: row.LastPostID, LastPostAt: row.LastPostAt, IsFollowing: followed, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func fallbackTopicDTOs(tenantID string) []TopicDTO {
	now := time.Now().UTC()
	topics := []postgrescore.CommunityTopic{
		{ID: "community-topic-general", TenantID: tenantID, Slug: "bang-tin-giao-vien", Name: "Bảng tin giáo viên", GroupName: "Đại sảnh giáo viên", Icon: "users", Color: "#172554", SortOrder: 10, IsFeatured: true, CreatedAt: now, UpdatedAt: now},
		{ID: "community-topic-review", TenantID: tenantID, Slug: "review-bai-giang", Name: "Review bài giảng", GroupName: "Chuyên môn & học liệu", Icon: "sparkles", Color: "#0f766e", SortOrder: 20, IsFeatured: true, CreatedAt: now, UpdatedAt: now},
		{ID: "community-topic-share", TenantID: tenantID, Slug: "kho-chia-se", Name: "Kho chia sẻ", GroupName: "Chuyên môn & học liệu", Icon: "file-text", Color: "#7c3aed", SortOrder: 30, IsFeatured: true, CreatedAt: now, UpdatedAt: now},
	}
	out := make([]TopicDTO, 0, len(topics))
	for _, topic := range topics {
		out = append(out, topicDTO(topic, false))
	}
	return out
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return page, limit
}

func marshalStrings(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(raw)
}
