// Package redirect owns the redirects table: its schema, its store
// functions, and its HTTP handlers.
package redirect

import (
	"database/sql"
	"time"

	"jump-pad/internal/api"
	"jump-pad/internal/sqlite"
	"jump-pad/internal/valid"
)

// Redirect is one row of the redirects table.
type Redirect struct {
	Slug      string `json:"slug"`
	TargetURL string `json:"target_url"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt *int64 `json:"expires_at"`
}

// Input is one create or edit request. Expiry stays the raw user string,
// so valid.Expiry runs in one place only.
type Input struct {
	Slug      string `json:"slug" form:"slug"`
	TargetURL string `json:"target_url" form:"target_url"`
	Expiry    string `json:"expiry" form:"expiry"`
}

// schema is the DDL for this table, run on every startup.
const schema = `
CREATE TABLE IF NOT EXISTS redirects (
	slug        TEXT PRIMARY KEY,
	target_url  TEXT NOT NULL,
	created_at  INTEGER NOT NULL,
	expires_at  INTEGER
);
`

// Migrate creates the redirects table when it does not exist yet.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

// Create stores a redirect. A taken slug gets a numeric suffix, and the
// slug that the row landed on comes back.
func Create(db *sql.DB, in Input, now time.Time) (string, error) {
	slug, targetURL, expiresAt, err := check(in, now)
	if err != nil {
		return "", err
	}

	return sqlite.InsertWithCollisionRetry(slug, func(candidate string) error {
		_, err := db.Exec(
			"INSERT INTO redirects (slug, target_url, created_at, expires_at) VALUES (?, ?, ?, ?)",
			candidate, targetURL, now.Unix(), expiresAt,
		)
		return err
	})
}

// Update replaces the row at oldSlug. A new slug is allowed, and a taken
// new slug returns api.ErrSlugTaken instead of a suffixed name, because an
// admin who types a name means that name.
func Update(db *sql.DB, oldSlug string, in Input, now time.Time) error {
	slug, targetURL, expiresAt, err := check(in, now)
	if err != nil {
		return err
	}

	result, err := db.Exec(
		"UPDATE redirects SET slug = ?, target_url = ?, expires_at = ? WHERE slug = ?",
		slug, targetURL, expiresAt, oldSlug,
	)
	if sqlite.IsUniqueViolation(err) {
		return api.ErrSlugTaken
	}
	if err != nil {
		return err
	}
	return oneRow(result)
}

// Get returns the row as stored, with no expiry check. The admin edits an
// expired row through this function.
func Get(db *sql.DB, slug string) (Redirect, error) {
	var one Redirect
	err := db.QueryRow(
		"SELECT slug, target_url, created_at, expires_at FROM redirects WHERE slug = ?", slug,
	).Scan(&one.Slug, &one.TargetURL, &one.CreatedAt, &one.ExpiresAt)
	if err == sql.ErrNoRows {
		return Redirect{}, api.ErrNotFound
	}
	return one, err
}

// Lookup returns the target URL for a slug. It is Get plus the expiry
// check, which is what a public visitor gets.
func Lookup(db *sql.DB, slug string, now time.Time) (string, error) {
	one, err := Get(db, slug)
	if err != nil {
		return "", err
	}
	if one.ExpiresAt != nil && now.Unix() > *one.ExpiresAt {
		return "", api.ErrExpired
	}
	return one.TargetURL, nil
}

// List returns every row, newest first.
func List(db *sql.DB) ([]Redirect, error) {
	return sqlite.QueryRows(db,
		"SELECT slug, target_url, created_at, expires_at FROM redirects ORDER BY created_at DESC",
		func(rows *sql.Rows) (Redirect, error) {
			var one Redirect
			err := rows.Scan(&one.Slug, &one.TargetURL, &one.CreatedAt, &one.ExpiresAt)
			return one, err
		})
}

// Delete removes one row.
func Delete(db *sql.DB, slug string) error {
	result, err := db.Exec("DELETE FROM redirects WHERE slug = ?", slug)
	if err != nil {
		return err
	}
	return oneRow(result)
}

// check validates every field of an input and returns the stored forms.
// Create and Update share it, so the two cannot drift apart.
func check(in Input, now time.Time) (slug, targetURL string, expiresAt *int64, err error) {
	if slug, err = valid.Slug(in.Slug); err != nil {
		return "", "", nil, err
	}
	if targetURL, err = valid.TargetURL(in.TargetURL); err != nil {
		return "", "", nil, err
	}
	if expiresAt, err = valid.Expiry(in.Expiry, now); err != nil {
		return "", "", nil, err
	}
	return slug, targetURL, expiresAt, nil
}

// oneRow turns "no row matched" into api.ErrNotFound.
func oneRow(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return api.ErrNotFound
	}
	return nil
}
