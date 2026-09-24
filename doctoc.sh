#!/bin/sh
# Rewrites the contents list in each long document, between its <!-- toc --> and
# <!-- /toc --> markers. Run it after adding, renaming or removing a heading.
#
# The list carries no line numbers on purpose: a line number is wrong the moment
# anything above it is edited, and a wrong one is worse than none. The heading
# name is enough — `grep -n '^## Token boxes' implementation.md` lands on it.
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
	function entry(name, indent,   a, n) {
		a = slug(name)
		n = seen[a]++
		if (n > 0) a = a "-" n
		return indent "- [" name "](#" a ")"
	}

	# First pass: collect the headings.
	NR == FNR {
		if ($0 ~ /^## /)  toc[++t] = entry(substr($0, 4), "")
		if ($0 ~ /^### /) toc[++t] = entry(substr($0, 5), "  ")
		next
	}

	# Second pass: copy the file, replacing whatever sits between the markers.
	/<!-- toc -->/ {
		print; print ""
		for (i = 1; i <= t; i++) print toc[i]
		print ""
		intoc = 1
		next
	}
	/<!-- \/toc -->/ { intoc = 0 }
	!intoc { print }

	END { printf "%s: %d entries\n", FILENAME, t > "/dev/stderr" }
	' "$f" "$f" > "$f.new"
	mv "$f.new" "$f"
done
