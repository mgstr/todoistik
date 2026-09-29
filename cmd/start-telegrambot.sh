#!/bin/sh
# Starts telegrambot with the secrets already in the environment.
#
# The three sync programs are started the same way and each needs a different
# piece of the environment, so the knowledge of which pieces sits in a script
# per program rather than in a shell history line that only one machine has.
# Everything they read is in .env/secrets.sh (see README, "Secrets").
#
# The chat is TELEGRAM_CHAT, passed as a flag the way mailsync's account is, so
# that this script is the one place saying what telegrambot needs. It is passed
# even when empty, because empty is a legal answer and not a missing one: the
# bot then answers nobody and prints the id of whoever writes, which is where
# you get the number to put in the file.
#
# The app's bearer token is handed over as a *path*, not a value: `ps` shows
# every flag on the machine, so -app-token-file names the file and the secret
# stays in it (see implementation.md, "Where a secret lives"). The file is
# TODOISTIK_TOKEN_FILE, .env/todoistik-token by default, and it is optional —
# without it the token comes from TODOISTIK_TOKEN in the environment, which is
# what a server started without a token wants anyway. The bot's own token is
# TELEGRAM_TOKEN and stays an export for the same reason the path is a path.
#
#   ./cmd/start-telegrambot.sh                    # the chat and token from the file
#   ./cmd/start-telegrambot.sh -chat 123456789    # overriding the environment
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd -- "$root"

[ -f .env/secrets.sh ]  || { echo "start-telegrambot: no .env/secrets.sh — see README, \"Secrets\"" >&2; exit 1; }
[ -x bin/telegrambot ]  || { echo "start-telegrambot: no bin/telegrambot — run ./cmd/build.sh" >&2; exit 1; }
. "$root/.env/secrets.sh"

# the token file if there is one, the environment if there is not, and one line
# if there is neither — a bot started with no token reaches the app and is
# refused, which is the same failure said later and further from its cause
token_file=${TODOISTIK_TOKEN_FILE:-.env/todoistik-token}
if [ -f "$token_file" ]; then
	set -- -app-token-file "$token_file" "$@"
elif [ -z "${TODOISTIK_TOKEN:-}" ]; then
	echo "start-telegrambot: no app token — put it in $token_file, or export TODOISTIK_TOKEN in .env/secrets.sh" >&2
	exit 1
fi

exec bin/telegrambot -chat "${TELEGRAM_CHAT:-}" "$@"
