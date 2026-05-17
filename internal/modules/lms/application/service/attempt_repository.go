package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (r *Repository) SaveAttemptDraft(ctx context.Context, tenantID, id string, req AttemptDraftRequestDTO) (*Attempt, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	set := bson.M{"updated_at": time.Now().UTC()}
	for questionID, answer := range req.Answers {
		field, err := answerField(questionID)
		if err != nil {
			return nil, err
		}
		set[field] = AttemptAnswer{QuestionID: questionID, Answer: answer}
	}
	if req.QuizVersion != "" {
		set["quiz_version"] = req.QuizVersion
	}
	if req.Events != nil {
		set["events"] = req.Events
	}
	if req.Client != nil {
		set["client"] = req.Client
	}
	res, err := r.attempts.UpdateOne(
		ctx,
		bson.M{"_id": oid, "tenant_id": tenantID, "status": bson.M{"$ne": "submitted"}},
		bson.M{"$set": set},
	)
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return r.GetAttempt(ctx, tenantID, id)
	}
	return r.GetAttempt(ctx, tenantID, id)
}
