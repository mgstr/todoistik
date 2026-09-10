// What a mail says once it is an inbox item: one capture, the subject on the
// first line and everything that makes the mail findable again underneath.
package main

import (
	"fmt"
	"mime"
	"net/url"
	"strings"

	"github.com/emersion/go-imap/v2"
)

// message is as much of a mail as a capture can hold. Everything else a
// message has — its date, its recipients, its body, its labels — is left in
// Gmail, which is where it is kept and where the link goes back to.
type message struct {
	UID       imap.UID
	Subject   string
	From      []imap.Address
	MessageID string
}

// captureText writes one message as the capture todoistik takes.
//
// The subject is the first line, so an inbox row reads like every other
// capture, and the sender and the link ride in the body — carried with the
// item, unshown, until it is processed (design.md, "Inbox item"). On one line
// the link would be most of what the row said, and the row is what the inbox
// is read by.
//
// Nothing is written as a token — no `#tag`, no `@context`, however obviously
// a label maps to one. A captured line is prose until someone processes it,
// and deciding what an item means at capture time is the one thing capture
// must never require (design.md, "Capture costs nothing").
func captureText(m message, account int) string {
	line := collapse(decodeHeader(m.Subject))

	var body []string
	if from := senderLine(m.From); from != "" {
		body = append(body, from)
	}
	if link := permalink(m.MessageID, account); link != "" {
		body = append(body, link)
	}

	switch {
	case line == "" && len(body) == 0:
		return ""
	case line == "":
		// a mail with no subject is still worth deciding about, and an empty
		// first line would be an inbox row with nothing on it
		line, body = body[0], body[1:]
	}
	if len(body) == 0 {
		return line
	}
	return line + "\n" + strings.Join(body, "\n")
}

// senderLine writes who the mail is from, as a person and not as an address
// alone: the name is what you recognise, and the address is what tells two
// people with the same name apart.
func senderLine(from []imap.Address) string {
	if len(from) == 0 {
		return ""
	}
	a := from[0]
	addr := strings.TrimSpace(a.Addr()) // empty when the envelope holds a group marker
	name := collapse(decodeHeader(a.Name))
	switch {
	case name == "" && addr == "":
		return ""
	case name == "":
		return "from " + addr
	case addr == "":
		return "from " + name
	}
	return fmt.Sprintf("from %s <%s>", name, addr)
}

// permalink is a search for the message id rather than a link to the thread.
// A message id is assigned once by whoever sent the mail and never changes,
// while a thread's own URL is per-account and moves when the thread does — and
// a link that has gone stale is worse than no link, because it is the one part
// of the capture that cannot be checked by reading it.
//
// A message with no id gets no link. Nothing is invented to stand in for one.
func permalink(messageID string, account int) string {
	id := strings.TrimSpace(messageID)
	id = strings.TrimPrefix(id, "<")
	id = strings.TrimSuffix(id, ">")
	if id == "" {
		return ""
	}
	return fmt.Sprintf("https://mail.google.com/mail/u/%d/#search/%s",
		account, url.PathEscape("rfc822msgid:"+id))
}

// decodeHeader reads `=?UTF-8?B?...?=` back into the words it stands for. The
// IMAP envelope hands headers over exactly as they arrived, and a subject in
// Estonian or Russian is the normal case here rather than the edge one.
//
// A charset the decoder does not know leaves the header as it was, which is
// ugly and still readable — losing the mail over its subject line would not be.
func decodeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	out, err := (&mime.WordDecoder{}).DecodeHeader(s)
	if err != nil {
		return s
	}
	return out
}

// collapse puts a header on one line. A subject folded across two lines by
// some sending program is one name, and the first line of a capture is the
// item (design.md, "Inbox item") — so it may not be split by an accident of
// wire format.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
