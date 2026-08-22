package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jump-pad/internal/admin"
	"jump-pad/internal/api"
	"jump-pad/internal/auth"
	"jump-pad/internal/config"
	"jump-pad/internal/paste"
	"jump-pad/internal/redirect"
	"jump-pad/internal/sqlite"
	"jump-pad/internal/valid"
)

// adminToken is long enough to pass config.Validate.
const adminToken = "admin-token-0123456789"

// newServer returns a server over a fresh migrated in-memory database.
func newServer(t *testing.T, cfg config.Config) *Server {
	t.Helper()
	auth.FailDelay = 0 // the delay guards a real server, and only slows a test
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.Migrate(db, redirect.Migrate, paste.Migrate); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(db, cfg)
}

// newPublicServer returns a server with the given create token and no
// admin token.
func newPublicServer(t *testing.T, token string) http.Handler {
	t.Helper()
	cfg := config.Defaults()
	cfg.AuthToken = token
	return newServer(t, cfg).Routes()
}

// newAdminServer returns a server with the admin routes registered.
func newAdminServer(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.Defaults()
	cfg.AdminToken = adminToken
	return newServer(t, cfg).Routes()
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

// send runs one request with an optional JSON body and admin token.
func send(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("X-Auth-Token", token)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

func TestCreateAndRedirect(t *testing.T) {
	h := newPublicServer(t, "")

	recorder := postForm(t, h, "/redirects", url.Values{"slug": {"docs"}, "target_url": {"https://example.com"}})
	if recorder.Code != http.StatusOK || recorder.Body.String() != "docs" {
		t.Fatalf("create = %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/docs", nil))
	if recorder.Code != http.StatusMovedPermanently || recorder.Header().Get("Location") != "https://example.com" {
		t.Fatalf("redirect = %d %q", recorder.Code, recorder.Header().Get("Location"))
	}

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/missing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing slug = %d, want 404", recorder.Code)
	}
}

func TestRedirectExpired(t *testing.T) {
	h := newPublicServer(t, "")
	postForm(t, h, "/redirects", url.Values{"slug": {"gone"}, "target_url": {"https://example.com"}, "expiry": {"-1h"}})

	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/gone", nil))
	if recorder.Code != http.StatusGone {
		t.Fatalf("expired redirect = %d, want 410", recorder.Code)
	}
}

func TestCreateAndServePaste(t *testing.T) {
	h := newPublicServer(t, "")

	recorder := postForm(t, h, "/pastes", url.Values{"content": {"hello"}, "language": {"python"}})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create paste = %d", recorder.Code)
	}
	id := recorder.Body.String()

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/pastes/"+id, nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "hello" {
		t.Fatalf("serve paste = %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Paste-Language") != "python" {
		t.Fatalf("X-Paste-Language = %q, want python", recorder.Header().Get("X-Paste-Language"))
	}
}

func TestCreatePasteTooLarge(t *testing.T) {
	h := newPublicServer(t, "")
	big := strings.Repeat("x", paste.MaxBytes+1)

	recorder := postForm(t, h, "/pastes", url.Values{"content": {big}})
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized paste = %d, want 413", recorder.Code)
	}
}

func TestStaticConfigReportsTokenState(t *testing.T) {
	body := send(t, newPublicServer(t, ""), "GET", "/static/config.js", "", "").Body.String()
	if !strings.Contains(body, `"authRequired":false`) || !strings.Contains(body, `"adminEnabled":false`) {
		t.Fatalf("config.js with no token = %q", body)
	}

	body = send(t, newAdminServer(t), "GET", "/static/config.js", "", "").Body.String()
	if !strings.Contains(body, `"adminEnabled":true`) {
		t.Fatalf("config.js with an admin token = %q", body)
	}
}

func TestConfigurablePrefixes(t *testing.T) {
	cfg := config.Config{RedirectPrefix: "/r/", PastePrefix: "/p/"}
	h := newServer(t, cfg).Routes()

	postForm(t, h, "/redirects", url.Values{"slug": {"docs"}, "target_url": {"https://example.com"}})
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/r/docs", nil))
	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /r/docs = %d, want 301", recorder.Code)
	}

	id := postForm(t, h, "/pastes", url.Values{"content": {"hello"}}).Body.String()
	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/p/"+id, nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "hello" {
		t.Fatalf("GET /p/%s = %d %q", id, recorder.Code, recorder.Body.String())
	}
}

func TestCreateRequiresToken(t *testing.T) {
	h := newPublicServer(t, "secret")
	form := url.Values{"slug": {"docs"}, "target_url": {"https://example.com"}}

	if code := postForm(t, h, "/redirects", form).Code; code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", code)
	}

	request := httptest.NewRequest("POST", "/redirects", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Auth-Token", "secret")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("with a token = %d, want 200", recorder.Code)
	}
}

func TestAdminRoutesDoNotExistWithoutAToken(t *testing.T) {
	h := newPublicServer(t, "")

	if code := send(t, h, "GET", "/admin/api/items", "", "").Code; code != http.StatusNotFound {
		t.Fatalf("admin list with no admin_token = %d, want 404", code)
	}
	if code := send(t, h, "GET", "/admin", "", "").Code; code != http.StatusOK {
		t.Fatalf("the admin page itself = %d, want 200", code)
	}
}

func TestAdminRequiresTheAdminToken(t *testing.T) {
	h := newAdminServer(t)

	if code := send(t, h, "GET", "/admin/api/items", "", "").Code; code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", code)
	}
	if code := send(t, h, "GET", "/admin/api/items", "", "wrong-token-0123456789").Code; code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", code)
	}
	recorder := send(t, h, "GET", "/admin/api/items", "", adminToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("right token = %d, want 200", recorder.Code)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
}

func TestAdminAddEditAndRemoveARedirect(t *testing.T) {
	h := newAdminServer(t)

	added := send(t, h, "POST", "/admin/api/redirects", `{"slug":"docs","target_url":"https://a.example"}`, adminToken)
	if added.Code != http.StatusCreated {
		t.Fatalf("add = %d %q, want 201", added.Code, added.Body.String())
	}

	edited := send(t, h, "PUT", "/admin/api/redirects/docs", `{"slug":"guide","target_url":"https://b.example","expiry":"1d"}`, adminToken)
	if edited.Code != http.StatusNoContent {
		t.Fatalf("edit = %d %q, want 204", edited.Code, edited.Body.String())
	}

	var items admin.Items
	listed := send(t, h, "GET", "/admin/api/items", "", adminToken)
	if err := json.Unmarshal(listed.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode the list: %v", err)
	}
	if len(items.Redirects) != 1 || items.Redirects[0].Slug != "guide" {
		t.Fatalf("list = %+v, want one row named guide", items.Redirects)
	}

	if code := send(t, h, "DELETE", "/admin/api/redirects/guide", "", adminToken).Code; code != http.StatusNoContent {
		t.Fatalf("remove = %d, want 204", code)
	}
	if code := send(t, h, "DELETE", "/admin/api/redirects/guide", "", adminToken).Code; code != http.StatusNotFound {
		t.Fatalf("second remove = %d, want 404", code)
	}
}

func TestAdminRenameToATakenSlug(t *testing.T) {
	h := newAdminServer(t)
	for _, slug := range []string{"docs", "notes"} {
		send(t, h, "POST", "/admin/api/redirects", `{"slug":"`+slug+`","target_url":"https://example.com"}`, adminToken)
	}

	recorder := send(t, h, "PUT", "/admin/api/redirects/notes", `{"slug":"docs","target_url":"https://example.com"}`, adminToken)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("rename to a taken slug = %d, want 409", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error"`) {
		t.Fatalf("body = %q, want a JSON error object", recorder.Body.String())
	}
}

func TestAdminReadsAndEditsAnExpiredPaste(t *testing.T) {
	h := newAdminServer(t)
	added := send(t, h, "POST", "/admin/api/pastes", `{"slug":"stale","content":"old text","expiry":"-1h"}`, adminToken)
	if added.Code != http.StatusCreated {
		t.Fatalf("add = %d %q", added.Code, added.Body.String())
	}

	// The public route refuses an expired paste.
	if code := send(t, h, "GET", "/pastes/stale", "", "").Code; code != http.StatusGone {
		t.Fatalf("public read = %d, want 410", code)
	}

	// The admin route reads it, so the content is editable as it stands.
	read := send(t, h, "GET", "/admin/api/pastes/stale", "", adminToken)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), "old text") {
		t.Fatalf("admin read = %d %q", read.Code, read.Body.String())
	}

	edited := send(t, h, "PUT", "/admin/api/pastes/stale", `{"slug":"stale","content":"new text","expiry":"1w"}`, adminToken)
	if edited.Code != http.StatusNoContent {
		t.Fatalf("edit = %d %q", edited.Code, edited.Body.String())
	}
	if body := send(t, h, "GET", "/pastes/stale", "", "").Body.String(); body != "new text" {
		t.Fatalf("public read after the edit = %q", body)
	}
}

func TestAdminRefusesInvalidInput(t *testing.T) {
	h := newAdminServer(t)

	recorder := send(t, h, "POST", "/admin/api/redirects", `{"slug":"docs","target_url":"https://idlip"}`, adminToken)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a host with no dot = %d, want 400", recorder.Code)
	}
	if code := send(t, h, "PUT", "/admin/api/redirects/missing", `{"slug":"missing","target_url":"https://example.com"}`, adminToken).Code; code != http.StatusNotFound {
		t.Fatalf("edit of a missing row = %d, want 404", code)
	}
}

func TestEveryRouteCarriesASummary(t *testing.T) {
	cfg := config.Defaults()
	cfg.AdminToken = adminToken
	for _, route := range newServer(t, cfg).RouteTable() {
		if strings.TrimSpace(route.Summary) == "" {
			t.Errorf("route %s %s has no summary, so it cannot be documented", route.Method, route.Pattern)
		}
		if route.Handler == nil {
			t.Errorf("route %s %s has no handler", route.Method, route.Pattern)
		}
	}
}

func TestEveryLiteralRouteNameIsReserved(t *testing.T) {
	cfg := config.Defaults()
	cfg.AdminToken = adminToken
	for _, route := range newServer(t, cfg).RouteTable() {
		name := firstSegment(route.Pattern)
		if name == "" || strings.HasPrefix(name, "{") {
			continue
		}
		if !valid.IsReserved(name) {
			t.Errorf("route %s %s owns the name %q, and a slug can still take it", route.Method, route.Pattern, name)
		}
	}
}

// firstSegment returns the first path segment of a route pattern.
func firstSegment(pattern string) string {
	return strings.Split(strings.TrimPrefix(pattern, "/"), "/")[0]
}

func TestSecurityHeaders(t *testing.T) {
	recorder := send(t, newAdminServer(t), "GET", "/admin", "", "")
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", recorder.Header().Get("X-Content-Type-Options"))
	}
	if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("Content-Security-Policy = %q", recorder.Header().Get("Content-Security-Policy"))
	}
}

func TestStatusForUnknownError(t *testing.T) {
	if got := api.StatusFor(errUnknown{}); got != http.StatusInternalServerError {
		t.Fatalf("StatusFor(unknown) = %d, want 500", got)
	}
}

// errUnknown stands for a fault that no sentinel covers.
type errUnknown struct{}

func (errUnknown) Error() string { return "some database fault" }
