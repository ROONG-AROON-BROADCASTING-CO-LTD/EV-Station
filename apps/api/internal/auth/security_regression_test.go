package auth

import (
	"context"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"testing"
)

func TestPasswordResetRevokesOldToken(t *testing.T) {
	ctx := context.Background()
	s := New(repository.NewMemory(), "synthetic-regression-secret")
	u, err := s.Register(ctx, "reset@example.invalid", "Reset", "password-before", domain.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, old, err := s.Login(ctx, u.Email, "password-before")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateUser(ctx, u, "password-after"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.VerifyUser(ctx, old); err == nil {
		t.Fatal("old token survived password reset")
	}
	_, fresh, err := s.Login(ctx, u.Email, "password-after")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.VerifyUser(ctx, fresh); err != nil {
		t.Fatal("fresh token rejected", err)
	}
	if _, _, err = s.Login(ctx, u.Email, "password-before"); err == nil {
		t.Fatal("old password accepted")
	}
}
