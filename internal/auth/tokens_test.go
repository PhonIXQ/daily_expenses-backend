package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokensRoundTrip(t *testing.T) {
	tok := NewTokens([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	id := uuid.Must(uuid.NewV7())

	raw, _, err := tok.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tok.Verify(raw)
	if err != nil || got != id {
		t.Fatalf("Verify = %v, %v; want %v", got, err, id)
	}
}

func TestTokensRejectsExpiredAndForeign(t *testing.T) {
	tok := NewTokens([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	raw, _, _ := tok.Issue(uuid.Must(uuid.NewV7()))

	tok.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if _, err := tok.Verify(raw); err == nil {
		t.Error("expired token accepted")
	}

	other := NewTokens([]byte("ffffffffffffffffffffffffffffffff"), time.Minute)
	if _, err := other.Verify(raw); err == nil {
		t.Error("token signed with another secret accepted")
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got, err := normalizeEmail("  Mae@Example.COM "); err != nil || got != "mae@example.com" {
		t.Errorf("normalizeEmail = %q, %v", got, err)
	}
	for _, bad := range []string{"", "nope", "Mae <mae@example.com>"} {
		if _, err := normalizeEmail(bad); err == nil {
			t.Errorf("normalizeEmail(%q) accepted", bad)
		}
	}
}
