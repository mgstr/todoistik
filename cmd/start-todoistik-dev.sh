#!/bin/sh
# Starts the app from source, rebuilding and restarting on every save.
#
# It is start-todoistik.sh's twin, and the difference is the whole of it: that
# one runs the built binary, this one compiles the tree on every change. That
# is running the app against working on it — a server that recompiles for three
# seconds on every keystroke is no use for the first, and one that has to be
# rebuilt by hand is no use for the second.
#
# templates/ and static/ are //go:embed-ed, so a template or CSS edit only
# reaches the browser through a recompile — hence the extra -file patterns.
#
# The secrets come from .env/secrets.sh like every other start script, and each
# setting below keeps what that file gave it. What is left is a fallback, and
# the one that matters is the token: with no token of your own this serves
# under a fixed throwaway one, so a restart does not log you out mid-session
# and there is nothing to set up before the first run.
#
#   ./cmd/start-todoistik-dev.sh                              # 127.0.0.1:8390
#   TODOISTIK_ADDR=0.0.0.0:8390 ./cmd/start-todoistik-dev.sh  # ...or any of them inline
#
# Needs wgo:  go install github.com/bokwoon95/wgo@latest   (~/go/bin on PATH)
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd -- "$root"

[ -f .env/secrets.sh ] || { echo "start-todoistik-dev: no .env/secrets.sh — see README, \"Secrets\"" >&2; exit 1; }
command -v wgo >/dev/null || { echo "start-todoistik-dev: no wgo — go install github.com/bokwoon95/wgo@latest" >&2; exit 1; }
. "$root/.env/secrets.sh"

# after the sourcing, so the file wins and these are only what is left
export TODOISTIK_TOKEN="${TODOISTIK_TOKEN}"
export TODOISTIK_ADDR="${TODOISTIK_ADDR:-127.0.0.1:8390}"
export TODOISTIK_DB="${TODOISTIK_DB:-$HOME/todoistik.db}"
export TODOISTIK_TZ="${TODOISTIK_TZ:-Europe/Tallinn}"

exec wgo run -file '\.html$' -file '\.css$' -file '\.js$' . "$@"
