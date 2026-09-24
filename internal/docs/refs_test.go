// The long docs are addressed by name. README.md, "Working on this codebase"
// says the conventions that keep design.md and implementation.md searchable are
// "conventions rather than tooling, so both can be broken by hand" - and the
// reference is the one that cannot be checked by hand, because breaking it is
// silent. Renaming a heading leaves the build green, the suite passing and the
// app working, with every citation of the old name pointing at nothing: there
// are some seven hundred of them, a third of those in comments that send a
// reader to the paragraph that decided the code they sit above. This package
// holds no production code. It exists so that a name is an address.
package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The documents a reference may point into. CLAUDE.md cites them and is cited
// by nothing, so it is read for references but declares no targets.
var docFiles = []string{"design.md", "implementation.md", "keys.md", "README.md"}

// A reference is a quoted string introduced by something that marks it as one:
// a document's own filename, or `see`. A bare quoted phrase is not a reference
// - the prose quotes button labels and screen names constantly, and matching
// those would report a hundred things that were never addresses.
var (
	qualifiedRef = regexp.MustCompile(`(?i)\b(design|implementation|keys|readme)\.md(?:'s)?[,:]?\s*"([^"]+)"`)
	seeRef       = regexp.MustCompile(`(?i)\bsee\s+"([^"]+)"`)
)

// A target is a heading, or the bold lead of a paragraph or list item. Both are
// cited today and both deserve to be: a heading names a section, and a bold
// lead names a rule that earns a name without earning a section of its own -
// design.md's "Capture costs nothing" is a bullet in the Principles list, and
// promoting it to a heading would break that list apart.
//
// A target has to be named in full. Most bold leads are whole sentences used
// for emphasis rather than names, so a rule that resolved a citation against
// the start of one would let a mistyped or stale name land on an unrelated
// sentence and report nothing. Written out in full, a match is never an
// accident, and a citation that names a rule by a shortened label - "Zen mode"
// for a sentence that opens with those two words - is reported, because that
// label is not a name anything declares.
var (
	headingTarget  = regexp.MustCompile(`^#+\s+(.*\S)\s*$`)
	boldLeadTarget = regexp.MustCompile(`^(?:[-*]\s+)?\*\*([^*]+)\*\*`)
)

// Prose is quoted as well as named: implementation.md cites design.md for the
// sentence it is obeying, not for a section. The first letter decides which of
// the two a citation is, and so which check it gets - a capital is a name and
// has to be declared, lower case is prose and has to appear in the document
// word for word, so that rewording a sentence another file quotes is caught the
// same way renaming a heading is. The letter is not a rule about what may be
// cited: a few headings and most bold leads open in lower case, being sentences
// rather than titles, and a citation of one is held to the document's words
// instead of to its list of names. That is the weaker of the two checks, and it
// is the right one to fall to, because a name nothing declares would otherwise
// have to be reported on the strength of its first letter alone.
func isQuotation(target string) bool {
	r := []rune(target)
	return len(r) > 0 && r[0] >= 'a' && r[0] <= 'z'
}

func docFileKeys() []string {
	var out []string
	for _, doc := range docFiles {
		out = append(out, strings.ToLower(doc))
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}

var manySpaces = regexp.MustCompile(`\s+`)

// flatten is how a document is searched for a quotation: the prose is wrapped
// at 80 columns, so a quoted sentence is almost always split across a line
// break and would never be found as it is written.
func flatten(body string) string {
	return manySpaces.ReplaceAllString(body, " ")
}

type corpus struct {
	names map[string]map[string]bool // document -> the names it declares
	pool  map[string]bool            // every name, whichever document declares it
	text  map[string]string          // document -> its prose, unwrapped
}

func read(t *testing.T, root string) corpus {
	t.Helper()
	c := corpus{names: map[string]map[string]bool{}, pool: map[string]bool{}, text: map[string]string{}}
	for _, doc := range docFiles {
		body, err := os.ReadFile(filepath.Join(root, doc))
		if err != nil {
			t.Fatal(err)
		}
		key := strings.ToLower(doc)
		c.text[key] = flatten(string(body))
		declared := map[string]bool{}
		for _, line := range strings.Split(string(body), "\n") {
			var declaredName string
			if m := headingTarget.FindStringSubmatch(line); m != nil {
				declaredName = m[1]
			} else if m := boldLeadTarget.FindStringSubmatch(line); m != nil {
				declaredName = m[1]
			}
			if declaredName == "" {
				continue
			}
			// A bold lead opens a sentence and usually closes with a full stop
			// that no citation of it repeats.
			for _, form := range []string{name(declaredName), name(strings.TrimRight(declaredName, ".:,;"))} {
				declared[form] = true
				c.pool[form] = true
			}
		}
		c.names[key] = declared
	}
	return c
}

// citable returns, for each line of a file, the text on it that may hold a
// reference. In a document that is the whole line. In source it is the comment
// only: internal/web/links_test.go builds a fixture out of the string `see "`,
// and code is not where this repository writes its citations anyway.
func citable(path, body string) []string {
	lines := strings.Split(body, "\n")
	out := make([]string, len(lines))
	if strings.HasSuffix(path, ".md") {
		return lines
	}
	inBlock := false
	for i, line := range lines {
		rest, text := line, ""
		for {
			if inBlock {
				end := strings.Index(rest, "*/")
				if end < 0 {
					text += " " + rest
					break
				}
				text += " " + rest[:end]
				rest, inBlock = rest[end+2:], false
				continue
			}
			// A template comment is {{/* ... */}}, a Go or JavaScript block
			// comment /* ... */; both end at the same two characters.
			if open := strings.Index(rest, "/*"); open >= 0 {
				rest, inBlock = rest[open+2:], true
				continue
			}
			if open := strings.Index(rest, "//"); open >= 0 {
				text += " " + rest[open+2:]
			}
			break
		}
		out[i] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "*"))
	}
	return out
}

// name is how a target is compared: a reference that wrapped across a line
// break arrives with the next line's indentation inside it, and a name is not
// two names because the prose ran out of room in the middle of it.
func name(s string) string {
	return strings.TrimSpace(flatten(s))
}

type ref struct {
	file, doc, target string
	line              int
}

// find looks for a reference in a window of three joined lines and reports it
// against the line it starts on. Comments are wrapped at 80 columns like the
// prose they cite, which splits some fifty citations across a line break; the
// window is what makes the wrap cost nothing.
func find(path string, lines []string) []ref {
	var out []ref
	for i := range lines {
		first := lines[i]
		if strings.TrimSpace(first) == "" {
			continue
		}
		window := first
		for j := i + 1; j < len(lines) && j <= i+2; j++ {
			window += " " + lines[j]
		}
		for _, m := range qualifiedRef.FindAllStringSubmatchIndex(window, -1) {
			if m[0] >= len(first) {
				continue // it starts on a later line, and is that line's to report
			}
			out = append(out, ref{file: path, line: i + 1,
				doc: strings.ToLower(window[m[2]:m[3]]) + ".md", target: name(window[m[4]:m[5]])})
		}
		for _, m := range seeRef.FindAllStringSubmatchIndex(window, -1) {
			if m[0] >= len(first) {
				continue
			}
			out = append(out, ref{file: path, line: i + 1, target: name(window[m[2]:m[3]])})
		}
	}
	return out
}

// sources are every file that may cite a document: the documents themselves,
// and the code that implements them.
func sources(t *testing.T, root string) []string {
	t.Helper()
	out := []string{filepath.Join(root, "CLAUDE.md")}
	for _, doc := range docFiles {
		out = append(out, filepath.Join(root, doc))
	}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir():
				// This package's own prose is full of examples of the shapes it
				// looks for, and they are not citations of anything.
				if path == filepath.Join(root, "internal", "docs") {
					return filepath.SkipDir
				}
				return nil
			case strings.HasSuffix(path, ".min.js"):
				return nil // vendored, minified, one line long, cites nothing
			}
			switch filepath.Ext(path) {
			case ".go", ".js", ".html":
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestEveryReferenceToADocumentResolves(t *testing.T) {
	root := repoRoot(t)
	c := read(t, root)

	var broken []string
	for _, path := range sources(t, root) {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range find(rel, citable(rel, string(body))) {
			switch {
			case isQuotation(r.target):
				// An unattributed quotation is looked for in every document,
				// the same way an unqualified name is.
				docs, where := []string{r.doc}, r.doc
				if r.doc == "" {
					docs, where = docFileKeys(), "any document"
				}
				found := false
				for _, doc := range docs {
					if strings.Contains(c.text[doc], flatten(r.target)) {
						found = true
					}
				}
				if !found {
					broken = append(broken, fmt.Sprintf("%s:%d quotes %s, %q - those words are not in it",
						r.file, r.line, where, r.target))
				}
			case r.doc != "":
				if !c.names[r.doc][r.target] {
					broken = append(broken, fmt.Sprintf("%s:%d cites %s, %q - it declares no heading or rule of that name",
						r.file, r.line, r.doc, r.target))
				}
			default:
				if !c.pool[r.target] {
					broken = append(broken, fmt.Sprintf("%s:%d says see %q - no document declares that name",
						r.file, r.line, r.target))
				}
			}
		}
	}
	sort.Strings(broken)
	if len(broken) > 0 {
		t.Errorf("%d reference(s) point at something no document says:\n%s", len(broken), strings.Join(broken, "\n"))
	}
}
