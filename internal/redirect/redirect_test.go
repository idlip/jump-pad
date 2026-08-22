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

func TestGetReturnsExpiredRowAndLookupRefusesIt(t *testing.T) {
	db := openTest(t)
	now := time.Now()

	if _, err := Create(db, Input{Slug: "gone", TargetURL: "https://example.com", Expiry: "-1h"}, now); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := Lookup(db, "gone", now); !errors.Is(err, api.ErrExpired) {
		t.Fatalf("Lookup = %v, want api.ErrExpired", err)
	}
	one, err := Get(db, "gone")
	if err != nil || one.TargetURL != "https://example.com" {
		t.Fatalf("Get = %+v, %v, want the row as stored", one, err)
	}
}

func TestUpdateReplacesEveryField(t *testing.T) {
	db := openTest(t)
	now := time.Now()
	if _, err := Create(db, Input{Slug: "docs", TargetURL: "https://a.example"}, now); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := Update(db, "docs", Input{Slug: "guide", TargetURL: "https://b.example", Expiry: "1d"}, now)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	one, err := Get(db, "guide")
	if err != nil {
		t.Fatalf("Get after rename: %v", err)
	}
	if one.TargetURL != "https://b.example" || one.ExpiresAt == nil {
		t.Fatalf("row after Update = %+v", one)
	}
	if _, err := Get(db, "docs"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("old slug still exists: %v", err)
	}
}

func TestUpdateToTakenSlug(t *testing.T) {
	db := openTest(t)
	now := time.Now()
	for _, slug := range []string{"docs", "notes"} {
		if _, err := Create(db, Input{Slug: slug, TargetURL: "https://example.com"}, now); err != nil {
			t.Fatalf("Create(%s): %v", slug, err)
		}
	}

	err := Update(db, "notes", Input{Slug: "docs", TargetURL: "https://example.com"}, now)
	if !errors.Is(err, api.ErrSlugTaken) {
		t.Fatalf("Update to a taken slug = %v, want api.ErrSlugTaken", err)
	}
}

func TestListAndDelete(t *testing.T) {
	db := openTest(t)
	now := time.Now()
	for _, slug := range []string{"docs", "notes"} {
		if _, err := Create(db, Input{Slug: slug, TargetURL: "https://example.com"}, now); err != nil {
			t.Fatalf("Create(%s): %v", slug, err)
		}
	}

	all, err := List(db)
	if err != nil || len(all) != 2 {
		t.Fatalf("List = %d rows, %v, want 2", len(all), err)
	}

	if err := Delete(db, "docs"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := Delete(db, "docs"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("second Delete = %v, want api.ErrNotFound", err)
	}
}
