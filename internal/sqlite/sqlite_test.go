package sqlite

import (
	"database/sql"
	"errors"
	"testing"

	"jump-pad/internal/api"
)

// openTest returns a fresh private database for one test.
func openTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestEnsureColumn(t *testing.T) {
	db := openTest(t)
	if _, err := db.Exec("CREATE TABLE items (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := EnsureColumn(db, "items", "language", "TEXT"); err != nil {
		t.Fatalf("EnsureColumn: %v", err)
	}
	if err := EnsureColumn(db, "items", "language", "TEXT"); err != nil {
		t.Fatalf("second EnsureColumn: %v", err) // must be safe on every startup
	}

	if _, err := db.Exec("INSERT INTO items (id, language) VALUES ('a', 'go')"); err != nil {
		t.Fatalf("insert after EnsureColumn: %v", err)
	}
}
