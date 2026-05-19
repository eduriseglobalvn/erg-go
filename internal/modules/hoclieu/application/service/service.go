package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	auditservice "erg.ninja/internal/modules/audit/application/service"
	. "erg.ninja/internal/modules/hoclieu/api/dto"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

var (
	ErrInvalidFileType      = errors.New("invalid selected file type")
	ErrInvalidAssetMetadata = errors.New("invalid asset metadata")
	ErrInvalidRequest       = errors.New("invalid hoclieu request")
	ErrNotFound             = errors.New("hoclieu entity not found")
)

type Service struct {
	mu              sync.RWMutex
	programs        []ProgramDTO
	grades          []TaxonomyOptionDTO
	subjects        []TaxonomyOptionDTO
	categories      []TaxonomyOptionDTO
	sections        []TaxonomyOptionDTO
	bookSeries      []TaxonomyOptionDTO
	topics          []TaxonomyOptionDTO
	designerPresets []LectureDesignerPresetDTO
	resources       map[string]*ResourceDetailDTO
	assets          map[string]*AssetRecord
	items           map[string][]ResourceItemDTO
	pages           map[string][]ViewerPageDTO
	progressEvents  []TeacherProgressEventDTO
	audit           auditservice.Publisher
	r2              *storage.R2Client
	repo            Repository
	defaultTenantID string
}

type AssetRecord struct {
	AssetDTO
	UpstreamURL string
}

type ListResourceParams struct {
	GradeID        string
	SubjectID      string
	CategoryID     string
	SectionID      string
	BookSeriesID   string
	TopicID        string
	LevelID        string
	DocumentTypeID string
	ProgramSlug    string
	FileType       AssetFileType
	Query          string
	Page           int
	Limit          int
}

type Repository interface {
	EnsureIndexes(ctx context.Context) error
	ListTaxonomy(ctx context.Context, tenantID string) (map[string][]TaxonomyOptionDTO, error)
	UpsertProgram(ctx context.Context, tenantID string, program ProgramDTO) error
	ListPrograms(ctx context.Context, tenantID string) ([]ProgramDTO, error)
	UpsertDesignerPreset(ctx context.Context, tenantID string, preset LectureDesignerPresetDTO) error
	ListDesignerPresets(ctx context.Context, tenantID string) ([]LectureDesignerPresetDTO, error)
	ListResources(ctx context.Context, tenantID string, params ListResourceParams) ([]ResourceDetailDTO, int64, error)
	ListAssets(ctx context.Context, tenantID string) ([]AssetRecord, error)
	GetResource(ctx context.Context, tenantID, id string) (*ResourceDetailDTO, error)
	ListResourceItems(ctx context.Context, tenantID, resourceID string) ([]ResourceItemDTO, error)
	UpsertResource(ctx context.Context, tenantID string, detail *ResourceDetailDTO) error
	UpsertAsset(ctx context.Context, tenantID string, asset AssetRecord) error
	ReplaceResourceItems(ctx context.Context, tenantID, resourceID string, items []ResourceItemDTO) error
	DeleteResource(ctx context.Context, tenantID, id string) error
	GetAsset(ctx context.Context, tenantID, id string) (*AssetRecord, error)
	UpsertTaxonomy(ctx context.Context, tenantID, kind string, option TaxonomyOptionDTO) error
	DeleteTaxonomy(ctx context.Context, tenantID, kind, id string) error
	AppendProgressEvent(ctx context.Context, tenantID string, event TeacherProgressEventDTO) error
	ListProgressEvents(ctx context.Context, tenantID string, schoolID, academicYear, subjectID string) ([]TeacherProgressEventDTO, error)
}

func NewService(r2 ...*storage.R2Client) *Service {
	s := &Service{
		resources:      map[string]*ResourceDetailDTO{},
		assets:         map[string]*AssetRecord{},
		items:          map[string][]ResourceItemDTO{},
		pages:          map[string][]ViewerPageDTO{},
		progressEvents: []TeacherProgressEventDTO{},
		audit:          auditservice.NoopPublisher{},
	}
	if len(r2) > 0 {
		s.r2 = r2[0]
	}
	return s
}

func (s *Service) UseRepository(repo Repository, defaultTenantID string) {
	s.repo = repo
	if strings.TrimSpace(defaultTenantID) == "" {
		defaultTenantID = "default"
	}
	s.defaultTenantID = defaultTenantID
}

func (s *Service) tenantID(ctx context.Context) string {
	if id := strings.TrimSpace(tenant.FromContext(ctx)); id != "" {
		return id
	}
	if strings.TrimSpace(s.defaultTenantID) != "" {
		return s.defaultTenantID
	}
	return "default"
}

func (s *Service) EnsurePersistentStore(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	if err := s.repo.EnsureIndexes(ctx); err != nil {
		return err
	}
	return s.LoadFromRepository(ctx)
}

func (s *Service) SeedDefaultContent(ctx context.Context) error {
	SeedService(s)
	if s.repo == nil {
		return nil
	}
	tenantID := s.tenantID(ctx)
	if err := s.purgeLegacyIC3GS6Seed(ctx, tenantID); err != nil {
		return err
	}
	s.mu.RLock()
	programs := append([]ProgramDTO{}, s.programs...)
	grades := append([]TaxonomyOptionDTO{}, s.grades...)
	subjects := append([]TaxonomyOptionDTO{}, s.subjects...)
	categories := append([]TaxonomyOptionDTO{}, s.categories...)
	sections := append([]TaxonomyOptionDTO{}, s.sections...)
	bookSeries := append([]TaxonomyOptionDTO{}, s.bookSeries...)
	topics := append([]TaxonomyOptionDTO{}, s.topics...)
	presets := append([]LectureDesignerPresetDTO{}, s.designerPresets...)
	resources := make([]*ResourceDetailDTO, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, cloneResource(resource))
	}
	assets := make([]AssetRecord, 0, len(s.assets))
	for _, asset := range s.assets {
		assets = append(assets, *asset)
	}
	items := make(map[string][]ResourceItemDTO, len(s.items))
	for resourceID, resourceItems := range s.items {
		items[resourceID] = append([]ResourceItemDTO{}, resourceItems...)
	}
	s.mu.RUnlock()

	for _, program := range programs {
		if err := s.repo.UpsertProgram(ctx, tenantID, program); err != nil {
			return err
		}
	}
	for kind, options := range map[string][]TaxonomyOptionDTO{
		"grades":      grades,
		"subjects":    subjects,
		"categories":  categories,
		"sections":    sections,
		"book-series": bookSeries,
		"topics":      topics,
	} {
		for _, option := range options {
			if err := s.repo.UpsertTaxonomy(ctx, tenantID, kind, option); err != nil {
				return err
			}
		}
	}
	for _, preset := range presets {
		if err := s.repo.UpsertDesignerPreset(ctx, tenantID, preset); err != nil {
			return err
		}
	}
	for _, resource := range resources {
		if err := s.repo.UpsertResource(ctx, tenantID, resource); err != nil {
			return err
		}
		if err := s.repo.ReplaceResourceItems(ctx, tenantID, resource.ID, items[resource.ID]); err != nil {
			return err
		}
	}
	for _, asset := range assets {
		if err := s.repo.UpsertAsset(ctx, tenantID, asset); err != nil {
			return err
		}
	}
	return s.LoadFromRepository(ctx)
}

func (s *Service) purgeLegacyIC3GS6Seed(ctx context.Context, tenantID string) error {
	taxonomy, err := s.repo.ListTaxonomy(ctx, tenantID)
	if err != nil {
		return err
	}

	validSectionIDs := map[string]bool{
		"ic3-gs6-level-1-lesson-1": true,
		"ic3-gs6-level-1-lesson-2": true,
		"ic3-gs6-level-2-lesson-1": true,
		"ic3-gs6-level-2-lesson-2": true,
		"ic3-gs6-level-3-lesson-1": true,
		"ic3-gs6-level-3-lesson-2": true,
	}

	shouldDeleteTaxonomy := func(kind string, option TaxonomyOptionDTO) bool {
		switch kind {
		case "topics":
			return option.SubjectID == "ic3-gs6" || strings.HasPrefix(option.ID, "ic3-gs6-topic-")
		case "sections":
			return option.SubjectID == "ic3-gs6" && !validSectionIDs[option.ID]
		default:
			return false
		}
	}

	for kind, options := range map[string][]TaxonomyOptionDTO{
		"topics":   taxonomy["topics"],
		"sections": taxonomy["sections"],
	} {
		for _, option := range options {
			if !shouldDeleteTaxonomy(kind, option) {
				continue
			}
			if err := s.repo.DeleteTaxonomy(ctx, tenantID, kind, option.ID); err != nil {
				return err
			}
		}
	}

	resources, _, err := s.repo.ListResources(ctx, tenantID, ListResourceParams{Page: 1, Limit: 200})
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.SubjectID != "ic3-gs6" {
			continue
		}
		if resource.TopicID != "" || resource.SectionID == "support-components" || !validSectionIDs[resource.SectionID] {
			if err := s.repo.DeleteResource(ctx, tenantID, resource.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) LoadFromRepository(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	tenantID := s.tenantID(ctx)
	taxonomy, err := s.repo.ListTaxonomy(ctx, tenantID)
	if err != nil {
		return err
	}
	programs, err := s.repo.ListPrograms(ctx, tenantID)
	if err != nil {
		return err
	}
	presets, err := s.repo.ListDesignerPresets(ctx, tenantID)
	if err != nil {
		return err
	}
	resources, _, err := s.repo.ListResources(ctx, tenantID, ListResourceParams{Page: 1, Limit: 100})
	if err != nil {
		return err
	}
	assets, err := s.repo.ListAssets(ctx, tenantID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.programs = programs
	s.designerPresets = presets
	s.grades = taxonomy["grades"]
	s.subjects = taxonomy["subjects"]
	s.categories = taxonomy["categories"]
	s.sections = taxonomy["sections"]
	s.bookSeries = taxonomy["book-series"]
	s.topics = taxonomy["topics"]
	s.resources = map[string]*ResourceDetailDTO{}
	s.items = map[string][]ResourceItemDTO{}
	for i := range resources {
		resource := resources[i]
		cp := cloneResource(&resource)
		s.resources[cp.ID] = cp
		s.items[cp.ID] = append([]ResourceItemDTO(nil), cp.Items...)
	}
	s.assets = map[string]*AssetRecord{}
	for i := range assets {
		asset := assets[i]
		s.assets[asset.ID] = &asset
	}
	return nil
}

func (s *Service) Home(context.Context) HomeResponseDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return HomeResponseDTO{
		Hero: HomeHeroDTO{
			Title:        "hoclieu.erg.edu.vn",
			Description:  "Cong hoc lieu noi bo giup giao vien ERG mo nhanh chuong trinh, hoc lieu va viewer dung dinh dang trong tiet day.",
			PrimaryCTA:   "Mo kho hoc lieu",
			SecondaryCTA: "Xem chuong trinh",
		},
		Metrics: []HomeMetricDTO{
			{Label: "Chuong trinh", Value: fmt.Sprintf("%02d", len(s.programs))},
			{Label: "Nhom tai nguyen", Value: fmt.Sprintf("%d+", len(s.categories))},
			{Label: "Dinh dang", Value: fmt.Sprintf("%d", len(fileTypes()))},
		},
		Programs: append([]ProgramDTO(nil), s.programs...),
		Shortcuts: []HomeShortcutDTO{
			{ID: "programs", Title: "Chuong trinh", Description: "Global Success, IC3, MOS, Tin hoc va STEM.", Href: "/chuong-trinh"},
			{ID: "library", Title: "Kho hoc lieu", Description: "Loc theo lop, mon, nhom tai nguyen va loai file.", Href: "/kho-hoc-lieu"},
			{ID: "viewer", Title: "Viewer", Description: "PDF, PPTX, video, audio va quiz co contract launch rieng.", Href: "/kho-hoc-lieu"},
		},
	}
}

func (s *Service) Programs(context.Context) []ProgramDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ProgramDTO(nil), s.programs...)
}

func (s *Service) Program(_ context.Context, slug string) (*ProgramDTO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.programs {
		if p.Slug == slug {
			cp := p
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (s *Service) Taxonomy(context.Context) TaxonomyResponseDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.publicTaxonomyResponseLocked()
}

func (s *Service) ListResources(ctx context.Context, params ListResourceParams) ([]ResourceCardDTO, int64) {
	if s.repo != nil {
		resources, _, err := s.repo.ListResources(ctx, s.tenantID(ctx), params)
		if err == nil {
			items := make([]ResourceCardDTO, 0, len(resources))
			for i := range resources {
				if !isPublicSubjectID(resources[i].SubjectID) {
					continue
				}
				items = append(items, resources[i].ResourceCardDTO)
			}
			return items, int64(len(items))
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	query := strings.ToLower(strings.TrimSpace(params.Query))
	items := make([]ResourceCardDTO, 0, len(s.resources))
	for _, resource := range s.resources {
		if !isPublicSubjectID(resource.SubjectID) {
			continue
		}
		if params.GradeID != "" && resource.GradeID != params.GradeID {
			continue
		}
		if params.SubjectID != "" && resource.SubjectID != params.SubjectID {
			continue
		}
		if params.CategoryID != "" && resource.CategoryID != params.CategoryID {
			continue
		}
		if params.SectionID != "" && resource.SectionID != params.SectionID {
			continue
		}
		if params.BookSeriesID != "" && resource.BookSeriesID != params.BookSeriesID {
			continue
		}
		if params.TopicID != "" && resource.TopicID != params.TopicID {
			continue
		}
		if params.LevelID != "" && resource.LevelID != params.LevelID {
			continue
		}
		if params.DocumentTypeID != "" && resource.DocumentTypeID != params.DocumentTypeID {
			continue
		}
		if params.ProgramSlug != "" && resource.ProgramSlug != params.ProgramSlug {
			continue
		}
		if params.FileType.Valid() && resource.SelectedFileType != params.FileType {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(resource.Title+" "+resource.Subtitle+" "+resource.Description), query) {
			continue
		}
		items = append(items, resource.ResourceCardDTO)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	total := int64(len(items))
	start := (params.Page - 1) * params.Limit
	if start >= len(items) {
		return []ResourceCardDTO{}, total
	}
	end := start + params.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total
}

func (s *Service) Resource(ctx context.Context, id string) (*ResourceDetailDTO, error) {
	if s.repo != nil {
		return s.repo.GetResource(ctx, s.tenantID(ctx), id)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	resource, ok := s.resources[id]
	if !ok {
		return nil, ErrNotFound
	}
	detail := cloneResource(resource)
	detail.Items = append([]ResourceItemDTO(nil), s.items[id]...)
	return detail, nil
}

func (s *Service) ResourceItems(ctx context.Context, resourceID, query string) ([]ResourceItemDTO, error) {
	if s.repo != nil {
		all, err := s.repo.ListResourceItems(ctx, s.tenantID(ctx), resourceID)
		if err != nil {
			return nil, err
		}
		query = strings.ToLower(strings.TrimSpace(query))
		if query == "" {
			return all, nil
		}
		filtered := make([]ResourceItemDTO, 0, len(all))
		for _, item := range all {
			if strings.Contains(strings.ToLower(item.UnitTitle+" "+item.LessonTitle), query) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.resources[resourceID]; !ok {
		return nil, ErrNotFound
	}
	all := append([]ResourceItemDTO(nil), s.items[resourceID]...)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return all, nil
	}
	filtered := make([]ResourceItemDTO, 0, len(all))
	for _, item := range all {
		if strings.Contains(strings.ToLower(item.UnitTitle+" "+item.LessonTitle), query) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *Service) CreateResource(ctx context.Context, req CreateResourceRequestDTO) (*ResourceDetailDTO, error) {
	validation, err := validateSelectedFileTypeMetadata(req.SelectedFileType, req.OriginalFileName, req.DetectedMimeType)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	canDownload := false
	if req.CanDownload != nil {
		canDownload = *req.CanDownload
	}
	if req.PriceType == "" {
		req.PriceType = "free"
	}
	if req.Slug == "" {
		req.Slug = slugify(req.Title)
	}
	id := "res-" + req.Slug
	assetID := "asset-" + req.Slug
	resourceAudit := newFileTypeAudit(now, req.ActorID, "resource.create", "", req.SelectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings)
	assetAudit := newFileTypeAudit(now, req.ActorID, "resource.create.asset", "", req.SelectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings)
	upstreamURL := strings.TrimSpace(req.UpstreamURL)
	storageURL := strings.TrimSpace(req.ThumbnailURL)
	if req.SelectedFileType == AssetFileTypeLink {
		if upstreamURL == "" && isHTTPURLValue(storageURL) {
			upstreamURL = storageURL
		}
		if storageURL == "" {
			storageURL = upstreamURL
		}
	}
	card := ResourceCardDTO{
		ID:             id,
		Slug:           req.Slug,
		Title:          req.Title,
		Subtitle:       req.Subtitle,
		ThumbnailURL:   req.ThumbnailURL,
		ProgramSlug:    req.ProgramSlug,
		SubjectID:      req.SubjectID,
		GradeID:        req.GradeID,
		CategoryID:     req.CategoryID,
		SectionID:      req.SectionID,
		BookSeriesID:   req.BookSeriesID,
		TopicID:        req.TopicID,
		LevelID:        req.LevelID,
		DocumentTypeID: req.DocumentTypeID,
		PriceType:      req.PriceType,
		Visibility:     normalizeVisibility(req.Visibility),
		Status:         normalizeStatus(req.Status),
		AccessState:    "open",
		CanDownload:    canDownload,
		UpdatedAt:      now,
	}
	asset := AssetRecord{AssetDTO: AssetDTO{
		ID:              assetID,
		ResourceID:      id,
		Title:           req.Title,
		StorageProvider: "r2",
		StorageURL:      storageURL,
		Status:          "ready",
		CanDownload:     canDownload,
		UpdatedAt:       now,
	}, UpstreamURL: upstreamURL}
	applyAssetFileTypeMetadata(&asset.AssetDTO, req.SelectedFileType, validation, assetAudit)
	detail := &ResourceDetailDTO{
		ResourceCardDTO: card,
		Description:     req.Description,
		Tags:            append([]string(nil), req.Tags...),
		Assets:          []AssetDTO{asset.AssetDTO},
		LectureDesign:   cloneLectureDesign(req.LectureDesign),
	}
	applyResourceFileTypeMetadata(detail, req.SelectedFileType, validation, resourceAudit)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[id] = detail
	s.assets[assetID] = &asset
	if len(req.Items) > 0 {
		s.items[id] = normalizeResourceItems(id, assetID, req.Items)
		detail.Items = append([]ResourceItemDTO(nil), s.items[id]...)
	}
	if s.repo != nil {
		tenantID := s.tenantID(ctx)
		if err := s.repo.UpsertResource(ctx, tenantID, detail); err != nil {
			return nil, err
		}
		if err := s.repo.UpsertAsset(ctx, tenantID, asset); err != nil {
			return nil, err
		}
		if err := s.repo.ReplaceResourceItems(ctx, tenantID, id, s.items[id]); err != nil {
			return nil, err
		}
	}
	return cloneResource(detail), nil
}

func (s *Service) UpdateResource(ctx context.Context, id string, req UpdateResourceRequestDTO) (*ResourceDetailDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resource, ok := s.resources[id]
	if !ok {
		return nil, ErrNotFound
	}
	if req.SelectedFileType != nil || req.OriginalFileName != nil || req.DetectedMimeType != nil {
		selectedFileType := resource.SelectedFileType
		if req.SelectedFileType != nil {
			selectedFileType = *req.SelectedFileType
		}
		originalFileName := resource.OriginalFileName
		if req.OriginalFileName != nil {
			originalFileName = *req.OriginalFileName
		}
		detectedMimeType := resource.DetectedMimeType
		if req.DetectedMimeType != nil {
			detectedMimeType = *req.DetectedMimeType
		}
		validation, err := validateSelectedFileTypeMetadata(selectedFileType, originalFileName, detectedMimeType)
		if err != nil {
			return nil, err
		}
		audit := newFileTypeAudit(time.Now().UTC(), req.ActorID, "resource.update", resource.SelectedFileType, selectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings)
		applyResourceFileTypeMetadata(resource, selectedFileType, validation, audit)
	}
	if req.Title != nil {
		resource.Title = *req.Title
	}
	if req.ProgramSlug != nil {
		resource.ProgramSlug = strings.TrimSpace(*req.ProgramSlug)
	}
	if req.SubjectID != nil {
		resource.SubjectID = strings.TrimSpace(*req.SubjectID)
	}
	if req.GradeID != nil {
		resource.GradeID = strings.TrimSpace(*req.GradeID)
	}
	if req.CategoryID != nil {
		resource.CategoryID = strings.TrimSpace(*req.CategoryID)
	}
	if req.SectionID != nil {
		resource.SectionID = strings.TrimSpace(*req.SectionID)
	}
	if req.Subtitle != nil {
		resource.Subtitle = *req.Subtitle
	}
	if req.Description != nil {
		resource.Description = *req.Description
	}
	if req.ThumbnailURL != nil {
		resource.ThumbnailURL = *req.ThumbnailURL
	}
	if req.BookSeriesID != nil {
		resource.BookSeriesID = *req.BookSeriesID
	}
	if req.TopicID != nil {
		resource.TopicID = *req.TopicID
	}
	if req.LevelID != nil {
		resource.LevelID = *req.LevelID
	}
	if req.DocumentTypeID != nil {
		resource.DocumentTypeID = *req.DocumentTypeID
	}
	if req.PriceType != nil {
		resource.PriceType = *req.PriceType
	}
	if req.Visibility != nil {
		resource.Visibility = normalizeVisibility(*req.Visibility)
	}
	if req.Status != nil {
		resource.Status = normalizeStatus(*req.Status)
	}
	if req.CanDownload != nil {
		resource.CanDownload = *req.CanDownload
	}
	if req.Tags != nil {
		resource.Tags = append([]string(nil), req.Tags...)
	}
	if req.LectureDesign != nil {
		resource.LectureDesign = cloneLectureDesign(req.LectureDesign)
	}
	if req.Items != nil {
		assetID := ""
		if len(resource.Assets) > 0 {
			assetID = resource.Assets[0].ID
		}
		s.items[id] = normalizeResourceItems(id, assetID, req.Items)
		resource.Items = append([]ResourceItemDTO(nil), s.items[id]...)
	}
	resource.UpdatedAt = time.Now().UTC()
	if s.repo != nil {
		tenantID := s.tenantID(ctx)
		if err := s.repo.UpsertResource(ctx, tenantID, resource); err != nil {
			return nil, err
		}
		if req.Items != nil {
			if err := s.repo.ReplaceResourceItems(ctx, tenantID, id, s.items[id]); err != nil {
				return nil, err
			}
		}
	}
	return cloneResource(resource), nil
}

func (s *Service) DeleteResource(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resource, ok := s.resources[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.resources, id)
	delete(s.items, id)
	for _, asset := range resource.Assets {
		delete(s.assets, asset.ID)
		delete(s.pages, asset.ID)
	}
	if s.repo != nil {
		return s.repo.DeleteResource(ctx, s.tenantID(ctx), id)
	}
	return nil
}

func (s *Service) CreateAsset(ctx context.Context, req CreateAssetRequestDTO) (*AssetDTO, error) {
	validation, err := validateSelectedFileTypeMetadata(req.SelectedFileType, req.OriginalFileName, req.DetectedMimeType)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.resources[req.ResourceID]; !ok {
		return nil, ErrNotFound
	}
	now := time.Now().UTC()
	canDownload := false
	if req.CanDownload != nil {
		canDownload = *req.CanDownload
	}
	id := fmt.Sprintf("asset-%s-%d", req.ResourceID, len(s.assets)+1)
	provider := req.StorageProvider
	if provider == "" {
		provider = "r2"
	}
	assetAudit := newFileTypeAudit(now, req.ActorID, "asset.create", "", req.SelectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings)
	record := &AssetRecord{
		AssetDTO: AssetDTO{
			ID:              id,
			ResourceID:      req.ResourceID,
			Title:           req.Title,
			FileSizeBytes:   req.FileSizeBytes,
			StorageProvider: provider,
			StorageURL:      req.StorageURL,
			Status:          "ready",
			CanDownload:     canDownload,
			UpdatedAt:       now,
		},
		UpstreamURL: req.UpstreamURL,
	}
	applyAssetFileTypeMetadata(&record.AssetDTO, req.SelectedFileType, validation, assetAudit)
	s.assets[id] = record
	resource := s.resources[req.ResourceID]
	resource.Assets = promotePrimaryAsset(resource.Assets, record.AssetDTO)
	applyResourceFileTypeMetadata(resource, req.SelectedFileType, validation, newFileTypeAudit(now, req.ActorID, "asset.create.resource_card", resource.SelectedFileType, req.SelectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings))
	resource.CanDownload = canDownload
	resource.UpdatedAt = now
	if s.repo != nil {
		tenantID := s.tenantID(ctx)
		if err := s.repo.UpsertAsset(ctx, tenantID, *record); err != nil {
			return nil, err
		}
		if err := s.repo.UpsertResource(ctx, tenantID, resource); err != nil {
			return nil, err
		}
	}
	asset := cloneAssetDTO(record.AssetDTO)
	return &asset, nil
}

func (s *Service) UpdateAsset(ctx context.Context, id string, req UpdateAssetRequestDTO) (*AssetDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.assets[id]
	if !ok {
		return nil, ErrNotFound
	}
	now := time.Now().UTC()
	metadataChanged := req.SelectedFileType != nil || req.OriginalFileName != nil || req.DetectedMimeType != nil
	var validation fileTypeValidationResult
	var audit *FileTypeAuditDTO
	if metadataChanged {
		selectedFileType := record.SelectedFileType
		if req.SelectedFileType != nil {
			selectedFileType = *req.SelectedFileType
		}
		originalFileName := record.OriginalFileName
		if req.OriginalFileName != nil {
			originalFileName = *req.OriginalFileName
		}
		detectedMimeType := record.DetectedMimeType
		if req.DetectedMimeType != nil {
			detectedMimeType = *req.DetectedMimeType
		}
		var err error
		validation, err = validateSelectedFileTypeMetadata(selectedFileType, originalFileName, detectedMimeType)
		if err != nil {
			return nil, err
		}
		audit = newFileTypeAudit(now, req.ActorID, "asset.update", record.SelectedFileType, selectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings)
		applyAssetFileTypeMetadata(&record.AssetDTO, selectedFileType, validation, audit)
	}
	if req.Title != nil {
		record.Title = *req.Title
	}
	if req.FileSizeBytes != nil {
		record.FileSizeBytes = *req.FileSizeBytes
	}
	if req.StorageProvider != nil {
		record.StorageProvider = *req.StorageProvider
	}
	if req.StorageURL != nil {
		record.StorageURL = *req.StorageURL
	}
	if req.UpstreamURL != nil {
		record.UpstreamURL = *req.UpstreamURL
	}
	if req.CanDownload != nil {
		record.CanDownload = *req.CanDownload
	}
	if req.Status != nil {
		record.Status = *req.Status
	}
	record.UpdatedAt = now
	if resource, ok := s.resources[record.ResourceID]; ok {
		syncAssetIntoResource(resource, record.AssetDTO)
		if metadataChanged {
			applyResourceFileTypeMetadata(resource, record.SelectedFileType, validation, newFileTypeAudit(now, req.ActorID, "asset.update.resource_card", resource.SelectedFileType, record.SelectedFileType, validation.OriginalFileName, validation.DetectedMimeType, validation.Warnings))
		}
		if req.CanDownload != nil {
			resource.CanDownload = record.CanDownload
		}
		resource.UpdatedAt = now
	}
	if s.repo != nil {
		tenantID := s.tenantID(ctx)
		if err := s.repo.UpsertAsset(ctx, tenantID, *record); err != nil {
			return nil, err
		}
		if resource, ok := s.resources[record.ResourceID]; ok {
			if err := s.repo.UpsertResource(ctx, tenantID, resource); err != nil {
				return nil, err
			}
		}
	}
	asset := cloneAssetDTO(record.AssetDTO)
	return &asset, nil
}

func (s *Service) CreateTaxonomyOption(ctx context.Context, kind string, req CreateTaxonomyOptionRequestDTO) (TaxonomyOptionDTO, error) {
	option := taxonomyOptionFromRequest(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.taxonomySlice(kind)
	if target == nil {
		return TaxonomyOptionDTO{}, ErrNotFound
	}
	*target = upsertTaxonomyOption(*target, option)
	if s.repo != nil {
		if err := s.repo.UpsertTaxonomy(ctx, s.tenantID(ctx), normalizeTaxonomyKind(kind), option); err != nil {
			return TaxonomyOptionDTO{}, err
		}
	}
	return option, nil
}

func (s *Service) ListTaxonomyOptions(ctx context.Context, kind string) ([]TaxonomyOptionDTO, error) {
	normalizedKind := normalizeTaxonomyKind(kind)
	if s.repo != nil {
		taxonomy, err := s.repo.ListTaxonomy(ctx, s.tenantID(ctx))
		if err != nil {
			return nil, err
		}
		if options, ok := taxonomy[normalizedKind]; ok {
			return filterPublicOptionsForKind(normalizedKind, options), nil
		}
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return publicTaxonomyOptionsForKind(normalizedKind, s.publicTaxonomyResponseLocked())
}

func (s *Service) publicTaxonomyResponseLocked() TaxonomyResponseDTO {
	return TaxonomyResponseDTO{
		Programs:        filterPublicPrograms(s.programs),
		Grades:          filterPublicGrades(s.grades),
		Subjects:        filterPublicSubjects(s.subjects),
		Categories:      filterPublicTaxonomyOptions(s.categories),
		Sections:        filterPublicOptionsForKind("sections", s.sections),
		BookSeries:      filterPublicTaxonomyOptions(s.bookSeries),
		Topics:          filterPublicTaxonomyOptions(s.topics),
		FileTypes:       fileTypes(),
		DesignerPresets: append([]LectureDesignerPresetDTO{}, s.designerPresets...),
	}
}

func publicTaxonomyOptionsForKind(kind string, model TaxonomyResponseDTO) ([]TaxonomyOptionDTO, error) {
	switch kind {
	case "grades":
		return append([]TaxonomyOptionDTO(nil), model.Grades...), nil
	case "subjects":
		return append([]TaxonomyOptionDTO(nil), model.Subjects...), nil
	case "categories":
		return append([]TaxonomyOptionDTO(nil), model.Categories...), nil
	case "sections":
		return append([]TaxonomyOptionDTO(nil), model.Sections...), nil
	case "book-series":
		return append([]TaxonomyOptionDTO(nil), model.BookSeries...), nil
	case "topics":
		return append([]TaxonomyOptionDTO(nil), model.Topics...), nil
	default:
		return nil, ErrNotFound
	}
}

func (s *Service) UpdateTaxonomyOption(ctx context.Context, kind, id string, req UpdateTaxonomyOptionRequestDTO) (TaxonomyOptionDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.taxonomySlice(kind)
	if target == nil {
		return TaxonomyOptionDTO{}, ErrNotFound
	}
	trimmedID := strings.TrimSpace(id)
	for index, option := range *target {
		if option.ID != trimmedID {
			continue
		}
		if req.Label != nil {
			option.Label = strings.TrimSpace(*req.Label)
			if option.Label == "" {
				return TaxonomyOptionDTO{}, ErrInvalidRequest
			}
		}
		if req.Slug != nil {
			option.Slug = strings.TrimSpace(*req.Slug)
			if option.Slug == "" {
				option.Slug = slugify(option.Label)
			}
		}
		if req.Description != nil {
			option.Description = strings.TrimSpace(*req.Description)
		}
		if req.ParentID != nil {
			option.ParentID = strings.TrimSpace(*req.ParentID)
		}
		if req.SubjectID != nil {
			option.SubjectID = strings.TrimSpace(*req.SubjectID)
		}
		if req.GradeID != nil {
			option.GradeID = strings.TrimSpace(*req.GradeID)
		}
		if req.CategoryID != nil {
			option.CategoryID = strings.TrimSpace(*req.CategoryID)
		}
		if req.BookSeriesID != nil {
			option.BookSeriesID = strings.TrimSpace(*req.BookSeriesID)
		}
		if req.TopicID != nil {
			option.TopicID = strings.TrimSpace(*req.TopicID)
		}
		if req.LevelIDs != nil {
			option.LevelIDs = append([]string(nil), req.LevelIDs...)
		}
		if req.SortOrder != nil {
			option.SortOrder = *req.SortOrder
		}
		if req.Status != nil {
			option.Status = strings.TrimSpace(*req.Status)
			if option.Status == "" {
				option.Status = "active"
			}
		}
		if req.Metadata != nil {
			option.Metadata = sanitizeStringMap(req.Metadata)
		}
		(*target)[index] = option
		if s.repo != nil {
			if err := s.repo.UpsertTaxonomy(ctx, s.tenantID(ctx), normalizeTaxonomyKind(kind), option); err != nil {
				return TaxonomyOptionDTO{}, err
			}
		}
		return option, nil
	}
	return TaxonomyOptionDTO{}, ErrNotFound
}

func (s *Service) CreateResourceWithUpload(ctx context.Context, req CreateResourceRequestDTO, buf []byte, originalFileName, detectedMimeType string) (*UploadResourceResponseDTO, error) {
	if strings.TrimSpace(req.Title) == "" {
		req.Title = originalFileName
	}
	if strings.TrimSpace(req.ProgramSlug) == "" {
		req.ProgramSlug = req.SubjectID
	}
	if strings.TrimSpace(req.Status) == "" {
		req.Status = "published"
	}
	if strings.TrimSpace(req.Visibility) == "" {
		req.Visibility = "public"
	}
	req.OriginalFileName = originalFileName
	req.DetectedMimeType = detectedMimeType
	resource, err := s.CreateResource(ctx, req)
	if err != nil {
		return nil, err
	}
	canDownload := false
	if req.CanDownload != nil {
		canDownload = *req.CanDownload
	}
	asset, err := s.UploadAsset(ctx, UploadAssetRequestDTO{
		ResourceID:       resource.ID,
		Title:            req.Title,
		SelectedFileType: req.SelectedFileType,
		CanDownload:      canDownload,
		ActorID:          req.ActorID,
	}, buf, originalFileName, detectedMimeType)
	if err != nil {
		return nil, err
	}
	updated, err := s.Resource(ctx, resource.ID)
	if err != nil {
		return nil, err
	}
	return &UploadResourceResponseDTO{Resource: updated, Asset: asset}, nil
}

func (s *Service) DeleteTaxonomyOption(ctx context.Context, kind, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.taxonomySlice(kind)
	if target == nil {
		return ErrNotFound
	}
	trimmedID := strings.TrimSpace(id)
	for index, option := range *target {
		if option.ID != trimmedID {
			continue
		}
		*target = append((*target)[:index], (*target)[index+1:]...)
		if s.repo != nil {
			if err := s.repo.DeleteTaxonomy(ctx, s.tenantID(ctx), normalizeTaxonomyKind(kind), trimmedID); err != nil {
				return err
			}
		}
		return nil
	}
	return ErrNotFound
}

func (s *Service) UploadAsset(ctx context.Context, req UploadAssetRequestDTO, buf []byte, originalFileName, detectedMimeType string) (*AssetDTO, error) {
	validation, err := validateSelectedFileTypeMetadata(req.SelectedFileType, originalFileName, detectedMimeType)
	if err != nil {
		return nil, err
	}
	storageURL := ""
	provider := "local"
	if s.r2 != nil {
		url, err := s.r2.UploadLearningAsset(ctx, buf, req.ResourceID, validation.OriginalFileName, validation.DetectedMimeType)
		if err != nil {
			return nil, err
		}
		storageURL = url
		provider = "r2"
	}
	canDownload := req.CanDownload
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = validation.OriginalFileName
	}
	return s.CreateAsset(ctx, CreateAssetRequestDTO{
		ResourceID:       req.ResourceID,
		Title:            title,
		SelectedFileType: req.SelectedFileType,
		OriginalFileName: validation.OriginalFileName,
		DetectedMimeType: validation.DetectedMimeType,
		FileSizeBytes:    int64(len(buf)),
		StorageProvider:  provider,
		StorageURL:       storageURL,
		UpstreamURL:      storageURL,
		CanDownload:      &canDownload,
		ActorID:          req.ActorID,
	})
}

func (s *Service) Launch(ctx context.Context, assetID, userID string) (*LaunchResponseDTO, error) {
	s.mu.RLock()
	record, ok := s.assets[assetID]
	if !ok {
		s.mu.RUnlock()
		return nil, ErrNotFound
	}
	asset := cloneAssetDTO(record.AssetDTO)
	UpstreamURL := record.UpstreamURL
	upstreamConfigured := UpstreamURL != ""
	s.mu.RUnlock()

	expires := time.Now().UTC().Add(15 * time.Minute)
	token := viewerToken(assetID, userID, expires)
	resp := &LaunchResponseDTO{
		AssetID:          asset.ID,
		ResourceID:       asset.ResourceID,
		SelectedFileType: asset.SelectedFileType,
		LaunchMode:       asset.LaunchMode,
		Title:            asset.Title,
		ViewerTokenURL:   fmt.Sprintf("/api/hoclieu/assets/%s/pages?token=%s", asset.ID, token),
		StreamURL:        fmt.Sprintf("/api/hoclieu/assets/%s/stream?token=%s", asset.ID, token),
		ExpiresAt:        expires,
		CanDownload:      asset.CanDownload,
		Watermark: &WatermarkDTO{
			Text:     "ERG",
			Opacity:  0.08,
			Position: "center",
		},
		Audit: newAuditMetadata("hoclieu.asset.launch", asset.ID, userID),
	}
	if asset.SelectedFileType == AssetFileTypeLink && upstreamConfigured {
		if embedURL := googleSlidesEmbedURL(UpstreamURL); embedURL != "" {
			resp.LaunchMode = LaunchModeGoogleSlide
			resp.EmbedURL = embedURL
		} else {
			resp.LaunchMode = LaunchModeExternal
			resp.EmbedURL = UpstreamURL
		}
	}
	s.publishAssetAuditEvent(ctx, buildAssetAuditEvent(ctx, auditservice.ActionAssetLaunched, asset, userID, map[string]any{
		"upstream_configured": upstreamConfigured,
	}))
	return resp, nil
}

func isHTTPURLValue(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

func googleSlidesEmbedURL(raw string) string {
	if !isHTTPURLValue(raw) {
		return ""
	}

	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(parsed.Host), "docs.google.com") || !strings.Contains(parsed.Path, "/presentation/") {
		return ""
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index := 0; index < len(segments)-1; index += 1 {
		if segments[index] == "d" && strings.TrimSpace(segments[index+1]) != "" {
			return fmt.Sprintf(
				"https://docs.google.com/presentation/d/%s/embed?start=false&loop=false&delayms=3000",
				segments[index+1],
			)
		}
	}

	return ""
}

func (s *Service) Pages(_ context.Context, assetID string) (*ViewerPagesResponseDTO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.assets[assetID]
	if !ok {
		return nil, ErrNotFound
	}
	pages := append([]ViewerPageDTO(nil), s.pages[assetID]...)
	if len(pages) == 0 {
		pages = []ViewerPageDTO{{Index: 1, Title: record.Title, Width: 1280, Height: 720}}
	}
	return &ViewerPagesResponseDTO{
		AssetID:          record.ID,
		ResourceID:       record.ResourceID,
		SelectedFileType: record.SelectedFileType,
		Pages:            pages,
	}, nil
}

func (s *Service) Asset(ctx context.Context, assetID string) (*AssetDTO, error) {
	if s.repo != nil {
		record, err := s.repo.GetAsset(ctx, s.tenantID(ctx), assetID)
		if err != nil {
			return nil, err
		}
		asset := cloneAssetDTO(record.AssetDTO)
		return &asset, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.assets[assetID]
	if !ok {
		return nil, ErrNotFound
	}
	asset := cloneAssetDTO(record.AssetDTO)
	return &asset, nil
}

func (s *Service) Portfolio(context.Context) []HomeShortcutDTO {
	return []HomeShortcutDTO{
		{ID: "portfolio-samples", Title: "Bai giang mau", Description: "Template va san pham giao vien da chon loc.", Href: "/portfolio"},
		{ID: "practice-files", Title: "File thuc hanh", Description: "Tai nguyen lop Tin hoc, IC3 va MOS.", Href: "/portfolio/practice"},
	}
}

func (s *Service) Community(context.Context) []HomeShortcutDTO {
	return []HomeShortcutDTO{
		{ID: "teacher-sharing", Title: "Cong dong giao vien", Description: "Chia se kinh nghiem lop hoc va hoc lieu noi bo.", Href: "/cong-dong"},
	}
}

func (s *Service) Quizzes(context.Context) []ResourceCardDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := []ResourceCardDTO{}
	for _, resource := range s.resources {
		if resource.SelectedFileType == AssetFileTypeQuiz || resource.CategoryID == "quiz-bank" {
			items = append(items, resource.ResourceCardDTO)
		}
	}
	return items
}

func fileTypes() []AssetFileType {
	return []AssetFileType{
		AssetFileTypePDF,
		AssetFileTypePPTX,
		AssetFileTypeVideo,
		AssetFileTypeAudio,
		AssetFileTypeHTML5,
		AssetFileTypeLink,
		AssetFileTypeQuiz,
		AssetFileTypeZIP,
		AssetFileTypeDOCX,
		AssetFileTypeXLSX,
		AssetFileTypeImage,
	}
}

func defaultLaunchMode(fileType AssetFileType) LaunchMode {
	return DefaultLaunchMode(fileType)
}

func cloneResource(resource *ResourceDetailDTO) *ResourceDetailDTO {
	cp := *resource
	cp.MetadataWarnings = append([]FileTypeWarningDTO(nil), resource.MetadataWarnings...)
	cp.FileTypeAudit = cloneFileTypeAudit(resource.FileTypeAudit)
	cp.Tags = append([]string(nil), resource.Tags...)
	cp.Assets = make([]AssetDTO, len(resource.Assets))
	for i, asset := range resource.Assets {
		cp.Assets[i] = cloneAssetDTO(asset)
	}
	cp.Items = append([]ResourceItemDTO(nil), resource.Items...)
	cp.LectureDesign = cloneLectureDesign(resource.LectureDesign)
	return &cp
}

func cloneAssetDTO(asset AssetDTO) AssetDTO {
	cp := asset
	cp.MetadataWarnings = append([]FileTypeWarningDTO(nil), asset.MetadataWarnings...)
	cp.FileTypeAudit = cloneFileTypeAudit(asset.FileTypeAudit)
	return cp
}

func cloneLectureDesign(design *LectureDesignDTO) *LectureDesignDTO {
	if design == nil {
		return nil
	}
	cp := *design
	cp.UnitLabels = append([]string(nil), design.UnitLabels...)
	return &cp
}

func (s *Service) taxonomySlice(kind string) *[]TaxonomyOptionDTO {
	switch normalizeTaxonomyKind(kind) {
	case "grades", "levels":
		return &s.grades
	case "subjects":
		return &s.subjects
	case "categories", "document-types":
		return &s.categories
	case "sections":
		return &s.sections
	case "book-series":
		return &s.bookSeries
	case "topics":
		return &s.topics
	default:
		return nil
	}
}

func normalizeTaxonomyKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "levels":
		return "grades"
	case "document-types":
		return "categories"
	default:
		return strings.TrimSpace(kind)
	}
}

func isPublicSubjectID(subjectID string) bool {
	return strings.TrimSpace(subjectID) == "ic3-gs6"
}

func filterPublicPrograms(programs []ProgramDTO) []ProgramDTO {
	out := make([]ProgramDTO, 0, len(programs))
	allowedGrades := map[string]bool{"6": true, "7": true, "8": true}
	for _, program := range programs {
		publicSubjects := make([]string, 0, len(program.SubjectIDs))
		publicGrades := make([]string, 0, len(program.GradeIDs))
		for _, subjectID := range program.SubjectIDs {
			if isPublicSubjectID(subjectID) {
				publicSubjects = append(publicSubjects, subjectID)
			}
		}
		for _, gradeID := range program.GradeIDs {
			if allowedGrades[strings.TrimSpace(gradeID)] {
				publicGrades = append(publicGrades, gradeID)
			}
		}
		if len(publicSubjects) == 0 {
			continue
		}
		program.SubjectIDs = publicSubjects
		program.GradeIDs = publicGrades
		out = append(out, program)
	}
	return out
}

func filterPublicGrades(grades []TaxonomyOptionDTO) []TaxonomyOptionDTO {
	allowed := map[string]bool{"6": true, "7": true, "8": true}
	out := make([]TaxonomyOptionDTO, 0, len(grades))
	for _, grade := range grades {
		if allowed[strings.TrimSpace(grade.ID)] {
			out = append(out, grade)
		}
	}
	return out
}

func filterPublicTaxonomyOptions(options []TaxonomyOptionDTO) []TaxonomyOptionDTO {
	out := make([]TaxonomyOptionDTO, 0, len(options))
	for _, option := range options {
		if isPublicSubjectID(option.SubjectID) {
			out = append(out, option)
		}
	}
	return out
}

func filterPublicSubjects(options []TaxonomyOptionDTO) []TaxonomyOptionDTO {
	out := make([]TaxonomyOptionDTO, 0, len(options))
	for _, option := range options {
		if isPublicSubjectID(option.ID) || isPublicSubjectID(option.Slug) {
			out = append(out, option)
		}
	}
	return out
}

func filterPublicOptionsForKind(kind string, options []TaxonomyOptionDTO) []TaxonomyOptionDTO {
	switch kind {
	case "grades":
		return filterPublicGrades(options)
	case "subjects":
		return filterPublicSubjects(options)
	case "categories", "book-series", "topics":
		return filterPublicTaxonomyOptions(options)
	case "sections":
		out := make([]TaxonomyOptionDTO, 0, len(options))
		for _, option := range options {
			if isPublicSubjectID(option.SubjectID) || option.ID == "support-components" {
				out = append(out, option)
			}
		}
		return out
	default:
		return append([]TaxonomyOptionDTO(nil), options...)
	}
}

func taxonomyOptionFromRequest(req CreateTaxonomyOptionRequestDTO) TaxonomyOptionDTO {
	label := strings.TrimSpace(req.Label)
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = slugify(label)
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = slug
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	return TaxonomyOptionDTO{
		ID:           id,
		Label:        label,
		Slug:         slug,
		ParentID:     strings.TrimSpace(req.ParentID),
		SubjectID:    strings.TrimSpace(req.SubjectID),
		GradeID:      strings.TrimSpace(req.GradeID),
		CategoryID:   strings.TrimSpace(req.CategoryID),
		BookSeriesID: strings.TrimSpace(req.BookSeriesID),
		TopicID:      strings.TrimSpace(req.TopicID),
		LevelIDs:     append([]string(nil), req.LevelIDs...),
		Description:  strings.TrimSpace(req.Description),
		SortOrder:    req.SortOrder,
		Status:       status,
		Metadata:     sanitizeStringMap(req.Metadata),
	}
}

func sanitizeStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		out[trimmedKey] = trimmedValue
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func upsertTaxonomyOption(items []TaxonomyOptionDTO, option TaxonomyOptionDTO) []TaxonomyOptionDTO {
	for i := range items {
		if items[i].ID == option.ID {
			items[i] = option
			return items
		}
	}
	return append(items, option)
}

func normalizeResourceItems(resourceID, fallbackAssetID string, items []ResourceItemDTO) []ResourceItemDTO {
	result := make([]ResourceItemDTO, 0, len(items))
	for idx, item := range items {
		item.ResourceID = resourceID
		if item.ID == "" {
			item.ID = fmt.Sprintf("%s-item-%d", resourceID, idx+1)
		}
		if item.AssetID == "" {
			item.AssetID = fallbackAssetID
		}
		if item.SortOrder == 0 {
			item.SortOrder = idx + 1
		}
		result = append(result, item)
	}
	return result
}

func normalizeVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "private", "center", "school", "public":
		return strings.TrimSpace(value)
	default:
		return "public"
	}
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "draft", "review", "published", "archived":
		return strings.TrimSpace(value)
	default:
		return "draft"
	}
}

func cloneFileTypeAudit(audit *FileTypeAuditDTO) *FileTypeAuditDTO {
	if audit == nil {
		return nil
	}
	cp := *audit
	if audit.ChangedAt != nil {
		changedAt := *audit.ChangedAt
		cp.ChangedAt = &changedAt
	}
	cp.Warnings = append([]string(nil), audit.Warnings...)
	return &cp
}

func syncAssetIntoResource(resource *ResourceDetailDTO, asset AssetDTO) {
	for i := range resource.Assets {
		if resource.Assets[i].ID == asset.ID {
			resource.Assets[i] = cloneAssetDTO(asset)
			return
		}
	}
	resource.Assets = append(resource.Assets, cloneAssetDTO(asset))
}

func promotePrimaryAsset(assets []AssetDTO, asset AssetDTO) []AssetDTO {
	next := []AssetDTO{cloneAssetDTO(asset)}
	for _, existing := range assets {
		if existing.ID == asset.ID {
			continue
		}
		if strings.HasPrefix(existing.ID, "asset-") && existing.StorageURL == "" && existing.FileSizeBytes == 0 {
			continue
		}
		next = append(next, cloneAssetDTO(existing))
	}
	return next
}

func viewerToken(assetID, userID string, expires time.Time) string {
	raw := fmt.Sprintf("%s:%s:%d", assetID, userID, expires.Unix())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func newAuditMetadata(event, assetID, userID string) AuditMetadataDTO {
	return AuditMetadataDTO{
		Event:   event,
		AssetID: assetID,
		UserID:  userID,
		At:      time.Now().UTC(),
	}
}

func slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	replacer := strings.NewReplacer(" ", "-", "/", "-", "_", "-", ".", "-")
	s = replacer.Replace(s)
	s = strings.Trim(s, "-")
	if s == "" {
		return fmt.Sprintf("resource-%d", time.Now().UnixNano())
	}
	return s
}
