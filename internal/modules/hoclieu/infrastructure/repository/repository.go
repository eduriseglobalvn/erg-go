package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	. "erg.ninja/internal/modules/hoclieu/api/dto"
	hoclieuservice "erg.ninja/internal/modules/hoclieu/application/service"
	"erg.ninja/pkg/database"
)

const (
	collectionTaxonomy       = "hoclieu_taxonomy"
	collectionPrograms       = "hoclieu_programs"
	collectionDesigner       = "hoclieu_designer_presets"
	collectionResources      = "hoclieu_resources"
	collectionAssets         = "hoclieu_assets"
	collectionResourceItems  = "hoclieu_resource_items"
	collectionProgressEvents = "hoclieu_teacher_progress_events"
)

type Repository struct {
	taxonomy       *mongo.Collection
	programs       *mongo.Collection
	presets        *mongo.Collection
	resources      *mongo.Collection
	assets         *mongo.Collection
	items          *mongo.Collection
	progressEvents *mongo.Collection
}

type resourceDocument struct {
	ID               string            `bson:"_id"`
	TenantID         string            `bson:"tenant_id"`
	ProgramSlug      string            `bson:"program_slug"`
	SubjectID        string            `bson:"subject_id"`
	GradeID          string            `bson:"grade_id,omitempty"`
	CategoryID       string            `bson:"category_id"`
	SectionID        string            `bson:"section_id,omitempty"`
	BookSeriesID     string            `bson:"book_series_id,omitempty"`
	TopicID          string            `bson:"topic_id,omitempty"`
	LevelID          string            `bson:"level_id,omitempty"`
	DocumentTypeID   string            `bson:"document_type_id,omitempty"`
	SelectedFileType AssetFileType     `bson:"selected_file_type"`
	Visibility       string            `bson:"visibility"`
	Status           string            `bson:"status"`
	Title            string            `bson:"title"`
	Subtitle         string            `bson:"subtitle,omitempty"`
	Description      string            `bson:"description,omitempty"`
	Tags             []string          `bson:"tags,omitempty"`
	Detail           ResourceDetailDTO `bson:"detail"`
	CreatedAt        time.Time         `bson:"created_at"`
	UpdatedAt        time.Time         `bson:"updated_at"`
}

type taxonomyDocument struct {
	ID        string            `bson:"_id"`
	TenantID  string            `bson:"tenant_id"`
	Kind      string            `bson:"kind"`
	Option    TaxonomyOptionDTO `bson:"option"`
	CreatedAt time.Time         `bson:"created_at"`
	UpdatedAt time.Time         `bson:"updated_at"`
}

type programDocument struct {
	ID        string     `bson:"_id"`
	TenantID  string     `bson:"tenant_id"`
	Program   ProgramDTO `bson:"program"`
	Slug      string     `bson:"slug"`
	CreatedAt time.Time  `bson:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at"`
}

type designerPresetDocument struct {
	ID        string                   `bson:"_id"`
	TenantID  string                   `bson:"tenant_id"`
	Preset    LectureDesignerPresetDTO `bson:"preset"`
	CreatedAt time.Time                `bson:"created_at"`
	UpdatedAt time.Time                `bson:"updated_at"`
}

type assetDocument struct {
	ID          string    `bson:"_id"`
	TenantID    string    `bson:"tenant_id"`
	ResourceID  string    `bson:"resource_id"`
	Asset       AssetDTO  `bson:"asset"`
	UpstreamURL string    `bson:"upstream_url,omitempty"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

type itemDocument struct {
	ID         string          `bson:"_id"`
	TenantID   string          `bson:"tenant_id"`
	ResourceID string          `bson:"resource_id"`
	Item       ResourceItemDTO `bson:"item"`
	CreatedAt  time.Time       `bson:"created_at"`
	UpdatedAt  time.Time       `bson:"updated_at"`
}

type progressEventDocument struct {
	ID           string                   `bson:"_id"`
	TenantID     string                   `bson:"tenant_id"`
	TeacherID    string                   `bson:"teacher_id"`
	SchoolID     string                   `bson:"school_id"`
	AcademicYear string                   `bson:"academic_year"`
	SubjectID    string                   `bson:"subject_id"`
	NodeID       string                   `bson:"node_id"`
	NodeKind     TeacherDashboardNodeKind `bson:"node_kind"`
	EventType    TeacherProgressEventType `bson:"event_type"`
	ResourceID   string                   `bson:"resource_id,omitempty"`
	OccurredAt   time.Time                `bson:"occurred_at"`
	CreatedAt    time.Time                `bson:"created_at"`
}

func NewRepository(mongoClient *database.MongoClient) *Repository {
	return &Repository{
		taxonomy:       mongoClient.Collection(collectionTaxonomy),
		programs:       mongoClient.Collection(collectionPrograms),
		presets:        mongoClient.Collection(collectionDesigner),
		resources:      mongoClient.Collection(collectionResources),
		assets:         mongoClient.Collection(collectionAssets),
		items:          mongoClient.Collection(collectionResourceItems),
		progressEvents: mongoClient.Collection(collectionProgressEvents),
	}
}

func (r *Repository) EnsureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "kind", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "kind", Value: 1}, {Key: "option.parentid", Value: 1}, {Key: "option.sortorder", Value: 1}}},
	}
	if _, err := r.taxonomy.Indexes().CreateMany(ctx, models); err != nil {
		return err
	}
	programIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	if _, err := r.programs.Indexes().CreateMany(ctx, programIndexes); err != nil {
		return err
	}
	presetIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	if _, err := r.presets.Indexes().CreateMany(ctx, presetIndexes); err != nil {
		return err
	}
	resourceIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}, {Key: "grade_id", Value: 1}, {Key: "subject_id", Value: 1}, {Key: "category_id", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "program_slug", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "updated_at", Value: -1}}},
	}
	if _, err := r.resources.Indexes().CreateMany(ctx, resourceIndexes); err != nil {
		return err
	}
	assetIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "resource_id", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	if _, err := r.assets.Indexes().CreateMany(ctx, assetIndexes); err != nil {
		return err
	}
	itemIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "resource_id", Value: 1}, {Key: "item.sortorder", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	if _, err := r.items.Indexes().CreateMany(ctx, itemIndexes); err != nil {
		return err
	}
	progressEventIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "school_id", Value: 1}, {Key: "academic_year", Value: 1}, {Key: "subject_id", Value: 1}, {Key: "occurred_at", Value: -1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "school_id", Value: 1}, {Key: "academic_year", Value: 1}, {Key: "subject_id", Value: 1}, {Key: "node_id", Value: 1}, {Key: "event_type", Value: 1}, {Key: "occurred_at", Value: -1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	_, err := r.progressEvents.Indexes().CreateMany(ctx, progressEventIndexes)
	return err
}

func (r *Repository) UpsertProgram(ctx context.Context, tenantID string, program ProgramDTO) error {
	now := time.Now().UTC()
	doc := programDocument{ID: program.ID, TenantID: tenantID, Program: program, Slug: program.Slug, CreatedAt: now, UpdatedAt: now}
	update := bson.M{
		"$set": bson.M{
			"tenant_id":  doc.TenantID,
			"program":    doc.Program,
			"slug":       doc.Slug,
			"updated_at": doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{"created_at": doc.CreatedAt},
	}
	_, err := r.programs.UpdateOne(ctx, bson.M{"_id": doc.ID, "tenant_id": tenantID}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Repository) ListPrograms(ctx context.Context, tenantID string) ([]ProgramDTO, error) {
	cur, err := r.programs.Find(ctx, bson.M{"tenant_id": tenantID}, options.Find().SetSort(bson.D{{Key: "slug", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []programDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]ProgramDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.Program)
	}
	return out, nil
}

func (r *Repository) UpsertDesignerPreset(ctx context.Context, tenantID string, preset LectureDesignerPresetDTO) error {
	now := time.Now().UTC()
	doc := designerPresetDocument{ID: preset.ID, TenantID: tenantID, Preset: preset, CreatedAt: now, UpdatedAt: now}
	update := bson.M{
		"$set": bson.M{
			"tenant_id":  doc.TenantID,
			"preset":     doc.Preset,
			"updated_at": doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{"created_at": doc.CreatedAt},
	}
	_, err := r.presets.UpdateOne(ctx, bson.M{"_id": doc.ID, "tenant_id": tenantID}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Repository) ListDesignerPresets(ctx context.Context, tenantID string) ([]LectureDesignerPresetDTO, error) {
	cur, err := r.presets.Find(ctx, bson.M{"tenant_id": tenantID}, options.Find().SetSort(bson.D{{Key: "preset.name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []designerPresetDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]LectureDesignerPresetDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.Preset)
	}
	return out, nil
}

func (r *Repository) UpsertTaxonomy(ctx context.Context, tenantID, kind string, option TaxonomyOptionDTO) error {
	now := time.Now().UTC()
	doc := taxonomyDocument{ID: taxonomyDocumentID(kind, option.ID), TenantID: tenantID, Kind: kind, Option: option, CreatedAt: now, UpdatedAt: now}
	update := bson.M{
		"$set": bson.M{
			"tenant_id":  doc.TenantID,
			"kind":       doc.Kind,
			"option":     doc.Option,
			"updated_at": doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{"_id": doc.ID, "created_at": doc.CreatedAt},
	}
	_, err := r.taxonomy.UpdateOne(ctx, bson.M{"tenant_id": tenantID, "kind": kind, "option.id": option.ID}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Repository) DeleteTaxonomy(ctx context.Context, tenantID, kind, id string) error {
	_, err := r.taxonomy.DeleteOne(ctx, bson.M{"tenant_id": tenantID, "kind": kind, "option.id": id})
	return err
}

func taxonomyDocumentID(kind, id string) string {
	if kind == "" {
		return id
	}
	return kind + ":" + id
}

func (r *Repository) ListTaxonomy(ctx context.Context, tenantID string) (map[string][]TaxonomyOptionDTO, error) {
	cur, err := r.taxonomy.Find(ctx, bson.M{"tenant_id": tenantID}, options.Find().SetSort(bson.D{{Key: "kind", Value: 1}, {Key: "option.sortorder", Value: 1}, {Key: "option.label", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := map[string][]TaxonomyOptionDTO{}
	for cur.Next(ctx) {
		var doc taxonomyDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out[doc.Kind] = append(out[doc.Kind], doc.Option)
	}
	return out, cur.Err()
}

func (r *Repository) UpsertResource(ctx context.Context, tenantID string, detail *ResourceDetailDTO) error {
	now := time.Now().UTC()
	doc := resourceDocFromDetail(tenantID, detail, now)
	update := bson.M{
		"$set": bson.M{
			"tenant_id":          doc.TenantID,
			"program_slug":       doc.ProgramSlug,
			"subject_id":         doc.SubjectID,
			"grade_id":           doc.GradeID,
			"category_id":        doc.CategoryID,
			"section_id":         doc.SectionID,
			"book_series_id":     doc.BookSeriesID,
			"topic_id":           doc.TopicID,
			"level_id":           doc.LevelID,
			"document_type_id":   doc.DocumentTypeID,
			"selected_file_type": doc.SelectedFileType,
			"visibility":         doc.Visibility,
			"status":             doc.Status,
			"title":              doc.Title,
			"subtitle":           doc.Subtitle,
			"description":        doc.Description,
			"tags":               doc.Tags,
			"detail":             doc.Detail,
			"updated_at":         doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{"created_at": doc.CreatedAt},
	}
	_, err := r.resources.UpdateOne(ctx, bson.M{"_id": doc.ID, "tenant_id": tenantID}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Repository) ListResources(ctx context.Context, tenantID string, params hoclieuservice.ListResourceParams) ([]ResourceDetailDTO, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	filter := bson.M{"tenant_id": tenantID}
	if params.GradeID != "" {
		filter["grade_id"] = params.GradeID
	}
	if params.SubjectID != "" {
		filter["subject_id"] = params.SubjectID
	}
	if params.CategoryID != "" {
		filter["category_id"] = params.CategoryID
	}
	if params.SectionID != "" {
		filter["section_id"] = params.SectionID
	}
	if params.BookSeriesID != "" {
		filter["book_series_id"] = params.BookSeriesID
	}
	if params.TopicID != "" {
		filter["topic_id"] = params.TopicID
	}
	if params.LevelID != "" {
		filter["level_id"] = params.LevelID
	}
	if params.DocumentTypeID != "" {
		filter["document_type_id"] = params.DocumentTypeID
	}
	if params.ProgramSlug != "" {
		filter["program_slug"] = params.ProgramSlug
	}
	if params.FileType.Valid() {
		filter["selected_file_type"] = params.FileType
	}
	if params.Query != "" {
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": params.Query, "$options": "i"}},
			{"subtitle": bson.M{"$regex": params.Query, "$options": "i"}},
			{"description": bson.M{"$regex": params.Query, "$options": "i"}},
		}
	}
	total, err := r.resources.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	skip := int64((params.Page - 1) * params.Limit)
	cur, err := r.resources.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetSkip(skip).SetLimit(int64(params.Limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var docs []resourceDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	out := make([]ResourceDetailDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.Detail)
	}
	return out, total, nil
}

func (r *Repository) GetResource(ctx context.Context, tenantID, id string) (*ResourceDetailDTO, error) {
	var doc resourceDocument
	if err := r.resources.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, hoclieuservice.ErrNotFound
		}
		return nil, err
	}
	detail := doc.Detail
	items, err := r.ListResourceItems(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	detail.Items = items
	return &detail, nil
}

func (r *Repository) DeleteResource(ctx context.Context, tenantID, id string) error {
	result, err := r.resources.DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return hoclieuservice.ErrNotFound
	}
	if _, err := r.assets.DeleteMany(ctx, bson.M{"tenant_id": tenantID, "resource_id": id}); err != nil {
		return err
	}
	if _, err := r.items.DeleteMany(ctx, bson.M{"tenant_id": tenantID, "resource_id": id}); err != nil {
		return err
	}
	return nil
}

func (r *Repository) UpsertAsset(ctx context.Context, tenantID string, asset hoclieuservice.AssetRecord) error {
	now := time.Now().UTC()
	doc := assetDocument{ID: asset.ID, TenantID: tenantID, ResourceID: asset.ResourceID, Asset: asset.AssetDTO, UpstreamURL: asset.UpstreamURL, CreatedAt: now, UpdatedAt: now}
	update := bson.M{
		"$set": bson.M{
			"tenant_id":    doc.TenantID,
			"resource_id":  doc.ResourceID,
			"asset":        doc.Asset,
			"upstream_url": doc.UpstreamURL,
			"updated_at":   doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{"created_at": doc.CreatedAt},
	}
	_, err := r.assets.UpdateOne(ctx, bson.M{"_id": doc.ID, "tenant_id": tenantID}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Repository) GetAsset(ctx context.Context, tenantID, id string) (*hoclieuservice.AssetRecord, error) {
	var doc assetDocument
	if err := r.assets.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, hoclieuservice.ErrNotFound
		}
		return nil, err
	}
	return &hoclieuservice.AssetRecord{AssetDTO: doc.Asset, UpstreamURL: doc.UpstreamURL}, nil
}

func (r *Repository) ListAssets(ctx context.Context, tenantID string) ([]hoclieuservice.AssetRecord, error) {
	cur, err := r.assets.Find(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []assetDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]hoclieuservice.AssetRecord, 0, len(docs))
	for _, doc := range docs {
		out = append(out, hoclieuservice.AssetRecord{AssetDTO: doc.Asset, UpstreamURL: doc.UpstreamURL})
	}
	return out, nil
}

func (r *Repository) ReplaceResourceItems(ctx context.Context, tenantID, resourceID string, items []ResourceItemDTO) error {
	if _, err := r.items.DeleteMany(ctx, bson.M{"tenant_id": tenantID, "resource_id": resourceID}); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	now := time.Now().UTC()
	docs := make([]any, 0, len(items))
	for _, item := range items {
		id := item.ID
		if id == "" {
			id = database.NewID()
			item.ID = id
		}
		docs = append(docs, itemDocument{ID: id, TenantID: tenantID, ResourceID: resourceID, Item: item, CreatedAt: now, UpdatedAt: now})
	}
	_, err := r.items.InsertMany(ctx, docs)
	return err
}

func (r *Repository) ListResourceItems(ctx context.Context, tenantID, resourceID string) ([]ResourceItemDTO, error) {
	cur, err := r.items.Find(ctx, bson.M{"tenant_id": tenantID, "resource_id": resourceID}, options.Find().SetSort(bson.D{{Key: "item.sortorder", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []itemDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]ResourceItemDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.Item)
	}
	return out, nil
}

func resourceDocFromDetail(tenantID string, detail *ResourceDetailDTO, now time.Time) resourceDocument {
	cp := *detail
	cp.MetadataWarnings = append([]FileTypeWarningDTO(nil), detail.MetadataWarnings...)
	cp.Tags = append([]string(nil), detail.Tags...)
	cp.Assets = append([]AssetDTO(nil), detail.Assets...)
	cp.Items = append([]ResourceItemDTO(nil), detail.Items...)
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = now
	}
	return resourceDocument{
		ID:               cp.ID,
		TenantID:         tenantID,
		ProgramSlug:      cp.ProgramSlug,
		SubjectID:        cp.SubjectID,
		GradeID:          cp.GradeID,
		CategoryID:       cp.CategoryID,
		SectionID:        cp.SectionID,
		BookSeriesID:     cp.BookSeriesID,
		TopicID:          cp.TopicID,
		LevelID:          cp.LevelID,
		DocumentTypeID:   cp.DocumentTypeID,
		SelectedFileType: cp.SelectedFileType,
		Visibility:       cp.Visibility,
		Status:           cp.Status,
		Title:            cp.Title,
		Subtitle:         cp.Subtitle,
		Description:      cp.Description,
		Tags:             append([]string(nil), cp.Tags...),
		Detail:           cp,
		CreatedAt:        now,
		UpdatedAt:        cp.UpdatedAt,
	}
}

func (r *Repository) AppendProgressEvent(ctx context.Context, tenantID string, event TeacherProgressEventDTO) error {
	now := time.Now().UTC()
	doc := progressEventDocument{
		ID:           event.ID,
		TenantID:     tenantID,
		TeacherID:    event.TeacherID,
		SchoolID:     event.SchoolID,
		AcademicYear: event.AcademicYear,
		SubjectID:    event.SubjectID,
		NodeID:       event.NodeID,
		NodeKind:     event.NodeKind,
		EventType:    event.EventType,
		ResourceID:   event.ResourceID,
		OccurredAt:   event.OccurredAt,
		CreatedAt:    now,
	}
	if doc.ID == "" {
		doc.ID = database.NewID()
	}
	if doc.OccurredAt.IsZero() {
		doc.OccurredAt = now
	}
	_, err := r.progressEvents.InsertOne(ctx, doc)
	return err
}

func (r *Repository) ListProgressEvents(ctx context.Context, tenantID string, schoolID, academicYear, subjectID string) ([]TeacherProgressEventDTO, error) {
	filter := bson.M{"tenant_id": tenantID}
	if schoolID != "" {
		filter["school_id"] = schoolID
	}
	if academicYear != "" {
		filter["academic_year"] = academicYear
	}
	if subjectID != "" {
		filter["subject_id"] = subjectID
	}
	cur, err := r.progressEvents.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "occurred_at", Value: -1}, {Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []progressEventDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]TeacherProgressEventDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, TeacherProgressEventDTO{
			ID:           doc.ID,
			TeacherID:    doc.TeacherID,
			SchoolID:     doc.SchoolID,
			AcademicYear: doc.AcademicYear,
			SubjectID:    doc.SubjectID,
			NodeID:       doc.NodeID,
			NodeKind:     doc.NodeKind,
			EventType:    doc.EventType,
			ResourceID:   doc.ResourceID,
			OccurredAt:   doc.OccurredAt,
		})
	}
	return out, nil
}
