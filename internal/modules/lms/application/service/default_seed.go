package service

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var defaultEducationUnits = []Center{
	{
		ID:      mustSeedObjectID("665000000000000000000201"),
		Type:    educationUnitTypeSystem,
		Name:    "Hệ thống ERG",
		Code:    "ERG-SYSTEM",
		Address: "Toàn hệ thống ERG",
		Status:  statusActive,
	},
	{
		ID:      mustSeedObjectID("665000000000000000000202"),
		Type:    educationUnitTypeSystem,
		Name:    "Hoclieu Studio",
		Code:    "HOCLIEU-STUDIO",
		Address: "Hệ thống học liệu ERG",
		Status:  statusActive,
	},
	{
		ID:      mustSeedObjectID("665000000000000000000101"),
		Type:    educationUnitTypeCenter,
		Name:    "ERG Bình Phú",
		Code:    "ERG-BINH-PHU",
		Address: "Bình Phú",
		Status:  statusActive,
	},
	{
		ID:       mustSeedObjectID("665000000000000000000102"),
		Type:     educationUnitTypeSchool,
		Name:     "THCS TRƯƠNG CÔNG ĐỊNH",
		Code:     "THCS-TCD",
		ParentID: mustSeedObjectID("665000000000000000000101"),
		Address:  "TP. Hồ Chí Minh",
		Status:   statusActive,
	},
	{
		ID:       mustSeedObjectID("665000000000000000000103"),
		Type:     educationUnitTypeSchool,
		Name:     "THCS CÁT LÁI",
		Code:     "THCS-CAT-LAI",
		ParentID: mustSeedObjectID("665000000000000000000101"),
		Address:  "TP. Hồ Chí Minh",
		Status:   statusActive,
	},
	{
		ID:       mustSeedObjectID("665000000000000000000104"),
		Type:     educationUnitTypeSchool,
		Name:     "THCS BÌNH AN",
		Code:     "THCS-BINH-AN",
		ParentID: mustSeedObjectID("665000000000000000000101"),
		Address:  "TP. Hồ Chí Minh",
		Status:   statusActive,
	},
}

func (s *Service) SeedDefaultEducationUnits(ctx context.Context, tenantID string) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("lms seed default education units requires repository")
	}
	if tenantID == "" {
		tenantID = "default"
	}
	var binhPhuID bson.ObjectID
	for _, unit := range defaultEducationUnits {
		unit.TenantID = tenantID
		if unit.ID == bson.NilObjectID {
			unit.ID = bson.NewObjectID()
		}
		if unit.Type == educationUnitTypeSchool && !binhPhuID.IsZero() {
			unit.ParentID = binhPhuID
		}
		upserted, err := s.repo.UpsertCenterByCode(ctx, unit)
		if err != nil {
			return fmt.Errorf("lms seed default education unit %s: %w", unit.Code, err)
		}
		if upserted != nil && unit.Code == "ERG-BINH-PHU" {
			binhPhuID = upserted.ID
		}
	}
	return nil
}
