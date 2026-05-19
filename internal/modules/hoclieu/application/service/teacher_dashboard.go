package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	. "erg.ninja/internal/modules/hoclieu/api/dto"
	"erg.ninja/pkg/database"
)

type dashboardLeaf struct {
	ID           string
	Label        string
	Kind         TeacherDashboardNodeKind
	ResourceID   string
	ResourceType string
}

type dashboardNodeMeta struct {
	ID            string
	Label         string
	Kind          TeacherDashboardNodeKind
	ParentID      string
	SubjectID     string
	BookSeriesID  string
	Description   string
	ResourceID    string
	ResourceType  string
	ThumbnailURL  string
	FileTypeBadge string
	UpdatedAt     *time.Time
}

const (
	dashboardFolderOrphanTopicsPrefix   = "folder:topics:"
	dashboardFolderOrphanSectionsPrefix = "folder:sections:"
)

type teachingStatus struct {
	Status  string
	Percent float64
}

func (s *Service) SubjectTree(ctx context.Context, subjectID, schoolID, academicYear, parentID string) (*TeacherSubjectTreeDTO, error) {
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		return nil, ErrInvalidRequest
	}
	subjectLabel := s.lookupSubjectLabel(subjectID)
	if subjectLabel == "" {
		subjectLabel = subjectID
	}

	children, breadcrumbs, err := s.dashboardChildren(ctx, subjectID, parentID)
	if err != nil {
		return nil, err
	}
	events := s.progressEventsForScope(ctx, schoolID, academicYear, subjectID)
	childNodes := make([]TeacherDashboardNodeDTO, 0, len(children))
	for _, child := range children {
		summary := s.progressSummaryForNode(ctx, subjectID, child.ID, schoolID, academicYear)
		lastOpenedAt := latestOpenForNode(events, child.ID)
		childNodes = append(childNodes, TeacherDashboardNodeDTO{
			ID:            child.ID,
			Label:         child.Label,
			Kind:          child.Kind,
			ParentID:      child.ParentID,
			SubjectID:     child.SubjectID,
			SubjectLabel:  subjectLabel,
			Description:   child.Description,
			ResourceID:    child.ResourceID,
			ResourceType:  child.ResourceType,
			ThumbnailURL:  child.ThumbnailURL,
			FileTypeBadge: child.FileTypeBadge,
			HasChildren:   s.nodeHasChildren(ctx, subjectID, child.ID),
			Progress:      summary,
			LastOpenedAt:  lastOpenedAt,
			UpdatedAt:     child.UpdatedAt,
		})
	}

	return &TeacherSubjectTreeDTO{
		SubjectID:    subjectID,
		SubjectLabel: subjectLabel,
		SchoolID:     strings.TrimSpace(schoolID),
		AcademicYear: strings.TrimSpace(academicYear),
		ParentID:     strings.TrimSpace(parentID),
		Breadcrumbs:  breadcrumbs,
		Children:     childNodes,
		Progress:     s.progressSummaryForNode(ctx, subjectID, parentID, schoolID, academicYear),
	}, nil
}

func (s *Service) RecentOpened(ctx context.Context, schoolID, academicYear string, limit int) []TeacherRecentOpenedItemDTO {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	all := s.progressEventsForScope(ctx, schoolID, academicYear, "")
	latestByNode := map[string]TeacherProgressEventDTO{}
	for _, event := range all {
		if event.EventType != TeacherProgressEventOpen {
			continue
		}
		key := event.SubjectID + "::" + event.NodeID
		current, ok := latestByNode[key]
		if !ok || current.OccurredAt.Before(event.OccurredAt) {
			latestByNode[key] = event
		}
	}
	items := make([]TeacherRecentOpenedItemDTO, 0, len(latestByNode))
	for _, event := range latestByNode {
		meta, _ := s.lookupNodeMeta(ctx, event.SubjectID, event.NodeID)
		subjectLabel := s.lookupSubjectLabel(event.SubjectID)
		if subjectLabel == "" {
			subjectLabel = event.SubjectID
		}
		item := TeacherRecentOpenedItemDTO{
			ID:           event.ID,
			SubjectID:    event.SubjectID,
			SubjectLabel: subjectLabel,
			NodeID:       event.NodeID,
			NodeLabel:    event.NodeID,
			NodeKind:     event.NodeKind,
			ResourceID:   event.ResourceID,
			OpenedAt:     event.OccurredAt,
		}
		if meta != nil {
			item.NodeLabel = meta.Label
			item.NodeKind = meta.Kind
			item.ResourceID = meta.ResourceID
			item.ResourceType = meta.ResourceType
			if meta.Kind == TeacherDashboardNodeKindLesson || meta.Kind == TeacherDashboardNodeKindResource {
				item.ResourceTitle = meta.Label
			}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].OpenedAt.After(items[j].OpenedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *Service) Progress(ctx context.Context, subjectID, nodeID, schoolID, academicYear string) (*TeacherProgressResponseDTO, error) {
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		return nil, ErrInvalidRequest
	}
	leaves := s.collectLeaves(ctx, subjectID, nodeID)
	statusByLeaf := s.statusByLeaf(ctx, subjectID, schoolID, academicYear)
	items := make([]TeacherProgressDetailItemDTO, 0, len(leaves))
	for _, leaf := range leaves {
		status := statusByLeaf[leaf.ID]
		if status.Status == "" {
			status = teachingStatus{Status: "pending", Percent: 0}
		}
		items = append(items, TeacherProgressDetailItemDTO{
			ID:           leaf.ID,
			Label:        leaf.Label,
			Kind:         leaf.Kind,
			Status:       status.Status,
			ProgressRate: status.Percent,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	return &TeacherProgressResponseDTO{
		SubjectID:    subjectID,
		NodeID:       strings.TrimSpace(nodeID),
		SchoolID:     strings.TrimSpace(schoolID),
		AcademicYear: strings.TrimSpace(academicYear),
		Summary:      summarizeLeafStatuses(leaves, statusByLeaf),
		Items:        items,
	}, nil
}

func (s *Service) TrackProgressEvent(ctx context.Context, req TrackTeacherProgressEventRequestDTO) (*TeacherProgressEventDTO, error) {
	req.SubjectID = strings.TrimSpace(req.SubjectID)
	req.SchoolID = strings.TrimSpace(req.SchoolID)
	req.AcademicYear = strings.TrimSpace(req.AcademicYear)
	req.NodeID = strings.TrimSpace(req.NodeID)
	req.ResourceID = strings.TrimSpace(req.ResourceID)
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	if req.SubjectID == "" || req.SchoolID == "" || req.AcademicYear == "" || req.NodeID == "" || req.TeacherID == "" {
		return nil, ErrInvalidRequest
	}
	if !req.EventType.Valid() || !validTeacherNodeKind(req.NodeKind) {
		return nil, ErrInvalidRequest
	}
	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil && !req.OccurredAt.IsZero() {
		occurredAt = req.OccurredAt.UTC()
	}
	event := TeacherProgressEventDTO{
		ID:           database.NewID(),
		TeacherID:    req.TeacherID,
		SchoolID:     req.SchoolID,
		AcademicYear: req.AcademicYear,
		SubjectID:    req.SubjectID,
		NodeID:       req.NodeID,
		NodeKind:     req.NodeKind,
		EventType:    req.EventType,
		ResourceID:   req.ResourceID,
		OccurredAt:   occurredAt,
	}
	s.mu.Lock()
	s.progressEvents = append([]TeacherProgressEventDTO{event}, s.progressEvents...)
	s.mu.Unlock()
	if s.repo != nil {
		if err := s.repo.AppendProgressEvent(ctx, s.tenantID(ctx), event); err != nil {
			return nil, err
		}
	}
	return &event, nil
}

func (s *Service) dashboardChildren(ctx context.Context, subjectID, parentID string) ([]dashboardNodeMeta, []TeacherDashboardBreadcrumbDTO, error) {
	parentID = strings.TrimSpace(parentID)
	nodes, breadcrumbs := s.taxonomyChildren(subjectID, parentID)
	if len(nodes) > 0 {
		return nodes, breadcrumbs, nil
	}
	resources := s.resourceChildren(ctx, subjectID, parentID)
	return resources, breadcrumbs, nil
}

func (s *Service) taxonomyChildren(subjectID, parentID string) ([]dashboardNodeMeta, []TeacherDashboardBreadcrumbDTO) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	categories := sortTaxonomyOptionsForDashboard(filterTaxonomyBySubject(s.categories, subjectID))
	sections := sortTaxonomyOptionsForDashboard(filterTaxonomyBySubject(s.sections, subjectID))
	bookSeries := sortTaxonomyOptionsForDashboard(filterTaxonomyBySubject(s.bookSeries, subjectID))
	topics := sortTaxonomyOptionsForDashboard(filterTaxonomyBySubject(s.topics, subjectID))

	lookup := make(map[string]dashboardNodeMeta, len(categories)+len(sections)+len(bookSeries)+len(topics)+2)
	addNode := func(item TaxonomyOptionDTO, kind TeacherDashboardNodeKind) dashboardNodeMeta {
		meta := dashboardNodeMeta{
			ID:           item.ID,
			Label:        item.Label,
			Kind:         kind,
			ParentID:     strings.TrimSpace(item.ParentID),
			SubjectID:    subjectID,
			BookSeriesID: strings.TrimSpace(item.BookSeriesID),
			Description:  item.Description,
		}
		lookup[item.ID] = meta
		return meta
	}

	categoryNodes := make([]dashboardNodeMeta, 0, len(categories))
	for _, item := range categories {
		categoryNodes = append(categoryNodes, addNode(item, TeacherDashboardNodeKindCategory))
	}
	sectionNodes := make([]dashboardNodeMeta, 0, len(sections))
	for _, item := range sections {
		sectionNodes = append(sectionNodes, addNode(item, TeacherDashboardNodeKindSection))
	}
	bookSeriesNodes := make([]dashboardNodeMeta, 0, len(bookSeries))
	for _, item := range bookSeries {
		bookSeriesNodes = append(bookSeriesNodes, addNode(item, TeacherDashboardNodeKindBookSeries))
	}
	topicNodes := make([]dashboardNodeMeta, 0, len(topics))
	for _, item := range topics {
		topicNodes = append(topicNodes, addNode(item, TeacherDashboardNodeKindTopic))
	}

	orphanTopicsFolderID := dashboardFolderOrphanTopicsPrefix + subjectID
	orphanSectionsFolderID := dashboardFolderOrphanSectionsPrefix + subjectID
	lookup[orphanTopicsFolderID] = dashboardNodeMeta{
		ID:          orphanTopicsFolderID,
		Label:       "Chủ đề chưa xếp nhóm",
		Kind:        TeacherDashboardNodeKindFolder,
		SubjectID:   subjectID,
		Description: "Những chủ đề chưa thuộc nhóm chương trình nào.",
	}
	lookup[orphanSectionsFolderID] = dashboardNodeMeta{
		ID:          orphanSectionsFolderID,
		Label:       "Học phần chưa xếp nhóm",
		Kind:        TeacherDashboardNodeKindFolder,
		SubjectID:   subjectID,
		Description: "Những học phần chưa nằm trong chủ đề hoặc chương trình nào.",
	}

	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		nodes := make([]dashboardNodeMeta, 0)
		for _, meta := range categoryNodes {
			if strings.TrimSpace(meta.ParentID) == "" {
				nodes = append(nodes, meta)
			}
		}
		for _, meta := range bookSeriesNodes {
			if strings.TrimSpace(meta.ParentID) == "" && strings.TrimSpace(meta.BookSeriesID) == "" {
				nodes = append(nodes, meta)
			}
		}
		hasOrphanTopics := false
		for _, item := range topics {
			if strings.TrimSpace(item.ParentID) == "" && strings.TrimSpace(item.CategoryID) == "" {
				hasOrphanTopics = true
				break
			}
		}
		if hasOrphanTopics {
			nodes = append(nodes, lookup[orphanTopicsFolderID])
		}
		hasOrphanSections := false
		for _, item := range sections {
			if strings.TrimSpace(item.ParentID) == "" &&
				strings.TrimSpace(item.CategoryID) == "" &&
				strings.TrimSpace(item.TopicID) == "" &&
				strings.TrimSpace(item.BookSeriesID) == "" {
				hasOrphanSections = true
				break
			}
		}
		if hasOrphanSections {
			nodes = append(nodes, lookup[orphanSectionsFolderID])
		}
		sortDashboardNodes(nodes)
		return nodes, nil
	}

	if parentID == orphanTopicsFolderID {
		nodes := make([]dashboardNodeMeta, 0)
		for index, item := range topics {
			if strings.TrimSpace(item.ParentID) == "" && strings.TrimSpace(item.CategoryID) == "" {
				nodes = append(nodes, topicNodes[index])
			}
		}
		sortDashboardNodes(nodes)
		return nodes, buildBreadcrumbs(parentID, lookup)
	}

	if parentID == orphanSectionsFolderID {
		nodes := make([]dashboardNodeMeta, 0)
		for index, item := range sections {
			if strings.TrimSpace(item.ParentID) == "" &&
				strings.TrimSpace(item.CategoryID) == "" &&
				strings.TrimSpace(item.TopicID) == "" &&
				strings.TrimSpace(item.BookSeriesID) == "" {
				nodes = append(nodes, sectionNodes[index])
			}
		}
		sortDashboardNodes(nodes)
		return nodes, buildBreadcrumbs(parentID, lookup)
	}

	parentMeta, hasParent := lookup[parentID]
	if !hasParent {
		return nil, nil
	}

	nodes := make([]dashboardNodeMeta, 0)
	for _, meta := range categoryNodes {
		if strings.TrimSpace(meta.ParentID) == parentID {
			nodes = append(nodes, meta)
		}
	}
	for index, item := range bookSeries {
		if strings.TrimSpace(item.ParentID) == parentID || (parentMeta.Kind == TeacherDashboardNodeKindCategory && strings.TrimSpace(item.CategoryID) == parentID) {
			nodes = append(nodes, bookSeriesNodes[index])
		}
	}
	for index, item := range topics {
		if strings.TrimSpace(item.ParentID) == parentID || (parentMeta.Kind == TeacherDashboardNodeKindCategory && strings.TrimSpace(item.CategoryID) == parentID) {
			nodes = append(nodes, topicNodes[index])
		}
	}
	for index, item := range sections {
		if strings.TrimSpace(item.ParentID) == parentID ||
			(parentMeta.Kind == TeacherDashboardNodeKindCategory && strings.TrimSpace(item.CategoryID) == parentID && strings.TrimSpace(item.TopicID) == "") ||
			(parentMeta.Kind == TeacherDashboardNodeKindTopic && strings.TrimSpace(item.TopicID) == parentID) ||
			(parentMeta.Kind == TeacherDashboardNodeKindBookSeries && strings.TrimSpace(item.BookSeriesID) == parentID) {
			nodes = append(nodes, sectionNodes[index])
		}
	}
	sortDashboardNodes(nodes)
	return nodes, buildBreadcrumbs(parentID, lookup)
}

func (s *Service) resourceChildren(ctx context.Context, subjectID, parentID string) []dashboardNodeMeta {
	resources := s.resourcesForSubject(ctx, subjectID)
	nodes := make([]dashboardNodeMeta, 0)
	for _, resource := range resources {
		if !resourceMatchesParent(resource, parentID) {
			continue
		}
		resourceType := strings.TrimSpace(resource.CategoryID)
		if resourceType == "" {
			resourceType = string(resource.SelectedFileType)
		}
		if len(resource.Items) == 0 {
			updatedAt := resource.UpdatedAt
			nodes = append(nodes, dashboardNodeMeta{
				ID:            resource.ID,
				Label:         resource.Title,
				Kind:          TeacherDashboardNodeKindResource,
				ParentID:      parentID,
				SubjectID:     subjectID,
				Description:   resource.Subtitle,
				ResourceID:    resource.ID,
				ResourceType:  resourceType,
				ThumbnailURL:  strings.TrimSpace(resource.ThumbnailURL),
				FileTypeBadge: strings.TrimSpace(resource.FileTypeBadge),
				UpdatedAt:     &updatedAt,
			})
			continue
		}
		for _, item := range resource.Items {
			label := strings.TrimSpace(item.LessonTitle)
			if label == "" {
				label = strings.TrimSpace(item.UnitTitle)
			}
			if label == "" {
				label = resource.Title
			}
			updatedAt := resource.UpdatedAt
			nodes = append(nodes, dashboardNodeMeta{
				ID:            item.ID,
				Label:         label,
				Kind:          TeacherDashboardNodeKindLesson,
				ParentID:      parentID,
				SubjectID:     subjectID,
				Description:   strings.TrimSpace(item.UnitTitle),
				ResourceID:    resource.ID,
				ResourceType:  resourceType,
				ThumbnailURL:  strings.TrimSpace(resource.ThumbnailURL),
				FileTypeBadge: strings.TrimSpace(resource.FileTypeBadge),
				UpdatedAt:     &updatedAt,
			})
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Label < nodes[j].Label })
	return nodes
}

func (s *Service) resourcesForSubject(ctx context.Context, subjectID string) []ResourceDetailDTO {
	if s.repo != nil {
		if resources, _, err := s.repo.ListResources(ctx, s.tenantID(ctx), ListResourceParams{SubjectID: subjectID, Page: 1, Limit: 100}); err == nil {
			for index := range resources {
				if len(resources[index].Items) == 0 {
					items, _ := s.repo.ListResourceItems(ctx, s.tenantID(ctx), resources[index].ID)
					resources[index].Items = items
				}
			}
			return resources
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceDetailDTO, 0)
	for _, resource := range s.resources {
		if resource.SubjectID != subjectID {
			continue
		}
		cp := cloneResource(resource)
		cp.Items = append([]ResourceItemDTO(nil), s.items[resource.ID]...)
		out = append(out, *cp)
	}
	return out
}

func (s *Service) collectLeaves(ctx context.Context, subjectID, nodeID string) []dashboardLeaf {
	subjectID = strings.TrimSpace(subjectID)
	nodeID = strings.TrimSpace(nodeID)
	if subjectID == "" {
		return nil
	}
	if nodeID == "" {
		leaves := make([]dashboardLeaf, 0)
		children, _, _ := s.dashboardChildren(ctx, subjectID, "")
		for _, child := range children {
			leaves = append(leaves, s.collectLeaves(ctx, subjectID, child.ID)...)
		}
		if len(leaves) == 0 {
			resources := s.resourcesForSubject(ctx, subjectID)
			for _, resource := range resources {
				leaves = append(leaves, resourceLeaves(resource)...)
			}
		}
		return dedupeLeaves(leaves)
	}
	children, _, _ := s.dashboardChildren(ctx, subjectID, nodeID)
	if len(children) == 0 {
		if meta, _ := s.lookupNodeMeta(ctx, subjectID, nodeID); meta != nil {
			if meta.Kind == TeacherDashboardNodeKindLesson || meta.Kind == TeacherDashboardNodeKindResource || meta.Kind == TeacherDashboardNodeKindSection {
				return []dashboardLeaf{{
					ID:           meta.ID,
					Label:        meta.Label,
					Kind:         meta.Kind,
					ResourceID:   meta.ResourceID,
					ResourceType: meta.ResourceType,
				}}
			}
		}
		return nil
	}
	leaves := make([]dashboardLeaf, 0)
	for _, child := range children {
		if child.Kind == TeacherDashboardNodeKindLesson || child.Kind == TeacherDashboardNodeKindResource {
			leaves = append(leaves, dashboardLeaf{
				ID:           child.ID,
				Label:        child.Label,
				Kind:         child.Kind,
				ResourceID:   child.ResourceID,
				ResourceType: child.ResourceType,
			})
			continue
		}
		leaves = append(leaves, s.collectLeaves(ctx, subjectID, child.ID)...)
	}
	return dedupeLeaves(leaves)
}

func (s *Service) progressSummaryForNode(ctx context.Context, subjectID, nodeID, schoolID, academicYear string) TeacherProgressSummaryDTO {
	leaves := s.collectLeaves(ctx, subjectID, nodeID)
	return summarizeLeafStatuses(leaves, s.statusByLeaf(ctx, subjectID, schoolID, academicYear))
}

func (s *Service) statusByLeaf(ctx context.Context, subjectID, schoolID, academicYear string) map[string]teachingStatus {
	events := s.progressEventsForScope(ctx, schoolID, academicYear, subjectID)
	latest := map[string]TeacherProgressEventDTO{}
	for _, event := range events {
		if event.EventType == TeacherProgressEventOpen {
			continue
		}
		current, ok := latest[event.NodeID]
		if !ok || current.OccurredAt.Before(event.OccurredAt) {
			latest[event.NodeID] = event
		}
	}
	out := make(map[string]teachingStatus, len(latest))
	for nodeID, event := range latest {
		switch event.EventType {
		case TeacherProgressEventComplete, TeacherProgressEventMarkTaught:
			out[nodeID] = teachingStatus{Status: "taught", Percent: 100}
		case TeacherProgressEventStartTeaching:
			out[nodeID] = teachingStatus{Status: "in_progress", Percent: 50}
		default:
			out[nodeID] = teachingStatus{Status: "pending", Percent: 0}
		}
	}
	return out
}

func (s *Service) progressEventsForScope(ctx context.Context, schoolID, academicYear, subjectID string) []TeacherProgressEventDTO {
	schoolID = strings.TrimSpace(schoolID)
	academicYear = strings.TrimSpace(academicYear)
	subjectID = strings.TrimSpace(subjectID)
	if s.repo != nil {
		if events, err := s.repo.ListProgressEvents(ctx, s.tenantID(ctx), schoolID, academicYear, subjectID); err == nil {
			return events
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TeacherProgressEventDTO, 0, len(s.progressEvents))
	for _, event := range s.progressEvents {
		if schoolID != "" && event.SchoolID != schoolID {
			continue
		}
		if academicYear != "" && event.AcademicYear != academicYear {
			continue
		}
		if subjectID != "" && event.SubjectID != subjectID {
			continue
		}
		out = append(out, event)
	}
	return out
}

func (s *Service) nodeHasChildren(ctx context.Context, subjectID, nodeID string) bool {
	children, _, _ := s.dashboardChildren(ctx, subjectID, nodeID)
	return len(children) > 0
}

func (s *Service) lookupNodeMeta(ctx context.Context, subjectID, nodeID string) (*dashboardNodeMeta, []TeacherDashboardBreadcrumbDTO) {
	children, breadcrumbs, _ := s.dashboardChildren(ctx, subjectID, "")
	for _, child := range children {
		if child.ID == nodeID {
			cp := child
			return &cp, breadcrumbs
		}
	}
	queue := append([]dashboardNodeMeta(nil), children...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.ID == nodeID {
			cp := current
			return &cp, nil
		}
		next, _, _ := s.dashboardChildren(ctx, subjectID, current.ID)
		queue = append(queue, next...)
	}
	return nil, nil
}

func (s *Service) lookupSubjectLabel(subjectID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, subject := range s.subjects {
		if subject.ID == subjectID || subject.Slug == subjectID {
			return subject.Label
		}
	}
	return ""
}

func summarizeLeafStatuses(leaves []dashboardLeaf, statusByLeaf map[string]teachingStatus) TeacherProgressSummaryDTO {
	if len(leaves) == 0 {
		return TeacherProgressSummaryDTO{}
	}
	total := len(leaves)
	taught := 0
	pending := 0
	progressSum := 0.0
	for _, leaf := range leaves {
		status, ok := statusByLeaf[leaf.ID]
		if !ok {
			status = teachingStatus{Status: "pending", Percent: 0}
		}
		progressSum += status.Percent
		if status.Status == "taught" {
			taught++
		} else {
			pending++
		}
	}
	return TeacherProgressSummaryDTO{
		ProgressRate: progressSum / float64(total),
		TaughtCount:  taught,
		TotalCount:   total,
		PendingCount: pending,
	}
}

func latestOpenForNode(events []TeacherProgressEventDTO, nodeID string) *time.Time {
	var latest *time.Time
	for _, event := range events {
		if event.NodeID != nodeID || event.EventType != TeacherProgressEventOpen {
			continue
		}
		if latest == nil || latest.Before(event.OccurredAt) {
			occurredAt := event.OccurredAt
			latest = &occurredAt
		}
	}
	return latest
}

func validTeacherNodeKind(kind TeacherDashboardNodeKind) bool {
	switch kind {
	case TeacherDashboardNodeKindFolder,
		TeacherDashboardNodeKindCategory,
		TeacherDashboardNodeKindSection,
		TeacherDashboardNodeKindBookSeries,
		TeacherDashboardNodeKindTopic,
		TeacherDashboardNodeKindResource,
		TeacherDashboardNodeKindLesson:
		return true
	default:
		return false
	}
}

func filterTaxonomyBySubject(items []TaxonomyOptionDTO, subjectID string) []TaxonomyOptionDTO {
	filtered := make([]TaxonomyOptionDTO, 0, len(items))
	for _, item := range items {
		trimmedSubjectID := strings.TrimSpace(item.SubjectID)
		if trimmedSubjectID == "" {
			continue
		}
		if trimmedSubjectID != subjectID && strings.TrimSpace(item.ID) != subjectID {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func sortTaxonomyOptionsForDashboard(items []TaxonomyOptionDTO) []TaxonomyOptionDTO {
	out := append([]TaxonomyOptionDTO(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return strings.ToLower(strings.TrimSpace(out[i].Label)) < strings.ToLower(strings.TrimSpace(out[j].Label))
	})
	return out
}

func sortDashboardNodes(nodes []dashboardNodeMeta) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return strings.ToLower(strings.TrimSpace(nodes[i].Label)) < strings.ToLower(strings.TrimSpace(nodes[j].Label))
	})
}

func buildBreadcrumbs(parentID string, lookup map[string]dashboardNodeMeta) []TeacherDashboardBreadcrumbDTO {
	if parentID == "" {
		return nil
	}
	currentID := parentID
	chain := make([]TeacherDashboardBreadcrumbDTO, 0)
	for currentID != "" {
		meta, ok := lookup[currentID]
		if !ok {
			break
		}
		chain = append(chain, TeacherDashboardBreadcrumbDTO{
			ID:    meta.ID,
			Label: meta.Label,
			Kind:  meta.Kind,
		})
		currentID = meta.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func resourceMatchesParent(resource ResourceDetailDTO, parentID string) bool {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return strings.TrimSpace(resource.CategoryID) == "" &&
			strings.TrimSpace(resource.SectionID) == "" &&
			strings.TrimSpace(resource.BookSeriesID) == "" &&
			strings.TrimSpace(resource.TopicID) == ""
	}
	return resource.CategoryID == parentID ||
		resource.SectionID == parentID ||
		resource.BookSeriesID == parentID ||
		resource.TopicID == parentID
}

func resourceLeaves(resource ResourceDetailDTO) []dashboardLeaf {
	resourceType := strings.TrimSpace(resource.CategoryID)
	if resourceType == "" {
		resourceType = string(resource.SelectedFileType)
	}
	if len(resource.Items) == 0 {
		return []dashboardLeaf{{
			ID:           resource.ID,
			Label:        resource.Title,
			Kind:         TeacherDashboardNodeKindResource,
			ResourceID:   resource.ID,
			ResourceType: resourceType,
		}}
	}
	leaves := make([]dashboardLeaf, 0, len(resource.Items))
	for _, item := range resource.Items {
		label := strings.TrimSpace(item.LessonTitle)
		if label == "" {
			label = strings.TrimSpace(item.UnitTitle)
		}
		if label == "" {
			label = fmt.Sprintf("%s - bài %d", resource.Title, item.SortOrder)
		}
		leaves = append(leaves, dashboardLeaf{
			ID:           item.ID,
			Label:        label,
			Kind:         TeacherDashboardNodeKindLesson,
			ResourceID:   resource.ID,
			ResourceType: resourceType,
		})
	}
	return leaves
}

func dedupeLeaves(leaves []dashboardLeaf) []dashboardLeaf {
	seen := make(map[string]bool, len(leaves))
	out := make([]dashboardLeaf, 0, len(leaves))
	for _, leaf := range leaves {
		if seen[leaf.ID] {
			continue
		}
		seen[leaf.ID] = true
		out = append(out, leaf)
	}
	return out
}
