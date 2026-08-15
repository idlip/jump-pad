package paste

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"jump-pad/internal/api"
)

// Routes returns the public routes of this feature. The create route
// answers with the final id as plain text, and the raw route answers with
// the content as plain text, so both work with curl.
func Routes(db *sql.DB, prefix string) []api.Route {
	return []api.Route{{
		Method:  "POST",
		Pattern: "/pastes",
		Access:  api.CreateToken,
		Summary: "Store a paste and answer with its final id as plain text.",
		Params: []api.Param{
			{Name: "content", Required: true, Description: "The text to store. The cap is 500 KB."},
			{Name: "slug", Description: "Wanted id. A random 12 character id takes its place when this is empty."},
			{Name: "language", Description: "Language hint for syntax highlighting, such as python."},
			{Name: "expiry", Description: "Empty for forever, or 1d, 1w, 1m, a future date, or a duration such as 72h."},
		},
		Handler: api.LimitBody(MaxBytes, createHandler(db)),
	}, {
		Method:  "GET",
		Pattern: fmt.Sprintf("%s{id}", prefix),
		Access:  api.Public,
		Summary: "Answer with the raw content of a paste as plain text.",
		Params:  []api.Param{{Name: "id", Required: true, Description: "The paste id in the path."}},
		Handler: rawHandler(db),
	}}
}

// createHandler stores one paste from a form or JSON body.
func createHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if err := api.Decode(r, &in); err != nil {
			http.Error(w, "paste is too large or malformed", api.StatusFor(err))
			return
		}

		id, err := Create(db, in, time.Now())
		if err != nil {
			http.Error(w, err.Error(), api.StatusFor(err))
			return
		}
		w.Write([]byte(id))
	}
}

// rawHandler answers with the content as plain text. The language hint
// travels in the X-Paste-Language header.
func rawHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, language, err := Lookup(db, r.PathValue("id"), time.Now())
		if err != nil {
			http.Error(w, err.Error(), api.StatusFor(err))
			return
		}
		if language != "" {
			w.Header().Set("X-Paste-Language", language)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(content))
	}
}
