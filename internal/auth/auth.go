// Package auth guards handlers behind a shared secret. The create token
// and the admin token differ on purpose: the create token accepts a query
// parameter for curl, and the admin token accepts the header only.
package auth

import (
	"crypto/subtle"
	"net/http"
	"time"

	"jump-pad/internal/api"
)

// FailDelay is the wait after a wrong admin token. It makes online
// guessing useless. A test sets it to zero.
var FailDelay = 500 * time.Millisecond

// Equal compares two secrets in constant time. A plain == returns faster
// on an early wrong byte, which leaks the secret one byte at a time.
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// RequireToken guards a create route. An empty token means that no token
// is configured, so every request passes.
func RequireToken(token string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" || hasCreateToken(r, token) {
			h(w, r)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

// hasCreateToken reads the X-Auth-Token header, then the token query
// parameter.
func hasCreateToken(r *http.Request, token string) bool {
	return Equal(r.Header.Get("X-Auth-Token"), token) || Equal(r.URL.Query().Get("token"), token)
}
