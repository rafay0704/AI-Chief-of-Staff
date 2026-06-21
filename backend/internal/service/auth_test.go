package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/auth"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

func newAuthService() *AuthService {
	return NewAuthService(newFakeQuerier(), auth.NewTokenManager("unit-test-secret", time.Hour))
}

func TestRegisterThenLogin(t *testing.T) {
	svc := newAuthService()
	ctx := context.Background()

	user, token, err := svc.Register(ctx, "Rafay", "Rafay@Example.com", "supersecret")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if token == "" {
		t.Fatal("expected a token on register")
	}
	if user.Email != "rafay@example.com" {
		t.Fatalf("email should be normalized to lowercase, got %q", user.Email)
	}

	// Login with correct credentials (and original-case email).
	if _, _, err := svc.Login(ctx, "rafay@example.com", "supersecret"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestRegisterDuplicateEmailConflict(t *testing.T) {
	svc := newAuthService()
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, "A", "dup@example.com", "password1"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, _, err := svc.Register(ctx, "B", "dup@example.com", "password2")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestLoginWrongPasswordUnauthorized(t *testing.T) {
	svc := newAuthService()
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, "A", "a@example.com", "rightpassword"); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, _, err := svc.Login(ctx, "a@example.com", "wrongpassword")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLoginUnknownEmailUnauthorized(t *testing.T) {
	svc := newAuthService()
	_, _, err := svc.Login(context.Background(), "ghost@example.com", "whatever")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
