package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ok answers 200, so a test sees whether the guard let the request through.
func ok(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequireToken(t *testing.T) {
	guarded := RequireToken("secret", ok)

	cases := []struct {
		name   string
		header string
		query  string
		want   int
	}{
		{"the right header", "secret", "", http.StatusOK},
		{"the right query parameter", "", "secret", http.StatusOK},
		{"a wrong token", "nope", "", http.StatusUnauthorized},
		{"no token", "", "", http.StatusUnauthorized},
	}

	for _, one := range cases {
		request := httptest.NewRequest("POST", "/redirects?token="+one.query, nil)
		if one.header != "" {
			request.Header.Set("X-Auth-Token", one.header)
		}
		recorder := httptest.NewRecorder()
		guarded(recorder, request)
		if recorder.Code != one.want {
			t.Errorf("%s: status = %d, want %d", one.name, recorder.Code, one.want)
		}
	}
}

func TestRequireTokenWithNoTokenConfigured(t *testing.T) {
	recorder := httptest.NewRecorder()
	RequireToken("", ok)(recorder, httptest.NewRequest("POST", "/redirects", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when no token is configured", recorder.Code)
	}
}

func TestRequireAdmin(t *testing.T) {
	FailDelay = 0 // the delay guards a real server, and only slows a test
	guarded := RequireAdmin("admin-token-0123456789", ok)

	cases := []struct {
		name   string
		header string
		query  string
		want   int
	}{
		{"the right header", "admin-token-0123456789", "", http.StatusOK},
		{"the token in the query only", "", "admin-token-0123456789", http.StatusUnauthorized},
		{"a wrong token", "nope", "", http.StatusUnauthorized},
		{"no token", "", "", http.StatusUnauthorized},
	}

	for _, one := range cases {
		request := httptest.NewRequest("GET", "/admin/api/items?token="+one.query, nil)
		if one.header != "" {
			request.Header.Set("X-Auth-Token", one.header)
		}
		recorder := httptest.NewRecorder()
		guarded(recorder, request)
		if recorder.Code != one.want {
			t.Errorf("%s: status = %d, want %d", one.name, recorder.Code, one.want)
		}
	}
}

func TestRequireAdminWithNoTokenConfigured(t *testing.T) {
	recorder := httptest.NewRecorder()
	RequireAdmin("", ok)(recorder, httptest.NewRequest("GET", "/admin/api/items", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no admin token is configured", recorder.Code)
	}
}
