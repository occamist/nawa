package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var fs embed.FS

type Store struct {
	*sql.DB
}

func New(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{DB: db}
	if err := s.apply(ctx); err != nil {
		return nil, fmt.Errorf("apply db: %w", err)
	}
	return s, nil
}

func (s *Store) apply(ctx context.Context) error {
	schema, err := fs.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := s.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

func (s *Store) UserExists(ctx context.Context, username string) (bool, error) {
	var count int
	err := s.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check user: %w", err)
	}
	return count > 0, nil
}

func (s *Store) Seed(ctx context.Context, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = s.ExecContext(ctx, "INSERT OR IGNORE INTO users (id, username, password_hash) VALUES (?, ?, ?)", uuid.NewString(), username, string(hash))
	if err != nil {
		return fmt.Errorf("seed user: %w", err)
	}
	return nil
}
