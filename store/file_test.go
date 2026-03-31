package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFileStore_CredentialsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)
	ctx := context.Background()

	// Load non-existent
	_, err := s.LoadCredentials(ctx, "bot1")
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}

	// Save and load
	creds := Credentials{Token: "tok123", BaseURL: "https://example.com", UserID: "u1", SavedAt: time.Now()}
	if err := s.SaveCredentials(ctx, "bot1", creds); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.LoadCredentials(ctx, "bot1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Token != "tok123" || got.BaseURL != "https://example.com" {
		t.Fatalf("mismatch: %+v", got)
	}

	// Delete
	s.DeleteCredentials(ctx, "bot1")
	_, err = s.LoadCredentials(ctx, "bot1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestFileStore_SyncBuf(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)
	ctx := context.Background()

	buf, _ := s.LoadSyncBuf(ctx, "bot1")
	if buf != "" {
		t.Fatalf("expected empty, got %q", buf)
	}

	s.SaveSyncBuf(ctx, "bot1", "sync-data-123")
	buf, _ = s.LoadSyncBuf(ctx, "bot1")
	if buf != "sync-data-123" {
		t.Fatalf("got %q", buf)
	}
}

func TestFileStore_ContextTokens(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)
	ctx := context.Background()

	s.SaveContextToken(ctx, "bot1", "user-a", "token-aaa")
	s.SaveContextToken(ctx, "bot1", "user-b", "token-bbb")

	tok, _ := s.LoadContextToken(ctx, "bot1", "user-a")
	if tok != "token-aaa" {
		t.Fatalf("got %q", tok)
	}

	tok, _ = s.LoadContextToken(ctx, "bot1", "user-b")
	if tok != "token-bbb" {
		t.Fatalf("got %q", tok)
	}

	// Different client
	tok, _ = s.LoadContextToken(ctx, "bot2", "user-a")
	if tok != "" {
		t.Fatalf("expected empty for different client, got %q", tok)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123", "abc123"},
		{"user-123", "user-123"},
		{"user_456", "user_456"},
		{"hex@im.bot", "hex-im-bot"},
		{"path/to/file", "path-to-file"},
		{"a:b*c?d", "a-b-c-d"},
	}

	for _, tt := range tests {
		got := sanitize(tt.input)
		if got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFileStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)
	ctx := context.Background()

	// Write credentials
	s.SaveCredentials(ctx, "bot1", Credentials{Token: "first"})

	// Overwrite
	s.SaveCredentials(ctx, "bot1", Credentials{Token: "second"})

	got, _ := s.LoadCredentials(ctx, "bot1")
	if got.Token != "second" {
		t.Fatalf("expected 'second', got %q", got.Token)
	}

	// No .tmp files should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".tmp" {
			t.Errorf("temp file not cleaned up: %s", e.Name())
		}
	}
}
