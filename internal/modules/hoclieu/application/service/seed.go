package service

import (
	"time"

	. "erg.ninja/internal/modules/hoclieu/api/dto"
)

func SeedService(s *Service) {
	now := time.Now().UTC()
	s.programs = []ProgramDTO{
		{ID: "program-global-success", Slug: "global-success", Name: "Global Success", ShortName: "GS", SubjectIDs: []string{"tieng-anh"}, GradeIDs: []string{"1", "2", "3", "4", "5", "6", "7"}, Description: "Sach mem, hop phan bo tro, audio, video va lesson kit cho giao vien Tieng Anh."},
		{ID: "program-ic3", Slug: "ic3-digital-literacy", Name: "IC3 Digital Literacy", ShortName: "IC3", SubjectIDs: []string{"ic3-gs6", "ic3", "tin-hoc"}, GradeIDs: []string{"6", "7", "8", "9", "10", "11", "12"}, Description: "Hoc lieu Computing Fundamentals, Key Applications va Living Online."},
		{ID: "program-mos", Slug: "mos-office-skills", Name: "MOS Office Skills", ShortName: "MOS", SubjectIDs: []string{"mos", "tin-hoc"}, GradeIDs: []string{"6", "7", "8", "9", "10", "11", "12"}, Description: "Word, Excel, PowerPoint voi bai giang, file thuc hanh va cau hoi theo objective."},
		{ID: "program-computing", Slug: "tin-hoc-pho-thong", Name: "Tin hoc pho thong", ShortName: "Tin hoc", SubjectIDs: []string{"tin-hoc"}, GradeIDs: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, Description: "Scratch, Python va hoat dong thuc hanh trong lop."},
		{ID: "program-stem", Slug: "giao-duc-stem", Name: "Giao duc STEM", ShortName: "STEM", SubjectIDs: []string{"giao-duc-stem"}, GradeIDs: []string{"1", "2", "3", "4", "5"}, Description: "Ke hoach day hoc, bai giang dien tu, phieu hoc tap va san pham mau STEM."},
	}
	s.grades = []TaxonomyOptionDTO{
		{ID: "mam-non", Label: "Mam non", Slug: "mam-non"},
		{ID: "1", Label: "Lop 1", Slug: "lop-1"},
		{ID: "2", Label: "Lop 2", Slug: "lop-2"},
		{ID: "3", Label: "Lop 3", Slug: "lop-3"},
		{ID: "4", Label: "Lop 4", Slug: "lop-4"},
		{ID: "5", Label: "Lop 5", Slug: "lop-5"},
		{ID: "6", Label: "Lop 6", Slug: "lop-6"},
		{ID: "7", Label: "Lop 7", Slug: "lop-7"},
		{ID: "8", Label: "Lop 8", Slug: "lop-8"},
		{ID: "9", Label: "Lop 9", Slug: "lop-9"},
		{ID: "10", Label: "Lop 10", Slug: "lop-10"},
		{ID: "11", Label: "Lop 11", Slug: "lop-11"},
		{ID: "12", Label: "Lop 12", Slug: "lop-12"},
	}
	s.subjects = []TaxonomyOptionDTO{
		{ID: "toan", Label: "Toan", Slug: "toan"},
		{ID: "tieng-viet", Label: "Tieng Viet", Slug: "tieng-viet"},
		{ID: "ngu-van", Label: "Ngu van", Slug: "ngu-van"},
		{ID: "tieng-anh", Label: "Tieng Anh", Slug: "tieng-anh"},
		{ID: "khoa-hoc-tu-nhien", Label: "Khoa hoc tu nhien", Slug: "khoa-hoc-tu-nhien"},
		{ID: "lich-su-dia-li", Label: "Lich su va Dia li", Slug: "lich-su-dia-li"},
		{ID: "giao-duc-stem", Label: "Giao duc STEM", Slug: "giao-duc-stem"},
		{ID: "tin-hoc", Label: "Tin hoc", Slug: "tin-hoc"},
		{ID: "ic3", Label: "IC3", Slug: "ic3"},
		{ID: "ic3-gs6", Label: "IC3 GS6", Slug: "ic3-gs6", Description: "Mon hoc IC3 GS6 gom 3 nhom hoc lieu Level 1, Level 2 va Level 3."},
		{ID: "mos", Label: "MOS", Slug: "mos"},
	}
	s.categories = []TaxonomyOptionDTO{
		{ID: "textbook", Label: "Sach mem", Slug: "sach-mem"},
		{ID: "program-distribution", Label: "Ke hoach day hoc", Slug: "ke-hoach-day-hoc"},
		{ID: "lecture-bank", Label: "Bai giang dien tu", Slug: "bai-giang-dien-tu"},
		{ID: "lesson-plan", Label: "Giao an", Slug: "giao-an"},
		{ID: "worksheet", Label: "Phieu hoc tap", Slug: "phieu-hoc-tap"},
		{ID: "video", Label: "Video", Slug: "video"},
		{ID: "audio", Label: "Audio", Slug: "audio"},
		{ID: "quiz-bank", Label: "Quiz bank", Slug: "quiz-bank"},
		{ID: "practice-file", Label: "File thuc hanh", Slug: "file-thuc-hanh"},
		{ID: "link", Label: "Lien ket ngoai", Slug: "lien-ket-ngoai"},
		{ID: "ic3-gs6-level-1", Label: "Level 1", Slug: "level-1", SubjectID: "ic3-gs6", SortOrder: 1, Status: "active", Description: "Nhom hoc lieu Level 1."},
		{ID: "ic3-gs6-level-2", Label: "Level 2", Slug: "level-2", SubjectID: "ic3-gs6", SortOrder: 2, Status: "active", Description: "Nhom hoc lieu Level 2."},
		{ID: "ic3-gs6-level-3", Label: "Level 3", Slug: "level-3", SubjectID: "ic3-gs6", SortOrder: 3, Status: "active", Description: "Nhom hoc lieu Level 3."},
	}
	s.sections = []TaxonomyOptionDTO{
		{ID: "student-book", Label: "Sach hoc sinh", Slug: "sach-hoc-sinh", ParentID: "textbook"},
		{ID: "workbook", Label: "Sach bai tap", Slug: "sach-bai-tap", ParentID: "textbook"},
		{ID: "support-components", Label: "Hop phan bo tro", Slug: "hop-phan-bo-tro"},
	}
	s.bookSeries = []TaxonomyOptionDTO{
		{ID: "global-success", Label: "Global Success", Slug: "global-success", SubjectID: "tieng-anh", LevelIDs: []string{"1", "2", "3", "4", "5", "6", "7"}, Status: "active", SortOrder: 1},
		{ID: "stem-hanh-trinh-sang-tao", Label: "STEM - Hanh trinh sang tao", Slug: "stem-hanh-trinh-sang-tao", SubjectID: "giao-duc-stem", LevelIDs: []string{"1", "2", "3", "4", "5"}, Status: "active", SortOrder: 2},
		{ID: "ic3-gs6", Label: "IC3 GS6", Slug: "ic3-gs6", SubjectID: "ic3", LevelIDs: []string{"6", "7", "8"}, Status: "active", SortOrder: 3, Description: "Lo trinh IC3 GS6 gom 3 level, moi level co nhom chu de rieng."},
	}
	s.topics = []TaxonomyOptionDTO{
		{ID: "ta1-playground", Label: "Unit 1: In the school playground", Slug: "unit-1-in-the-school-playground", SubjectID: "tieng-anh", GradeID: "1", CategoryID: "lecture-bank", BookSeriesID: "global-success", SortOrder: 1, Status: "active", Depth: 1},
		{ID: "ta1-dining-room", Label: "Unit 2: In the dining room", Slug: "unit-2-in-the-dining-room", SubjectID: "tieng-anh", GradeID: "1", CategoryID: "lecture-bank", BookSeriesID: "global-success", SortOrder: 2, Status: "active", Depth: 1},
		{ID: "ta7-my-new-school", Label: "Unit 1: My new school", Slug: "unit-1-my-new-school", SubjectID: "tieng-anh", GradeID: "7", CategoryID: "lecture-bank", BookSeriesID: "global-success", SortOrder: 1, Status: "active", Depth: 1},
		{ID: "stem1-dream-house", Label: "Bai 1: Ngoi nha mo uoc", Slug: "bai-1-ngoi-nha-mo-uoc", SubjectID: "giao-duc-stem", GradeID: "1", CategoryID: "lecture-bank", BookSeriesID: "stem-hanh-trinh-sang-tao", SortOrder: 1, Status: "active", Depth: 1},
		{ID: "ic3-gs6-level-1", Label: "Level 1", Slug: "level-1", SubjectID: "ic3", GradeID: "6", CategoryID: "lecture-bank", BookSeriesID: "ic3-gs6", SortOrder: 1, Status: "active", Depth: 1, Description: "Làm quen với thiết bị số, Internet an toàn và kỹ năng thao tác cơ bản."},
		{ID: "ic3-gs6-level-2", Label: "Level 2", Slug: "level-2", SubjectID: "ic3", GradeID: "7", CategoryID: "lecture-bank", BookSeriesID: "ic3-gs6", SortOrder: 2, Status: "active", Depth: 1, Description: "Ứng dụng văn phòng và làm việc số."},
		{ID: "ic3-gs6-level-3", Label: "Level 3", Slug: "level-3", SubjectID: "ic3", GradeID: "8", CategoryID: "lecture-bank", BookSeriesID: "ic3-gs6", SortOrder: 3, Status: "active", Depth: 1, Description: "Bảo mật, dữ liệu và mô phỏng kiểm tra IC3."},
	}
	s.sections = append(s.sections,
		TaxonomyOptionDTO{ID: "ic3-gs6-level-1-lesson-1", Label: "Bài 01. Thiết bị số và thao tác cơ bản", Slug: "bai-01-thiet-bi-so-va-thao-tac-co-ban", SubjectID: "ic3-gs6", GradeID: "6", CategoryID: "ic3-gs6-level-1", SortOrder: 1, Status: "active", Description: "Bài học mở đầu cho Level 1."},
		TaxonomyOptionDTO{ID: "ic3-gs6-level-1-lesson-2", Label: "Bài 02. Internet an toàn", Slug: "bai-02-internet-an-toan", SubjectID: "ic3-gs6", GradeID: "6", CategoryID: "ic3-gs6-level-1", SortOrder: 2, Status: "active"},
		TaxonomyOptionDTO{ID: "ic3-gs6-level-2-lesson-1", Label: "Bài 01. Excel nhập môn", Slug: "bai-01-excel-nhap-mon", SubjectID: "ic3-gs6", GradeID: "7", CategoryID: "ic3-gs6-level-2", SortOrder: 1, Status: "active", Description: "Bài học mở đầu cho Level 2."},
		TaxonomyOptionDTO{ID: "ic3-gs6-level-2-lesson-2", Label: "Bài 02. PowerPoint cơ bản", Slug: "bai-02-powerpoint-co-ban", SubjectID: "ic3-gs6", GradeID: "7", CategoryID: "ic3-gs6-level-2", SortOrder: 2, Status: "active"},
		TaxonomyOptionDTO{ID: "ic3-gs6-level-3-lesson-1", Label: "Bài 01. Bảo mật tài khoản", Slug: "bai-01-bao-mat-tai-khoan", SubjectID: "ic3-gs6", GradeID: "8", CategoryID: "ic3-gs6-level-3", SortOrder: 1, Status: "active", Description: "Bài học mở đầu cho Level 3."},
		TaxonomyOptionDTO{ID: "ic3-gs6-level-3-lesson-2", Label: "Bài 02. Kiểm tra mô phỏng", Slug: "bai-02-kiem-tra-mo-phong", SubjectID: "ic3-gs6", GradeID: "8", CategoryID: "ic3-gs6-level-3", SortOrder: 2, Status: "active"},
	)
	s.designerPresets = []LectureDesignerPresetDTO{
		{ID: "hoclieu-lecture-grid", Name: "Bai giang dien tu", Description: "Hero lon, nen chu de va danh sach unit 2 cot nhu hoclieu.vn.", AccentColor: "#0891b2", Layout: "hero-unit-grid"},
		{ID: "ebook-clean", Name: "Sach dien tu", Description: "Card sach gon, uu tien bia sach va thong tin giao vien.", AccentColor: "#1d4ed8", Layout: "ebook-card-grid"},
	}

	s.addSeedResource(ResourceDetailDTO{
		ResourceCardDTO: ResourceCardDTO{
			ID:               "res-global-success-7-student-book",
			Slug:             "global-success-7-student-book",
			Title:            "Tieng Anh 7 - Global Success",
			Subtitle:         "Sach hoc sinh",
			ThumbnailURL:     "/mock/hoclieu/global-success-7-student-book.png",
			ProgramSlug:      "global-success",
			SubjectID:        "tieng-anh",
			GradeID:          "7",
			CategoryID:       "textbook",
			SectionID:        "student-book",
			BookSeriesID:     "global-success",
			LevelID:          "7",
			DocumentTypeID:   "textbook",
			SelectedFileType: AssetFileTypePDF,
			FileTypeBadge:    "PDF",
			LaunchMode:       LaunchModePDFReader,
			PriceType:        "free",
			AccessState:      "open",
			CanDownload:      false,
			UpdatedAt:        now.Add(-1 * time.Hour),
		},
		Description: "Mock PDF sach hoc sinh de FE kiem tra viewer full man hinh va badge PDF.",
		Tags:        []string{"global-success", "grade-7", "pdf"},
	}, AssetRecord{
		AssetDTO: AssetDTO{
			ID:               "asset-global-success-7-student-book-pdf",
			ResourceID:       "res-global-success-7-student-book",
			Title:            "Tieng Anh 7 - Global Success",
			SelectedFileType: AssetFileTypePDF,
			FileTypeBadge:    "PDF",
			LaunchMode:       LaunchModePDFReader,
			OriginalFileName: "global-success-7-student-book.pdf",
			FileSizeBytes:    2145024,
			StorageProvider:  "gdrive",
			Status:           "ready",
			CanDownload:      false,
			UpdatedAt:        now.Add(-1 * time.Hour),
		},
		UpstreamURL: "https://drive.google.com/file/d/13pG_0Kra-K6iQheqSkHmVlq0Jtnjmx8I/view",
	}, []ViewerPageDTO{
		{Index: 1, Title: "Cover", Width: 960, Height: 1280},
		{Index: 2, Title: "Unit 1", Width: 960, Height: 1280},
	})

	s.addSeedResource(ResourceDetailDTO{
		ResourceCardDTO: ResourceCardDTO{
			ID:               "res-global-success-7-lecture-bank",
			Slug:             "global-success-7-lecture-bank",
			Title:            "Bai giang dien tu - Tieng Anh 7",
			Subtitle:         "Unit/Lesson presentation",
			ThumbnailURL:     "/mock/hoclieu/global-success-7-lecture.png",
			ProgramSlug:      "global-success",
			SubjectID:        "tieng-anh",
			GradeID:          "7",
			CategoryID:       "lecture-bank",
			SectionID:        "support-components",
			BookSeriesID:     "global-success",
			TopicID:          "ta7-my-new-school",
			LevelID:          "7",
			DocumentTypeID:   "lecture-bank",
			SelectedFileType: AssetFileTypePPTX,
			FileTypeBadge:    "PPTX",
			LaunchMode:       LaunchModeSlideImageProxy,
			PriceType:        "free",
			AccessState:      "open",
			CanDownload:      false,
			UpdatedAt:        now.Add(-2 * time.Hour),
		},
		Description: "Mock PPTX de FE dung man hinh danh sach unit truoc khi mo trinh chieu.",
		Tags:        []string{"global-success", "lecture", "pptx"},
		LectureDesign: &LectureDesignDTO{
			TemplateID:     "hoclieu-lecture-grid",
			BannerTitle:    "Tieng Anh 1",
			BannerSubtitle: "BAI GIANG DIEN TU",
			AccentColor:    "#0891b2",
			SecondaryColor: "#cffafe",
			ItemColumns:    2,
			ShowDownload:   true,
			UnitLabels:     []string{"Unit 1: In the school playground", "Unit 2: In the dining room", "Unit 3: At the street market"},
		},
	}, AssetRecord{
		AssetDTO: AssetDTO{
			ID:               "asset-global-success-7-lecture-pptx",
			ResourceID:       "res-global-success-7-lecture-bank",
			Title:            "Lesson 1 - GETTING STARTED",
			SelectedFileType: AssetFileTypePPTX,
			FileTypeBadge:    "PPTX",
			LaunchMode:       LaunchModeSlideImageProxy,
			OriginalFileName: "lesson-1-getting-started.pptx",
			FileSizeBytes:    7356416,
			StorageProvider:  "google_slides",
			Status:           "ready",
			CanDownload:      false,
			UpdatedAt:        now.Add(-2 * time.Hour),
		},
		UpstreamURL: "https://docs.google.com/presentation/d/1dDhYpe7Ri4mDM3VR0LpDkalzpJDhDkMm/edit",
	}, []ViewerPageDTO{
		{Index: 1, Title: "Unit 1 - My new school", Width: 1600, Height: 900},
		{Index: 2, Title: "Lesson 1 - GETTING STARTED", Width: 1600, Height: 900},
		{Index: 3, Title: "A special day", Width: 1600, Height: 900},
	})
	s.items["res-global-success-7-lecture-bank"] = []ResourceItemDTO{
		{ID: "item-ta7-u1-l1", ResourceID: "res-global-success-7-lecture-bank", AssetID: "asset-global-success-7-lecture-pptx", UnitTitle: "Unit 1: My new school", LessonTitle: "Lesson 1 - GETTING STARTED", SortOrder: 1, PageCount: 3},
		{ID: "item-ta7-u1-l2", ResourceID: "res-global-success-7-lecture-bank", AssetID: "asset-global-success-7-lecture-pptx", UnitTitle: "Unit 1: My new school", LessonTitle: "Lesson 2 - A CLOSER LOOK 1", SortOrder: 2, PageCount: 4},
		{ID: "item-ta7-u2", ResourceID: "res-global-success-7-lecture-bank", AssetID: "asset-global-success-7-lecture-pptx", UnitTitle: "Unit 2: My house", SortOrder: 3, PageCount: 4},
		{ID: "item-ta7-u3", ResourceID: "res-global-success-7-lecture-bank", AssetID: "asset-global-success-7-lecture-pptx", UnitTitle: "Unit 3: My friends", SortOrder: 4, PageCount: 5},
	}

	seedSimpleResource(s, now, "res-ic3-gs6-living-online", "ic3-gs6-living-online", "IC3 GS6 - Living Online", "Chu de Internet an toan va cong dan so co ban.", "ic3-digital-literacy", "ic3", "6", "lecture-bank", AssetFileTypePPTX)
	seedSimpleResource(s, now, "res-ic3-gs6-level-1-slide", "ic3-gs6-level-1-slide", "IC3 GS6 Level 1 - Thiết bị số và Internet an toàn", "Bài giảng cho Level 1.", "ic3-digital-literacy", "ic3-gs6", "6", "ic3-gs6-level-1", AssetFileTypePPTX)
	seedSimpleResource(s, now, "res-ic3-gs6-level-2-slide", "ic3-gs6-level-2-slide", "IC3 GS6 Level 2 - Excel nhập môn", "Bài giảng cho Level 2.", "ic3-digital-literacy", "ic3-gs6", "7", "ic3-gs6-level-2", AssetFileTypePPTX)
	seedSimpleResource(s, now, "res-ic3-gs6-level-3-quiz", "ic3-gs6-level-3-quiz", "IC3 GS6 Level 3 - Kiểm tra mô phỏng", "Quiz cho Level 3.", "ic3-digital-literacy", "ic3-gs6", "8", "ic3-gs6-level-3", AssetFileTypeQuiz)
	seedSimpleResource(s, now, "res-mos-excel-practice", "mos-excel-practice", "MOS Excel - Practice file", "Workbook va file thuc hanh", "mos-office-skills", "mos", "8", "practice-file", AssetFileTypeXLSX)
	seedSimpleResource(s, now, "res-tin-hoc-python-starter", "tin-hoc-python-starter", "Python Starter Kit", "Bai tap Python cho lop Tin hoc", "tin-hoc-pho-thong", "tin-hoc", "8", "practice-file", AssetFileTypeZIP)
	seedSimpleResource(s, now, "res-stem-1-lesson-plan", "stem-1-lesson-plan", "Giao duc STEM 1 - Ke hoach bai day", "Hanh trinh sang tao", "giao-duc-stem", "giao-duc-stem", "1", "lesson-plan", AssetFileTypePDF)
	seedSimpleResource(s, now, "res-ic3-cf-quiz", "ic3-cf-quiz", "IC3 Computing Fundamentals - Quiz", "Cau hoi on tap theo objective cua Level 1.", "ic3-digital-literacy", "ic3", "6", "quiz-bank", AssetFileTypeQuiz)
	seedSimpleResource(s, now, "res-global-success-7-audio-unit-1", "global-success-7-audio-unit-1", "Global Success 7 - Audio Unit 1", "Audio nghe va luyen phat am", "global-success", "tieng-anh", "7", "audio", AssetFileTypeAudio)
	seedSimpleResource(s, now, "res-stem-2-video-robot-milo", "stem-2-video-robot-milo", "STEM 2 - Robot Milo demo", "Video mo phong hoat dong STEM tren lop", "giao-duc-stem", "giao-duc-stem", "2", "video", AssetFileTypeVideo)
	seedSimpleResource(s, now, "res-tin-hoc-scratch-link", "tin-hoc-scratch-link", "Tin hoc pho thong - Scratch Studio", "Lien ket hoc lieu Scratch thuc hanh", "tin-hoc-pho-thong", "tin-hoc", "5", "link", AssetFileTypeLink)
}

func (s *Service) addSeedResource(resource ResourceDetailDTO, asset AssetRecord, pages []ViewerPageDTO) {
	resource.Assets = []AssetDTO{asset.AssetDTO}
	resource.Items = s.items[resource.ID]
	s.resources[resource.ID] = &resource
	s.assets[asset.ID] = &asset
	if pages != nil {
		s.pages[asset.ID] = append([]ViewerPageDTO(nil), pages...)
	}
}

func seedSimpleResource(s *Service, now time.Time, id, slug, title, subtitle, program, subject, grade, category string, fileType AssetFileType) {
	assetID := "asset-" + slug
	canDownload := fileType != AssetFileTypePDF && fileType != AssetFileTypePPTX
	bookSeriesID := seedBookSeriesFor(program)
	if subject == "ic3-gs6" {
		bookSeriesID = ""
	}
	sectionID := seedSectionFor(subject, grade, category)
	if sectionID == "" {
		sectionID = "support-components"
	}
	topicID := seedTopicFor(subject, grade, category, bookSeriesID)
	s.addSeedResource(ResourceDetailDTO{
		ResourceCardDTO: ResourceCardDTO{
			ID:               id,
			Slug:             slug,
			Title:            title,
			Subtitle:         subtitle,
			ThumbnailURL:     "/mock/hoclieu/" + slug + ".png",
			ProgramSlug:      program,
			SubjectID:        subject,
			GradeID:          grade,
			CategoryID:       category,
			SectionID:        sectionID,
			BookSeriesID:     bookSeriesID,
			TopicID:          topicID,
			LevelID:          grade,
			DocumentTypeID:   category,
			SelectedFileType: fileType,
			FileTypeBadge:    string(fileType),
			LaunchMode:       defaultLaunchMode(fileType),
			PriceType:        "free",
			AccessState:      "open",
			CanDownload:      canDownload,
			UpdatedAt:        now.Add(-3 * time.Hour),
		},
		Description: subtitle,
		Tags:        []string{program, subject, string(fileType)},
	}, AssetRecord{
		AssetDTO: AssetDTO{
			ID:               assetID,
			ResourceID:       id,
			Title:            title,
			SelectedFileType: fileType,
			FileTypeBadge:    string(fileType),
			LaunchMode:       defaultLaunchMode(fileType),
			OriginalFileName: slug + "." + extensionFor(fileType),
			FileSizeBytes:    1024 * 1024,
			StorageProvider:  "r2",
			Status:           "ready",
			CanDownload:      canDownload,
			UpdatedAt:        now.Add(-3 * time.Hour),
		},
	}, []ViewerPageDTO{{Index: 1, Title: title, Width: 1280, Height: 720}})
}

func seedBookSeriesFor(programSlug string) string {
	switch programSlug {
	case "global-success":
		return "global-success"
	case "giao-duc-stem":
		return "stem-hanh-trinh-sang-tao"
	case "ic3-digital-literacy":
		return "ic3-gs6"
	default:
		return ""
	}
}

func seedTopicFor(subjectID, gradeID, categoryID, bookSeriesID string) string {
	switch {
	case subjectID == "tieng-anh" && gradeID == "1" && categoryID == "lecture-bank" && bookSeriesID == "global-success":
		return "ta1-playground"
	case subjectID == "giao-duc-stem" && gradeID == "1" && categoryID == "lecture-bank" && bookSeriesID == "stem-hanh-trinh-sang-tao":
		return "stem1-dream-house"
	case subjectID == "ic3" && gradeID == "6" && categoryID == "lecture-bank" && bookSeriesID == "ic3-gs6":
		return "ic3-gs6-level-1"
	default:
		return ""
	}
}

func seedSectionFor(subjectID, gradeID, categoryID string) string {
	switch {
	case subjectID == "ic3-gs6" && gradeID == "6" && categoryID == "ic3-gs6-level-1":
		return "ic3-gs6-level-1-lesson-1"
	case subjectID == "ic3-gs6" && gradeID == "7" && categoryID == "ic3-gs6-level-2":
		return "ic3-gs6-level-2-lesson-1"
	case subjectID == "ic3-gs6" && gradeID == "8" && categoryID == "ic3-gs6-level-3":
		return "ic3-gs6-level-3-lesson-2"
	default:
		return ""
	}
}

func extensionFor(fileType AssetFileType) string {
	switch fileType {
	case AssetFileTypePDF:
		return "pdf"
	case AssetFileTypePPTX:
		return "pptx"
	case AssetFileTypeXLSX:
		return "xlsx"
	case AssetFileTypeZIP:
		return "zip"
	case AssetFileTypeVideo:
		return "mp4"
	case AssetFileTypeAudio:
		return "mp3"
	case AssetFileTypeLink:
		return "url"
	default:
		return "bin"
	}
}
