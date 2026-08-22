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

func TestInsertWithCollisionRetry(t *testing.T) {
	db := openTest(t)
	if _, err := db.Exec("CREATE TABLE items (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	insert := func(candidate string) error {
		_, err := db.Exec("INSERT INTO items (id) VALUES (?)", candidate)
		return err
	}

	first, err := InsertWithCollisionRetry("docs", insert)
	if err != nil || first != "docs" {
		t.Fatalf("first = %q, %v", first, err)
	}

	second, err := InsertWithCollisionRetry("docs", insert)
	if err != nil || second != "docs-2" {
		t.Fatalf("second = %q, %v, want docs-2", second, err)
	}
}

func TestQueryRows(t *testing.T) {
	db := openTest(t)
	if _, err := db.Exec("CREATE TABLE items (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO items (id) VALUES ('a'), ('b')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	ids, err := QueryRows(db, "SELECT id FROM items ORDER BY id", func(rows *sql.Rows) (string, error) {
		var id string
		err := rows.Scan(&id)
		return id, err
	})
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("QueryRows = %v, want [a b]", ids)
	}
}

func TestCollisionRetryGivesUp(t *testing.T) {
	_, err := InsertWithCollisionRetry("docs", func(string) error {
		return errors.New("UNIQUE constraint failed: items.id")
	})
	if !errors.Is(err, api.ErrSlugTaken) {
		t.Fatalf("err = %v, want api.ErrSlugTaken", err)
	}
}
