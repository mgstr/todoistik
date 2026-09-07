// todoistik: a single-user GTD app. See design.md for what it does and why,
// implementation.md for what it is built out of.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"todoistik/internal/app"
	"todoistik/internal/conf"
	"todoistik/internal/web"
)

func main() {
	addr := flag.String("addr", env("TODOISTIK_ADDR", "127.0.0.1:8390"), "listen address")
	dbPath := flag.String("db", env("TODOISTIK_DB", "todoistik.db"), "path to the SQLite database file")
	token := flag.String("token", os.Getenv("TODOISTIK_TOKEN"), "bearer token; empty disables auth (bind to localhost only)")
	tz := flag.String("tz", env("TODOISTIK_TZ", "Local"), "the one timezone that defines the day")
	cfgPath := flag.String("config", env("TODOISTIK_CONFIG", "todoistik.conf"), "settings file; missing is fine, wrong is fatal")
	flag.Parse()

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

	s, err := web.New(a, *token, cfg)
	if err != nil {
		log.Fatalf("web: %v", err)
	}

	fmt.Printf("todoistik listening on http://%s (db %s, tz %s, auth %s, config %s)\n",
		*addr, *dbPath, loc, onOff(*token != ""), *cfgPath)
	log.Fatal(http.ListenAndServe(*addr, s.Handler()))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
