// Package httpserver assembles the route table of every feature, registers
// it, and serves the single page frontend. It is the only package that
// knows about more than one feature.
package httpserver

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"jump-pad/internal/admin"
	"jump-pad/internal/api"
	"jump-pad/internal/auth"
	"jump-pad/internal/config"
	"jump-pad/internal/paste"
	"jump-pad/internal/redirect"
)

//go:embed web
var embeddedWeb embed.FS

// contentPolicy allows same origin files only. Every script and style of
// this project is a local file, so no host outside this origin is needed.
const contentPolicy = "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"

// Server holds what every handler needs: the database, the configuration,
// and the resolved path prefixes.
type Server struct {
	db             *sql.DB
	cfg            config.Config
	redirectPrefix string
	pastePrefix    string
}

// New builds a Server from cfg. An empty AuthToken leaves the create
// routes open, and an empty AdminToken removes the admin routes.
func New(db *sql.DB, cfg config.Config) *Server {
	return &Server{
		db:             db,
		cfg:            cfg,
		redirectPrefix: config.NormalizePrefix(cfg.RedirectPrefix),
		pastePrefix:    config.NormalizePrefix(cfg.PastePrefix),
	}
}

// RouteTable is every route this server serves. The registration below and
// cmd/apidocs both read it, so a route documents itself.
func (s *Server) RouteTable() []api.Route {
	routes := []api.Route{
		{
			Method:  "GET",
			Pattern: "/",
			Access:  api.Public,
			Summary: "Serve the landing page. A path that no other route claims gets 404 here.",
			Handler: s.rootPage,
		},
		s.pageRoute("/new-redirect", "Serve the redirect creation form."),
		s.pageRoute("/new-paste", "Serve the paste creation form."),
		s.pageRoute("/view/{id}", "Serve the highlighted read page for a paste."),
		s.pageRoute("/admin", "Serve the admin page. The page itself is public, and its API needs the admin token."),
		{
			Method:  "GET",
			Pattern: "/static/config.js",
			Access:  api.Public,
			Summary: "Answer with the resolved prefixes and the auth state as JavaScript, for the frontend.",
			Handler: s.staticConfig,
		},
		{
			Method:  "GET",
			Pattern: "/static/",
			Access:  api.Public,
			Summary: "Serve the stylesheet, the scripts, and the vendored files.",
			Handler: s.static,
		},
	}

	routes = append(routes, redirect.Routes(s.db, s.redirectPrefix)...)
	routes = append(routes, paste.Routes(s.db, s.pastePrefix)...)
	if s.cfg.AdminToken != "" {
		routes = append(routes, admin.Routes(s.db)...)
	}
	return routes
}

// Routes registers every route behind the guard its access level asks for.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	for _, route := range s.RouteTable() {
		mux.HandleFunc(fmt.Sprintf("%s %s", route.Method, route.Pattern), s.guard(route))
	}
	return withSecurityHeaders(mux)
}

// guard wraps one handler with the check that its access level asks for.
func (s *Server) guard(route api.Route) http.HandlerFunc {
	switch route.Access {
	case api.CreateToken:
		return auth.RequireToken(s.cfg.AuthToken, route.Handler)
	case api.AdminToken:
		return auth.RequireAdmin(s.cfg.AdminToken, withNoStore(route.Handler))
	}
	return route.Handler
}

// withSecurityHeaders sets the headers that every answer carries.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", contentPolicy)
		next.ServeHTTP(w, r)
	})
}

// withNoStore keeps an admin answer out of the browser cache, so a shared
// browser holds no copy of the rows.
func withNoStore(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h(w, r)
	}
}

// pageRoute describes one route that serves the single page shell.
func (s *Server) pageRoute(pattern, summary string) api.Route {
	return api.Route{Method: "GET", Pattern: pattern, Access: api.Public, Summary: summary, Handler: s.page}
}

// frontend returns the on-disk override when one is configured, and the
// embedded default otherwise. It resolves per request, so an override
// takes effect without a restart.
func (s *Server) frontend() (fs.FS, error) {
	if s.cfg.WebDir != "" {
		return os.DirFS(s.cfg.WebDir), nil
	}
	return fs.Sub(embeddedWeb, "web")
}

// rootPage serves the landing page. The "/" pattern also matches every
// path that no other route claims, and those get 404 instead of the page
// shell, so a probe of a route that does not exist gets a true answer.
func (s *Server) rootPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.page(w, r)
}

// page serves index.html. The client side script picks the section from
// the path.
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	files, err := s.frontend()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.ServeFileFS(w, r, files, "index.html")
}

// static serves everything under static/.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	files, err := s.frontend()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	staticFiles, err := fs.Sub(files, "static")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.StripPrefix("/static/", http.FileServerFS(staticFiles)).ServeHTTP(w, r)
}

// staticConfig tells the frontend the resolved prefixes and which tokens
// the server asks for, so the page builds correct links and hides the
// fields that have nothing to fill.
func (s *Server) staticConfig(w http.ResponseWriter, r *http.Request) {
	settings, _ := json.Marshal(map[string]any{
		"redirectPrefix": s.redirectPrefix,
		"pastePrefix":    s.pastePrefix,
		"authRequired":   s.cfg.AuthToken != "",
		"adminEnabled":   s.cfg.AdminToken != "",
	})
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	fmt.Fprintf(w, "window.JUMPPAD_CONFIG = %s;\n", settings)
}
