package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/pkg/database"
)

type Repository struct {
	memory        *memoryStore
	centers       *mongo.Collection
	classes       *mongo.Collection
	students      *mongo.Collection
	currentScopes *mongo.Collection
	previews      *mongo.Collection
	importJobs    *mongo.Collection
	subjects      *mongo.Collection
	levels        *mongo.Collection
	topics        *mongo.Collection
	questions     *mongo.Collection
	quizzes       *mongo.Collection
	assignments   *mongo.Collection
	attempts      *mongo.Collection
	threads       *mongo.Collection
	replies       *mongo.Collection
	attachments   *mongo.Collection
	announcements *mongo.Collection
	internalDocs  *mongo.Collection
}

func NewRepository(mongoClient *database.MongoClient) *Repository {
	return &Repository{
		centers:       mongoClient.Collection(centerCollection),
		classes:       mongoClient.Collection(classCollection),
		students:      mongoClient.Collection(studentCollection),
		currentScopes: mongoClient.Collection(currentScopeCollection),
		previews:      mongoClient.Collection(importPreviewCollection),
		importJobs:    mongoClient.Collection(importJobCollection),
		subjects:      mongoClient.Collection(subjectCollection),
		levels:        mongoClient.Collection(levelCollection),
		topics:        mongoClient.Collection(topicCollection),
		questions:     mongoClient.Collection(questionCollection),
		quizzes:       mongoClient.Collection(quizCollection),
		assignments:   mongoClient.Collection(assignmentCollection),
		attempts:      mongoClient.Collection(attemptCollection),
		threads:       mongoClient.Collection(discussionThreadCollection),
		replies:       mongoClient.Collection(discussionReplyCollection),
		attachments:   mongoClient.Collection(discussionAttachmentCollection),
		announcements: mongoClient.Collection(announcementCollection),
		internalDocs:  mongoClient.Collection(internalDocumentCollection),
	}
}

func objectID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.NilObjectID, fmt.Errorf("invalid object id %q", id)
	}
	return oid, nil
}

func paged(page, limit int64) (int64, int64) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = defaultManagementPageSize
	}
	return (page - 1) * limit, limit
}

func appendFilterAnd(existing any, clauses ...bson.M) []bson.M {
	out, _ := existing.([]bson.M)
	return append(out, clauses...)
}

func cursorOffset(cursor string) int64 {
	if cursor == "" {
		return 0
	}
	n, _ := strconv.ParseInt(cursor, 10, 64)
	if n < 0 {
		return 0
	}
	return n
}

func (r *Repository) ListCenters(ctx context.Context, tenantID string, req CenterListRequestDTO, managerUserID string) ([]Center, int64, error) {
	if r.memory != nil {
		return r.memory.listCenters(tenantID, req, managerUserID)
	}
	filter := bson.M{"tenant_id": tenantID}
	if req.Status != "" {
		filter["status"] = req.Status
	}
	if req.Type != "" {
		if req.Type == educationUnitTypeCenter {
			filter["$and"] = appendFilterAnd(filter["$and"], bson.M{"$or": []bson.M{
				{"type": educationUnitTypeCenter},
				{"type": bson.M{"$exists": false}},
				{"type": ""},
			}})
		} else {
			filter["type"] = req.Type
		}
	}
	if req.Keyword != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"code": bson.M{"$regex": req.Keyword, "$options": "i"}},
		}
	}
	if managerUserID != "" {
		filter["manager_user_id"] = managerUserID
	}
	return r.findCenters(ctx, filter, req.Page, req.Limit)
}

func (r *Repository) findCenters(ctx context.Context, filter bson.M, page, limit int64) ([]Center, int64, error) {
	total, err := r.centers.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	offset, limit := paged(page, limit)
	cur, err := r.centers.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var items []Center
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) CreateCenter(ctx context.Context, center *Center) error {
	if r.memory != nil {
		return r.memory.createCenter(center)
	}
	now := time.Now().UTC()
	center.CreatedAt = now
	center.UpdatedAt = now
	if center.Type == "" {
		center.Type = educationUnitTypeCenter
	}
	if center.Status == "" {
		center.Status = statusActive
	}
	res, err := r.centers.InsertOne(ctx, center)
	if err == nil {
		if oid, ok := res.InsertedID.(bson.ObjectID); ok {
			center.ID = oid
		}
	}
	return err
}

func (r *Repository) UpsertCenterByCode(ctx context.Context, center Center) (*Center, error) {
	if r.memory != nil {
		return r.memory.upsertCenterByCode(center)
	}
	now := time.Now().UTC()
	if center.Type == "" {
		center.Type = educationUnitTypeCenter
	}
	if center.Status == "" {
		center.Status = statusActive
	}
	filter := bson.M{"tenant_id": center.TenantID, "code": center.Code}
	update := bson.M{
		"$set": bson.M{
			"type":            center.Type,
			"name":            center.Name,
			"parent_id":       center.ParentID,
			"avatar_url":      center.AvatarURL,
			"address":         center.Address,
			"description":     center.Description,
			"phone":           center.Phone,
			"email":           center.Email,
			"website":         center.Website,
			"status":          center.Status,
			"manager_user_id": center.ManagerUserID,
			"updated_at":      now,
		},
		"$setOnInsert": bson.M{
			"_id":        center.ID,
			"tenant_id":  center.TenantID,
			"code":       center.Code,
			"created_at": now,
		},
	}
	if center.ID.IsZero() {
		update["$setOnInsert"].(bson.M)["_id"] = bson.NewObjectID()
	}
	_, err := r.centers.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return nil, err
	}
	var out Center
	if err := r.centers.FindOne(ctx, filter).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) GetCenter(ctx context.Context, tenantID, id string) (*Center, error) {
	if r.memory != nil {
		return r.memory.getCenter(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var center Center
	err = r.centers.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&center)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &center, nil
}

func (r *Repository) UpdateCenter(ctx context.Context, tenantID, id string, update bson.M) (*Center, error) {
	if r.memory != nil {
		return r.memory.updateCenter(tenantID, id, update)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	update["updated_at"] = time.Now().UTC()
	_, err = r.centers.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.GetCenter(ctx, tenantID, id)
}

func (r *Repository) ListClasses(ctx context.Context, tenantID string, req ClassListRequestDTO, allowedCenterIDs []bson.ObjectID, homeroomTeacherID string) ([]Class, int64, error) {
	if r.memory != nil {
		return r.memory.listClasses(tenantID, req, allowedCenterIDs, homeroomTeacherID)
	}
	filter := bson.M{"tenant_id": tenantID}
	centerIDValue := req.CenterID
	if centerIDValue == "" {
		centerIDValue = req.UnitID
	}
	if centerIDValue != "" {
		centerID, err := objectID(centerIDValue)
		if err != nil {
			return nil, 0, err
		}
		filter["center_id"] = centerID
	} else if len(allowedCenterIDs) > 0 {
		filter["center_id"] = bson.M{"$in": allowedCenterIDs}
	}
	if homeroomTeacherID != "" {
		filter["homeroom_teacher_id"] = homeroomTeacherID
	}
	if req.Grade != "" {
		filter["grade"] = req.Grade
	}
	if req.AcademicYear != "" {
		filter["academic_year"] = req.AcademicYear
	}
	if req.Status != "" {
		filter["status"] = req.Status
	}
	if req.Keyword != "" {
		filter["name"] = bson.M{"$regex": req.Keyword, "$options": "i"}
	}
	return r.findClasses(ctx, filter, req.Page, req.Limit)
}

func (r *Repository) findClasses(ctx context.Context, filter bson.M, page, limit int64) ([]Class, int64, error) {
	total, err := r.classes.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	offset, limit := paged(page, limit)
	cur, err := r.classes.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var items []Class
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) CreateClass(ctx context.Context, class *Class) error {
	if r.memory != nil {
		return r.memory.createClass(class)
	}
	now := time.Now().UTC()
	class.CreatedAt = now
	class.UpdatedAt = now
	if class.Status == "" {
		class.Status = statusActive
	}
	res, err := r.classes.InsertOne(ctx, class)
	if err == nil {
		if oid, ok := res.InsertedID.(bson.ObjectID); ok {
			class.ID = oid
		}
	}
	return err
}

func (r *Repository) GetClass(ctx context.Context, tenantID, id string) (*Class, error) {
	if r.memory != nil {
		return r.memory.getClass(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var class Class
	err = r.classes.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&class)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &class, nil
}

func (r *Repository) GetClassByName(ctx context.Context, tenantID string, centerID bson.ObjectID, name string) (*Class, error) {
	if r.memory != nil {
		return r.memory.getClassByName(tenantID, centerID, name)
	}
	var class Class
	err := r.classes.FindOne(ctx, bson.M{"tenant_id": tenantID, "center_id": centerID, "name": name}).Decode(&class)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &class, nil
}

func (r *Repository) UpdateClass(ctx context.Context, tenantID, id string, update bson.M) (*Class, error) {
	if r.memory != nil {
		return r.memory.updateClass(tenantID, id, update)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	update["updated_at"] = time.Now().UTC()
	_, err = r.classes.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.GetClass(ctx, tenantID, id)
}

func (r *Repository) CreateStudent(ctx context.Context, student *Student) error {
	if r.memory != nil {
		return r.memory.createStudent(student)
	}
	now := time.Now().UTC()
	student.CreatedAt = now
	student.UpdatedAt = now
	if student.Status == "" {
		student.Status = statusActive
	}
	_, err := r.students.InsertOne(ctx, student)
	return err
}

func (r *Repository) UsernameExists(ctx context.Context, tenantID, username string) (bool, error) {
	if r.memory != nil {
		return r.memory.usernameExists(tenantID, username), nil
	}
	count, err := r.students.CountDocuments(ctx, bson.M{"tenant_id": tenantID, "username": username})
	return count > 0, err
}

func (r *Repository) GetStudent(ctx context.Context, tenantID, id string) (*Student, error) {
	if r.memory != nil {
		return r.memory.getStudent(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var student Student
	err = r.students.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&student)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &student, nil
}

func (r *Repository) ListStudents(ctx context.Context, tenantID string, req StudentListRequestDTO, allowedCenterIDs, allowedClassIDs []bson.ObjectID) ([]Student, int64, string, error) {
	if r.memory != nil {
		return r.memory.listStudents(tenantID, req, allowedCenterIDs, allowedClassIDs)
	}
	filter := bson.M{"tenant_id": tenantID}
	if req.CenterID != "" {
		centerID, err := objectID(req.CenterID)
		if err != nil {
			return nil, 0, "", err
		}
		filter["center_id"] = centerID
	} else if len(allowedCenterIDs) > 0 {
		filter["center_id"] = bson.M{"$in": allowedCenterIDs}
	}
	if req.ClassID != "" {
		classID, err := objectID(req.ClassID)
		if err != nil {
			return nil, 0, "", err
		}
		filter["class_id"] = classID
	} else if len(allowedClassIDs) > 0 {
		filter["class_id"] = bson.M{"$in": allowedClassIDs}
	}
	if req.Status != "" {
		filter["status"] = req.Status
	}
	if req.Keyword != "" {
		filter["$or"] = []bson.M{
			{"student_code": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"full_name": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"username": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"email": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"phone": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"parent_name": bson.M{"$regex": req.Keyword, "$options": "i"}},
			{"parent_phone": bson.M{"$regex": req.Keyword, "$options": "i"}},
		}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultStudentBatchSize
	}
	if limit > maxStudentBatchSize {
		limit = maxStudentBatchSize
	}
	offset := cursorOffset(req.Cursor)
	total, err := r.students.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, "", err
	}
	cur, err := r.students.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, "", err
	}
	defer cur.Close(ctx)
	var items []Student
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, "", err
	}
	next := ""
	if offset+int64(len(items)) < total {
		next = strconv.FormatInt(offset+int64(len(items)), 10)
	}
	return items, total, next, nil
}

func (r *Repository) UpdateStudent(ctx context.Context, tenantID, id string, update bson.M) (*Student, error) {
	if r.memory != nil {
		return r.memory.updateStudent(tenantID, id, update)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	update["updated_at"] = time.Now().UTC()
	_, err = r.students.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.GetStudent(ctx, tenantID, id)
}

func (r *Repository) SetStudentClass(ctx context.Context, tenantID, studentID string, targetClass *Class) error {
	if r.memory != nil {
		return r.memory.setStudentClass(tenantID, studentID, targetClass)
	}
	oid, err := objectID(studentID)
	if err != nil {
		return err
	}
	_, err = r.students.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": bson.M{
		"center_id":  targetClass.CenterID,
		"class_id":   targetClass.ID,
		"updated_at": time.Now().UTC(),
	}})
	return err
}

func (r *Repository) GetCurrentScope(ctx context.Context, tenantID, userID string) (*CurrentScope, error) {
	if r.memory != nil {
		return r.memory.getCurrentScope(tenantID, userID), nil
	}
	var scope CurrentScope
	err := r.currentScopes.FindOne(ctx, bson.M{"tenant_id": tenantID, "user_id": userID}).Decode(&scope)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scope, nil
}

func (r *Repository) UpsertCurrentScope(ctx context.Context, scope *CurrentScope) error {
	if r.memory != nil {
		return r.memory.upsertCurrentScope(scope)
	}
	_, err := r.currentScopes.UpdateOne(ctx,
		bson.M{"tenant_id": scope.TenantID, "user_id": scope.UserID},
		bson.M{"$set": bson.M{
			"level":      scope.Level,
			"center_id":  scope.CenterID,
			"class_id":   scope.ClassID,
			"updated_at": time.Now().UTC(),
		}},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (r *Repository) CreateImportPreview(ctx context.Context, preview *ImportPreview) error {
	preview.CreatedAt = time.Now().UTC()
	res, err := r.previews.InsertOne(ctx, preview)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		preview.ID = oid
	}
	return nil
}

func (r *Repository) GetImportPreview(ctx context.Context, tenantID, previewID string) (*ImportPreview, error) {
	oid, err := objectID(previewID)
	if err != nil {
		return nil, err
	}
	var preview ImportPreview
	err = r.previews.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&preview)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &preview, nil
}

func (r *Repository) CreateImportJob(ctx context.Context, job *ImportJob) error {
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	res, err := r.importJobs.InsertOne(ctx, job)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		job.ID = oid
	}
	return nil
}

func (r *Repository) GetImportJob(ctx context.Context, tenantID, jobID string) (*ImportJob, error) {
	oid, err := objectID(jobID)
	if err != nil {
		return nil, err
	}
	var job ImportJob
	err = r.importJobs.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&job)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *Repository) ListSubjects(ctx context.Context, tenantID, scopeType, centerID string) ([]Subject, error) {
	if r.memory != nil {
		return r.memory.listSubjects(tenantID, scopeType, centerID)
	}
	filter := bson.M{"tenant_id": tenantID, "status": bson.M{"$ne": statusArchived}}
	if scopeType != "" {
		filter["scope.type"] = scopeType
	}
	if centerID != "" {
		filter["scope.center_id"] = centerID
	}
	cur, err := r.subjects.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Subject
	err = cur.All(ctx, &items)
	return items, err
}

func (r *Repository) ListLevels(ctx context.Context, tenantID, subjectID string) ([]Level, error) {
	if r.memory != nil {
		return r.memory.listLevels(tenantID, subjectID)
	}
	oid, err := objectID(subjectID)
	if err != nil {
		return nil, err
	}
	cur, err := r.levels.Find(ctx, bson.M{"tenant_id": tenantID, "subject_id": oid, "status": bson.M{"$ne": statusArchived}}, options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Level
	err = cur.All(ctx, &items)
	return items, err
}

func (r *Repository) ListTopics(ctx context.Context, tenantID, levelID string) ([]Topic, error) {
	if r.memory != nil {
		return r.memory.listTopics(tenantID, levelID)
	}
	oid, err := objectID(levelID)
	if err != nil {
		return nil, err
	}
	cur, err := r.topics.Find(ctx, bson.M{"tenant_id": tenantID, "level_id": oid, "status": bson.M{"$ne": statusArchived}}, options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Topic
	err = cur.All(ctx, &items)
	return items, err
}

func (r *Repository) CreateQuestion(ctx context.Context, q *Question) error {
	if r.memory != nil {
		return r.memory.createQuestion(q)
	}
	now := time.Now().UTC()
	q.CreatedAt = now
	q.UpdatedAt = now
	if q.Status == "" {
		q.Status = statusActive
	}
	res, err := r.questions.InsertOne(ctx, q)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		q.ID = oid
	}
	return nil
}

func (r *Repository) GetQuestion(ctx context.Context, tenantID, id string) (*Question, error) {
	if r.memory != nil {
		return r.memory.getQuestion(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var q Question
	err = r.questions.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&q)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &q, err
}

func (r *Repository) ListQuestions(ctx context.Context, tenantID string, filter bson.M, cursor string, limit int64) ([]Question, int64, string, error) {
	if r.memory != nil {
		return r.memory.listQuestions(tenantID, filter, cursor, limit)
	}
	filter["tenant_id"] = tenantID
	filter["status"] = bson.M{"$ne": statusArchived}
	if limit <= 0 || limit > maxStudentBatchSize {
		limit = defaultStudentBatchSize
	}
	offset := cursorOffset(cursor)
	total, err := r.questions.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, "", err
	}
	cur, err := r.questions.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, "", err
	}
	defer cur.Close(ctx)
	var items []Question
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, "", err
	}
	next := ""
	if offset+int64(len(items)) < total {
		next = strconv.FormatInt(offset+int64(len(items)), 10)
	}
	return items, total, next, nil
}

func (r *Repository) UpdateQuestion(ctx context.Context, tenantID, id string, update bson.M) (*Question, error) {
	if r.memory != nil {
		return r.memory.updateQuestion(tenantID, id, update)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	update["updated_at"] = time.Now().UTC()
	_, err = r.questions.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.GetQuestion(ctx, tenantID, id)
}

func (r *Repository) RandomQuestions(ctx context.Context, tenantID string, filter bson.M, limit int64, seed int64) ([]Question, error) {
	if r.memory != nil {
		return r.memory.randomQuestions(tenantID, filter, limit)
	}
	filter["tenant_id"] = tenantID
	filter["status"] = bson.M{"$ne": statusArchived}
	cur, err := r.questions.Aggregate(ctx, []bson.M{{"$match": filter}, {"$sample": bson.M{"size": limit}}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Question
	err = cur.All(ctx, &items)
	return items, err
}

func (r *Repository) CreateQuiz(ctx context.Context, q *Quiz) error {
	if r.memory != nil {
		return r.memory.createQuiz(q)
	}
	now := time.Now().UTC()
	q.CreatedAt = now
	q.UpdatedAt = now
	if q.Status == "" {
		q.Status = "draft"
	}
	res, err := r.quizzes.InsertOne(ctx, q)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		q.ID = oid
	}
	return nil
}

func (r *Repository) GetQuiz(ctx context.Context, tenantID, id string) (*Quiz, error) {
	if r.memory != nil {
		return r.memory.getQuiz(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var q Quiz
	err = r.quizzes.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&q)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &q, err
}

func (r *Repository) ListQuizzes(ctx context.Context, tenantID string, filter bson.M, cursor string, limit int64) ([]Quiz, int64, string, error) {
	if r.memory != nil {
		return r.memory.listQuizzes(tenantID, filter, cursor, limit)
	}
	filter["tenant_id"] = tenantID
	if limit <= 0 || limit > maxStudentBatchSize {
		limit = defaultStudentBatchSize
	}
	offset := cursorOffset(cursor)
	total, err := r.quizzes.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, "", err
	}
	cur, err := r.quizzes.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, "", err
	}
	defer cur.Close(ctx)
	var items []Quiz
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, "", err
	}
	next := ""
	if offset+int64(len(items)) < total {
		next = strconv.FormatInt(offset+int64(len(items)), 10)
	}
	return items, total, next, nil
}

func (r *Repository) UpdateQuiz(ctx context.Context, tenantID, id string, update bson.M) (*Quiz, error) {
	if r.memory != nil {
		return r.memory.updateQuiz(tenantID, id, update)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	update["updated_at"] = time.Now().UTC()
	_, err = r.quizzes.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.GetQuiz(ctx, tenantID, id)
}

func (r *Repository) CreateAssignment(ctx context.Context, a *Assignment) error {
	if r.memory != nil {
		return r.memory.createAssignment(a)
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "open"
	}
	res, err := r.assignments.InsertOne(ctx, a)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		a.ID = oid
	}
	return nil
}

func (r *Repository) GetAssignment(ctx context.Context, tenantID, id string) (*Assignment, error) {
	if r.memory != nil {
		return r.memory.getAssignment(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var a Assignment
	err = r.assignments.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&a)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &a, err
}

func (r *Repository) ListAssignments(ctx context.Context, tenantID string, filter bson.M) ([]Assignment, error) {
	if r.memory != nil {
		return r.memory.listAssignments(tenantID, filter)
	}
	filter["tenant_id"] = tenantID
	cur, err := r.assignments.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Assignment
	err = cur.All(ctx, &items)
	return items, err
}

func (r *Repository) CreateAttempt(ctx context.Context, a *Attempt) error {
	if r.memory != nil {
		return r.memory.createAttempt(a)
	}
	now := time.Now().UTC()
	a.StartedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "in_progress"
	}
	if a.Answers == nil {
		a.Answers = map[string]AttemptAnswer{}
	}
	res, err := r.attempts.InsertOne(ctx, a)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		a.ID = oid
	}
	return nil
}

func (r *Repository) EnsureIndexes(ctx context.Context) error {
	attemptIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "assignment_id", Value: 1},
				{Key: "quiz_id", Value: 1},
				{Key: "student_id", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().
				SetName("idx_lms_attempt_active_student").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"status": "in_progress"}),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "assignment_id", Value: 1},
				{Key: "updated_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_attempt_assignment_updated"),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "quiz_id", Value: 1},
				{Key: "assignment_id", Value: 1},
				{Key: "student_id", Value: 1},
			},
			Options: options.Index().SetName("idx_lms_attempt_quiz_progress"),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "student_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "updated_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_attempt_student_scores"),
		},
	}
	assignmentIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "student_ids", Value: 1},
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_assignment_student_status"),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "quiz_id", Value: 1},
				{Key: "class_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_assignment_quiz_class"),
		},
	}
	quizIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "subject_id", Value: 1},
				{Key: "level_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_quiz_catalog"),
		},
	}
	classIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "center_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetName("idx_lms_class_center_status"),
		},
	}
	studentIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "username", Value: 1},
			},
			Options: options.Index().
				SetName("idx_lms_student_unique_username").
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "class_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "full_name", Value: 1},
			},
			Options: options.Index().SetName("idx_lms_student_class_status"),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "student_code", Value: 1},
			},
			Options: options.Index().
				SetName("idx_lms_student_unique_student_code").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"student_code": bson.M{"$exists": true, "$ne": ""}}),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "center_id", Value: 1},
				{Key: "class_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "student_code", Value: 1},
			},
			Options: options.Index().SetName("idx_lms_student_roster_export"),
		},
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "auth_user_id", Value: 1},
			},
			Options: options.Index().
				SetName("idx_lms_student_unique_auth_user").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"auth_user_id": bson.M{"$exists": true, "$ne": ""}}),
		},
	}
	questionIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "subject_id", Value: 1},
				{Key: "level_id", Value: 1},
				{Key: "topic_id", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("idx_lms_question_catalog"),
		},
	}
	threadIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "class_id", Value: 1},
				{Key: "assignment_id", Value: 1},
				{Key: "updated_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_thread_class_assignment"),
		},
	}
	replyIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "thread_id", Value: 1},
				{Key: "created_at", Value: 1},
			},
			Options: options.Index().SetName("idx_lms_reply_thread_created"),
		},
	}
	announcementIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "class_ids", Value: 1},
				{Key: "target_type", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_lms_announcement_class_target"),
		},
	}
	currentScopeIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().
				SetName("idx_lms_current_scope_unique_user").
				SetUnique(true),
		},
	}
	if _, err := r.attempts.Indexes().CreateMany(ctx, attemptIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.attempts: %w", err)
	}
	if _, err := r.assignments.Indexes().CreateMany(ctx, assignmentIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.assignments: %w", err)
	}
	if _, err := r.quizzes.Indexes().CreateMany(ctx, quizIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.quizzes: %w", err)
	}
	if _, err := r.classes.Indexes().CreateMany(ctx, classIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.classes: %w", err)
	}
	if _, err := r.students.Indexes().CreateMany(ctx, studentIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.students: %w", err)
	}
	if _, err := r.questions.Indexes().CreateMany(ctx, questionIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.questions: %w", err)
	}
	if _, err := r.threads.Indexes().CreateMany(ctx, threadIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.threads: %w", err)
	}
	if _, err := r.replies.Indexes().CreateMany(ctx, replyIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.replies: %w", err)
	}
	if _, err := r.announcements.Indexes().CreateMany(ctx, announcementIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.announcements: %w", err)
	}
	if _, err := r.currentScopes.Indexes().CreateMany(ctx, currentScopeIndexes); err != nil {
		return fmt.Errorf("lms.ensureIndexes.currentScopes: %w", err)
	}
	return nil
}

func (r *Repository) GetAttempt(ctx context.Context, tenantID, id string) (*Attempt, error) {
	if r.memory != nil {
		return r.memory.getAttempt(tenantID, id)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var a Attempt
	err = r.attempts.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&a)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &a, err
}

func (r *Repository) FindActiveAttempt(ctx context.Context, tenantID string, assignmentID, quizID, studentID bson.ObjectID) (*Attempt, error) {
	if r.memory != nil {
		return r.memory.findActiveAttempt(tenantID, assignmentID, quizID, studentID)
	}
	var a Attempt
	err := r.attempts.FindOne(ctx, bson.M{"tenant_id": tenantID, "assignment_id": assignmentID, "quiz_id": quizID, "student_id": studentID, "status": "in_progress"}).Decode(&a)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &a, err
}

func (r *Repository) SaveAttemptAnswer(ctx context.Context, tenantID, id, questionID string, answer AttemptAnswer) (*Attempt, error) {
	field, err := answerField(questionID)
	if err != nil {
		return nil, err
	}
	if r.memory != nil {
		return r.memory.saveAttemptAnswer(tenantID, id, questionID, answer)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res, err := r.attempts.UpdateOne(
		ctx,
		bson.M{"_id": oid, "tenant_id": tenantID, "status": bson.M{"$ne": "submitted"}},
		bson.M{"$set": bson.M{field: answer, "updated_at": now}},
	)
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return r.GetAttempt(ctx, tenantID, id)
	}
	return r.GetAttempt(ctx, tenantID, id)
}

func (r *Repository) SubmitAttempt(ctx context.Context, tenantID, id string, answers map[string]AttemptAnswer, submittedAt time.Time) (*Attempt, bool, error) {
	if r.memory != nil {
		return r.memory.submitAttempt(tenantID, id, answers, submittedAt)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, false, err
	}
	now := time.Now().UTC()
	set := bson.M{
		"status":       "submitted",
		"submitted_at": submittedAt,
		"updated_at":   now,
	}
	for questionID, answer := range answers {
		field, err := answerField(questionID)
		if err != nil {
			return nil, false, err
		}
		set[field] = answer
	}
	res, err := r.attempts.UpdateOne(
		ctx,
		bson.M{"_id": oid, "tenant_id": tenantID, "status": bson.M{"$ne": "submitted"}},
		bson.M{"$set": set},
	)
	if err != nil {
		return nil, false, err
	}
	attempt, err := r.GetAttempt(ctx, tenantID, id)
	return attempt, res.MatchedCount > 0, err
}

func (r *Repository) UpdateAttemptScore(ctx context.Context, tenantID, id string, score, maxScore, percent float64, passed bool) (*Attempt, error) {
	return r.UpdateAttempt(ctx, tenantID, id, bson.M{
		"score":     score,
		"max_score": maxScore,
		"percent":   percent,
		"passed":    passed,
	})
}

func (r *Repository) UpdateAttempt(ctx context.Context, tenantID, id string, update bson.M) (*Attempt, error) {
	if r.memory != nil {
		return r.memory.updateAttempt(tenantID, id, update)
	}
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	update["updated_at"] = time.Now().UTC()
	_, err = r.attempts.UpdateOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.GetAttempt(ctx, tenantID, id)
}

func answerField(questionID string) (string, error) {
	questionID = strings.TrimSpace(questionID)
	if questionID == "" || strings.Contains(questionID, ".") || strings.HasPrefix(questionID, "$") {
		return "", errInvalidFieldKey
	}
	return "answers." + questionID, nil
}

func (r *Repository) ListAttempts(ctx context.Context, tenantID string, filter bson.M) ([]Attempt, error) {
	if r.memory != nil {
		return r.memory.listAttempts(tenantID, filter)
	}
	filter["tenant_id"] = tenantID
	cur, err := r.attempts.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Attempt
	err = cur.All(ctx, &items)
	return items, err
}

func (r *Repository) CreateDiscussionThread(ctx context.Context, thread *DiscussionThread) error {
	now := time.Now().UTC()
	thread.CreatedAt = now
	thread.UpdatedAt = now
	thread.LatestActivityAt = now
	res, err := r.threads.InsertOne(ctx, thread)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		thread.ID = oid
	}
	return nil
}

func (r *Repository) ListDiscussionThreads(ctx context.Context, tenantID, classID, cursor string, limit int64) ([]DiscussionThread, int64, string, error) {
	classOID, err := objectID(classID)
	if err != nil {
		return nil, 0, "", err
	}
	filter := bson.M{"tenant_id": tenantID, "class_id": classOID}
	if limit <= 0 || limit > maxStudentBatchSize {
		limit = defaultStudentBatchSize
	}
	offset := cursorOffset(cursor)
	total, err := r.threads.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, "", err
	}
	cur, err := r.threads.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "latest_activity_at", Value: -1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, "", err
	}
	defer cur.Close(ctx)
	var items []DiscussionThread
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, "", err
	}
	next := ""
	if offset+int64(len(items)) < total {
		next = strconv.FormatInt(offset+int64(len(items)), 10)
	}
	return items, total, next, nil
}

func (r *Repository) GetDiscussionThread(ctx context.Context, tenantID, id string) (*DiscussionThread, error) {
	oid, err := objectID(id)
	if err != nil {
		return nil, err
	}
	var thread DiscussionThread
	err = r.threads.FindOne(ctx, bson.M{"_id": oid, "tenant_id": tenantID}).Decode(&thread)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &thread, err
}

func (r *Repository) CreateDiscussionReply(ctx context.Context, reply *DiscussionReply) error {
	reply.CreatedAt = time.Now().UTC()
	res, err := r.replies.InsertOne(ctx, reply)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		reply.ID = oid
	}
	_, _ = r.threads.UpdateOne(ctx, bson.M{"_id": reply.ThreadID, "tenant_id": reply.TenantID}, bson.M{"$inc": bson.M{"reply_count": 1}, "$set": bson.M{"latest_activity_at": reply.CreatedAt, "updated_at": reply.CreatedAt}})
	return nil
}

func (r *Repository) CreateDiscussionAttachment(ctx context.Context, item *DiscussionAttachment) error {
	item.CreatedAt = time.Now().UTC()
	res, err := r.attachments.InsertOne(ctx, item)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		item.ID = oid
	}
	return nil
}

func (r *Repository) CreateAnnouncement(ctx context.Context, item *Announcement) error {
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
	res, err := r.announcements.InsertOne(ctx, item)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		item.ID = oid
	}
	return nil
}

func (r *Repository) ListAnnouncements(ctx context.Context, tenantID string, filter bson.M, cursor string, limit int64) ([]Announcement, int64, string, error) {
	filter["tenant_id"] = tenantID
	if limit <= 0 || limit > maxStudentBatchSize {
		limit = defaultStudentBatchSize
	}
	offset := cursorOffset(cursor)
	total, err := r.announcements.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, "", err
	}
	cur, err := r.announcements.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "pinned", Value: -1}, {Key: "created_at", Value: -1}}).SetSkip(offset).SetLimit(limit))
	if err != nil {
		return nil, 0, "", err
	}
	defer cur.Close(ctx)
	var items []Announcement
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, "", err
	}
	next := ""
	if offset+int64(len(items)) < total {
		next = strconv.FormatInt(offset+int64(len(items)), 10)
	}
	return items, total, next, nil
}

func (r *Repository) CreateInternalDocument(ctx context.Context, doc *InternalDocument) error {
	now := time.Now().UTC()
	doc.CreatedAt = now
	doc.UpdatedAt = now
	res, err := r.internalDocs.InsertOne(ctx, doc)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		doc.ID = oid
	}
	return nil
}

func (r *Repository) ListInternalDocuments(ctx context.Context, tenantID string, filter bson.M) ([]InternalDocument, int64, error) {
	filter["tenant_id"] = tenantID
	total, err := r.internalDocs.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.internalDocs.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var items []InternalDocument
	err = cur.All(ctx, &items)
	return items, total, err
}
