package web

import (
	"strings"
	"testing"
)

// The link mail capture writes, which is the one this exists for: it carries a
// `#`, a `:` and an `@`, all of which a lazier pattern would stop at.
const mailLink = "https://mail.google.com/mail/u/0/#search/rfc822msgid:3E.F7.26533.605A0AA6@i-0a00b522b554e170a.mta1vrest.sd.prd.sparkpost"

func TestLinksFindsTheWholeMailPermalink(t *testing.T) {
	text := "from DevClub <noreply@campaign.eventbrite.com>\n" + mailLink
	got := links(text)
	if len(got) != 1 || got[0] != mailLink {
		t.Fatalf("links = %q, want just the permalink", got)
	}
}

func TestLinksIgnoresWhatIsNotWrittenAsALink(t *testing.T) {
	// an address is not a link, a bare domain is not a link, and a scheme with
	// nothing behind it is not one either
	for _, s := range []string{
		"mail marju@gmail.com about it",
		"see example.com and www.example.com",
		"https://",
		"read https://.",
	} {
		if got := links(s); len(got) != 0 {
			t.Errorf("links(%q) = %q, want none", s, got)
		}
	}
}

// Deliberate, not an oversight: a plain `http://` address is left as words for
// the reason design.md, "Following a link" gives — pressing it would be the
// app vouching for it.
func TestLinksLeavesPlainHTTPAsWords(t *testing.T) {
	if got := links("the invoice is at http://vana.arved.ee/2026/03 apparently"); len(got) != 0 {
		t.Errorf("links = %q, want none", got)
	}
	got := string(linkify("go to http://example.com now"))
	if strings.Contains(got, "<a") {
		t.Errorf("linkify = %q, want no link", got)
	}
	// and the words themselves are still all there
	if got != "go to http://example.com now" {
		t.Errorf("linkify = %q, want the text unchanged", got)
	}
}

func TestLinksTakesTheSecureOneOutOfALineHoldingBoth(t *testing.T) {
	got := links("old http://example.com/a new https://example.com/b")
	if len(got) != 1 || got[0] != "https://example.com/b" {
		t.Fatalf("links = %q, want just the https one", got)
	}
}

func TestLinksHandsBackTheSentencesPunctuation(t *testing.T) {
	cases := map[string]string{
		"open https://example.com/a.":                             "https://example.com/a",
		"open https://example.com/a, then go":                     "https://example.com/a",
		"(see https://example.com/a)":                             "https://example.com/a",
		"https://en.wikipedia.org/wiki/Go_(programming_language)": "https://en.wikipedia.org/wiki/Go_(programming_language)",
	}
	for text, want := range cases {
		got := links(text)
		if len(got) != 1 || got[0] != want {
			t.Errorf("links(%q) = %q, want [%q]", text, got, want)
		}
	}
}

func TestLinksAreListedOnceEachInReadingOrder(t *testing.T) {
	text := "https://b.example/2 and https://a.example/1 and https://b.example/2 again"
	got := links(text)
	want := []string{"https://b.example/2", "https://a.example/1"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("links = %q, want %q", got, want)
	}
}

func TestLinkifyLeavesTheTextSayingWhatItSaid(t *testing.T) {
	got := string(linkify("open " + mailLink + " now"))
	if !strings.HasPrefix(got, `open <a class="ext" `) || !strings.HasSuffix(got, "</a> now") {
		t.Fatalf("linkify = %q", got)
	}
	// the words shown are the URL itself: a link may not say one place and go
	// to another
	if strings.Count(got, mailLink) != 2 {
		t.Errorf("linkify = %q, want the URL as both href and text", got)
	}
	if !strings.Contains(got, `target="_blank"`) || !strings.Contains(got, `rel="noopener noreferrer"`) {
		t.Errorf("linkify = %q, want it to open in its own tab", got)
	}
}

func TestLinkifyEscapesTheProseAroundTheLink(t *testing.T) {
	got := string(linkify(`a <b> & "c" https://example.com/?x=1&y=2`))
	if strings.Contains(got, "<b>") {
		t.Errorf("linkify = %q, want the angle brackets escaped", got)
	}
	if !strings.Contains(got, `href="https://example.com/?x=1&amp;y=2"`) {
		t.Errorf("linkify = %q, want the query string escaped in the href", got)
	}
}

func TestLinkifyTextWithNoLinkIsJustEscapedText(t *testing.T) {
	if got := string(linkify("call the dentist <today>")); got != "call the dentist &lt;today&gt;" {
		t.Fatalf("linkify = %q", got)
	}
}

func TestLinkLabelKeepsBothEndsOfALongLink(t *testing.T) {
	got := linkLabel(mailLink)
	if len([]rune(got)) != labelWidth {
		t.Fatalf("linkLabel = %q, %d runes, want %d", got, len([]rune(got)), labelWidth)
	}
	if !strings.HasPrefix(got, "mail.google.com/") {
		t.Errorf("linkLabel = %q, want it to start at the host", got)
	}
	tail := string([]rune(mailLink)[len([]rune(mailLink))-labelTail:])
	if !strings.HasSuffix(got, tail) {
		t.Errorf("linkLabel = %q, want it to end in %q — the part that tells two of these apart", got, tail)
	}
}

func TestLinkLabelLeavesAShortLinkAlone(t *testing.T) {
	if got := linkLabel("https://www.example.com/a/"); got != "example.com/a" {
		t.Fatalf("linkLabel = %q", got)
	}
}

// The attribute the key layer follows. It has to be exactly what the chips and
// the live words say the text holds, and in the same order, because ^o and the
// eye are looking at the same item.
func TestItemLinksIsTheSameSetOnOneLine(t *testing.T) {
	text := "spec https://a.example/1 and thread https://b.example/2"
	got := itemLinks(text)
	want := "https://a.example/1 https://b.example/2"
	if got != want {
		t.Fatalf("itemlinks = %q, want %q", got, want)
	}
	if strings.Join(links(text), " ") != got {
		t.Fatalf("itemlinks disagrees with links, which is the one thing it must not do")
	}
}

// A capture is a line and a body and is decided about as one item, so both are
// read — and the same link written twice is one link.
func TestItemLinksReadsEveryFieldAndSaysEachLinkOnce(t *testing.T) {
	got := itemLinks("see "+mailLink, "from marju\n"+mailLink+" https://a.example/1")
	want := mailLink + " https://a.example/1"
	if got != want {
		t.Fatalf("itemlinks = %q, want %q", got, want)
	}
}

// Empty is the answer that lets a template leave the attribute off, which is
// how the key bar knows not to offer ^o (implementation.md, "Links in item
// text"). A plain `http://` address is words, so an item holding only one holds
// no link at all.
func TestItemLinksIsEmptyWhenThereIsNothingToFollow(t *testing.T) {
	for _, text := range []string{"", "buy milk", "http://plain.example/a"} {
		if got := itemLinks(text); got != "" {
			t.Errorf("itemlinks(%q) = %q, want empty", text, got)
		}
	}
}
