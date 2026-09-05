#!/bin/sh
# Development server: rebuilds and restarts on every save.
#
# templates/ and static/ are //go:embed-ed, so a template or CSS edit only
# reaches the browser through a recompile — hence the extra -file patterns.
# Every setting below is an env var main.go already reads; override any of
# them inline, e.g. TODOISTIK_ADDR=0.0.0.0:8390 ./dev.sh
#
# Needs wgo:  go install github.com/bokwoon95/wgo@latest   (~/go/bin on PATH)
set -eu

# a fixed throwaway token, so restarts do not log you out mid-session
export TODOISTIK_TOKEN="${TODOISTIK_TOKEN:-go}"
export TODOISTIK_ADDR="${TODOISTIK_ADDR:-127.0.0.1:8390}"
export TODOISTIK_DB="${TODOISTIK_DB:-$HOME/todoistik.db}"
export TODOISTIK_TZ="${TODOISTIK_TZ:-Europe/Tallinn}"

exec wgo run -file '\.html$' -file '\.css$' -file '\.js$' .
