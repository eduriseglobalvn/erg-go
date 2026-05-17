package service

import (
	"context"
	"fmt"
	"strings"
)

const (
	QuestionKindSingleChoice   = "single_choice"
	QuestionKindMultipleChoice = "multiple_choice"
	QuestionKindTrueFalse      = "true_false"
	QuestionKindShortAnswer    = "short_answer"
	QuestionKindEssay          = "essay"
	QuestionKindOrdering       = "ordering"
	QuestionKindMatching       = "matching"
)

var questionKindAliases = map[string]string{
	"single":          QuestionKindSingleChoice,
	"single_choice":   QuestionKindSingleChoice,
	"single-choice":   QuestionKindSingleChoice,
	"choice":          QuestionKindSingleChoice,
	"multiple":        QuestionKindMultipleChoice,
	"multiple_choice": QuestionKindMultipleChoice,
	"multiple-choice": QuestionKindMultipleChoice,
	"mcq":             QuestionKindMultipleChoice,
	"true_false":      QuestionKindTrueFalse,
	"true-false":      QuestionKindTrueFalse,
	"boolean":         QuestionKindTrueFalse,
	"short":           QuestionKindShortAnswer,
	"short_answer":    QuestionKindShortAnswer,
	"short-answer":    QuestionKindShortAnswer,
	"text":            QuestionKindShortAnswer,
	"essay":           QuestionKindEssay,
	"ordering":        QuestionKindOrdering,
	"matching":        QuestionKindMatching,
}

type QuestionBankCategoryResponseDTO struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	SubjectID string `json:"subjectId,omitempty"`
	LevelID   string `json:"levelId,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Order     int    `json:"order"`
	Status    string `json:"status"`
}

type QuestionBankCategoryListResponseDTO struct {
	Items []QuestionBankCategoryResponseDTO `json:"items"`
}

func normalizeQuestionKind(kind string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(kind))
	key = strings.ReplaceAll(key, " ", "_")
	if normalized, ok := questionKindAliases[key]; ok {
		return normalized, nil
	}
	return "", fmt.Errorf("%w: %s", errInvalidQuestionKind, kind)
}

func questionKindFromDTO(kind, typ string) (string, error) {
	if strings.TrimSpace(kind) != "" {
		return normalizeQuestionKind(kind)
	}
	return normalizeQuestionKind(typ)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Service) ListQuestionBankCategories(ctx context.Context, tenantID, subjectID, levelID string) (QuestionBankCategoryListResponseDTO, error) {
	if strings.TrimSpace(levelID) != "" {
		topics, err := s.ListTopics(ctx, tenantID, levelID, true)
		if err != nil {
			return QuestionBankCategoryListResponseDTO{}, err
		}
		items := make([]QuestionBankCategoryResponseDTO, 0, len(topics.Items))
		for _, topic := range topics.Items {
			items = append(items, QuestionBankCategoryResponseDTO{
				ID:       topic.ID,
				Type:     "topic",
				LevelID:  topic.LevelID,
				ParentID: topic.LevelID,
				Name:     topic.Name,
				Code:     topic.Code,
				Order:    topic.Order,
				Status:   topic.Status,
			})
		}
		return QuestionBankCategoryListResponseDTO{Items: items}, nil
	}

	if strings.TrimSpace(subjectID) == "" {
		return QuestionBankCategoryListResponseDTO{Items: []QuestionBankCategoryResponseDTO{}}, nil
	}
	levels, err := s.ListLevels(ctx, tenantID, subjectID)
	if err != nil {
		return QuestionBankCategoryListResponseDTO{}, err
	}
	items := make([]QuestionBankCategoryResponseDTO, 0, len(levels.Items))
	for _, level := range levels.Items {
		items = append(items, QuestionBankCategoryResponseDTO{
			ID:        level.ID,
			Type:      "level",
			SubjectID: level.SubjectID,
			ParentID:  level.SubjectID,
			Name:      level.Name,
			Code:      level.Code,
			Order:     level.Order,
			Status:    level.Status,
		})
	}
	return QuestionBankCategoryListResponseDTO{Items: items}, nil
}
