#!/bin/sh
# The keyboard trial server: the normalized keys on their own port, against a
# copy of the database, so pressing the new delete key cannot reach real work.
#
# It is separate from cmd/start-todoistik-dev.sh so both can run at once — the
# app as it was on 8390, this on 8391 — which is the only way to compare a key
# by feel.
#
# There is one keyboard now (keys.md, "The two modes"); this stays for trying a
# key that is not built yet.
set -eu

here=$(cd "$(dirname "$0")" && pwd)

export TODOISTIK_TOKEN="${TODOISTIK_TOKEN:-go}"
export TODOISTIK_ADDR="${TODOISTIK_ADDR:-127.0.0.1:8391}"
export TODOISTIK_DB="${TODOISTIK_DB:-$here/.keytest/keytest.db}"
export TODOISTIK_CONFIG="${TODOISTIK_CONFIG:-$here/.keytest/todoistik.conf}"
export TODOISTIK_TZ="${TODOISTIK_TZ:-Europe/Tallinn}"

exec go run "$here"
