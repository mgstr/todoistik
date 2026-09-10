package web

import (
	"html/template"
	"regexp"
	"strings"
)

// Links in item text.
//
// An item's text is prose, stored exactly as it was typed or captured, and
// nothing here changes that: no field is parsed out of it, nothing is
// rewritten, and the database never sees any of this. It is only how text is
// shown — which is why it lives in internal/web and not in internal/app.
//
// It exists because a capture can arrive with a link already in it. Mail
// capture writes one (implementation.md, "Mail into the inbox"), a reminder's
// note can carry one, and design.md, "Action" names a URL as the first thing
// a description is for. A link that has to be selected and pasted into a
// browser is a link that stops being followed.

// What counts as a link: an explicit `https://` and everything up to
// whitespace. Deliberately nothing else — no bare `www.`, no `example.com` —
// because a rule that guesses at prose would turn a sentence that happens to
// name a domain, or an address like `marju@gmail.com`, into something
// clickable. The same argument the meta line makes for never inventing a token
// out of prose (design.md, "Writing an action").
//
// `http://` is not a link either, and that one is a judgement rather than a
// technicality: a plain-text address in 2026 is either a mistake or bait, and
// making it pressable would be the app vouching for it. It stays as the words
// it is, readable and copyable, which is the right amount of work for
// something worth looking at twice before following.
var linkPattern = regexp.MustCompile(`https://[^\s<>"']+`)

// trimLink takes back the characters a URL swallowed from the sentence around
// it. `<see https://example.com/a.>` ends in a full stop that belongs to the
// prose, and a closing bracket is the link's own only if the text opened one.
func trimLink(u string) string {
	for u != "" {
		last := u[len(u)-1]
		switch last {
		case '.', ',', ';', ':', '!', '?', '\'', '"':
		case ')', ']', '}':
			open := map[byte]byte{')': '(', ']': '[', '}': '{'}[last]
			if strings.Count(u, string(open)) >= strings.Count(u, string(last)) {
				return u
			}
		default:
			return u
		}
		u = u[:len(u)-1]
	}
	return u
}

// findLinks walks the text once and hands back each link with where it sat, so
// that both the inline rendering and the list of links read the same string
// the same way.
func findLinks(s string) [][2]int {
	var out [][2]int
	for _, m := range linkPattern.FindAllStringIndex(s, -1) {
		u := trimLink(s[m[0]:m[1]])
		// `https://.` trims down to a scheme with nothing behind it
		if i := strings.Index(u, "://"); i < 0 || len(u) == i+3 {
			continue
		}
		out = append(out, [2]int{m[0], m[0] + len(u)})
	}
	return out
}

// linkify is the text as it was written, with its links live. The visible
// words are the URL exactly as it sits in the item — the screen is showing
// that text, not a tidied version of it, and a link whose label differs from
// where it goes is the one thing a link may never be.
func linkify(s string) template.HTML {
	spans := findLinks(s)
	if len(spans) == 0 {
		return template.HTML(template.HTMLEscapeString(s))
	}
	var b strings.Builder
	at := 0
	for _, sp := range spans {
		u := s[sp[0]:sp[1]]
		b.WriteString(template.HTMLEscapeString(s[at:sp[0]]))
		b.WriteString(`<a class="ext" href="`)
		b.WriteString(template.HTMLEscapeString(u))
		b.WriteString(`" target="_blank" rel="noopener noreferrer">`)
		b.WriteString(template.HTMLEscapeString(u))
		b.WriteString(`</a>`)
		at = sp[1]
	}
	b.WriteString(template.HTMLEscapeString(s[at:]))
	return template.HTML(b.String())
}

// links is every link in the text, in the order it reads, each one once. It is
// what the screens use that cannot make the text itself live: a box being
// written in, and a row whose title is already a link somewhere else.
func links(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, sp := range findLinks(s) {
		u := s[sp[0]:sp[1]]
		if seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// How wide a link chip is allowed to read, and how much of the tail survives
// when it is cut. Middle and not end, because both ends carry: the host says
// where the chip goes and the tail is what tells two links to the same place
// apart — a Gmail permalink is the same forty characters up to the message id.
const (
	labelWidth = 44
	labelTail  = 12
)

// linkLabel is what a chip says. The scheme is dropped because every link here
// is `https://` and a word every chip carries distinguishes none of them; the
// full URL stays on the chip as its title, so nothing is hidden, only
// shortened.
func linkLabel(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "www.")
	s = strings.TrimSuffix(s, "/")
	r := []rune(s)
	if len(r) <= labelWidth {
		return s
	}
	return string(r[:labelWidth-labelTail-1]) + "…" + string(r[len(r)-labelTail:])
}
