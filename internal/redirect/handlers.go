package redirect

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"jump-pad/internal/api"
)

// Routes returns the public routes of this feature. The create route
// answers with the final slug as plain text, because a person with curl
// reads that answer.
func Routes(db *sql.DB, prefix string) []api.Route {
	return []api.Route{{
		Method:  "POST",
		Pattern: "/redirects",
		Access:  api.CreateToken,
		Summary: "Create a redirect and answer with its final slug as plain text.",
		Params: []api.Param{
			{Name: "slug", Required: true, Description: "Wanted short name. A taken name gets a numeric suffix."},
			{Name: "target_url", Required: true, Description: "Full http or https URL to send the visitor to."},
			{Name: "expiry", Description: "Empty for forever, or 1d, 1w, 1m, a future date, or a duration such as 72h."},
		},
		Handler: createHandler(db),
	}, {
		Method:  "GET",
		Pattern: fmt.Sprintf("%s{slug}", prefix),
		Access:  api.Public,
		Summary: "Send a visitor to the target URL of a slug.",
		Params:  []api.Param{{Name: "slug", Required: true, Description: "The short name in the path."}},
		Handler: lookupHandler(db),
	}}
}

// createHandler stores one redirect from a form or JSON body.
func createHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if err := api.Decode(r, &in); err != nil {
			http.Error(w, err.Error(), api.StatusFor(err))
			return
		}

		slug, err := Create(db, in, time.Now())
		if err != nil {
			http.Error(w, err.Error(), api.StatusFor(err))
			return
		}
		w.Write([]byte(slug))
	}
}

// lookupHandler answers 301 to the target URL, or 404 or 410.
func lookupHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, err := Lookup(db, r.PathValue("slug"), time.Now())
		if err != nil {
			http.Error(w, err.Error(), api.StatusFor(err))
			return
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}
