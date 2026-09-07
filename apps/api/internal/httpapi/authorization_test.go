package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
)

func TestSitePermissionsKeepUsersWithinAssignedSites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	handler := &Handler{repo: repo}

	siteA := domain.Site{ID: uuid.New(), Name: "Assigned site"}
	siteB := domain.Site{ID: uuid.New(), Name: "Other site"}
	if _, err := repo.CreateSite(context.Background(), siteA); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateSite(context.Background(), siteB); err != nil {
		t.Fatal(err)
	}

	salesID := uuid.New()
	viewerID := uuid.New()
	if err := repo.SetSiteAccess(context.Background(), domain.SiteAccess{SiteID: siteA.ID, UserID: salesID, Role: string(domain.RoleSales)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetSiteAccess(context.Background(), domain.SiteAccess{SiteID: siteA.ID, UserID: viewerID, Role: string(domain.RoleViewer)}); err != nil {
		t.Fatal(err)
	}

	assertSitePermission(t, handler, salesID, domain.RoleSales, siteA.ID, siteRead, true)
	assertSitePermission(t, handler, salesID, domain.RoleSales, siteA.ID, siteWrite, true)
	assertSitePermission(t, handler, salesID, domain.RoleSales, siteB.ID, siteRead, false)
	assertSitePermission(t, handler, viewerID, domain.RoleViewer, siteA.ID, siteRead, true)
	assertSitePermission(t, handler, viewerID, domain.RoleViewer, siteA.ID, siteWrite, false)
	assertSitePermission(t, handler, uuid.New(), domain.RoleOwner, siteB.ID, siteWrite, true)
}

func assertSitePermission(t *testing.T, handler *Handler, userID uuid.UUID, role domain.UserRole, siteID uuid.UUID, permission sitePermission, want bool) {
	t.Helper()
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	context.Set("userId", userID.String())
	context.Set("role", role)
	if got := handler.requireSitePermission(context, siteID, permission); got != want {
		t.Fatalf("permission %q for role %q on site %s: got %v, want %v", permission, role, siteID, got, want)
	}
	if !want && response.Code != http.StatusForbidden {
		t.Fatalf("denied permission returned HTTP %d, want %d", response.Code, http.StatusForbidden)
	}
}
