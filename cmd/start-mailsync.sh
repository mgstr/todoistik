#!/bin/sh
# Starts mailsync with the secrets already in the environment.
#
# The three sync programs are started the same way and each needs a different
# piece of the environment, so the knowledge of which pieces sits in a script
# per program rather than in a shell history line that only one machine has.
# Everything they read is in .env/secrets.sh (see README, "Secrets"): sourcing
# it here is what keeps the app password off the command line, where `ps` would
# show it to anyone on the machine.
#
# The account is MAILSYNC_USER and is required; the label is MAILSYNC_LABEL and
# defaults to todoistik. Both are passed as flags rather than left for mailsync
# to read out of the environment itself, so that this script is the one place
# saying what mailsync needs, and a missing account is refused here — naming the
# file to put it in — instead of surfacing as the program's own "required" a
# layer further away. The address may be a flag where the password may not: it
# is an identifier, not a credential, and it is on every message in the mailbox
# already. Anything you add wins over both, since Go's flag parsing takes the
# last occurrence.
#
#   ./cmd/start-mailsync.sh                     # the account and label from the file
#   ./cmd/start-mailsync.sh -dry-run            # ...and anything else, passed through
#   ./cmd/start-mailsync.sh -user other@gmail.com   # overriding the environment
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd -- "$root"

[ -f .env/secrets.sh ] || { echo "start-mailsync: no .env/secrets.sh — see README, \"Secrets\"" >&2; exit 1; }
[ -x bin/mailsync ]    || { echo "start-mailsync: no bin/mailsync — run ./cmd/build.sh" >&2; exit 1; }
. "$root/.env/secrets.sh"

[ -n "${MAILSYNC_USER:-}" ] || { echo "start-mailsync: no MAILSYNC_USER in .env/secrets.sh — the Gmail account the label is on" >&2; exit 1; }

exec bin/mailsync -user "$MAILSYNC_USER" -label "${MAILSYNC_LABEL:-todoistik}" "$@"
