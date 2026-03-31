package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// SQLiteStore implements Store using SQLite via database/sql.
//
// Usage:
//
//	import _ "github.com/mattn/go-sqlite3" // or modernc.org/sqlite
//
//	db, _ := sql.Open("sqlite3", "./wechat.db")
//	s := store.NewSQLiteStore(db)
//	s.EnsureSchema(ctx)
type SQLiteStore struct {
	db          *sql.DB
	tablePrefix string
}

// SQLiteOption configures a SQLiteStore.
type SQLiteOption func(*SQLiteStore)

// WithSQLiteTablePrefix sets a custom table prefix (default: "wechat").
func WithSQLiteTablePrefix(prefix string) SQLiteOption {
	return func(s *SQLiteStore) { s.tablePrefix = prefix }
}

// NewSQLiteStore creates a Store backed by SQLite.
func NewSQLiteStore(db *sql.DB, opts ...SQLiteOption) *SQLiteStore {
	s := &SQLiteStore{db: db, tablePrefix: "wechat"}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *SQLiteStore) credTable() string  { return s.tablePrefix + "_credentials" }
func (s *SQLiteStore) syncTable() string  { return s.tablePrefix + "_sync" }
func (s *SQLiteStore) tokenTable() string { return s.tablePrefix + "_context_tokens" }

// EnsureSchema creates tables if they don't exist.
func (s *SQLiteStore) EnsureSchema(ctx context.Context) error {
	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			client_id TEXT PRIMARY KEY,
			data TEXT NOT NULL
		)`, s.credTable()),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			client_id TEXT PRIMARY KEY,
			buf TEXT NOT NULL
		)`, s.syncTable()),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			client_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			token TEXT NOT NULL,
			PRIMARY KEY (client_id, user_id)
		)`, s.tokenTable()),
	}
	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("creating table: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SaveCredentials(ctx context.Context, clientID string, creds Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (client_id, data) VALUES (?, ?)
			ON CONFLICT (client_id) DO UPDATE SET data = excluded.data`, s.credTable()),
		clientID, string(data))
	if err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}
	return nil
}

func (s *SQLiteStore) LoadCredentials(ctx context.Context, clientID string) (Credentials, error) {
	var data string
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT data FROM %s WHERE client_id = ?", s.credTable()),
		clientID).Scan(&data)
	if err == sql.ErrNoRows {
		return Credentials{}, fmt.Errorf("credentials not found for %q", clientID)
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("loading credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return Credentials{}, fmt.Errorf("unmarshaling credentials: %w", err)
	}
	return creds, nil
}

func (s *SQLiteStore) DeleteCredentials(ctx context.Context, clientID string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE client_id = ?", s.credTable()), clientID)
	if err != nil {
		return fmt.Errorf("deleting credentials: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveSyncBuf(ctx context.Context, clientID string, buf string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (client_id, buf) VALUES (?, ?)
			ON CONFLICT (client_id) DO UPDATE SET buf = excluded.buf`, s.syncTable()),
		clientID, buf)
	if err != nil {
		return fmt.Errorf("saving sync buf: %w", err)
	}
	return nil
}

func (s *SQLiteStore) LoadSyncBuf(ctx context.Context, clientID string) (string, error) {
	var buf string
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT buf FROM %s WHERE client_id = ?", s.syncTable()),
		clientID).Scan(&buf)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("loading sync buf: %w", err)
	}
	return buf, nil
}

func (s *SQLiteStore) SaveContextToken(ctx context.Context, clientID, userID, token string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (client_id, user_id, token) VALUES (?, ?, ?)
			ON CONFLICT (client_id, user_id) DO UPDATE SET token = excluded.token`, s.tokenTable()),
		clientID, userID, token)
	if err != nil {
		return fmt.Errorf("saving context token: %w", err)
	}
	return nil
}

func (s *SQLiteStore) LoadContextToken(ctx context.Context, clientID, userID string) (string, error) {
	var token string
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT token FROM %s WHERE client_id = ? AND user_id = ?", s.tokenTable()),
		clientID, userID).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("loading context token: %w", err)
	}
	return token, nil
}
