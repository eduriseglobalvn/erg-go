package service

import (
	"context"
	"testing"
)

func TestSeedDefaultEducationUnitsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	svc := NewService(repo, nil)

	if err := svc.SeedDefaultEducationUnits(ctx, "tenant-default-seed"); err != nil {
		t.Fatalf("seed default education units: %v", err)
	}
	if err := svc.SeedDefaultEducationUnits(ctx, "tenant-default-seed"); err != nil {
		t.Fatalf("seed default education units second run: %v", err)
	}

	items, total, err := repo.ListCenters(ctx, "tenant-default-seed", CenterListRequestDTO{Page: 1, Limit: 100}, "")
	if err != nil {
		t.Fatalf("list centers: %v", err)
	}
	if total != 6 || len(items) != 6 {
		t.Fatalf("seeded centers total=%d len=%d, want 6", total, len(items))
	}

	assertSeededUnit(t, items, "ERG-SYSTEM", educationUnitTypeSystem, "Hệ thống ERG")
	assertSeededUnit(t, items, "HOCLIEU-STUDIO", educationUnitTypeSystem, "Hoclieu Studio")
	assertSeededUnit(t, items, "ERG-BINH-PHU", educationUnitTypeCenter, "ERG Bình Phú")
	assertSeededUnit(t, items, "THCS-TCD", educationUnitTypeSchool, "THCS TRƯƠNG CÔNG ĐỊNH")
	assertSeededUnit(t, items, "THCS-CAT-LAI", educationUnitTypeSchool, "THCS CÁT LÁI")
	assertSeededUnit(t, items, "THCS-BINH-AN", educationUnitTypeSchool, "THCS BÌNH AN")
}

func assertSeededUnit(t *testing.T, items []Center, code, unitType, name string) {
	t.Helper()
	for _, item := range items {
		if item.Code == code {
			if item.Type != unitType || item.Name != name || item.Status != statusActive {
				t.Fatalf("seeded unit %s = %+v", code, item)
			}
			return
		}
	}
	t.Fatalf("seeded unit %s not found in %+v", code, items)
}
