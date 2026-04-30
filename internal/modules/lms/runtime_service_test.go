package lms

import "testing"

func TestAnswerFieldRejectsUnsafeQuestionIDs(t *testing.T) {
	tests := []string{"", "question.with.dot", "$question"}
	for _, questionID := range tests {
		if _, err := answerField(questionID); err == nil {
			t.Fatalf("expected question id %q to be rejected", questionID)
		}
	}
}

func TestAnswerFieldBuildsAtomicAnswerPath(t *testing.T) {
	got, err := answerField("question-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "answers.question-1" {
		t.Fatalf("expected answers.question-1, got %q", got)
	}
}

func TestAttemptSubmitResultFallsBackToAnswerCount(t *testing.T) {
	result := attemptSubmitResult(Attempt{
		Answers: map[string]AttemptAnswer{
			"q1": {QuestionID: "q1", Answer: "a"},
			"q2": {QuestionID: "q2", Answer: "b"},
		},
	})

	if result.Score != 2 || result.MaxScore != 100 || result.Percent != 2 || result.Passed {
		t.Fatalf("unexpected submit result: %+v", result)
	}
}
