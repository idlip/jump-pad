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

// WriteJSON sends v as JSON with the given status. A nil value sends the
// status and no body.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if v == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError sends {"error": "..."} with the status that fits err.
func WriteError(w http.ResponseWriter, err error) {
	WriteJSON(w, StatusFor(err), map[string]string{"error": err.Error()})
}

// Decode fills v from a JSON body when the request says JSON, and from
// form values in every other case. One decode path therefore serves the
// browser forms, the admin page, and curl.
func Decode(r *http.Request, v any) error {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(v); err != nil {
			return decodeError("JSON", err)
		}
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return decodeError("form", err)
	}
	return fillFromForm(r, v)
}

// decodeError separates an oversized body from a malformed one, because
// the two answer with different status codes.
func decodeError(kind string, err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return ErrTooLarge
	}
	return Invalid("malformed %s body: %v", kind, err)
}

// fillFromForm copies form values into the string fields of v. A field
// takes its name from its form tag, or from its lowercased field name.
func fillFromForm(r *http.Request, v any) error {
	pointer := reflect.ValueOf(v)
	if pointer.Kind() != reflect.Pointer || pointer.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("Decode needs a pointer to a struct, and got %T", v)
	}

	target := pointer.Elem()
	for i := range target.NumField() {
		field := target.Type().Field(i)
		if field.Type.Kind() != reflect.String {
			continue
		}
		name := field.Tag.Get("form")
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		target.Field(i).SetString(r.FormValue(name))
	}
	return nil
}
// LimitBody caps a request body before a handler reads it.
func LimitBody(max int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, max)
		h(w, r)
	}
}
