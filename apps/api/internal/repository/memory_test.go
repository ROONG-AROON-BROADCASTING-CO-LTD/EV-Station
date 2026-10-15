package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestMemoryListAccessibleSiteIDs(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory()
	userID := uuid.New()
	assigned := domain.Site{ID: uuid.New(), Name: "Assigned"}
	unassigned := domain.Site{ID: uuid.New(), Name: "Unassigned"}

	for _, site := range []domain.Site{assigned, unassigned} {
		if _, err := repo.CreateSite(ctx, site); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.SetSiteAccess(ctx, domain.SiteAccess{SiteID: assigned.ID, UserID: userID, Role: string(domain.RoleSales)}); err != nil {
		t.Fatal(err)
	}

	ids, err := repo.ListAccessibleSiteIDs(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != assigned.ID {
		t.Fatalf("ListAccessibleSiteIDs() = %v, want [%s]", ids, assigned.ID)
	}
}

func TestMemorySiteEvidenceQuota(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory()
	siteID := uuid.New()
	if _, err := repo.CreateSite(ctx, domain.Site{ID: siteID, Name: "Quota"}); err != nil {
		t.Fatal(err)
	}
	images := make([]domain.SiteImage, MaxSiteEvidenceFiles)
	for i := range images {
		images[i] = domain.SiteImage{ID: uuid.New(), SiteID: siteID, Data: []byte{byte(i)}}
	}
	if err := repo.AddSiteImages(ctx, siteID, images); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddSiteImages(ctx, siteID, []domain.SiteImage{{ID: uuid.New(), SiteID: siteID, Data: []byte("extra")}}); !errors.Is(err, ErrSiteEvidenceLimit) {
		t.Fatalf("extra file: got %v, want quota error", err)
	}
	stored, err := repo.GetSiteImages(ctx, siteID)
	if err != nil || len(stored) != MaxSiteEvidenceFiles {
		t.Fatalf("quota rejection changed stored files: count=%d err=%v", len(stored), err)
	}

	otherSiteID := uuid.New()
	if _, err := repo.CreateSite(ctx, domain.Site{ID: otherSiteID, Name: "Byte quota"}); err != nil {
		t.Fatal(err)
	}
	large := make([]byte, MaxSiteEvidenceBytes/2+1)
	if err := repo.AddSiteImages(ctx, otherSiteID, []domain.SiteImage{{ID: uuid.New(), SiteID: otherSiteID, Data: large}}); err != nil {
		t.Fatal(err)
	}
	different := append([]byte(nil), large...)
	different[0] = 1
	if err := repo.AddSiteImages(ctx, otherSiteID, []domain.SiteImage{{ID: uuid.New(), SiteID: otherSiteID, Data: different}}); !errors.Is(err, ErrSiteEvidenceLimit) {
		t.Fatalf("byte overage: got %v, want quota error", err)
	}
}

func TestEvidenceRetriesDoNotConsumeQuota(t *testing.T) {
	repo := NewMemory()
	ctx := context.Background()
	id := uuid.New()
	repo.CreateSite(ctx, domain.Site{ID: id})
	images := []domain.SiteImage{}
	for i := range MaxSiteEvidenceFiles {
		images = append(images, domain.SiteImage{ID: uuid.New(), SiteID: id, MIMEType: "image/png", Data: []byte{byte(i)}})
	}
	if err := repo.AddSiteImages(ctx, id, images); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddSiteImages(ctx, id, images); err != nil {
		t.Fatal("retry at full quota failed", err)
	}
	stored, _ := repo.GetSiteImages(ctx, id)
	if len(stored) != MaxSiteEvidenceFiles {
		t.Fatal("duplicate evidence retained", len(stored))
	}
}
