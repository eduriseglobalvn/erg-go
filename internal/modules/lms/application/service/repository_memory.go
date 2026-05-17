package service

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type memoryStore struct {
	centers       []Center
	classes       []Class
	students      []Student
	subjects      []Subject
	levels        []Level
	topics        []Topic
	questions     []Question
	quizzes       []Quiz
	assignments   []Assignment
	attempts      []Attempt
	currentScopes []CurrentScope
}

func newMemoryRepository() *Repository {
	return &Repository{memory: &memoryStore{}}
}

func NewMemoryRepository() *Repository {
	return newMemoryRepository()
}

func (m *memoryStore) listCenters(tenantID string, req CenterListRequestDTO, managerUserID string) ([]Center, int64, error) {
	items := make([]Center, 0, len(m.centers))
	for _, center := range m.centers {
		if center.TenantID != tenantID {
			continue
		}
		if req.Status != "" && center.Status != req.Status {
			continue
		}
		if req.Type != "" && centerType(center) != req.Type {
			continue
		}
		if managerUserID != "" && center.ManagerUserID != managerUserID {
			continue
		}
		if req.Keyword != "" && !containsFold(center.Name, req.Keyword) && !containsFold(center.Code, req.Keyword) {
			continue
		}
		items = append(items, center)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return pageCenters(items, req.Page, req.Limit)
}

func (m *memoryStore) createCenter(center *Center) error {
	now := time.Now().UTC()
	if center.ID.IsZero() {
		center.ID = bson.NewObjectID()
	}
	if center.Type == "" {
		center.Type = educationUnitTypeCenter
	}
	if center.Status == "" {
		center.Status = statusActive
	}
	center.CreatedAt = now
	center.UpdatedAt = now
	m.centers = append(m.centers, *center)
	return nil
}

func (m *memoryStore) upsertCenterByCode(center Center) (*Center, error) {
	now := time.Now().UTC()
	if center.ID.IsZero() {
		center.ID = bson.NewObjectID()
	}
	if center.Type == "" {
		center.Type = educationUnitTypeCenter
	}
	if center.Status == "" {
		center.Status = statusActive
	}
	for i := range m.centers {
		if m.centers[i].TenantID != center.TenantID || m.centers[i].Code != center.Code {
			continue
		}
		m.centers[i].Type = center.Type
		m.centers[i].Name = center.Name
		m.centers[i].ParentID = center.ParentID
		m.centers[i].AvatarURL = center.AvatarURL
		m.centers[i].Address = center.Address
		m.centers[i].Description = center.Description
		m.centers[i].Phone = center.Phone
		m.centers[i].Email = center.Email
		m.centers[i].Website = center.Website
		m.centers[i].Status = center.Status
		m.centers[i].ManagerUserID = center.ManagerUserID
		m.centers[i].UpdatedAt = now
		copy := m.centers[i]
		return &copy, nil
	}
	center.CreatedAt = now
	center.UpdatedAt = now
	m.centers = append(m.centers, center)
	copy := center
	return &copy, nil
}

func (m *memoryStore) getCenter(tenantID, id string) (*Center, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, center := range m.centers {
		if center.TenantID == tenantID && center.ID == oid {
			copy := center
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) updateCenter(tenantID, id string, update bson.M) (*Center, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.centers {
		if m.centers[i].TenantID != tenantID || m.centers[i].ID != oid {
			continue
		}
		if v, ok := update["type"].(string); ok {
			m.centers[i].Type = v
		}
		if v, ok := update["name"].(string); ok {
			m.centers[i].Name = v
		}
		if v, ok := update["parent_id"].(bson.ObjectID); ok {
			m.centers[i].ParentID = v
		}
		if v, ok := update["avatar_url"].(string); ok {
			m.centers[i].AvatarURL = v
		}
		if v, ok := update["address"].(string); ok {
			m.centers[i].Address = v
		}
		if v, ok := update["description"].(string); ok {
			m.centers[i].Description = v
		}
		if v, ok := update["phone"].(string); ok {
			m.centers[i].Phone = v
		}
		if v, ok := update["email"].(string); ok {
			m.centers[i].Email = v
		}
		if v, ok := update["website"].(string); ok {
			m.centers[i].Website = v
		}
		if v, ok := update["status"].(string); ok {
			m.centers[i].Status = v
		}
		if v, ok := update["manager_user_id"].(string); ok {
			m.centers[i].ManagerUserID = v
		}
		m.centers[i].UpdatedAt = time.Now().UTC()
		copy := m.centers[i]
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryStore) listClasses(tenantID string, req ClassListRequestDTO, allowedCenterIDs []bson.ObjectID, homeroomTeacherID string) ([]Class, int64, error) {
	var requestedCenterID bson.ObjectID
	centerIDValue := req.CenterID
	if centerIDValue == "" {
		centerIDValue = req.UnitID
	}
	if centerIDValue != "" {
		oid, err := objectID(centerIDValue)
		if err != nil {
			return nil, 0, err
		}
		requestedCenterID = oid
	}
	items := make([]Class, 0, len(m.classes))
	for _, class := range m.classes {
		if class.TenantID != tenantID {
			continue
		}
		if centerIDValue != "" {
			if class.CenterID != requestedCenterID {
				continue
			}
		} else if len(allowedCenterIDs) > 0 && !objectIDIn(class.CenterID, allowedCenterIDs) {
			continue
		}
		if homeroomTeacherID != "" && class.HomeroomTeacherID != homeroomTeacherID {
			continue
		}
		if req.Grade != "" && class.Grade != req.Grade {
			continue
		}
		if req.AcademicYear != "" && class.AcademicYear != req.AcademicYear {
			continue
		}
		if req.Status != "" && class.Status != req.Status {
			continue
		}
		if req.Keyword != "" && !containsFold(class.Name, req.Keyword) {
			continue
		}
		items = append(items, class)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return pageClasses(items, req.Page, req.Limit)
}

func (m *memoryStore) createClass(class *Class) error {
	now := time.Now().UTC()
	if class.ID.IsZero() {
		class.ID = bson.NewObjectID()
	}
	if class.Status == "" {
		class.Status = statusActive
	}
	class.CreatedAt = now
	class.UpdatedAt = now
	m.classes = append(m.classes, *class)
	return nil
}

func (m *memoryStore) getClass(tenantID, id string) (*Class, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, class := range m.classes {
		if class.TenantID == tenantID && class.ID == oid {
			copy := class
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) getClassByName(tenantID string, centerID bson.ObjectID, name string) (*Class, error) {
	for _, class := range m.classes {
		if class.TenantID == tenantID && class.CenterID == centerID && class.Name == name {
			copy := class
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) updateClass(tenantID, id string, update bson.M) (*Class, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.classes {
		if m.classes[i].TenantID != tenantID || m.classes[i].ID != oid {
			continue
		}
		if v, ok := update["name"].(string); ok {
			m.classes[i].Name = v
		}
		if v, ok := update["grade"].(string); ok {
			m.classes[i].Grade = v
		}
		if v, ok := update["academic_year"].(string); ok {
			m.classes[i].AcademicYear = v
		}
		if v, ok := update["status"].(string); ok {
			m.classes[i].Status = v
		}
		if v, ok := update["homeroom_teacher_id"].(string); ok {
			m.classes[i].HomeroomTeacherID = v
		}
		m.classes[i].UpdatedAt = time.Now().UTC()
		copy := m.classes[i]
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryStore) createStudent(student *Student) error {
	now := time.Now().UTC()
	if student.ID.IsZero() {
		student.ID = bson.NewObjectID()
	}
	if student.Status == "" {
		student.Status = statusActive
	}
	student.CreatedAt = now
	student.UpdatedAt = now
	m.students = append(m.students, *student)
	return nil
}

func (m *memoryStore) usernameExists(tenantID, username string) bool {
	for _, student := range m.students {
		if student.TenantID == tenantID && student.Username == username {
			return true
		}
	}
	return false
}

func (m *memoryStore) getStudent(tenantID, id string) (*Student, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, student := range m.students {
		if student.TenantID == tenantID && student.ID == oid {
			copy := student
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) listStudents(tenantID string, req StudentListRequestDTO, allowedCenterIDs, allowedClassIDs []bson.ObjectID) ([]Student, int64, string, error) {
	var requestedCenterID bson.ObjectID
	var requestedClassID bson.ObjectID
	if req.CenterID != "" {
		oid, err := objectID(req.CenterID)
		if err != nil {
			return nil, 0, "", err
		}
		requestedCenterID = oid
	}
	if req.ClassID != "" {
		oid, err := objectID(req.ClassID)
		if err != nil {
			return nil, 0, "", err
		}
		requestedClassID = oid
	}
	items := make([]Student, 0, len(m.students))
	for _, student := range m.students {
		if student.TenantID != tenantID {
			continue
		}
		if req.CenterID != "" {
			if student.CenterID != requestedCenterID {
				continue
			}
		} else if len(allowedCenterIDs) > 0 && !objectIDIn(student.CenterID, allowedCenterIDs) {
			continue
		}
		if req.ClassID != "" {
			if student.ClassID != requestedClassID {
				continue
			}
		} else if len(allowedClassIDs) > 0 && !objectIDIn(student.ClassID, allowedClassIDs) {
			continue
		}
		if req.Status != "" && student.Status != req.Status {
			continue
		}
		if req.Keyword != "" &&
			!containsFold(student.StudentCode, req.Keyword) &&
			!containsFold(student.FullName, req.Keyword) &&
			!containsFold(student.Username, req.Keyword) &&
			!containsFold(student.Email, req.Keyword) &&
			!containsFold(student.Phone, req.Keyword) &&
			!containsFold(student.ParentName, req.Keyword) &&
			!containsFold(student.ParentPhone, req.Keyword) {
			continue
		}
		items = append(items, student)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	total := int64(len(items))
	limit := req.Limit
	if limit <= 0 {
		limit = defaultStudentBatchSize
	}
	if limit > maxStudentBatchSize {
		limit = maxStudentBatchSize
	}
	offset := cursorOffset(req.Cursor)
	if offset > int64(len(items)) {
		return []Student{}, total, "", nil
	}
	end := offset + limit
	if end > int64(len(items)) {
		end = int64(len(items))
	}
	next := ""
	if end < total {
		next = strconv.FormatInt(end, 10)
	}
	return append([]Student(nil), items[offset:end]...), total, next, nil
}

func (m *memoryStore) updateStudent(tenantID, id string, update bson.M) (*Student, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.students {
		if m.students[i].TenantID != tenantID || m.students[i].ID != oid {
			continue
		}
		if v, ok := update["full_name"].(string); ok {
			m.students[i].FullName = v
		}
		if v, ok := update["student_code"].(string); ok {
			m.students[i].StudentCode = v
		}
		if v, ok := update["email"].(string); ok {
			m.students[i].Email = v
		}
		if v, ok := update["gender"].(string); ok {
			m.students[i].Gender = v
		}
		if v, ok := update["birthday"].(time.Time); ok {
			m.students[i].Birthday = &v
		}
		if v, ok := update["phone"].(string); ok {
			m.students[i].Phone = v
		}
		if v, ok := update["address"].(string); ok {
			m.students[i].Address = v
		}
		if v, ok := update["parent_name"].(string); ok {
			m.students[i].ParentName = v
		}
		if v, ok := update["parent_phone"].(string); ok {
			m.students[i].ParentPhone = v
		}
		if v, ok := update["parent_email"].(string); ok {
			m.students[i].ParentEmail = v
		}
		if v, ok := update["parent_relationship"].(string); ok {
			m.students[i].ParentRelationship = v
		}
		if v, ok := update["enrollment_date"].(time.Time); ok {
			m.students[i].EnrollmentDate = &v
		}
		if v, ok := update["note"].(string); ok {
			m.students[i].Note = v
		}
		if v, ok := update["status"].(string); ok {
			m.students[i].Status = v
		}
		if v, ok := update["class_id"].(bson.ObjectID); ok {
			m.students[i].ClassID = v
		}
		if v, ok := update["center_id"].(bson.ObjectID); ok {
			m.students[i].CenterID = v
		}
		m.students[i].UpdatedAt = time.Now().UTC()
		copy := m.students[i]
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryStore) setStudentClass(tenantID, studentID string, targetClass *Class) error {
	oid, err := objectID(studentID)
	if err != nil {
		return err
	}
	for i := range m.students {
		if m.students[i].TenantID == tenantID && m.students[i].ID == oid {
			m.students[i].CenterID = targetClass.CenterID
			m.students[i].ClassID = targetClass.ID
			m.students[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return nil
}

func (m *memoryStore) createAssignment(assignment *Assignment) error {
	now := time.Now().UTC()
	if assignment.ID.IsZero() {
		assignment.ID = bson.NewObjectID()
	}
	if assignment.Status == "" {
		assignment.Status = "open"
	}
	assignment.CreatedAt = now
	assignment.UpdatedAt = now
	m.assignments = append(m.assignments, *assignment)
	return nil
}

func (m *memoryStore) getAssignment(tenantID, id string) (*Assignment, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, assignment := range m.assignments {
		if assignment.TenantID == tenantID && assignment.ID == oid {
			copy := assignment
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) listAssignments(tenantID string, filter bson.M) ([]Assignment, error) {
	items := make([]Assignment, 0, len(m.assignments))
	for _, assignment := range m.assignments {
		if assignment.TenantID != tenantID || !memoryAssignmentMatches(assignment, filter) {
			continue
		}
		items = append(items, assignment)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return append([]Assignment(nil), items...), nil
}

func (m *memoryStore) createAttempt(attempt *Attempt) error {
	now := time.Now().UTC()
	if attempt.ID.IsZero() {
		attempt.ID = bson.NewObjectID()
	}
	if attempt.Status == "" {
		attempt.Status = "in_progress"
	}
	if attempt.Answers == nil {
		attempt.Answers = map[string]AttemptAnswer{}
	}
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = now
	}
	attempt.UpdatedAt = now
	m.attempts = append(m.attempts, *attempt)
	return nil
}

func (m *memoryStore) getAttempt(tenantID, id string) (*Attempt, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, attempt := range m.attempts {
		if attempt.TenantID == tenantID && attempt.ID == oid {
			copy := attempt
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) findActiveAttempt(tenantID string, assignmentID, quizID, studentID bson.ObjectID) (*Attempt, error) {
	for _, attempt := range m.attempts {
		if attempt.TenantID == tenantID &&
			attempt.AssignmentID == assignmentID &&
			attempt.QuizID == quizID &&
			attempt.StudentID == studentID &&
			attempt.Status == "in_progress" {
			copy := attempt
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) saveAttemptAnswer(tenantID, id, questionID string, answer AttemptAnswer) (*Attempt, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.attempts {
		if m.attempts[i].TenantID != tenantID || m.attempts[i].ID != oid {
			continue
		}
		if m.attempts[i].Status == "submitted" {
			copy := m.attempts[i]
			return &copy, nil
		}
		if m.attempts[i].Answers == nil {
			m.attempts[i].Answers = map[string]AttemptAnswer{}
		}
		m.attempts[i].Answers[questionID] = answer
		m.attempts[i].UpdatedAt = time.Now().UTC()
		copy := m.attempts[i]
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryStore) submitAttempt(tenantID, id string, answers map[string]AttemptAnswer, submittedAt time.Time) (*Attempt, bool, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, false, err
	}
	for i := range m.attempts {
		if m.attempts[i].TenantID != tenantID || m.attempts[i].ID != oid {
			continue
		}
		if m.attempts[i].Status == "submitted" {
			copy := m.attempts[i]
			return &copy, false, nil
		}
		if m.attempts[i].Answers == nil {
			m.attempts[i].Answers = map[string]AttemptAnswer{}
		}
		for questionID, answer := range answers {
			m.attempts[i].Answers[questionID] = answer
		}
		m.attempts[i].Status = "submitted"
		m.attempts[i].SubmittedAt = &submittedAt
		m.attempts[i].UpdatedAt = time.Now().UTC()
		copy := m.attempts[i]
		return &copy, true, nil
	}
	return nil, false, nil
}

func (m *memoryStore) updateAttempt(tenantID, id string, update bson.M) (*Attempt, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.attempts {
		if m.attempts[i].TenantID != tenantID || m.attempts[i].ID != oid {
			continue
		}
		applyMemoryAttemptUpdate(&m.attempts[i], update)
		copy := m.attempts[i]
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryStore) listAttempts(tenantID string, filter bson.M) ([]Attempt, error) {
	items := make([]Attempt, 0, len(m.attempts))
	for _, attempt := range m.attempts {
		if attempt.TenantID != tenantID || !memoryAttemptMatches(attempt, filter) {
			continue
		}
		items = append(items, attempt)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return append([]Attempt(nil), items...), nil
}

func (m *memoryStore) getCurrentScope(tenantID, userID string) *CurrentScope {
	for _, scope := range m.currentScopes {
		if scope.TenantID == tenantID && scope.UserID == userID {
			copy := scope
			return &copy
		}
	}
	return nil
}

func (m *memoryStore) upsertCurrentScope(scope *CurrentScope) error {
	scope.UpdatedAt = time.Now().UTC()
	for i := range m.currentScopes {
		if m.currentScopes[i].TenantID == scope.TenantID && m.currentScopes[i].UserID == scope.UserID {
			m.currentScopes[i] = *scope
			return nil
		}
	}
	if scope.ID.IsZero() {
		scope.ID = bson.NewObjectID()
	}
	m.currentScopes = append(m.currentScopes, *scope)
	return nil
}

func (m *memoryStore) listSubjects(tenantID, scopeType, centerID string) ([]Subject, error) {
	items := make([]Subject, 0, len(m.subjects))
	for _, subject := range m.subjects {
		if subject.TenantID != tenantID || subject.Status == statusArchived {
			continue
		}
		if scopeType != "" && subject.Scope.Type != scopeType {
			continue
		}
		if centerID != "" && subject.Scope.CenterID != centerID {
			continue
		}
		items = append(items, subject)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return append([]Subject(nil), items...), nil
}

func (m *memoryStore) listLevels(tenantID, subjectID string) ([]Level, error) {
	oid, err := objectID(subjectID)
	if err != nil {
		return nil, err
	}
	items := make([]Level, 0, len(m.levels))
	for _, level := range m.levels {
		if level.TenantID == tenantID && level.SubjectID == oid && level.Status != statusArchived {
			items = append(items, level)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Order < items[j].Order })
	return append([]Level(nil), items...), nil
}

func (m *memoryStore) listTopics(tenantID, levelID string) ([]Topic, error) {
	oid, err := objectID(levelID)
	if err != nil {
		return nil, err
	}
	items := make([]Topic, 0, len(m.topics))
	for _, topic := range m.topics {
		if topic.TenantID == tenantID && topic.LevelID == oid && topic.Status != statusArchived {
			items = append(items, topic)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Order < items[j].Order })
	return append([]Topic(nil), items...), nil
}

func (m *memoryStore) createQuestion(q *Question) error {
	now := time.Now().UTC()
	if q.ID.IsZero() {
		q.ID = bson.NewObjectID()
	}
	if q.Status == "" {
		q.Status = statusActive
	}
	q.CreatedAt = now
	q.UpdatedAt = now
	m.questions = append(m.questions, *q)
	return nil
}

func (m *memoryStore) getQuestion(tenantID, id string) (*Question, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, question := range m.questions {
		if question.TenantID == tenantID && question.ID == oid {
			copy := question
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) listQuestions(tenantID string, filter bson.M, cursor string, limit int64) ([]Question, int64, string, error) {
	items := make([]Question, 0, len(m.questions))
	for _, question := range m.questions {
		if question.TenantID == tenantID && question.Status != statusArchived && memoryQuestionMatches(question, filter) {
			items = append(items, question)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	pagedItems, total, next := pageCursor(items, cursor, limit)
	return pagedItems, total, next, nil
}

func (m *memoryStore) updateQuestion(tenantID, id string, update bson.M) (*Question, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.questions {
		if m.questions[i].TenantID != tenantID || m.questions[i].ID != oid {
			continue
		}
		applyMemoryQuestionUpdate(&m.questions[i], update)
		copy := m.questions[i]
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryStore) randomQuestions(tenantID string, filter bson.M, limit int64) ([]Question, error) {
	items, _, _, err := m.listQuestions(tenantID, filter, "", limit)
	return items, err
}

func (m *memoryStore) createQuiz(q *Quiz) error {
	now := time.Now().UTC()
	if q.ID.IsZero() {
		q.ID = bson.NewObjectID()
	}
	if q.Status == "" {
		q.Status = "draft"
	}
	q.CreatedAt = now
	q.UpdatedAt = now
	m.quizzes = append(m.quizzes, *q)
	return nil
}

func (m *memoryStore) getQuiz(tenantID, id string) (*Quiz, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for _, quiz := range m.quizzes {
		if quiz.TenantID == tenantID && quiz.ID == oid {
			copy := quiz
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *memoryStore) listQuizzes(tenantID string, filter bson.M, cursor string, limit int64) ([]Quiz, int64, string, error) {
	items := make([]Quiz, 0, len(m.quizzes))
	for _, quiz := range m.quizzes {
		if quiz.TenantID == tenantID && memoryQuizMatches(quiz, filter) {
			items = append(items, quiz)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	pagedItems, total, next := pageCursor(items, cursor, limit)
	return pagedItems, total, next, nil
}

func (m *memoryStore) updateQuiz(tenantID, id string, update bson.M) (*Quiz, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	for i := range m.quizzes {
		if m.quizzes[i].TenantID != tenantID || m.quizzes[i].ID != oid {
			continue
		}
		applyMemoryQuizUpdate(&m.quizzes[i], update)
		copy := m.quizzes[i]
		return &copy, nil
	}
	return nil, nil
}

func pageCenters(items []Center, page, limit int64) ([]Center, int64, error) {
	offset, limit := paged(page, limit)
	total := int64(len(items))
	if offset > total {
		return []Center{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return append([]Center(nil), items[offset:end]...), total, nil
}

func pageClasses(items []Class, page, limit int64) ([]Class, int64, error) {
	offset, limit := paged(page, limit)
	total := int64(len(items))
	if offset > total {
		return []Class{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return append([]Class(nil), items[offset:end]...), total, nil
}

func objectIDIn(id bson.ObjectID, ids []bson.ObjectID) bool {
	for _, item := range ids {
		if item == id {
			return true
		}
	}
	return false
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func memoryAssignmentMatches(assignment Assignment, filter bson.M) bool {
	for key, value := range filter {
		switch key {
		case "class_id":
			if oid, ok := value.(bson.ObjectID); ok && assignment.ClassID != oid {
				return false
			}
		case "quiz_id":
			if oid, ok := value.(bson.ObjectID); ok && assignment.QuizID != oid {
				return false
			}
		case "subject_id":
			if oid, ok := value.(bson.ObjectID); ok && assignment.SubjectID != oid {
				return false
			}
		case "student_ids":
			if oid, ok := value.(bson.ObjectID); ok && !objectIDIn(oid, assignment.StudentIDs) {
				return false
			}
		case "status":
			if status, ok := value.(string); ok && assignment.Status != status {
				return false
			}
		}
	}
	return true
}

func memoryAttemptMatches(attempt Attempt, filter bson.M) bool {
	for key, value := range filter {
		switch key {
		case "assignment_id":
			if oid, ok := value.(bson.ObjectID); ok && attempt.AssignmentID != oid {
				return false
			}
		case "quiz_id":
			if oid, ok := value.(bson.ObjectID); ok && attempt.QuizID != oid {
				return false
			}
		case "student_id":
			if oid, ok := value.(bson.ObjectID); ok && attempt.StudentID != oid {
				return false
			}
		case "status":
			if status, ok := value.(string); ok && attempt.Status != status {
				return false
			}
		}
	}
	return true
}

func memoryQuestionMatches(question Question, filter bson.M) bool {
	for key, value := range filter {
		switch key {
		case "subject_id":
			if oid, ok := value.(bson.ObjectID); ok && question.SubjectID != oid {
				return false
			}
		case "level_id":
			if oid, ok := value.(bson.ObjectID); ok && question.LevelID != oid {
				return false
			}
		case "topic_id":
			switch v := value.(type) {
			case bson.ObjectID:
				if question.TopicID != v {
					return false
				}
			case bson.M:
				if ids, ok := v["$in"].([]bson.ObjectID); ok && !objectIDIn(question.TopicID, ids) {
					return false
				}
			}
		case "_id":
			if v, ok := value.(bson.M); ok {
				if ids, ok := v["$nin"].([]bson.ObjectID); ok && objectIDIn(question.ID, ids) {
					return false
				}
			}
		case "type":
			if typ, ok := value.(string); ok && question.Type != typ {
				return false
			}
		case "stem":
			if v, ok := value.(bson.M); ok {
				if keyword, ok := v["$regex"].(string); ok && !containsFold(question.Stem, keyword) {
					return false
				}
			}
		case "scope.type":
			if typ, ok := value.(string); ok && question.Scope.Type != typ {
				return false
			}
		case "scope.center_id":
			if centerID, ok := value.(string); ok && question.Scope.CenterID != centerID {
				return false
			}
		}
	}
	return true
}

func memoryQuizMatches(quiz Quiz, filter bson.M) bool {
	for key, value := range filter {
		switch key {
		case "subject_id":
			if oid, ok := value.(bson.ObjectID); ok && quiz.SubjectID != oid {
				return false
			}
		case "level_id":
			if oid, ok := value.(bson.ObjectID); ok && quiz.LevelID != oid {
				return false
			}
		case "kind":
			if kind, ok := value.(string); ok && quiz.Kind != kind {
				return false
			}
		case "title":
			if v, ok := value.(bson.M); ok {
				if keyword, ok := v["$regex"].(string); ok && !containsFold(quiz.Title, keyword) {
					return false
				}
			}
		case "scope.type":
			if typ, ok := value.(string); ok && quiz.Scope.Type != typ {
				return false
			}
		case "scope.center_id":
			if centerID, ok := value.(string); ok && quiz.Scope.CenterID != centerID {
				return false
			}
		}
	}
	return true
}

func applyMemoryQuestionUpdate(question *Question, update bson.M) {
	if v, ok := update["scope"].(ContentScope); ok {
		question.Scope = v
	}
	if v, ok := update["topic_id"].(bson.ObjectID); ok {
		question.TopicID = v
	}
	if v, ok := update["type"].(string); ok {
		question.Type = v
	}
	if v, ok := update["stem"].(string); ok {
		question.Stem = v
	}
	if v, ok := update["choices"].([]QuestionChoice); ok {
		question.Choices = v
	}
	if v, ok := update["answer"]; ok {
		question.Answer = v
	}
	if v, ok := update["metadata"].(map[string]any); ok {
		question.Metadata = v
	}
	if v, ok := update["status"].(string); ok {
		question.Status = v
	}
	if v, ok := update["archived_at"].(time.Time); ok {
		question.ArchivedAt = &v
	}
	question.UpdatedAt = time.Now().UTC()
}

func applyMemoryQuizUpdate(quiz *Quiz, update bson.M) {
	if v, ok := update["slides"].([]any); ok {
		quiz.Slides = v
	}
	if v, ok := update["settings"].(map[string]any); ok {
		quiz.Settings = v
	}
	if v, ok := update["result"].(map[string]any); ok {
		quiz.Result = v
	}
	if v, ok := update["theme"].(map[string]any); ok {
		quiz.Theme = v
	}
	if v, ok := update["status"].(string); ok {
		quiz.Status = v
	}
	if v, ok := update["version"].(int); ok {
		quiz.Version = v
	}
	if v, ok := update["package_hash"].(string); ok {
		quiz.PackageHash = v
	}
	if v, ok := update["published_at"].(time.Time); ok {
		quiz.PublishedAt = &v
	}
	quiz.UpdatedAt = time.Now().UTC()
}

func pageCursor[T any](items []T, cursor string, limit int64) ([]T, int64, string) {
	if limit <= 0 || limit > maxStudentBatchSize {
		limit = defaultStudentBatchSize
	}
	total := int64(len(items))
	offset := cursorOffset(cursor)
	if offset > total {
		return []T{}, total, ""
	}
	end := offset + limit
	if end > total {
		end = total
	}
	next := ""
	if end < total {
		next = strconv.FormatInt(end, 10)
	}
	return append([]T(nil), items[offset:end]...), total, next
}

func applyMemoryAttemptUpdate(attempt *Attempt, update bson.M) {
	if v, ok := update["status"].(string); ok {
		attempt.Status = v
	}
	if v, ok := update["quiz_version"].(string); ok {
		attempt.QuizVersion = v
	}
	if v, ok := update["events"].([]map[string]any); ok {
		attempt.Events = v
	}
	if v, ok := update["client"].(map[string]any); ok {
		attempt.Client = v
	}
	if v, ok := update["score"].(float64); ok {
		attempt.Score = v
	}
	if v, ok := update["max_score"].(float64); ok {
		attempt.MaxScore = v
	}
	if v, ok := update["percent"].(float64); ok {
		attempt.Percent = v
	}
	if v, ok := update["passed"].(bool); ok {
		attempt.Passed = v
	}
	if v, ok := update["submitted_at"].(time.Time); ok {
		attempt.SubmittedAt = &v
	}
	for key, value := range update {
		if !strings.HasPrefix(key, "answers.") {
			continue
		}
		if attempt.Answers == nil {
			attempt.Answers = map[string]AttemptAnswer{}
		}
		questionID := strings.TrimPrefix(key, "answers.")
		if answer, ok := value.(AttemptAnswer); ok {
			attempt.Answers[questionID] = answer
		}
	}
	attempt.UpdatedAt = time.Now().UTC()
}
