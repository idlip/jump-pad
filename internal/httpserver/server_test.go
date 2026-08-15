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
