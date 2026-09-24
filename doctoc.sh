#!/bin/sh
# Rewrites the contents list in each long document, between its <!-- toc --> and
# <!-- /toc --> markers. Run it after adding, renaming or removing a heading.
#
# The list carries no line numbers on purpose: a line number is wrong the moment
# anything above it is edited, and a wrong one is worse than none. The heading
# name is enough — `grep -n '^## Token boxes' implementation.md` lands on it.
#
# Each entry may carry a one-line gloss after an em dash, so that a section can
# be ruled in or out without opening it. The glosses are written by hand and
# live in the list itself; this script carries the existing one across to the
# rewritten entry, keyed on the heading, and leaves a renamed or new heading
# bare for someone to gloss.
set -e

cd "$(dirname "$0")"

for f in design.md implementation.md; do
	awk '
	function slug(s,   t) {
		t = tolower(s)
		gsub(/[^a-z0-9 _-]/, "", t)
		gsub(/ /, "-", t)
		return t
	}
	function entry(name, indent,   a, n, line) {
		a = slug(name)
		n = seen[a]++
		if (n > 0) a = a "-" n
		line = indent "- [" name "](#" a ")"
		if (name in gloss) line = line " — " gloss[name]
		return line
	}

	# First pass: the headings, and the glosses already written against them.
	NR == FNR {
		if ($0 ~ /<!-- toc -->/)  { intoc = 1; next }
		if ($0 ~ /<!-- \/toc -->/) { intoc = 0; next }
		if (intoc) {
			if ($0 ~ /^ *- \[/) {
				name = $0
				sub(/^ *- \[/, "", name)
				sub(/\]\(#.*/, "", name)
				if (match($0, /\) — /))
					gloss[name] = substr($0, RSTART + RLENGTH)
			}
			next
		}
		if ($0 ~ /^## /)  heads[++t] = substr($0, 4)
		if ($0 ~ /^### /) { heads[++t] = substr($0, 5); deep[t] = 1 }
		next
	}

	# Second pass: copy the file, replacing whatever sits between the markers.
	/<!-- toc -->/ {
		print; print ""
		for (i = 1; i <= t; i++) print entry(heads[i], deep[i] ? "  " : "")
		print ""
		intoc2 = 1
		next
	}
	/<!-- \/toc -->/ { intoc2 = 0 }
	!intoc2 { print }

	END {
		g = 0
		for (k in gloss) g++
		printf "%s: %d entries, %d glossed\n", FILENAME, t, g > "/dev/stderr"
	}
	' "$f" "$f" > "$f.new"
	mv "$f.new" "$f"
done
