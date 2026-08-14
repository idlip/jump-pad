// Package api holds what every feature package needs to serve HTTP: the
// route table, the shared error sentinels, request decoding, and response
// writing. It imports no other package of this project, so any package can
// import it without a cycle.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// Access says who can reach a route.
type Access int

const (
	Public      Access = iota // anyone
	CreateToken               // the shared create token, when one is set
	AdminToken                // the admin token only
)

// String names the access level for the generated documentation.
func (a Access) String() string {
	switch a {
	case CreateToken:
		return "create token"
	case AdminToken:
		return "admin token"
	}
	return "public"
}

// Param is one field that a route reads. The documentation generator
// prints it.
type Param struct {
	Name        string
	Required    bool
	Description string
}

// Route is one endpoint. The server registers from this list, and
// cmd/apidocs writes docs/api.org from the same list. A route without a
// table entry does not serve, so a route cannot stay undocumented.
type Route struct {
	Method  string
	Pattern string
	Access  Access
	Summary string
	Params  []Param
	Handler http.HandlerFunc
}

// The shared error sentinels. Every package returns these, and StatusFor
// maps them to a status code in one place.
var (
	ErrNotFound  = errors.New("not found")
	ErrExpired   = errors.New("expired")
	ErrSlugTaken = errors.New("slug is already taken")
	ErrInvalid   = errors.New("invalid input")
	ErrTooLarge  = errors.New("body is too large")
)

// Invalid returns an error that wraps ErrInvalid, so it maps to 400.
func Invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// StatusFor maps an error to its HTTP status. An unknown error is a server
// fault, so it maps to 500.
func StatusFor(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrExpired):
		return http.StatusGone
	case errors.Is(err, ErrSlugTaken):
		return http.StatusConflict
	case errors.Is(err, ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusInternalServerError
}
