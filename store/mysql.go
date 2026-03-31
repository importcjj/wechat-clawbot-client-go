package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// MySQLStore implements Store using MySQL via database/sql.
//
// Usage:
//
//	import _ "github.com/go-sql-driver/mysql"
//
//	db, _ := sql.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/dbname")
//	s := store.NewMySQLStore(db)
//	s.EnsureSchema(ctx)
type MySQLStore struct {
	db          *sql.DB
	tablePrefix string
}

// MySQLOption configures a MySQLStore.
type MySQLOption func(*MySQLStore)

// WithMySQLTablePrefix sets a custom table prefix (default: "wechat").
func WithMySQLTablePrefix(prefix string) MySQLOption {
	return func(s *MySQLStore) { s.tablePrefix = prefix }
}

// NewMySQLStore creates a Store backed by MySQL.
func NewMySQLStore(db *sql.DB, opts ...MySQLOption) *MySQLStore {
	s := &MySQLStore{db: db, tablePrefix: "wechat"}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *MySQLStore) credTable() string  { return s.tablePrefix + "_credentials" }
func (s *MySQLStore) syncTable() string  { return s.tablePrefix + "_sync" }
func (s *MySQLStore) tokenTable() string { return s.tablePrefix + "_context_tokens" }

// EnsureSchema creates tables if they don't exist.
func (s *MySQLStore) EnsureSchema(ctx context.Context) error {
	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			client_id VARCHAR(255) PRIMARY KEY,
			data TEXT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, s.credTable()),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			client_id VARCHAR(255) PRIMARY KEY,
			buf MEDIUMTEXT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, s.syncTable()),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			client_id VARCHAR(255) NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			token TEXT NOT NULL,
			PRIMARY KEY (client_id, user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, s.tokenTable()),
	}
	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("creating table: %w", err)
		}
	}
	return nil
}

func (s *MySQLStore) SaveCredentials(ctx context.Context, clientID string, creds Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (client_id, data) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE data = VALUES(data)`, s.credTable()),
		clientID, string(data))
	if err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}
	return nil
}

func (s *MySQLStore) LoadCredentials(ctx context.Context, clientID string) (Credentials, error) {
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

func (s *MySQLStore) DeleteCredentials(ctx context.Context, clientID string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE client_id = ?", s.credTable()), clientID)
	if err != nil {
		return fmt.Errorf("deleting credentials: %w", err)
	}
	return nil
}

func (s *MySQLStore) SaveSyncBuf(ctx context.Context, clientID string, buf string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (client_id, buf) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE buf = VALUES(buf)`, s.syncTable()),
		clientID, buf)
	if err != nil {
		return fmt.Errorf("saving sync buf: %w", err)
	}
	return nil
}

func (s *MySQLStore) LoadSyncBuf(ctx context.Context, clientID string) (string, error) {
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

func (s *MySQLStore) SaveContextToken(ctx context.Context, clientID, userID, token string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (client_id, user_id, token) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE token = VALUES(token)`, s.tokenTable()),
		clientID, userID, token)
	if err != nil {
		return fmt.Errorf("saving context token: %w", err)
	}
	return nil
}

func (s *MySQLStore) LoadContextToken(ctx context.Context, clientID, userID string) (string, error) {
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
