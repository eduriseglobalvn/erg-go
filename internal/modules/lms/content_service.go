package lms

import (
	"context"
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) ListSubjects(ctx context.Context, tenantID string, actor Actor, scopeType, centerID string) (SubjectListResponseDTO, error) {
	if scopeType == "global" && !actor.canAccessGlobal() {
		return SubjectListResponseDTO{}, errScopeForbidden
	}
	if centerID != "" && !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, centerID) {
		return SubjectListResponseDTO{}, errScopeForbidden
	}
	items, err := s.repo.ListSubjects(ctx, tenantID, scopeType, centerID)
	return SubjectListResponseDTO{Items: subjectsToDTO(items)}, err
}

func (s *Service) ListLevels(ctx context.Context, tenantID, subjectID string) (LevelListResponseDTO, error) {
	items, err := s.repo.ListLevels(ctx, tenantID, subjectID)
	return LevelListResponseDTO{Items: levelsToDTO(items)}, err
}

func (s *Service) ListTopics(ctx context.Context, tenantID, levelID string, includeOther bool) (TopicListResponseDTO, error) {
	items, err := s.repo.ListTopics(ctx, tenantID, levelID)
	if err != nil {
		return TopicListResponseDTO{}, err
	}
	if includeOther && len(items) == 0 {
		levelOID, _ := objectID(levelID)
		items = append(items, Topic{ID: bson.NewObjectID(), TenantID: tenantID, LevelID: levelOID, Name: "Khác", Code: "other", Order: 999, Status: statusActive})
	}
	return TopicListResponseDTO{Items: topicsToDTO(items)}, nil
}

func (s *Service) ListQuestions(ctx context.Context, tenantID string, actor Actor, req QuestionListRequestDTO) (QuestionListResponseDTO, error) {
	filter, err := questionFilter(req)
	if err != nil {
		return QuestionListResponseDTO{}, err
	}
	if err := s.applyScopeFilter(ctx, tenantID, actor, req.Scope, filter); err != nil {
		return QuestionListResponseDTO{}, err
	}
	items, total, next, err := s.repo.ListQuestions(ctx, tenantID, filter, req.Cursor, req.Limit)
	return QuestionListResponseDTO{Items: questionsToDTO(items), Total: total, NextCursor: next}, err
}

func (s *Service) CreateQuestion(ctx context.Context, tenantID string, actor Actor, req CreateQuestionRequestDTO) (QuestionResponseDTO, error) {
	if err := s.validateContentScope(ctx, tenantID, actor, req.Scope); err != nil {
		return QuestionResponseDTO{}, err
	}
	subjectID, err := objectID(req.SubjectID)
	if err != nil {
		return QuestionResponseDTO{}, err
	}
	levelID, err := objectID(req.LevelID)
	if err != nil {
		return QuestionResponseDTO{}, err
	}
	topicID := bson.NilObjectID
	if req.TopicID != "" {
		topicID, err = objectID(req.TopicID)
		if err != nil {
			return QuestionResponseDTO{}, err
		}
	}
	q := &Question{TenantID: tenantID, Scope: dtoToScope(req.Scope), SubjectID: subjectID, LevelID: levelID, TopicID: topicID, Type: req.Type, Stem: req.Stem, Choices: choicesFromDTO(req.Choices), Answer: req.Answer, Metadata: req.Metadata}
	if err := s.repo.CreateQuestion(ctx, q); err != nil {
		return QuestionResponseDTO{}, err
	}
	return questionToDTO(*q), nil
}

func (s *Service) UpdateQuestion(ctx context.Context, tenantID string, actor Actor, id string, req UpdateQuestionRequestDTO) (QuestionResponseDTO, error) {
	existing, err := s.repo.GetQuestion(ctx, tenantID, id)
	if err != nil {
		return QuestionResponseDTO{}, err
	}
	if existing == nil {
		return QuestionResponseDTO{}, errNotFound
	}
	if err := s.validateContentScope(ctx, tenantID, actor, scopeToDTO(existing.Scope)); err != nil {
		return QuestionResponseDTO{}, err
	}
	update := bson.M{}
	if req.Scope != nil {
		if err := s.validateContentScope(ctx, tenantID, actor, *req.Scope); err != nil {
			return QuestionResponseDTO{}, err
		}
		update["scope"] = dtoToScope(*req.Scope)
	}
	if req.TopicID != nil {
		topicID := bson.NilObjectID
		if *req.TopicID != "" {
			topicID, err = objectID(*req.TopicID)
			if err != nil {
				return QuestionResponseDTO{}, err
			}
		}
		update["topic_id"] = topicID
	}
	if req.Type != nil {
		update["type"] = *req.Type
	}
	if req.Stem != nil {
		update["stem"] = *req.Stem
	}
	if req.Choices != nil {
		update["choices"] = choicesFromDTO(req.Choices)
	}
	if req.Answer != nil {
		update["answer"] = req.Answer
	}
	if req.Metadata != nil {
		update["metadata"] = req.Metadata
	}
	q, err := s.repo.UpdateQuestion(ctx, tenantID, id, update)
	if err != nil {
		return QuestionResponseDTO{}, err
	}
	return questionToDTO(*q), nil
}

func (s *Service) ArchiveQuestion(ctx context.Context, tenantID string, actor Actor, id string) (ArchiveQuestionResponseDTO, error) {
	existing, err := s.repo.GetQuestion(ctx, tenantID, id)
	if err != nil {
		return ArchiveQuestionResponseDTO{}, err
	}
	if existing == nil {
		return ArchiveQuestionResponseDTO{}, errNotFound
	}
	if err := s.validateContentScope(ctx, tenantID, actor, scopeToDTO(existing.Scope)); err != nil {
		return ArchiveQuestionResponseDTO{}, err
	}
	now := time.Now().UTC()
	_, err = s.repo.UpdateQuestion(ctx, tenantID, id, bson.M{"status": statusArchived, "archived_at": now})
	return ArchiveQuestionResponseDTO{ArchivedAt: now}, err
}

func (s *Service) RandomPickQuestions(ctx context.Context, tenantID string, req RandomPickQuestionsRequestDTO) (RandomPickQuestionsResponseDTO, error) {
	filter, err := randomQuestionFilter(req)
	if err != nil {
		return RandomPickQuestionsResponseDTO{}, err
	}
	seed := time.Now().UnixNano()
	items, err := s.repo.RandomQuestions(ctx, tenantID, filter, req.Count, seed)
	return RandomPickQuestionsResponseDTO{Questions: questionsToDTO(items), Seed: seed}, err
}

func (s *Service) ListQuizzes(ctx context.Context, tenantID string, actor Actor, req QuizListRequestDTO) (QuizListResponseDTO, error) {
	filter, err := quizFilter(req)
	if err != nil {
		return QuizListResponseDTO{}, err
	}
	if err := s.applyScopeFilter(ctx, tenantID, actor, req.Scope, filter); err != nil {
		return QuizListResponseDTO{}, err
	}
	items, total, next, err := s.repo.ListQuizzes(ctx, tenantID, filter, req.Cursor, req.Limit)
	return QuizListResponseDTO{Items: quizzesToDTO(items), Total: total, NextCursor: next}, err
}

func (s *Service) CreateQuiz(ctx context.Context, tenantID string, actor Actor, req CreateQuizRequestDTO) (QuizResponseDTO, error) {
	if err := s.validateContentScope(ctx, tenantID, actor, req.Scope); err != nil {
		return QuizResponseDTO{}, err
	}
	quiz, err := quizFromCreateReq(tenantID, req)
	if err != nil {
		return QuizResponseDTO{}, err
	}
	if err := s.repo.CreateQuiz(ctx, quiz); err != nil {
		return QuizResponseDTO{}, err
	}
	return quizToDTO(*quiz), nil
}

func (s *Service) CreateQuizFromQuestions(ctx context.Context, tenantID string, actor Actor, req CreateQuizFromQuestionsRequestDTO) (QuizResponseDTO, error) {
	if len(req.QuestionIDs) == 0 {
		return QuizResponseDTO{}, fmt.Errorf("questionIds required")
	}
	first, err := s.repo.GetQuestion(ctx, tenantID, req.QuestionIDs[0])
	if err != nil {
		return QuizResponseDTO{}, err
	}
	if first == nil {
		return QuizResponseDTO{}, errNotFound
	}
	scope := scopeToDTO(first.Scope)
	if err := s.validateContentScope(ctx, tenantID, actor, scope); err != nil {
		return QuizResponseDTO{}, err
	}
	quiz := &Quiz{TenantID: tenantID, Scope: first.Scope, Title: req.Title, Kind: req.Kind, SubjectID: first.SubjectID, LevelID: first.LevelID, QuestionIDs: objectIDsOrNil(req.QuestionIDs), Settings: req.Settings}
	if err := s.repo.CreateQuiz(ctx, quiz); err != nil {
		return QuizResponseDTO{}, err
	}
	return quizToDTO(*quiz), nil
}

func (s *Service) CreateRandomQuiz(ctx context.Context, tenantID string, actor Actor, req CreateRandomQuizRequestDTO) (QuizResponseDTO, error) {
	questionIDs := []string{}
	topicIDs := []string{}
	for _, rule := range req.TopicRules {
		topicIDs = append(topicIDs, rule.TopicID)
		picked, err := s.RandomPickQuestions(ctx, tenantID, RandomPickQuestionsRequestDTO{SubjectID: req.SubjectID, LevelID: req.LevelID, TopicIDs: []string{rule.TopicID}, Count: rule.Count})
		if err != nil {
			return QuizResponseDTO{}, err
		}
		for _, q := range picked.Questions {
			questionIDs = append(questionIDs, q.ID)
		}
	}
	return s.CreateQuiz(ctx, tenantID, actor, CreateQuizRequestDTO{Scope: ContentScopeDTO{Type: "global"}, Title: "Random quiz", Kind: req.Kind, SubjectID: req.SubjectID, LevelID: req.LevelID, TopicIDs: topicIDs, QuestionIDs: questionIDs, Settings: req.Settings})
}

func (s *Service) GetQuizDetail(ctx context.Context, tenantID, id string) (QuizDetailResponseDTO, error) {
	quiz, err := s.repo.GetQuiz(ctx, tenantID, id)
	if err != nil {
		return QuizDetailResponseDTO{}, err
	}
	if quiz == nil {
		return QuizDetailResponseDTO{}, errNotFound
	}
	return quizDetailToDTO(*quiz), nil
}

func (s *Service) UpdateQuiz(ctx context.Context, tenantID, id string, req UpdateQuizRequestDTO) (QuizDetailResponseDTO, error) {
	quiz, err := s.repo.UpdateQuiz(ctx, tenantID, id, bson.M{"slides": req.Slides, "settings": req.Settings, "result": req.Result, "theme": req.Theme})
	if err != nil {
		return QuizDetailResponseDTO{}, err
	}
	if quiz == nil {
		return QuizDetailResponseDTO{}, errNotFound
	}
	return quizDetailToDTO(*quiz), nil
}

func (s *Service) PublishQuiz(ctx context.Context, tenantID, id string) (PublishQuizResponseDTO, error) {
	quiz, err := s.repo.GetQuiz(ctx, tenantID, id)
	if err != nil {
		return PublishQuizResponseDTO{}, err
	}
	if quiz == nil {
		return PublishQuizResponseDTO{}, errNotFound
	}
	version := quiz.Version + 1
	hash := hashQuiz(*quiz, version)
	now := time.Now().UTC()
	_, err = s.repo.UpdateQuiz(ctx, tenantID, id, bson.M{"status": "published", "version": version, "package_hash": hash, "published_at": now})
	return PublishQuizResponseDTO{Version: version, PackageHash: hash}, err
}

func (s *Service) QuizPackage(ctx context.Context, tenantID, id string) (QuizPackageResponseDTO, error) {
	detail, err := s.GetQuizDetail(ctx, tenantID, id)
	if err != nil {
		return QuizPackageResponseDTO{}, err
	}
	hash := hashAny(detail)
	return QuizPackageResponseDTO{ContentHash: hash, GradingMode: "server", Quiz: detail}, nil
}

func (s *Service) validateContentScope(ctx context.Context, tenantID string, actor Actor, scope ContentScopeDTO) error {
	if scope.Type == "global" {
		if actor.canAccessGlobal() {
			return nil
		}
		return errScopeForbidden
	}
	if scope.Type == "center" && scope.CenterID != "" && s.canAccessCenter(ctx, tenantID, actor, scope.CenterID) {
		return nil
	}
	if actor.canAccessGlobal() {
		return nil
	}
	return errScopeForbidden
}

func (s *Service) applyScopeFilter(ctx context.Context, tenantID string, actor Actor, scope ContentScopeDTO, filter bson.M) error {
	if scope.Type != "" {
		if err := s.validateContentScope(ctx, tenantID, actor, scope); err != nil {
			return err
		}
		filter["scope.type"] = scope.Type
		if scope.CenterID != "" {
			filter["scope.center_id"] = scope.CenterID
		}
		return nil
	}
	if !actor.canAccessGlobal() {
		filter["scope.type"] = "center"
	}
	return nil
}

type QuestionListRequestDTO struct {
	Scope     ContentScopeDTO
	SubjectID string
	LevelID   string
	TopicID   string
	Keyword   string
	Type      string
	Cursor    string
	Limit     int64
}

type QuizListRequestDTO struct {
	Scope     ContentScopeDTO
	SubjectID string
	LevelID   string
	Kind      string
	Keyword   string
	Cursor    string
	Limit     int64
}

func questionFilter(req QuestionListRequestDTO) (bson.M, error) {
	filter := bson.M{}
	if req.SubjectID != "" {
		oid, err := objectID(req.SubjectID)
		if err != nil {
			return nil, err
		}
		filter["subject_id"] = oid
	}
	if req.LevelID != "" {
		oid, err := objectID(req.LevelID)
		if err != nil {
			return nil, err
		}
		filter["level_id"] = oid
	}
	if req.TopicID != "" {
		oid, err := objectID(req.TopicID)
		if err != nil {
			return nil, err
		}
		filter["topic_id"] = oid
	}
	if req.Type != "" {
		filter["type"] = req.Type
	}
	if req.Keyword != "" {
		filter["stem"] = bson.M{"$regex": req.Keyword, "$options": "i"}
	}
	return filter, nil
}

func randomQuestionFilter(req RandomPickQuestionsRequestDTO) (bson.M, error) {
	filter, err := questionFilter(QuestionListRequestDTO{SubjectID: req.SubjectID, LevelID: req.LevelID})
	if err != nil {
		return nil, err
	}
	if len(req.TopicIDs) > 0 {
		filter["topic_id"] = bson.M{"$in": objectIDsOrNil(req.TopicIDs)}
	}
	if len(req.ExcludeQuestionIDs) > 0 {
		filter["_id"] = bson.M{"$nin": objectIDsOrNil(req.ExcludeQuestionIDs)}
	}
	return filter, nil
}

func quizFilter(req QuizListRequestDTO) (bson.M, error) {
	filter := bson.M{}
	if req.SubjectID != "" {
		oid, err := objectID(req.SubjectID)
		if err != nil {
			return nil, err
		}
		filter["subject_id"] = oid
	}
	if req.LevelID != "" {
		oid, err := objectID(req.LevelID)
		if err != nil {
			return nil, err
		}
		filter["level_id"] = oid
	}
	if req.Kind != "" {
		filter["kind"] = req.Kind
	}
	if req.Keyword != "" {
		filter["title"] = bson.M{"$regex": req.Keyword, "$options": "i"}
	}
	return filter, nil
}

func quizFromCreateReq(tenantID string, req CreateQuizRequestDTO) (*Quiz, error) {
	subjectID, err := objectID(req.SubjectID)
	if err != nil {
		return nil, err
	}
	levelID, err := objectID(req.LevelID)
	if err != nil {
		return nil, err
	}
	return &Quiz{TenantID: tenantID, Scope: dtoToScope(req.Scope), Title: req.Title, Kind: req.Kind, SubjectID: subjectID, LevelID: levelID, TopicIDs: objectIDsOrNil(req.TopicIDs), QuestionIDs: objectIDsOrNil(req.QuestionIDs), Settings: req.Settings, ThemeID: req.ThemeID}, nil
}

func objectIDsOrNil(ids []string) []bson.ObjectID {
	out := []bson.ObjectID{}
	for _, id := range ids {
		if oid, err := objectID(id); err == nil {
			out = append(out, oid)
		}
	}
	return out
}

func dtoToScope(scope ContentScopeDTO) ContentScope {
	return ContentScope{Type: scope.Type, CenterID: scope.CenterID}
}
func scopeToDTO(scope ContentScope) ContentScopeDTO {
	return ContentScopeDTO{Type: scope.Type, CenterID: scope.CenterID}
}

func subjectsToDTO(items []Subject) []SubjectResponseDTO {
	out := make([]SubjectResponseDTO, 0, len(items))
	for _, i := range items {
		out = append(out, SubjectResponseDTO{ID: i.ID.Hex(), Scope: scopeToDTO(i.Scope), Name: i.Name, Code: i.Code, Status: i.Status, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt})
	}
	return out
}

func levelsToDTO(items []Level) []LevelResponseDTO {
	out := make([]LevelResponseDTO, 0, len(items))
	for _, i := range items {
		out = append(out, LevelResponseDTO{ID: i.ID.Hex(), SubjectID: i.SubjectID.Hex(), Name: i.Name, Code: i.Code, Order: i.Order, Status: i.Status, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt})
	}
	return out
}

func topicsToDTO(items []Topic) []TopicResponseDTO {
	out := make([]TopicResponseDTO, 0, len(items))
	for _, i := range items {
		out = append(out, TopicResponseDTO{ID: i.ID.Hex(), LevelID: i.LevelID.Hex(), Name: i.Name, Code: i.Code, Order: i.Order, Status: i.Status, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt})
	}
	return out
}

func questionsToDTO(items []Question) []QuestionResponseDTO {
	out := make([]QuestionResponseDTO, 0, len(items))
	for _, i := range items {
		out = append(out, questionToDTO(i))
	}
	return out
}

func questionToDTO(q Question) QuestionResponseDTO {
	topicID := ""
	if q.TopicID != bson.NilObjectID {
		topicID = q.TopicID.Hex()
	}
	return QuestionResponseDTO{ID: q.ID.Hex(), Scope: scopeToDTO(q.Scope), SubjectID: q.SubjectID.Hex(), LevelID: q.LevelID.Hex(), TopicID: topicID, Type: q.Type, Stem: q.Stem, Choices: choicesToDTO(q.Choices), Answer: q.Answer, Metadata: q.Metadata, Status: q.Status, CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt}
}

func choicesFromDTO(items []QuestionChoiceDTO) []QuestionChoice {
	out := make([]QuestionChoice, 0, len(items))
	for _, i := range items {
		out = append(out, QuestionChoice{ID: i.ID, Label: i.Label, Correct: i.Correct})
	}
	return out
}

func choicesToDTO(items []QuestionChoice) []QuestionChoiceDTO {
	out := make([]QuestionChoiceDTO, 0, len(items))
	for _, i := range items {
		out = append(out, QuestionChoiceDTO{ID: i.ID, Label: i.Label, Correct: i.Correct})
	}
	return out
}

func quizzesToDTO(items []Quiz) []QuizResponseDTO {
	out := make([]QuizResponseDTO, 0, len(items))
	for _, i := range items {
		out = append(out, quizToDTO(i))
	}
	return out
}

func quizToDTO(q Quiz) QuizResponseDTO {
	return QuizResponseDTO{ID: q.ID.Hex(), Scope: scopeToDTO(q.Scope), Title: q.Title, Kind: q.Kind, SubjectID: q.SubjectID.Hex(), LevelID: q.LevelID.Hex(), TopicIDs: objectIDsToHex(q.TopicIDs), QuestionIDs: objectIDsToHex(q.QuestionIDs), Status: q.Status, Version: q.Version, CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt}
}

func quizDetailToDTO(q Quiz) QuizDetailResponseDTO {
	return QuizDetailResponseDTO{Quiz: quizToDTO(q), Slides: q.Slides, Settings: q.Settings, Result: q.Result, Theme: q.Theme}
}

func objectIDsToHex(ids []bson.ObjectID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Hex())
	}
	return out
}

func hashQuiz(q Quiz, version int) string {
	return hashAny(map[string]any{"quiz": q.ID.Hex(), "version": version, "questions": objectIDsToHex(q.QuestionIDs), "time": time.Now().UTC().Format(time.RFC3339Nano)})
}

func hashAny(v any) string {
	bytes, _ := json.Marshal(v)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func randomSeededTitle(prefix string) string {
	n, err := crypto_rand.Int(crypto_rand.Reader, big.NewInt(100_000))
	if err != nil {
		return fmt.Sprintf("%s %d", strings.TrimSpace(prefix), time.Now().UnixNano()%100000)
	}
	return fmt.Sprintf("%s %d", strings.TrimSpace(prefix), n.Int64())
}
