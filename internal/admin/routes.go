// Package admin holds the routes that manage existing rows: the list that
// spans both tables, and the add, edit, and remove routes. It owns no
// table and no SQL of its own, so a new feature adds its own routes here
// and keeps its storage in its own package.
package admin

import (
	"database/sql"
	"net/http"
	"time"

	"jump-pad/internal/api"
	"jump-pad/internal/paste"
	"jump-pad/internal/redirect"
)

// Items is the answer of the list route. Pastes carry no content, because
// a few large pastes would make every page load heavy.
type Items struct {
	Redirects []redirect.Redirect `json:"redirects"`
	Pastes    []paste.Paste       `json:"pastes"`
}

// slugParams are the fields that an admin add or edit sends for a
// redirect. An edit sends every field, because a PUT is a full
// replacement.
var redirectParams = []api.Param{
	{Name: "slug", Required: true, Description: "Short name. An edit to a taken name answers 409."},
	{Name: "target_url", Required: true, Description: "Full http or https URL."},
	{Name: "expiry", Description: "Empty for forever, or 1d, 1w, 1m, a future date, or a duration such as 72h."},
}

var pasteParams = []api.Param{
	{Name: "content", Required: true, Description: "The text to store. The cap is 500 KB."},
	{Name: "slug", Required: true, Description: "The paste id. An edit to a taken id answers 409."},
	{Name: "language", Description: "Language hint for syntax highlighting."},
	{Name: "expiry", Description: "Empty for forever, or 1d, 1w, 1m, a future date, or a duration such as 72h."},
}

// Routes returns every admin API route. The server registers these only
// when an admin token is configured.
func Routes(db *sql.DB) []api.Route {
	return []api.Route{{
		Method:  "GET",
		Pattern: "/admin/api/items",
		Access:  api.AdminToken,
		Summary: "List every redirect and every paste, expired rows included. Pastes carry no content.",
		Handler: api.Handle(func(r *http.Request) (any, int, error) {
			redirects, err := redirect.List(db)
			if err != nil {
				return nil, 0, err
			}
			pastes, err := paste.List(db)
			if err != nil {
				return nil, 0, err
			}
			return Items{Redirects: redirects, Pastes: pastes}, http.StatusOK, nil
		}),
	}, {
		Method:  "GET",
		Pattern: "/admin/api/pastes/{id}",
		Access:  api.AdminToken,
		Summary: "Read one paste with its content, whether it is expired or not.",
		Params:  []api.Param{{Name: "id", Required: true, Description: "The paste id in the path."}},
		Handler: api.Handle(func(r *http.Request) (any, int, error) {
			one, err := paste.Get(db, r.PathValue("id"))
			if err != nil {
				return nil, 0, err
			}
			return one, http.StatusOK, nil
		}),
	}, {
		Method:  "POST",
		Pattern: "/admin/api/redirects",
		Access:  api.AdminToken,
		Summary: "Add a redirect. A taken slug gets a numeric suffix, and the answer names the final slug.",
		Params:  redirectParams,
		Handler: api.Handler(func(r *http.Request, in redirect.Input) (any, int, error) {
			slug, err := redirect.Create(db, in, time.Now())
			if err != nil {
				return nil, 0, err
			}
			return map[string]string{"slug": slug}, http.StatusCreated, nil
		}),
	}, {
		Method:  "PUT",
		Pattern: "/admin/api/redirects/{slug}",
		Access:  api.AdminToken,
		Summary: "Replace a redirect. A new slug renames the row, and every link to the old slug stops working.",
		Params:  redirectParams,
		Handler: api.Handler(func(r *http.Request, in redirect.Input) (any, int, error) {
			if err := redirect.Update(db, r.PathValue("slug"), in, time.Now()); err != nil {
				return nil, 0, err
			}
			return nil, http.StatusNoContent, nil
		}),
	}, {
		Method:  "DELETE",
		Pattern: "/admin/api/redirects/{slug}",
		Access:  api.AdminToken,
		Summary: "Remove a redirect.",
		Params:  []api.Param{{Name: "slug", Required: true, Description: "The slug in the path."}},
		Handler: api.Handle(func(r *http.Request) (any, int, error) {
			if err := redirect.Delete(db, r.PathValue("slug")); err != nil {
				return nil, 0, err
			}
			return nil, http.StatusNoContent, nil
		}),
	}, {
		Method:  "POST",
		Pattern: "/admin/api/pastes",
		Access:  api.AdminToken,
		Summary: "Add a paste. An empty slug gets a random id, and the answer names the final id.",
		Params:  pasteParams,
		Handler: api.LimitBody(paste.MaxBytes, api.Handler(func(r *http.Request, in paste.Input) (any, int, error) {
			id, err := paste.Create(db, in, time.Now())
			if err != nil {
				return nil, 0, err
			}
			return map[string]string{"id": id}, http.StatusCreated, nil
		})),
	}, {
		Method:  "PUT",
		Pattern: "/admin/api/pastes/{id}",
		Access:  api.AdminToken,
		Summary: "Replace a paste. A new id renames the row, and every link to the old id stops working.",
		Params:  pasteParams,
		Handler: api.LimitBody(paste.MaxBytes, api.Handler(func(r *http.Request, in paste.Input) (any, int, error) {
			if err := paste.Update(db, r.PathValue("id"), in, time.Now()); err != nil {
				return nil, 0, err
			}
			return nil, http.StatusNoContent, nil
		})),
	}, {
		Method:  "DELETE",
		Pattern: "/admin/api/pastes/{id}",
		Access:  api.AdminToken,
		Summary: "Remove a paste.",
		Params:  []api.Param{{Name: "id", Required: true, Description: "The paste id in the path."}},
		Handler: api.Handle(func(r *http.Request) (any, int, error) {
			if err := paste.Delete(db, r.PathValue("id")); err != nil {
				return nil, 0, err
			}
			return nil, http.StatusNoContent, nil
		}),
	}}
}
