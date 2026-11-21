// Package store keeps the app's state in a local SQLite file.
package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaV1 string

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if v < 1 {
		if _, err := s.db.Exec(schemaV1); err != nil {
			return fmt.Errorf("migrate v1: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 1"); err != nil {
			return err
		}
		v = 1
	}
	if v < 2 {
		// v2: remember the process group of a running job so a crash can be cleaned up.
		if _, err := s.db.Exec("ALTER TABLE jobs ADD COLUMN pid INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("migrate v2: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 2"); err != nil {
			return err
		}
	}
	return nil
}

const timeFmt = time.RFC3339Nano

func now() time.Time { return time.Now().UTC() }

func fmtTime(t time.Time) string { return t.UTC().Format(timeFmt) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFmt, s)
	return t
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
