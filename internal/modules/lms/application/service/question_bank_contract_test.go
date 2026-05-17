package service

import "testing"

func TestNormalizeQuestionKindAcceptsCanonicalAndLegacyAliases(t *testing.T) {
	tests := map[string]string{
		"single":          QuestionKindSingleChoice,
		"single-choice":   QuestionKindSingleChoice,
		"multiple_choice": QuestionKindMultipleChoice,
		"mcq":             QuestionKindMultipleChoice,
		"true-false":      QuestionKindTrueFalse,
		"text":            QuestionKindShortAnswer,
		"essay":           QuestionKindEssay,
	}

	for input, want := range tests {
		got, err := normalizeQuestionKind(input)
		if err != nil {
			t.Fatalf("normalizeQuestionKind(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeQuestionKind(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeQuestionKindRejectsUnknownKind(t *testing.T) {
	if _, err := normalizeQuestionKind("free_draw"); err == nil {
		t.Fatal("expected unknown question kind to be rejected")
	}
}

func TestQuestionFilterNormalizesKind(t *testing.T) {
	filter, err := questionFilter(QuestionListRequestDTO{Type: "single-choice"})
	if err != nil {
		t.Fatalf("questionFilter returned error: %v", err)
	}
	if got := filter["type"]; got != QuestionKindSingleChoice {
		t.Fatalf("filter type = %v, want %s", got, QuestionKindSingleChoice)
	}
}
