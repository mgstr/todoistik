// todoistik: a single-user GTD app. See design.md for what it does and why,
// implementation.md for what it is built out of.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"todoistik/internal/app"
	"todoistik/internal/conf"
	"todoistik/internal/web"
)

func main() {
	addr := flag.String("addr", env("TODOISTIK_ADDR", "127.0.0.1:8390"), "listen address")
	dbPath := flag.String("db", env("TODOISTIK_DB", "todoistik.db"), "path to the SQLite database file")
	token := flag.String("token", os.Getenv("TODOISTIK_TOKEN"), "bearer token (required)")
	tz := flag.String("tz", env("TODOISTIK_TZ", "Local"), "the one timezone that defines the day")
	cfgPath := flag.String("config", env("TODOISTIK_CONFIG", "todoistik.conf"), "settings file; missing is fine, wrong is fatal")
	flag.Parse()

	tok, err := checkedToken(*token)
	if err != nil {
		log.Fatal(err)
	}

	// read once, at startup, deliberately: see internal/conf
	cfg, err := conf.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	loc, err := time.LoadLocation(*tz)
	if err != nil {
		log.Fatalf("bad timezone %q: %v", *tz, err)
	}

	a, err := app.Open(*dbPath, loc)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer a.Close()
	a.SetSomedayReviewDays(cfg.ReviewSomedayDays)

	s, err := web.New(a, tok, cfg)
	if err != nil {
		log.Fatalf("web: %v", err)
	}

	// one snapshot now and one on every hour, for as long as this runs
	go a.BackupHourly(cfg.BackupDays)

	// no "auth on" in this line any more: it is the only thing it could say
	fmt.Printf("todoistik listening on http://%s (db %s, tz %s, config %s, backups %s)\n",
		*addr, *dbPath, loc, *cfgPath, backupsSay(cfg.BackupDays, *dbPath))
	log.Fatal(http.ListenAndServe(*addr, s.Handler()))
}

// checkedToken is the one rule the server will not start without: there has to
// be a token. A server without one answers anyone who can route to it, and this
// one holds everything you are working on.
//
// It used to be optional, on the argument that an empty token is fine behind
// localhost. That is true of the afternoon it is typed and not of the app:
// binding it to 0.0.0.0 later is one word in a unit file, and nothing about
// that word says it is also turning authentication off. A rule that holds only
// while nobody edits the address is not a rule, so the address is not part of
// this one — there is simply no way to run without a token.
//
// The trimmed value is the one that counts, so a token that picked up a newline
// from a file or an editor is the token you meant, rather than a login that
// silently never matches (implementation.md, "API authentication").
func checkedToken(raw string) (string, error) {
	if tok := strings.TrimSpace(raw); tok != "" {
		return tok, nil
	}
	return "", errors.New(`no token: set TODOISTIK_TOKEN (see README, "Secrets") or pass -token. Make one with: openssl rand -hex 24`)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// backupsSay is the startup line's word on backups: where they are and how
// many, or that there are none. A directory the app writes to unasked is a
// thing it should say out loud once.
func backupsSay(days int, dbPath string) string {
	if days <= 0 {
		return "off"
	}
	return fmt.Sprintf("%d days hourly in %s", days, app.BackupDir(dbPath))
}
