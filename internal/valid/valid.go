// Package valid turns raw user input into a checked value: a slug, a
// target URL, or an expiry time. Every error wraps api.ErrInvalid, so a
// handler maps it to 400 without a type switch.
package valid

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"jump-pad/internal/api"
)

// slugPattern is the only allowed slug character set, after the case fold.
var slugPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// reserved holds the top level path names that a slug must not take. A
// feature package adds its own names with Reserve.
var (
	reservedLock sync.RWMutex
	reserved     = map[string]bool{
		"admin":        true,
		"new-paste":    true,
		"new-redirect": true,
		"pastes":       true,
		"redirects":    true,
		"static":       true,
		"view":         true,
	}
)

// Reserve adds path names that a slug must not take.
func Reserve(names ...string) {
	reservedLock.Lock()
	defer reservedLock.Unlock()
	for _, name := range names {
		reserved[strings.ToLower(name)] = true
	}
}

// IsReserved says whether a name is already taken by a route.
func IsReserved(name string) bool {
	reservedLock.RLock()
	defer reservedLock.RUnlock()
	return reserved[strings.ToLower(name)]
}

// Slug folds a user slug to lowercase, then checks the character set and
// the reserved names. It returns the folded slug.
func Slug(s string) (string, error) {
	folded := strings.ToLower(s)

	if !slugPattern.MatchString(folded) {
		return "", api.Invalid("slug %q must match %s", s, slugPattern.String())
	}
	if IsReserved(folded) {
		return "", api.Invalid("slug %q is reserved", folded)
	}

	return folded, nil
}

// TargetURL accepts an http or https URL with a host that looks real. A
// bare label such as "idlip" is not a domain, and a scheme such as
// javascript: is never a legitimate target.
func TargetURL(raw string) (string, error) {
	if raw == "" {
		return "", api.Invalid("target_url must not be empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", api.Invalid("target_url %q does not parse: %v", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", api.Invalid("target_url %q must use http or https, and not %q", raw, parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", api.Invalid("target_url %q has no host", raw)
	}
	if host != "localhost" && net.ParseIP(host) == nil && !strings.Contains(host, ".") {
		return "", api.Invalid("target_url %q: %q does not look like a domain", raw, host)
	}

	return raw, nil
}

// absoluteDateFormats are tried before a relative duration. They cover the
// date and datetime-local input shapes that the frontend sends.
var absoluteDateFormats = []string{"2006-01-02", "2006-01-02T15:04"}

// Expiry turns a user expiry choice into an absolute unix time, or nil for
// "no expiry". It accepts "", "forever", "permanent", "1d", "1w", "1m", an
// absolute future date, or any duration that time.ParseDuration reads.
func Expiry(choice string, now time.Time) (*int64, error) {
	switch choice {
	case "", "forever", "permanent":
		return nil, nil
	case "1d":
		return unixPointer(now.Add(24 * time.Hour)), nil
	case "1w":
		return unixPointer(now.Add(7 * 24 * time.Hour)), nil
	case "1m":
		return unixPointer(now.Add(30 * 24 * time.Hour)), nil
	}

	for _, layout := range absoluteDateFormats {
		if at, err := time.Parse(layout, choice); err == nil {
			if !at.After(now) {
				return nil, api.Invalid("expiry date %q must be in the future", choice)
			}
			return unixPointer(at), nil
		}
	}

	span, err := time.ParseDuration(choice)
	if err != nil {
		return nil, api.Invalid("expiry %q is not a date or a duration", choice)
	}
	return unixPointer(now.Add(span)), nil
}

// unixPointer boxes a unix time, so it fits a nullable database column.
func unixPointer(at time.Time) *int64 {
	value := at.Unix()
	return &value
}
