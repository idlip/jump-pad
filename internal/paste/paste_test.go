package paste

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"jump-pad/internal/api"
	"jump-pad/internal/sqlite"
)

// openTest returns a fresh migrated database for one test.
func openTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateAndLookup(t *testing.T) {
	db := openTest(t)
	now := time.Now()

	id, err := Create(db, Input{Content: "hello world", Language: "python"}, now)
	if err != nil || id == "" {
		t.Fatalf("Create = %q, %v", id, err)
	}

	content, language, err := Lookup(db, id, now)
	if err != nil || content != "hello world" || language != "python" {
		t.Fatalf("Lookup = %q, %q, %v", content, language, err)
	}

	if _, _, err := Lookup(db, "missing", now); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Lookup(missing) = %v, want api.ErrNotFound", err)
	}
}

func TestCreateCustomSlugCollision(t *testing.T) {
	db := openTest(t)
	now := time.Now()

	id, err := Create(db, Input{Slug: "notes", Content: "hello"}, now)
	if err != nil || id != "notes" {
		t.Fatalf("Create = %q, %v", id, err)
	}
	second, err := Create(db, Input{Slug: "notes", Content: "other"}, now)
	if err != nil || second != "notes-2" {
		t.Fatalf("second Create = %q, %v, want notes-2", second, err)
	}
}

func TestMigrateAddsLanguageColumnToOldDatabase(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// A database from before the language column existed.
	if _, err := db.Exec(`CREATE TABLE pastes (id TEXT PRIMARY KEY, content TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER)`); err != nil {
		t.Fatalf("seed the old schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := Create(db, Input{Content: "hi", Language: "python"}, time.Now()); err != nil {
		t.Fatalf("Create after Migrate: %v", err)
	}
}
func TestCreateRefusesEmptyContent(t *testing.T) {
	db := openTest(t)
	if _, err := Create(db, Input{}, time.Now()); !errors.Is(err, api.ErrInvalid) {
		t.Fatalf("Create with no content = %v, want api.ErrInvalid", err)
	}
}
