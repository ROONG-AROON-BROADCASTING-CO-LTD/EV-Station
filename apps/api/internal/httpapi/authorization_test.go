package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/auth"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/site"
)

func TestCreateUserRequiresSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	handler := &Handler{repo: repo, auth: auth.New(repo, "test-secret")}
	body := []byte(`{"email":"sales@example.com","displayName":"Sales","password":"password123","role":"sales"}`)

	operatorResponse := httptest.NewRecorder()
	operatorContext, _ := gin.CreateTestContext(operatorResponse)
	operatorContext.Request = httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	operatorContext.Request.Header.Set("Content-Type", "application/json")
	operatorContext.Set("userId", uuid.NewString())
	operatorContext.Set("role", domain.RoleAdmin)
	handler.CreateUser(operatorContext)
	if operatorResponse.Code != http.StatusForbidden {
		t.Fatalf("operator create user status = %d, want %d", operatorResponse.Code, http.StatusForbidden)
	}

	superAdminResponse := httptest.NewRecorder()
	superAdminContext, _ := gin.CreateTestContext(superAdminResponse)
	superAdminContext.Request = httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	superAdminContext.Request.Header.Set("Content-Type", "application/json")
	superAdminContext.Set("userId", uuid.NewString())
	superAdminContext.Set("role", domain.RoleSuperAdmin)
	handler.CreateUser(superAdminContext)
	if superAdminResponse.Code != http.StatusCreated {
		t.Fatalf("super admin create user status = %d, want %d: %s", superAdminResponse.Code, http.StatusCreated, superAdminResponse.Body.String())
	}
	var response struct {
		Data domain.User `json:"data"`
	}
	if err := json.Unmarshal(superAdminResponse.Body.Bytes(), &response); err != nil || response.Data.Role != domain.RoleSales {
		t.Fatalf("unexpected created user: err=%v user=%+v", err, response.Data)
	}
}

func TestRequireAuthRejectsMissingAndForgedTokensAndSetsVerifiedClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	service := auth.New(repo, "test-secret")
	handler := &Handler{auth: service}
	user, err := service.Register(context.Background(), "sales@example.com", "Sales", "password123", domain.RoleSales)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := service.Login(context.Background(), user.Email, "password123")
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name          string
		authorization string
		wantStatus    int
	}{
		{name: "missing authorization header", wantStatus: http.StatusUnauthorized},
		{name: "forged token", authorization: "Bearer " + token + "forged", wantStatus: http.StatusUnauthorized},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(response)
			requestContext.Request = httptest.NewRequest(http.MethodGet, "/protected", nil)
			requestContext.Request.Header.Set("Authorization", testCase.authorization)
			handler.RequireAuth(requestContext)
			if response.Code != testCase.wantStatus || !requestContext.IsAborted() {
				t.Fatalf("RequireAuth() status/aborted = %d/%v, want %d/true", response.Code, requestContext.IsAborted(), testCase.wantStatus)
			}
		})
	}

	response := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(response)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/protected", nil)
	requestContext.Request.Header.Set("Authorization", "Bearer "+token)
	handler.RequireAuth(requestContext)
	if requestContext.IsAborted() {
		t.Fatal("RequireAuth() aborted a valid authenticated request")
	}
	if userID, ok := requestContext.Get("userId"); !ok || userID != user.ID.String() {
		t.Fatalf("RequireAuth() userId = %v, want %s", userID, user.ID)
	}
	if role, ok := requestContext.Get("role"); !ok || role != domain.RoleSales {
		t.Fatalf("RequireAuth() role = %v, want sales", role)
	}
}

func TestListUsersRequiresSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	handler := &Handler{repo: repo}

	for _, testCase := range []struct {
		name        string
		role        domain.UserRole
		withSession bool
		wantStatus  int
	}{
		{name: "missing session", wantStatus: http.StatusUnauthorized},
		{name: "sales role", role: domain.RoleSales, withSession: true, wantStatus: http.StatusForbidden},
		{name: "admin role", role: domain.RoleAdmin, withSession: true, wantStatus: http.StatusForbidden},
		{name: "super admin role", role: domain.RoleSuperAdmin, withSession: true, wantStatus: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(response)
			requestContext.Request = httptest.NewRequest(http.MethodGet, "/users", nil)
			if testCase.withSession {
				requestContext.Set("userId", uuid.NewString())
				requestContext.Set("role", testCase.role)
			}
			handler.ListUsers(requestContext)
			if response.Code != testCase.wantStatus {
				t.Fatalf("ListUsers() status = %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
		})
	}
}

func TestLoginValidatesRequestAndDoesNotAuthenticateWrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	service := auth.New(repo, "test-secret")
	handler := &Handler{auth: service}
	if _, err := service.Register(context.Background(), "sales@example.com", "Sales", "password123", domain.RoleSales); err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "malformed JSON", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "invalid email", body: `{"email":"not-an-email","password":"password123"}`, wantStatus: http.StatusBadRequest},
		{name: "wrong password", body: `{"email":"sales@example.com","password":"incorrect"}`, wantStatus: http.StatusUnauthorized},
		{name: "successful login", body: `{"email":"sales@example.com","password":"password123"}`, wantStatus: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(response)
			requestContext.Request = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(testCase.body))
			requestContext.Request.Header.Set("Content-Type", "application/json")
			handler.Login(requestContext)
			if response.Code != testCase.wantStatus {
				t.Fatalf("Login() status = %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
			if testCase.wantStatus == http.StatusOK {
				var payload struct {
					Data struct {
						Token string `json:"token"`
					} `json:"data"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Data.Token == "" {
					t.Fatalf("Login() successful response did not include a token: err=%v body=%s", err, response.Body.String())
				}
			}
		})
	}
}

func TestUpdateUserRequiresSuperAdminAndKeepsAnActiveSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	handler := &Handler{repo: repo, auth: auth.New(repo, "test-secret")}
	team, err := handler.auth.Register(context.Background(), "sales@example.com", "Sales", "password123", domain.RoleSales)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"email":"sales@example.com","displayName":"Updated sales","role":"admin","isActive":false}`)

	operatorResponse := httptest.NewRecorder()
	operatorContext, _ := gin.CreateTestContext(operatorResponse)
	operatorContext.Request = httptest.NewRequest(http.MethodPut, "/users/"+team.ID.String(), bytes.NewReader(body))
	operatorContext.Request.Header.Set("Content-Type", "application/json")
	operatorContext.Params = gin.Params{{Key: "id", Value: team.ID.String()}}
	operatorContext.Set("userId", uuid.NewString())
	operatorContext.Set("role", domain.RoleAdmin)
	handler.UpdateUser(operatorContext)
	if operatorResponse.Code != http.StatusForbidden {
		t.Fatalf("operator update status = %d, want %d", operatorResponse.Code, http.StatusForbidden)
	}

	superResponse := httptest.NewRecorder()
	superContext, _ := gin.CreateTestContext(superResponse)
	superContext.Request = httptest.NewRequest(http.MethodPut, "/users/"+team.ID.String(), bytes.NewReader(body))
	superContext.Request.Header.Set("Content-Type", "application/json")
	superContext.Params = gin.Params{{Key: "id", Value: team.ID.String()}}
	superContext.Set("userId", uuid.NewString())
	superContext.Set("role", domain.RoleSuperAdmin)
	handler.UpdateUser(superContext)
	if superResponse.Code != http.StatusOK {
		t.Fatalf("super admin update status = %d, want %d: %s", superResponse.Code, http.StatusOK, superResponse.Body.String())
	}
	updated, err := repo.GetUserByID(context.Background(), team.ID)
	if err != nil || updated.Role != domain.RoleAdmin || updated.IsActive {
		t.Fatalf("unexpected updated user: err=%v user=%+v", err, updated)
	}

	owner, err := handler.auth.Register(context.Background(), "owner2@example.com", "Owner", "password123", domain.RoleSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handler.auth.Register(context.Background(), "owner3@example.com", "Backup owner", "password123", domain.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	ownerBody := []byte(`{"email":"owner2-updated@example.com","displayName":"Updated owner","role":"admin","isActive":false}`)
	ownerResponse := httptest.NewRecorder()
	ownerContext, _ := gin.CreateTestContext(ownerResponse)
	ownerContext.Request = httptest.NewRequest(http.MethodPut, "/users/"+owner.ID.String(), bytes.NewReader(ownerBody))
	ownerContext.Request.Header.Set("Content-Type", "application/json")
	ownerContext.Params = gin.Params{{Key: "id", Value: owner.ID.String()}}
	ownerContext.Set("userId", uuid.NewString())
	ownerContext.Set("role", domain.RoleSuperAdmin)
	handler.UpdateUser(ownerContext)
	if ownerResponse.Code != http.StatusOK {
		t.Fatalf("super admin target update status = %d, want %d: %s", ownerResponse.Code, http.StatusOK, ownerResponse.Body.String())
	}
}

func TestDeleteUserRequiresSuperAdminAndRemovesAssignedAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	handler := &Handler{repo: repo, auth: auth.New(repo, "test-secret")}
	actor, err := handler.auth.Register(context.Background(), "owner@example.com", "Owner", "password123", domain.RoleSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	target, err := handler.auth.Register(context.Background(), "customer@example.com", "Customer", "password123", domain.RoleCustomer)
	if err != nil {
		t.Fatal(err)
	}
	site := domain.Site{ID: uuid.New(), Name: "Assigned site"}
	if _, err = repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetSiteAccess(context.Background(), domain.SiteAccess{SiteID: site.ID, UserID: target.ID, Role: string(domain.RoleCustomer)}); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(response)
	requestContext.Request = httptest.NewRequest(http.MethodDelete, "/users/"+target.ID.String(), nil)
	requestContext.Params = gin.Params{{Key: "id", Value: target.ID.String()}}
	requestContext.Set("userId", actor.ID.String())
	requestContext.Set("role", domain.RoleSuperAdmin)
	handler.DeleteUser(requestContext)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if _, err = repo.GetUserByID(context.Background(), target.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("deleted user lookup error = %v, want not found", err)
	}
	access, err := repo.ListSiteAccess(context.Background(), site.ID)
	if err != nil || len(access) != 0 {
		t.Fatalf("access after delete = %+v, err=%v", access, err)
	}
}

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
	assertSitePermission(t, handler, viewerID, domain.RoleViewer, siteA.ID, siteWrite, true)
	assertSitePermission(t, handler, uuid.New(), domain.RoleAdmin, siteB.ID, siteWrite, true)
	assertSitePermission(t, handler, uuid.New(), domain.RoleOwner, siteB.ID, siteWrite, true)
}

func TestListSitesDoesNotExposeUnassignedSitesToSalesOrCustomers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	handler := &Handler{repo: repo, sites: site.NewService(repo)}
	assigned := domain.Site{ID: uuid.New(), Name: "Assigned site"}
	other := domain.Site{ID: uuid.New(), Name: "Other site"}
	for _, candidate := range []domain.Site{assigned, other} {
		if _, err := repo.CreateSite(context.Background(), candidate); err != nil {
			t.Fatal(err)
		}
	}
	salesID := uuid.New()
	if err := repo.SetSiteAccess(context.Background(), domain.SiteAccess{SiteID: assigned.ID, UserID: salesID, Role: string(domain.RoleSales)}); err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name      string
		userID    uuid.UUID
		role      domain.UserRole
		wantCount int
	}{
		{name: "sales receives only assigned site", userID: salesID, role: domain.RoleSales, wantCount: 1},
		{name: "customer without grant receives no sites", userID: uuid.New(), role: domain.RoleCustomer, wantCount: 0},
		{name: "admin receives all sites", userID: uuid.New(), role: domain.RoleAdmin, wantCount: 2},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(response)
			requestContext.Request = httptest.NewRequest(http.MethodGet, "/sites", nil)
			requestContext.Set("userId", testCase.userID.String())
			requestContext.Set("role", testCase.role)
			handler.ListSites(requestContext)
			if response.Code != http.StatusOK {
				t.Fatalf("ListSites() status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
			}
			var payload struct {
				Data []domain.Site `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Data) != testCase.wantCount {
				t.Fatalf("ListSites() returned %d sites, want %d: %+v", len(payload.Data), testCase.wantCount, payload.Data)
			}
			for _, returned := range payload.Data {
				if testCase.role != domain.RoleAdmin && returned.ID != assigned.ID {
					t.Fatalf("ListSites() exposed unassigned site %s to %s", returned.ID, testCase.role)
				}
			}
		})
	}
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
