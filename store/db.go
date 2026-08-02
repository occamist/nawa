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

func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := apply(ctx, db); err != nil {
		return nil, fmt.Errorf("apply db: %w", err)
	}
	return db, nil
}

func apply(ctx context.Context, db *sql.DB) error {
	schema, err := fs.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

func UserExists(ctx context.Context, db *sql.DB, username string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check user: %w", err)
	}
	return count > 0, nil
}

func Seed(ctx context.Context, db *sql.DB, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.ExecContext(ctx, "INSERT OR IGNORE INTO users (id, username, password_hash) VALUES (?, ?, ?)", uuid.NewString(), username, string(hash))
	if err != nil {
		return fmt.Errorf("seed user: %w", err)
	}
	return nil
}
