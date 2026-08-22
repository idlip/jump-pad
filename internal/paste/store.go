// Package paste owns the pastes table: its schema, its store functions,
// and its HTTP handlers.
package paste

import (
	"database/sql"
	"time"

	"jump-pad/internal/api"
	"jump-pad/internal/sqlite"
	"jump-pad/internal/valid"
)

// MaxBytes is the size cap for paste content, checked on the server for
// every route that stores content.
const MaxBytes = 500 * 1024

// Paste is one row of the pastes table. A list answer leaves Content
// empty, because a few large pastes would make every page load heavy.
type Paste struct {
	ID        string `json:"id"`
	Content   string `json:"content,omitempty"`
	Language  string `json:"language"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt *int64 `json:"expires_at"`
}

// Input is one create or edit request. Slug is optional on create, and a
// random id takes its place.
type Input struct {
	Slug     string `json:"slug" form:"slug"`
	Content  string `json:"content" form:"content"`
	Language string `json:"language" form:"language"`
	Expiry   string `json:"expiry" form:"expiry"`
}

// schema is the DDL for this table, run on every startup.
const schema = `
CREATE TABLE IF NOT EXISTS pastes (
	id          TEXT PRIMARY KEY,
	content     TEXT NOT NULL,
	created_at  INTEGER NOT NULL,
	expires_at  INTEGER,
	language    TEXT
);
`

// Migrate creates the pastes table, and adds the language column to a
// database that was created before that column existed.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return sqlite.EnsureColumn(db, "pastes", "language", "TEXT")
}

// Create stores content under the given slug, or under a random id when
// the slug is empty. It returns the id that the row landed on.
func Create(db *sql.DB, in Input, now time.Time) (string, error) {
	expiresAt, err := check(in, now)
	if err != nil {
		return "", err
	}

	base := sqlite.RandomID()
	if in.Slug != "" {
		if base, err = valid.Slug(in.Slug); err != nil {
			return "", err
		}
	}

	return sqlite.InsertWithCollisionRetry(base, func(candidate string) error {
		_, err := db.Exec(
			"INSERT INTO pastes (id, content, created_at, expires_at, language) VALUES (?, ?, ?, ?, ?)",
			candidate, in.Content, now.Unix(), expiresAt, in.Language,
		)
		return err
	})
}

// Update replaces the row at oldID. A new id is allowed, and a taken new
// id returns api.ErrSlugTaken.
func Update(db *sql.DB, oldID string, in Input, now time.Time) error {
	expiresAt, err := check(in, now)
	if err != nil {
		return err
	}
	id, err := valid.Slug(in.Slug)
	if err != nil {
		return err
	}

	result, err := db.Exec(
		"UPDATE pastes SET id = ?, content = ?, language = ?, expires_at = ? WHERE id = ?",
		id, in.Content, in.Language, expiresAt, oldID,
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
func Get(db *sql.DB, id string) (Paste, error) {
	var one Paste
	var language sql.NullString
	err := db.QueryRow(
		"SELECT id, content, created_at, expires_at, language FROM pastes WHERE id = ?", id,
	).Scan(&one.ID, &one.Content, &one.CreatedAt, &one.ExpiresAt, &language)
	if err == sql.ErrNoRows {
		return Paste{}, api.ErrNotFound
	}
	one.Language = language.String
	return one, err
}

// Lookup returns the content and the language for an id. It is Get plus
// the expiry check, which is what a public visitor gets.
func Lookup(db *sql.DB, id string, now time.Time) (content, language string, err error) {
	one, err := Get(db, id)
	if err != nil {
		return "", "", err
	}
	if one.ExpiresAt != nil && now.Unix() > *one.ExpiresAt {
		return "", "", api.ErrExpired
	}
	return one.Content, one.Language, nil
}

// List returns every row without its content, newest first.
func List(db *sql.DB) ([]Paste, error) {
	return sqlite.QueryRows(db,
		"SELECT id, created_at, expires_at, language FROM pastes ORDER BY created_at DESC",
		func(rows *sql.Rows) (Paste, error) {
			var one Paste
			var language sql.NullString
			err := rows.Scan(&one.ID, &one.CreatedAt, &one.ExpiresAt, &language)
			one.Language = language.String
			return one, err
		})
}

// Delete removes one row.
func Delete(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM pastes WHERE id = ?", id)
	if err != nil {
		return err
	}
	return oneRow(result)
}

// check validates the fields that Create and Update share.
func check(in Input, now time.Time) (*int64, error) {
	if in.Content == "" {
		return nil, api.Invalid("content must not be empty")
	}
	if len(in.Content) > MaxBytes {
		return nil, api.ErrTooLarge
	}
	return valid.Expiry(in.Expiry, now)
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
