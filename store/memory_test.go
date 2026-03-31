package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_Credentials(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	// Load non-existent
	_, err := s.LoadCredentials(ctx, "test")
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}

	// Save and load
	creds := Credentials{Token: "tok", BaseURL: "https://example.com", SavedAt: time.Now()}
	if err := s.SaveCredentials(ctx, "test", creds); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.LoadCredentials(ctx, "test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Token != "tok" {
		t.Fatalf("token mismatch: got %q", got.Token)
	}

	// Delete
	if err := s.DeleteCredentials(ctx, "test"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.LoadCredentials(ctx, "test")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestMemoryStore_SyncBuf(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	buf, _ := s.LoadSyncBuf(ctx, "test")
	if buf != "" {
		t.Fatalf("expected empty, got %q", buf)
	}

	s.SaveSyncBuf(ctx, "test", "abc123")
	buf, _ = s.LoadSyncBuf(ctx, "test")
	if buf != "abc123" {
		t.Fatalf("got %q", buf)
	}
}

func TestMemoryStore_ContextToken(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	tok, _ := s.LoadContextToken(ctx, "bot1", "user1")
	if tok != "" {
		t.Fatalf("expected empty, got %q", tok)
	}

	s.SaveContextToken(ctx, "bot1", "user1", "token-abc")
	tok, _ = s.LoadContextToken(ctx, "bot1", "user1")
	if tok != "token-abc" {
		t.Fatalf("got %q", tok)
	}

	// Different user
	tok, _ = s.LoadContextToken(ctx, "bot1", "user2")
	if tok != "" {
		t.Fatalf("expected empty for different user, got %q", tok)
	}
}

func TestMemoryStore_Concurrent(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SaveSyncBuf(ctx, "bot", "data")
			s.LoadSyncBuf(ctx, "bot")
			s.SaveContextToken(ctx, "bot", "user", "tok")
			s.LoadContextToken(ctx, "bot", "user")
		}()
	}
	wg.Wait()
}
