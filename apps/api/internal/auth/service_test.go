package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
)

func TestServiceRegisterLoginAndVerifyNormalizesAccount(t *testing.T) {
	repo := repository.NewMemory()
	service := New(repo, "test-secret", 2*time.Hour)
	if _, err := service.Register(context.Background(), "invalid@example.com", "Invalid", "password123", domain.UserRole("operator")); err == nil {
		t.Fatal("Register() accepted an unsupported role")
	}

	user, err := service.Register(context.Background(), "  SALES@Example.com ", "  Sales User  ", "password123", domain.RoleSales)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Email != "sales@example.com" || user.DisplayName != "Sales User" || !user.IsActive {
		t.Fatalf("Register() user = %+v, want normalized active user", user)
	}

	loggedIn, token, err := service.Login(context.Background(), " SALES@example.com ", "password123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if loggedIn.ID != user.ID || token == "" {
		t.Fatalf("Login() user/token = %+v/%q, want registered user and token", loggedIn, token)
	}

	claims, err := service.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.Subject != user.ID.String() || claims.Role != domain.RoleSales {
		t.Fatalf("Verify() claims = %+v, want subject %q and sales role", claims, user.ID)
	}
	if claims.ExpiresAt < time.Now().Add(90*time.Minute).Unix() || claims.ExpiresAt > time.Now().Add(2*time.Hour+time.Minute).Unix() {
		t.Fatalf("Verify() expiry = %d, want approximately two hours from now", claims.ExpiresAt)
	}
}

func TestServiceLoginRejectsInvalidAndInactiveAccounts(t *testing.T) {
	repo := repository.NewMemory()
	service := New(repo, "test-secret")
	user, err := service.Register(context.Background(), "user@example.com", "User", "password123", domain.RoleCustomer)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name     string
		email    string
		password string
	}{
		{name: "unknown email", email: "missing@example.com", password: "password123"},
		{name: "wrong password", email: user.Email, password: "not-the-password"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, loginErr := service.Login(context.Background(), testCase.email, testCase.password)
			if !errors.Is(loginErr, ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", loginErr)
			}
		})
	}

	user.IsActive = false
	if _, err = repo.UpdateUser(context.Background(), user, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.Login(context.Background(), user.Email, "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() inactive user error = %v, want ErrInvalidCredentials", err)
	}
}

func TestServiceVerifyRejectsTamperedExpiredAndMalformedTokens(t *testing.T) {
	service := New(repository.NewMemory(), "test-secret")
	validSubject := uuid.NewString()
	expired := service.sign(Claims{Subject: validSubject, Role: domain.RoleAdmin, ExpiresAt: time.Now().Add(-time.Second).Unix()})
	valid := service.sign(Claims{Subject: validSubject, Role: domain.RoleAdmin, ExpiresAt: time.Now().Add(time.Hour).Unix()})

	for _, testCase := range []struct {
		name  string
		token string
	}{
		{name: "expired token", token: expired},
		{name: "tampered signature", token: valid + "x"},
		{name: "malformed token", token: "not-a-jwt"},
		{name: "invalid subject", token: service.sign(Claims{Subject: "not-a-uuid", Role: domain.RoleAdmin, ExpiresAt: time.Now().Add(time.Hour).Unix()})},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := service.Verify(testCase.token); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Verify() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestServiceUpdateUserNormalizesDetailsAndChangesPasswordOnlyWhenProvided(t *testing.T) {
	repo := repository.NewMemory()
	service := New(repo, "test-secret")
	user, err := service.Register(context.Background(), "sales@example.com", "Sales", "password123", domain.RoleSales)
	if err != nil {
		t.Fatal(err)
	}

	user.Email = "  ADMIN@Example.com "
	user.DisplayName = "  Updated Admin  "
	user.Role = domain.RoleAdmin
	updated, err := service.UpdateUser(context.Background(), user, "new-password-123")
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.Email != "admin@example.com" || updated.DisplayName != "Updated Admin" || updated.Role != domain.RoleAdmin {
		t.Fatalf("UpdateUser() = %+v, want normalized updated user", updated)
	}
	if _, _, err := service.Login(context.Background(), updated.Email, "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() with old password error = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := service.Login(context.Background(), updated.Email, "new-password-123"); err != nil {
		t.Fatalf("Login() with replacement password error = %v", err)
	}
}
