#!/bin/sh
# Starts the app: the built binary, with the secrets already in the environment.
#
# The fourth start script, and the one the other three talk to. Like them it
# sources .env/secrets.sh and refuses with one line when something it needs is
# missing; unlike them it passes nothing on the command line, because every
# setting the server has is an env var main.go already reads (-addr, -db,
# -token, -tz, -config). Naming them again as flags would buy nothing and cost
# the one thing that matters: -token on the command line is the bearer token on
# the process list for anyone with `ps` (see implementation.md, "Where a secret
# lives"). Anything you give this script is passed through, so a flag still
# wins for one run.
#
# The app will not start without a token. If it refuses, .env/secrets.sh is
# where to put one, not here — the refusal is the binary's, so that it reaches
# every way of starting the app and not only this one.
#
#   ./cmd/start-todoistik.sh                  # the address and token from the file
#   ./cmd/start-todoistik.sh -addr :8391      # ...and anything else, passed through
#
# For working on the app rather than running it, its twin is
# start-todoistik-dev.sh, which compiles the tree on every save.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd -- "$root"

[ -f .env/secrets.sh ] || { echo "start-todoistik: no .env/secrets.sh — see README, \"Secrets\"" >&2; exit 1; }
[ -x bin/todoistik ]   || { echo "start-todoistik: no bin/todoistik — run ./cmd/build.sh" >&2; exit 1; }
. "$root/.env/secrets.sh"

exec bin/todoistik "$@"
