package hoclieu

import (
	"context"
	"testing"
)

func TestSeedIncludesERG81ProgramsAndFileTypes(t *testing.T) {
	svc := NewService()
	seedService(svc)
	ctx := context.Background()

	wantPrograms := map[string]bool{
		"global-success":       false,
		"giao-duc-stem":        false,
		"ic3-digital-literacy": false,
		"mos-office-skills":    false,
		"tin-hoc-pho-thong":    false,
	}
	for _, program := range svc.Programs(ctx) {
		if _, ok := wantPrograms[program.Slug]; ok {
			wantPrograms[program.Slug] = true
		}
	}
	for slug, found := range wantPrograms {
		if !found {
			t.Fatalf("seed program %q not found", slug)
		}
	}

	wantFileTypes := map[AssetFileType]bool{
		AssetFileTypePDF:   false,
		AssetFileTypePPTX:  false,
		AssetFileTypeVideo: false,
		AssetFileTypeAudio: false,
		AssetFileTypeQuiz:  false,
		AssetFileTypeLink:  false,
	}
	items, total := svc.ListResources(ctx, ListResourceParams{Limit: 100})
	if total < int64(len(wantFileTypes)) {
		t.Fatalf("expected enough seed resources for file type coverage, got total=%d", total)
	}
	for _, item := range items {
		if _, ok := wantFileTypes[item.SelectedFileType]; ok {
			wantFileTypes[item.SelectedFileType] = true
		}
	}
	for fileType, found := range wantFileTypes {
		if !found {
			t.Fatalf("seed file type %q not found in resource list", fileType)
		}
	}
}

func TestSeedResourceListHappyPathForProgramFilters(t *testing.T) {
	svc := NewService()
	seedService(svc)
	ctx := context.Background()

	cases := []struct {
		name        string
		programSlug string
		fileType    AssetFileType
	}{
		{name: "global success audio", programSlug: "global-success", fileType: AssetFileTypeAudio},
		{name: "stem video", programSlug: "giao-duc-stem", fileType: AssetFileTypeVideo},
		{name: "ic3 quiz", programSlug: "ic3-digital-literacy", fileType: AssetFileTypeQuiz},
		{name: "tin hoc link", programSlug: "tin-hoc-pho-thong", fileType: AssetFileTypeLink},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			items, total := svc.ListResources(ctx, ListResourceParams{ProgramSlug: tt.programSlug, FileType: tt.fileType, Limit: 20})
			if total == 0 || len(items) == 0 {
				t.Fatalf("expected seed resources for program=%s fileType=%s", tt.programSlug, tt.fileType)
			}
			for _, item := range items {
				if item.ProgramSlug != tt.programSlug || item.SelectedFileType != tt.fileType {
					t.Fatalf("resource escaped filter: %+v", item)
				}
				if item.FileTypeBadge == "" || item.LaunchMode == "" {
					t.Fatalf("resource missing FE contract fields: %+v", item)
				}
			}
		})
	}
}

func TestSeedContentModelRelationshipsAreConsistent(t *testing.T) {
	svc := NewService()
	seedService(svc)
	ctx := context.Background()
	model := svc.Taxonomy(ctx)

	if len(model.Programs) == 0 {
		t.Fatalf("expected programs to be exposed in content model")
	}

	subjects := taxonomyIDs(model.Subjects)
	grades := taxonomyIDs(model.Grades)
	categories := taxonomyIDs(model.Categories)
	sections := taxonomyIDs(model.Sections)
	bookSeries := taxonomyIDs(model.BookSeries)
	topics := taxonomyIDs(model.Topics)
	topicByID := taxonomyByID(model.Topics)

	for _, program := range model.Programs {
		if program.ID == "" || program.Slug == "" {
			t.Fatalf("program missing stable ID/slug: %+v", program)
		}
		for _, subjectID := range program.SubjectIDs {
			if !subjects[subjectID] {
				t.Fatalf("program %s points to missing subject %s", program.Slug, subjectID)
			}
		}
		for _, gradeID := range program.GradeIDs {
			if !grades[gradeID] {
				t.Fatalf("program %s points to missing grade %s", program.Slug, gradeID)
			}
		}
	}

	for _, series := range model.BookSeries {
		if series.SubjectID == "" || !subjects[series.SubjectID] {
			t.Fatalf("book series %s points to missing subject %s", series.ID, series.SubjectID)
		}
		for _, levelID := range series.LevelIDs {
			if !grades[levelID] {
				t.Fatalf("book series %s points to missing level %s", series.ID, levelID)
			}
		}
	}

	for _, topic := range model.Topics {
		if topic.SubjectID == "" || !subjects[topic.SubjectID] {
			t.Fatalf("topic %s points to missing subject %s", topic.ID, topic.SubjectID)
		}
		if topic.GradeID == "" || !grades[topic.GradeID] {
			t.Fatalf("topic %s points to missing grade %s", topic.ID, topic.GradeID)
		}
		if topic.CategoryID == "" || !categories[topic.CategoryID] {
			t.Fatalf("topic %s points to missing category %s", topic.ID, topic.CategoryID)
		}
		if topic.BookSeriesID != "" && !bookSeries[topic.BookSeriesID] {
			t.Fatalf("topic %s points to missing book series %s", topic.ID, topic.BookSeriesID)
		}
		if topic.ParentID != "" && !topics[topic.ParentID] {
			t.Fatalf("topic %s points to missing parent %s", topic.ID, topic.ParentID)
		}
	}

	resources, _ := svc.ListResources(ctx, ListResourceParams{Limit: 100})
	if len(resources) == 0 {
		t.Fatalf("expected seed resources")
	}
	for _, resource := range resources {
		if !subjects[resource.SubjectID] {
			t.Fatalf("resource %s points to missing subject %s", resource.ID, resource.SubjectID)
		}
		if resource.GradeID != "" && !grades[resource.GradeID] {
			t.Fatalf("resource %s points to missing grade %s", resource.ID, resource.GradeID)
		}
		if !categories[resource.CategoryID] {
			t.Fatalf("resource %s points to missing category %s", resource.ID, resource.CategoryID)
		}
		if resource.SectionID != "" && !sections[resource.SectionID] {
			t.Fatalf("resource %s points to missing section %s", resource.ID, resource.SectionID)
		}
		if resource.BookSeriesID != "" && !bookSeries[resource.BookSeriesID] {
			t.Fatalf("resource %s points to missing book series %s", resource.ID, resource.BookSeriesID)
		}
		if resource.TopicID != "" && !topics[resource.TopicID] {
			t.Fatalf("resource %s points to missing topic %s", resource.ID, resource.TopicID)
		}
		if resource.TopicID != "" {
			topic := topicByID[resource.TopicID]
			if topic.SubjectID != resource.SubjectID {
				t.Fatalf("resource %s topic subject mismatch: resource=%s topic=%s", resource.ID, resource.SubjectID, topic.SubjectID)
			}
			if resource.GradeID != "" && topic.GradeID != resource.GradeID {
				t.Fatalf("resource %s topic grade mismatch: resource=%s topic=%s", resource.ID, resource.GradeID, topic.GradeID)
			}
			if topic.CategoryID != resource.CategoryID {
				t.Fatalf("resource %s topic category mismatch: resource=%s topic=%s", resource.ID, resource.CategoryID, topic.CategoryID)
			}
			if resource.BookSeriesID != "" && topic.BookSeriesID != resource.BookSeriesID {
				t.Fatalf("resource %s topic book series mismatch: resource=%s topic=%s", resource.ID, resource.BookSeriesID, topic.BookSeriesID)
			}
		}
		if resource.LevelID != "" && !grades[resource.LevelID] {
			t.Fatalf("resource %s points to missing level %s", resource.ID, resource.LevelID)
		}
		if resource.GradeID != "" && resource.LevelID != "" && resource.GradeID != resource.LevelID {
			t.Fatalf("resource %s grade/level mismatch: grade=%s level=%s", resource.ID, resource.GradeID, resource.LevelID)
		}
		if resource.DocumentTypeID != "" && !categories[resource.DocumentTypeID] {
			t.Fatalf("resource %s points to missing document type %s", resource.ID, resource.DocumentTypeID)
		}
		detail, err := svc.Resource(ctx, resource.ID)
		if err != nil {
			t.Fatalf("resource %s detail missing: %v", resource.ID, err)
		}
		assetIDs := make(map[string]bool, len(detail.Assets))
		for _, asset := range detail.Assets {
			if asset.ResourceID != resource.ID {
				t.Fatalf("asset %s points to resource %s, want %s", asset.ID, asset.ResourceID, resource.ID)
			}
			assetIDs[asset.ID] = true
		}
		for _, item := range detail.Items {
			if item.ResourceID != resource.ID {
				t.Fatalf("item %s points to resource %s, want %s", item.ID, item.ResourceID, resource.ID)
			}
			if item.AssetID != "" && !assetIDs[item.AssetID] {
				t.Fatalf("item %s points to missing asset %s on resource %s", item.ID, item.AssetID, resource.ID)
			}
		}
	}
}

func taxonomyIDs(options []TaxonomyOptionDTO) map[string]bool {
	out := make(map[string]bool, len(options))
	for _, option := range options {
		out[option.ID] = true
	}
	return out
}

func taxonomyByID(options []TaxonomyOptionDTO) map[string]TaxonomyOptionDTO {
	out := make(map[string]TaxonomyOptionDTO, len(options))
	for _, option := range options {
		out[option.ID] = option
	}
	return out
}
