// Package config holds the runtime settings and the key=value file parser.
package config

import (
	"errors"
	"fmt"
	"strings"
)

// MinAdminTokenLength is the shortest admin token the server accepts. A
// shorter secret is not worth guarding.
const MinAdminTokenLength = 16

// Config holds the runtime settings, layered defaults, then file, then
// flags.
type Config struct {
	Addr           string // e.g. ":8080"
	DBPath         string // sqlite file path
	AuthToken      string // secret for the create routes; empty means open
	AdminToken     string // secret for the admin routes; empty removes them
	WebDir         string // on-disk frontend override; empty uses the embedded copy
	RedirectPrefix string // path before a redirect slug, e.g. "/r/"; empty means the root
	PastePrefix    string // path before a raw paste id, e.g. "/p/"; empty means "/pastes/"
}

// Defaults returns the built-in settings, before any file or flag.
func Defaults() Config {
	return Config{
		Addr:        ":8080",
		DBPath:      "jumppad.db",
		PastePrefix: "/pastes/",
	}
}

// NormalizePrefix gives a prefix a leading and a trailing slash. An empty
// prefix becomes the root.
func NormalizePrefix(prefix string) string {
	if prefix == "" {
		return "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	return prefix
}

// Validate refuses a configuration that cannot serve safely. The server
// stops at startup instead of running with a silent weakness.
func (c Config) Validate() error {
	if NormalizePrefix(c.RedirectPrefix) == NormalizePrefix(c.PastePrefix) {
		return fmt.Errorf("redirect_prefix and paste_prefix must differ, and both resolve to %q", NormalizePrefix(c.RedirectPrefix))
	}
	if c.AdminToken == "" {
		return nil
	}
	if len(c.AdminToken) < MinAdminTokenLength {
		return fmt.Errorf("admin_token must be at least %d characters", MinAdminTokenLength)
	}
	if c.AdminToken == c.AuthToken {
		return errors.New("admin_token must differ from auth_token, or the create token gains admin power")
	}
	return nil
}

// WithFile parses a key=value file, where a blank line and a #-comment are
// skipped, and returns a copy of c with every known key applied.
func (c Config) WithFile(data string) (Config, error) {
	for number, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return c, fmt.Errorf("line %d has no '=': %q", number+1, line)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		switch key {
		case "addr":
			c.Addr = value
		case "db_path":
			c.DBPath = value
		case "auth_token":
			c.AuthToken = value
		case "admin_token":
			c.AdminToken = value
		case "web_dir":
			c.WebDir = value
		case "redirect_prefix":
			c.RedirectPrefix = value
		case "paste_prefix":
			c.PastePrefix = value
		default:
			return c, fmt.Errorf("line %d has an unknown key %q", number+1, key)
		}
	}

	return c, nil
}
