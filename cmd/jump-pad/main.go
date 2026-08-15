// Command jump-pad runs the redirector and pastebin HTTP server.
package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"

	"jump-pad/internal/config"
	"jump-pad/internal/httpserver"
	"jump-pad/internal/paste"
	"jump-pad/internal/redirect"
	"jump-pad/internal/sqlite"
)

func main() {
	addr := flag.String("addr", "", "listen address, e.g. :8080")
	dbPath := flag.String("db", "", "sqlite file path")
	token := flag.String("token", "", "secret required on the create routes")
	webDir := flag.String("web-dir", "", "on-disk frontend override dir")
	redirectPrefix := flag.String("redirect-prefix", "", `path prefix before a redirect slug, e.g. "/r/"`)
	pastePrefix := flag.String("paste-prefix", "", `path prefix before a raw paste id, e.g. "/p/"`)
	configPath := flag.String("config", "", "path to a key=value config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	cfg = applyFlags(cfg, flagValues{
		addr:           *addr,
		dbPath:         *dbPath,
		token:          *token,
		webDir:         *webDir,
		redirectPrefix: *redirectPrefix,
		pastePrefix:    *pastePrefix,
	})
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	srv := httpserver.New(db, cfg)
	log.Printf("jump-pad listening on %s, admin routes enabled: %t", cfg.Addr, cfg.AdminToken != "")
	log.Fatal(http.ListenAndServe(cfg.Addr, srv.Routes()))
}

// loadConfig starts from the defaults, then layers an optional file.
func loadConfig(path string) (config.Config, error) {
	cfg := config.Defaults()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	return cfg.WithFile(string(data))
}

// flagValues carries the command line values, so applyFlags keeps one
// parameter instead of a long list.
type flagValues struct {
	addr, dbPath, token, webDir, redirectPrefix, pastePrefix string
}

// applyFlags overrides cfg with every non-empty flag. A flag wins over the
// file.
func applyFlags(cfg config.Config, flags flagValues) config.Config {
	for _, pair := range []struct {
		value  string
		target *string
	}{
		{flags.addr, &cfg.Addr},
		{flags.dbPath, &cfg.DBPath},
		{flags.token, &cfg.AuthToken},
		{flags.webDir, &cfg.WebDir},
		{flags.redirectPrefix, &cfg.RedirectPrefix},
		{flags.pastePrefix, &cfg.PastePrefix},
	} {
		if pair.value != "" {
			*pair.target = pair.value
		}
	}
	return cfg
}

// openDB opens the database file and runs the migration of every feature.
func openDB(path string) (*sql.DB, error) {
	db, err := sqlite.Open(path)
	if err != nil {
		return nil, err
	}
	if err := sqlite.Migrate(db, redirect.Migrate, paste.Migrate); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
