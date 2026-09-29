#!/bin/sh
# Rebuilds every binary in this repository into bin/.
#
# There is one of these rather than five `go build -o` lines in the README
# because the set of binaries is not fixed: the app is the module root, and
# every directory under cmd/ is a second program. Discovering them instead of
# listing them means a new command is built by the next run without this file
# being touched — which is the only way a build script stays true.
#
# It lives in cmd/, beside the programs it builds, rather than in bin/ beside
# what it produces: bin/ holds build products and is ignored whole, and a
# source file kept there survives only by an exception in .gitignore that one
# `rm -rf bin` undoes. A file under cmd/ that is not a directory is not a
# command — go and the loop below both pass it by.
#
# Run it from anywhere:  ./cmd/build.sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin=$root/bin

build() {
	printf '%s\n' "$1"
	go build -o "$bin/$1" "$2"
}

cd -- "$root"
mkdir -p -- "$bin"
build todoistik .
for dir in cmd/*/; do
	[ -d "$dir" ] || continue
	build "$(basename -- "$dir")" "./$dir"
done
