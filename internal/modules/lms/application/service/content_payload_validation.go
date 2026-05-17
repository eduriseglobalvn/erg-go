package service

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const (
	maxQuestionStemLength = 4000
	maxChoiceCount        = 20
	maxChoiceLabelLength  = 1000
	maxQuizTitleLength    = 240
	maxQuizJSONBytes      = 128 * 1024
	maxAttemptAnswers     = 500
	maxAttemptEvents      = 1000
	maxAttemptJSONBytes   = 256 * 1024
)

func validateQuestionPayload(kind, stem string, choices []QuestionChoiceDTO, answer any, metadata map[string]any) error {
	stem = strings.TrimSpace(stem)
	if stem == "" || utf8.RuneCountInString(stem) > maxQuestionStemLength || hasControlChars(stem) {
		return errInvalidContentPayload
	}
	if metadata != nil && jsonSize(metadata) > maxQuizJSONBytes {
		return errInvalidContentPayload
	}
	switch kind {
	case QuestionKindSingleChoice:
		return validateChoiceQuestion(choices, answer, 1)
	case QuestionKindMultipleChoice:
		return validateChoiceQuestion(choices, answer, 0)
	case QuestionKindTrueFalse:
		if _, ok := answer.(bool); ok || answer == nil {
			return nil
		}
		if value, ok := answer.(string); ok && (strings.EqualFold(value, "true") || strings.EqualFold(value, "false")) {
			return nil
		}
		return errInvalidContentPayload
	case QuestionKindShortAnswer, QuestionKindEssay:
		if answer == nil {
			return nil
		}
		if value, ok := answer.(string); ok && utf8.RuneCountInString(value) <= maxChoiceLabelLength && !hasControlChars(value) {
			return nil
		}
		return errInvalidContentPayload
	case QuestionKindOrdering, QuestionKindMatching:
		if len(choices) == 0 || len(choices) > maxChoiceCount {
			return errInvalidContentPayload
		}
		return validateChoiceLabels(choices)
	default:
		return errInvalidContentPayload
	}
}

func validateQuizPayload(title string, slides []any, settings, result, theme map[string]any) error {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxQuizTitleLength || hasControlChars(title) {
		return errInvalidContentPayload
	}
	if len(slides) > 200 || jsonSize(slides) > maxQuizJSONBytes || jsonSize(settings) > maxQuizJSONBytes || jsonSize(result) > maxQuizJSONBytes || jsonSize(theme) > maxQuizJSONBytes {
		return errInvalidContentPayload
	}
	return nil
}

func validateAttemptPayload(answers map[string]any, events []map[string]any, client map[string]any) error {
	if len(answers) > maxAttemptAnswers || len(events) > maxAttemptEvents {
		return errInvalidContentPayload
	}
	if jsonSize(answers) > maxAttemptJSONBytes || jsonSize(events) > maxAttemptJSONBytes || jsonSize(client) > maxAttemptJSONBytes {
		return errInvalidContentPayload
	}
	for questionID := range answers {
		if _, err := objectID(questionID); err != nil {
			return errInvalidContentPayload
		}
	}
	return nil
}

func validateSingleAnswerPayload(answer any, clientResult map[string]any) error {
	if jsonSize(answer) > maxAttemptJSONBytes || jsonSize(clientResult) > maxAttemptJSONBytes {
		return errInvalidContentPayload
	}
	return nil
}

func validateChoiceQuestion(choices []QuestionChoiceDTO, answer any, exactCorrect int) error {
	if len(choices) < 2 || len(choices) > maxChoiceCount {
		return errInvalidContentPayload
	}
	if err := validateChoiceLabels(choices); err != nil {
		return err
	}
	correct := 0
	for _, choice := range choices {
		if choice.Correct {
			correct++
		}
	}
	if exactCorrect > 0 && correct != exactCorrect {
		return errInvalidContentPayload
	}
	if exactCorrect == 0 && correct == 0 {
		return errInvalidContentPayload
	}
	if answer == nil {
		return nil
	}
	if jsonSize(answer) > maxChoiceLabelLength*maxChoiceCount {
		return errInvalidContentPayload
	}
	return nil
}

func validateChoiceLabels(choices []QuestionChoiceDTO) error {
	seen := map[string]bool{}
	for _, choice := range choices {
		id := strings.TrimSpace(choice.ID)
		label := strings.TrimSpace(choice.Label)
		if id == "" || label == "" || seen[id] || utf8.RuneCountInString(label) > maxChoiceLabelLength || hasControlChars(label) {
			return errInvalidContentPayload
		}
		seen[id] = true
	}
	return nil
}

func jsonSize(value any) int {
	if value == nil {
		return 0
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return maxAttemptJSONBytes + 1
	}
	return len(bytes)
}
