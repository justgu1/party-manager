package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword(hash, "s3cret") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestIssueAndParse(t *testing.T) {
	svc := NewService("test-secret", time.Hour)
	u := User{ID: uuid.New(), Email: "a@b.com", Name: "Alice"}

	tok, err := svc.Issue(u)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := svc.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.ID != u.ID || got.Email != u.Email || got.Name != u.Name {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, u)
	}
}

func TestParseRejectsBadSecret(t *testing.T) {
	tok, _ := NewService("secret-a", time.Hour).Issue(User{ID: uuid.New()})
	if _, err := NewService("secret-b", time.Hour).Parse(tok); err == nil {
		t.Fatal("expected parse to fail with wrong secret")
	}
}
