#!/bin/sh
# Starts remindersync's loop with the secrets already in the environment.
#
# The three sync programs are started the same way and each needs a different
# piece of the environment, so the knowledge of which pieces sits in a script
# per program rather than in a shell history line that only one machine has.
# Everything they read is in .env/secrets.sh (see README, "Secrets").
#
# Of the three directions this is `loop`, because it is the one that keeps
# running: `move` and `sync` are single passes and are typed out when one list
# is wanted now. The runs it makes are the lines of REMINDERSYNC_CONF, not
# anything decided here.
#
# The file is REMINDERSYNC_CONF, .env/remindersync.conf by default. It sits
# with the secrets rather than in the repository root because it is the same
# kind of thing: the names of your lists, the filters you watch, and a line may
# carry a token of its own. .env/ is 0700, ignored whole, and the one place
# where what is yours rather than the app's is kept.
#
#   ./cmd/start-remindersync.sh              # .env/remindersync.conf, every 5 minutes
#   ./cmd/start-remindersync.sh -period 10   # ...and anything else, passed through
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd -- "$root"

[ -f .env/secrets.sh ]  || { echo "start-remindersync: no .env/secrets.sh — see README, \"Secrets\"" >&2; exit 1; }
[ -x bin/remindersync ] || { echo "start-remindersync: no bin/remindersync — run ./cmd/build.sh" >&2; exit 1; }
. "$root/.env/secrets.sh"

# after the sourcing, not before it: REMINDERSYNC_CONF is one of the things
# that file sets, so reading it first is reading it from the shell that started
# this script and never from the file that documents it
conf=${REMINDERSYNC_CONF:-.env/remindersync.conf}
[ -f "$conf" ] || { echo "start-remindersync: no $conf — see README, \"loop\"" >&2; exit 1; }

# the config file last: the flags this script is given are the loop's own, and
# Go's flag parsing stops at the first word that is not one
exec bin/remindersync loop "$@" "$conf"
