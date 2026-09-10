package main

import (
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"
)

func addr(name, mailbox, host string) []imap.Address {
	return []imap.Address{{Name: name, Mailbox: mailbox, Host: host}}
}

// What a mail becomes is the whole of what this program decides, so it is the
// whole of what is worth pinning: which half is the line and which is the body
// (design.md, "Inbox item"), and what happens when a mail is missing one of
// the three things a capture is written from.
func TestCaptureText(t *testing.T) {
	cases := []struct {
		name string
		in   message
		want string
	}{
		{
			"the subject is the line, the sender and the link are the body",
			message{Subject: "Re: winter tyre quote", From: addr("Marju Tamm", "marju", "example.com"),
				MessageID: "<CAF9x2k@mail.gmail.com>"},
			"Re: winter tyre quote\n" +
				"from Marju Tamm <marju@example.com>\n" +
				"https://mail.google.com/mail/u/0/#search/rfc822msgid:CAF9x2k@mail.gmail.com",
		},
		{
			"an encoded subject is read back into the words it stands for",
			message{Subject: "=?UTF-8?B?0LfQsNC70L7Qs9C40YDQvtCy0LDRgtGMINC60LXRiA==?=",
				From: addr("", "log", "example.com")},
			"залогировать кеш\nfrom log@example.com",
		},
		{
			"a subject folded across two lines is still one line",
			message{Subject: "Re: the quote\r\n for the winter tyres", From: addr("", "m", "example.com")},
			"Re: the quote for the winter tyres\nfrom m@example.com",
		},
		{
			"a mail with no message id captures without a link, not with a broken one",
			message{Subject: "Re: winter tyre quote", From: addr("Marju", "marju", "example.com")},
			"Re: winter tyre quote\nfrom Marju <marju@example.com>",
		},
		{
			"a mail with no subject leads with the sender, so no row is blank",
			message{From: addr("Marju", "marju", "example.com"), MessageID: "<abc@x>"},
			"from Marju <marju@example.com>\nhttps://mail.google.com/mail/u/0/#search/rfc822msgid:abc@x",
		},
		{
			"an address with no name is the address alone",
			message{Subject: "Invoice 12345", From: addr("", "billing", "example.com")},
			"Invoice 12345\nfrom billing@example.com",
		},
		{
			"a mail with nothing to say makes no capture at all",
			message{Subject: "   "},
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := captureText(c.in, 0); got != c.want {
				t.Errorf("captureText() =\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}

// The link is the one part of a capture that cannot be checked by reading it,
// so what goes into it is worth pinning on its own: the angle brackets a
// Message-ID header wears are not part of the id, and a second signed-in
// account is a different /u/N/.
func TestPermalink(t *testing.T) {
	got := permalink("<CAF+9x/2k@mail.gmail.com>", 0)
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("the brackets are the header's, not the id's: %s", got)
	}
	if !strings.Contains(got, "rfc822msgid:CAF+9x%2F2k@mail.gmail.com") {
		t.Fatalf("a slash in an id has to survive the URL: %s", got)
	}
	if permalink("   ", 0) != "" {
		t.Fatal("no id means no link")
	}
	if !strings.Contains(permalink("<abc@x>", 2), "/mail/u/2/") {
		t.Fatalf("the account number is what opens the right mailbox: %s", permalink("<abc@x>", 2))
	}
}

// The inbox list shows the first line of a capture and the log line here shows
// the same one, so what is printed and what is read line up.
func TestOneLineIsTheCapturesOwnFirstLine(t *testing.T) {
	if got := oneLine("Re: the quote\nfrom Marju\nhttps://example"); got != "Re: the quote …" {
		t.Fatalf("oneLine() = %q", got)
	}
	if got := oneLine("Buy milk"); got != "Buy milk" {
		t.Fatalf("a one-line capture gains nothing: %q", got)
	}
}
