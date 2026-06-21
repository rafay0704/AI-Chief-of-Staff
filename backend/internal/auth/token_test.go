package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenIssueAndParse(t *testing.T) {
	m := NewTokenManager("test-secret-please-change", time.Hour)
	id := uuid.New()

	token, err := m.Issue(id)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := m.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != id {
		t.Fatalf("round-trip mismatch: got %s want %s", got, id)
	}
}

func TestParseRejectsExpired(t *testing.T) {
	m := NewTokenManager("test-secret", -time.Minute) // already expired
	token, err := m.Issue(uuid.New())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := m.Parse(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	issuer := NewTokenManager("secret-a", time.Hour)
	verifier := NewTokenManager("secret-b", time.Hour)

	token, err := issuer.Issue(uuid.New())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("expected token signed with different secret to be rejected")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	m := NewTokenManager("secret", time.Hour)
	if _, err := m.Parse("not-a-jwt"); err == nil {
		t.Fatal("expected garbage token to be rejected")
	}
}
