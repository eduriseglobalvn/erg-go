package service

import (
	"context"
	"fmt"

	"erg.ninja/internal/modules/elearning/api/dto"
	entities "erg.ninja/internal/modules/elearning/domain/entity"
)

// SeedData contains the default eLearning structural data.
var SeedData = []struct {
	Title     string
	Subtitle  string
	Slug      string
	SortOrder int
	Levels    []struct {
		Title       string
		Description string
		Slug        string
		SortOrder   int
		Units       []struct {
			Title       string
			Description string
			SortOrder   int
		}
	}
}{
	{
		Title:     "Tiểu học (Spark)",
		Subtitle:  "Bám sát chương trình IC3 Spark",
		Slug:      "primary",
		SortOrder: 0,
		Levels: []struct {
			Title       string
			Description string
			Slug        string
			SortOrder   int
			Units       []struct {
				Title       string
				Description string
				SortOrder   int
			}
		}{
			{
				Title:       "IC3 Spark Level 1",
				Description: "Làm quen với máy tính và công nghệ",
				Slug:        "spark-level-1",
				SortOrder:   0,
				Units: []struct {
					Title       string
					Description string
					SortOrder   int
				}{
					{"Khám phá máy tính", "Làm quen với các bộ phận và chức năng cơ bản của máy tính.", 0},
					{"Phần cứng & Phần mềm", "Phân biệt thiết bị ngoại vi và các chương trình máy tính.", 1},
					{"Sử dụng bàn phím", "Kỹ năng gõ phím 10 ngón và các phím chức năng.", 2},
					{"Hệ điều hành cơ bản", "Cách quản lý cửa sổ và thư mục đơn giản.", 3},
					{"An toàn thiết bị", "Bảo quản máy tính và sử dụng thiết bị đúng cách.", 4},
				},
			},
			{
				Title:       "IC3 Spark Level 2",
				Description: "Ứng dụng máy tính trong học tập",
				Slug:        "spark-level-2",
				SortOrder:   1,
				Units: []struct {
					Title       string
					Description string
					SortOrder   int
				}{
					{"Phần mềm ứng dụng", "Tìm hiểu các loại phần mềm phục vụ học tập.", 0},
					{"Soạn thảo văn bản", "Kỹ năng trình bày văn bản đơn giản.", 1},
					{"Bảng tính cơ bản", "Làm quen với các ô dữ liệu và tính toán.", 2},
					{"Trình chiếu sáng tạo", "Thiết kế slide bài thuyết trình sinh động.", 3},
					{"Quản lý tệp tin", "Cách sắp xếp dữ liệu khoa học trên máy tính.", 4},
				},
			},
			{
				Title:       "IC3 Spark Level 3",
				Description: "An toàn mạng cho trẻ em",
				Slug:        "spark-level-3",
				SortOrder:   2,
				Units: []struct {
					Title       string
					Description string
					SortOrder   int
				}{
					{"Mạng máy tính", "Cách các máy tính kết nối với nhau.", 0},
					{"Internet & Web", "Kỹ năng duyệt web và tìm kiếm thông tin.", 1},
					{"Liên lạc trực tuyến", "Sử dụng email và các công cụ nhắn tin.", 2},
					{"An toàn thông tin", "Bảo vệ mật khẩu và thông tin cá nhân.", 3},
					{"Đạo đức số", "Quy tắc ứng xử văn minh trên không gian mạng.", 4},
				},
			},
		},
	},
	{
		Title:     "THCS (GS6)",
		Subtitle:  "Bám sát chương trình IC3 GS6",
		Slug:      "secondary",
		SortOrder: 1,
		Levels: []struct {
			Title       string
			Description string
			Slug        string
			SortOrder   int
			Units       []struct {
				Title       string
				Description string
				SortOrder   int
			}
		}{
			{
				Title:       "IC3 GS6 Level 1",
				Description: "Nền tảng về thiết bị và hệ điều hành",
				Slug:        "gs6-level-1",
				SortOrder:   0,
				Units: []struct {
					Title       string
					Description string
					SortOrder   int
				}{
					{"Thiết bị số", "Cấu tạo và nguyên lý hoạt động của thiết bị.", 0},
					{"Hệ điều hành", "Quản trị hệ thống và cài đặt môi trường.", 1},
					{"Tùy chỉnh máy tính", "Cài đặt cá nhân hóa và quản lý người dùng.", 2},
					{"Ứng dụng & Phần mềm", "Quản lý vòng đời phần mềm trên PC.", 3},
					{"Bảo mật cơ bản", "Phòng chống mã độc và bảo mật thiết bị.", 4},
				},
			},
			{
				Title:       "IC3 GS6 Level 2",
				Description: "Kỹ năng mạng và giao tiếp trực tuyến",
				Slug:        "gs6-level-2",
				SortOrder:   1,
				Units: []struct {
					Title       string
					Description string
					SortOrder   int
				}{
					{"Kết nối mạng", "Giao thức mạng và hạ tầng kết nối.", 0},
					{"Trình duyệt Web", "Tận dụng tối đa các công nghệ duyệt web.", 1},
					{"Tìm kiếm thông tin", "Kỹ khai thác thông tin trên Internet.", 2},
					{"Giao tiếp số", "Các hình thức trao đổi thông tin hiện đại.", 3},
					{"Cộng tác trực tuyến", "Làm việc nhóm trên các nền tảng Cloud.", 4},
				},
			},
			{
				Title:       "IC3 GS6 Level 3",
				Description: "Xử lý văn bản và bảng tính nâng cao",
				Slug:        "gs6-level-3",
				SortOrder:   2,
				Units: []struct {
					Title       string
					Description string
					SortOrder   int
				}{
					{"Soạn thảo chuyên nghiệp", "Xử lý văn bản cấp độ nâng cao.", 0},
					{"Bảng tính nâng cao", "Phân tích dữ liệu và hàm phức tạp.", 1},
					{"Quản lý dữ liệu", "Tổ chức và bảo vệ an toàn dữ liệu số.", 2},
					{"Giải quyết vấn đề", "Kỹ năng xử lý sự cố công nghệ.", 3},
					{"Tư duy lập trình", "Nền tảng logic và thuật toán cơ bản.", 4},
				},
			},
		},
	},
}

// SeedElearningData seeds the default categories, levels, and units for eLearning into the database.
func (s *Service) SeedElearningData(ctx context.Context, tenantID string) error {
	s.log.Info().Msg("elearning: starting seed data process")

	for _, catData := range SeedData {
		// Check if category already exists
		existingCat, _ := s.repo.GetCategoryBySlug(ctx, tenantID, catData.Slug)
		var catID string

		if existingCat != nil {
			s.log.Info().Str("slug", catData.Slug).Msg("elearning: category already exists, skipping creation")
			catID = existingCat.ID
		} else {
			// Create new category
			isActive := true
			cat, err := s.CreateCategory(ctx, &dto.CreateCategoryRequest{
				TenantID:    tenantID,
				Name:        catData.Title,
				Slug:        catData.Slug,
				Description: catData.Subtitle,
				Order:       catData.SortOrder,
				IsActive:    &isActive,
			})
			if err != nil {
				return fmt.Errorf("failed to seed category %s: %w", catData.Title, err)
			}
			catID = cat.ID
			s.log.Info().Str("title", catData.Title).Msg("elearning: seeded category")
		}

		for _, lvlData := range catData.Levels {
			existingLvl, _ := s.repo.GetLevelBySlug(ctx, tenantID, lvlData.Slug)
			var lvlID string

			if existingLvl != nil {
				lvlID = existingLvl.ID
			} else {
				lvl, err := s.CreateLevel(ctx, &dto.CreateLevelRequest{
					TenantID:    tenantID,
					CategoryID:  catID,
					Name:        lvlData.Title,
					Slug:        lvlData.Slug,
					Description: lvlData.Description,
					Order:       lvlData.SortOrder,
				})
				if err != nil {
					return fmt.Errorf("failed to seed level %s: %w", lvlData.Title, err)
				}
				lvlID = lvl.ID
				s.log.Info().Str("title", lvlData.Title).Msg("elearning: seeded level")
			}

			// For units, we just fetch existing ones to avoid duplicates if they exist,
			// or simply create them. Since units don't have slugs strictly checked in data here, we'll
			// check by name within the level.
			existingUnits, _ := s.repo.ListUnitsByLevel(ctx, tenantID, lvlID)
			unitNameMap := make(map[string]*entities.ElearningUnit)
			for _, eu := range existingUnits {
				unitNameMap[eu.Name] = eu
			}

			for _, uData := range lvlData.Units {
				if _, ok := unitNameMap[uData.Title]; ok {
					continue // already exists
				}
				_, err := s.CreateUnit(ctx, &dto.CreateUnitRequest{
					TenantID:    tenantID,
					LevelID:     lvlID,
					Name:        uData.Title,
					Description: uData.Description,
					Order:       uData.SortOrder,
				})
				if err != nil {
					return fmt.Errorf("failed to seed unit %s: %w", uData.Title, err)
				}
			}
		}
	}

	s.log.Info().Msg("elearning: seed data process finished")
	return nil
}
