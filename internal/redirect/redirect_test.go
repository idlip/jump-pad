package redirect

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

	slug, err := Create(db, Input{Slug: "docs", TargetURL: "https://example.com"}, now)
	if err != nil || slug != "docs" {
		t.Fatalf("Create = %q, %v", slug, err)
	}

	target, err := Lookup(db, "docs", now)
	if err != nil || target != "https://example.com" {
		t.Fatalf("Lookup = %q, %v", target, err)
	}

	if _, err := Lookup(db, "missing", now); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Lookup(missing) = %v, want api.ErrNotFound", err)
	}
}

func TestCreateCollisionGetsSuffix(t *testing.T) {
	db := openTest(t)
	now := time.Now()

	if _, err := Create(db, Input{Slug: "docs", TargetURL: "https://a.example"}, now); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	slug, err := Create(db, Input{Slug: "docs", TargetURL: "https://b.example"}, now)
	if err != nil || slug != "docs-2" {
		t.Fatalf("second Create = %q, %v, want docs-2", slug, err)
	}
}
