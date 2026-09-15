package site

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
)

func TestValidateLocation(t *testing.T) {
	lat, lng := 13.7563, 100.5018
	cases := []struct {
		name     string
		address  string
		lat, lng *float64
		wantErr  bool
	}{
		{"address", "Bangkok", nil, nil, false},
		{"coordinates", "", &lat, &lng, false},
		{"coordinate boundaries", "", float64Pointer(-90), float64Pointer(180), false},
		{"latitude below boundary", "", float64Pointer(-90.0001), float64Pointer(180), true},
		{"longitude above boundary", "", float64Pointer(90), float64Pointer(180.0001), true},
		{"missing", "", nil, nil, true},
		{"partial", "", &lat, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLocation(tc.address, tc.lat, tc.lng)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestServiceCreateAndUpdatePreserveIdentityAndNormalizeDefaults(t *testing.T) {
	repo := repository.NewMemory()
	service := NewService(repo)
	input := domain.CreateSiteInput{
		Name: "  Riverside plot  ", Address: "  Bangkok  ", LandSize: 100, LandSizeUnit: "sqwah",
		ElectricalSupplyType: "",
	}

	created, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Riverside plot" || created.Address != "Bangkok" || created.ElectricalSupplyType != "unknown" || created.InputStatus != domain.DataPreliminary {
		t.Fatalf("Create() = %+v, want normalized preliminary site with unknown supply type", created)
	}
	if created.ID == uuid.Nil || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("Create() did not assign stable identity and timestamps: %+v", created)
	}

	updateInput := input
	updateInput.Name = "Updated plot"
	updateInput.ElectricalSupplyType = "overhead"
	updated, err := service.Update(context.Background(), created.ID, updateInput)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) || updated.Name != "Updated plot" || updated.ElectricalSupplyType != "overhead" {
		t.Fatalf("Update() = %+v, want original identity and updated safe fields", updated)
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) || updated.UpdatedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("Update() timestamp = %v, want current timestamp", updated.UpdatedAt)
	}
}

func TestServiceUpdateDoesNotCreateMissingSite(t *testing.T) {
	service := NewService(repository.NewMemory())
	_, err := service.Update(context.Background(), uuid.New(), domain.CreateSiteInput{Name: "Missing", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqwah"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("Update() missing site error = %v, want ErrNotFound", err)
	}
}

func float64Pointer(value float64) *float64 { return &value }
