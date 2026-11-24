package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var coreMigrationsFS embed.FS

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// Migration represents a forward-only versioned SQL migration.
// Both core storage and pluggable modules use this type with distinct namespaces.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Store encapsulates the SQLite database connection and all core repositories.
type Store struct {
	db       *sql.DB
	inMemory bool
}

// Open opens (or creates) a file-backed SQLite database in WAL mode and runs core migrations.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	return initStore(ctx, db, false)
}

// OpenInMemory opens an isolated in-memory SQLite database (useful for tests).
func OpenInMemory(ctx context.Context) (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open in-memory sqlite: %w", err)
	}
	return initStore(ctx, db, true)
}

func initStore(ctx context.Context, db *sql.DB, inMemory bool) (*Store, error) {
	// Single connection per process prevents intra-process SQLITE_BUSY and guarantees
	// consistent PRAGMAs (and single shared DB for :memory:). Multi-process safety
	// between daemon, TUI, and CLI is handled by WAL + busy_timeout.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
	}
	if !inMemory {
		pragmas = append([]string{"PRAGMA journal_mode = WAL;"}, pragmas...)
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	s := &Store{db: db, inMemory: inMemory}
	if err := s.ensureMigrationsTable(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	coreSQL, err := coreMigrationsFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read embedded 0001_init.sql: %w", err)
	}

	if err := s.ApplyMigrations(ctx, "core", []Migration{
		{Version: 1, Name: "init", SQL: string(coreSQL)},
	}); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying SQLite connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the underlying *sql.DB for pluggable extensions that manage their own ext_* tables.
func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) ensureMigrationsTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			namespace  TEXT NOT NULL,
			version    INTEGER NOT NULL,
			applied_at TEXT NOT NULL,
			PRIMARY KEY (namespace, version)
		);
	`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

// ApplyMigrations runs any unapplied migrations for the given namespace ('core' or 'ext:<id>')
// in ascending version order, each inside a transaction.
func (s *Store) ApplyMigrations(ctx context.Context, namespace string, migrations []Migration) error {
	if strings.TrimSpace(namespace) == "" {
		return errors.New("migration namespace is required")
	}

	sorted := append([]Migration(nil), migrations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version < sorted[j].Version })

	applied := map[int]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations WHERE namespace = ?`, namespace)
	if err != nil {
		return fmt.Errorf("query migrations for %s: %w", namespace, err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scan migration version: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range sorted {
		if applied[m.Version] {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s v%d: %w", namespace, m.Version, err)
		}
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s v%d (%s): %w", namespace, m.Version, m.Name, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (namespace, version, applied_at) VALUES (?, ?, ?)`,
			namespace,
			m.Version,
			formatTime(time.Now().UTC()),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s v%d: %w", namespace, m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s v%d: %w", namespace, m.Version, err)
		}
	}

	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatNullTime(t *time.Time) sql.NullString {
	if t == nil || t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func parseNullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || strings.TrimSpace(ns.String) == "" {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
