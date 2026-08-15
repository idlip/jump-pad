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
