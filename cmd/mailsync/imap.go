// The Gmail side: a label's messages, and the move that takes a label off one.
//
// A Gmail label is an IMAP folder, and a message sits in the folder of every
// label it wears. That is the whole of what this file relies on: reading the
// label is selecting its folder, and removing the label is moving the message
// out of it into All Mail — which deletes nothing, marks nothing read, and
// leaves the mail exactly where it was before it was labelled.
package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type mailbox struct {
	c       *imapclient.Client
	allMail string
	holds   uint32 // how many messages the label holds, from SELECT
}

// dialTLS is the one thing a test replaces. Everything above the connection —
// which folder is All Mail, what is selected, what is moved where — is the
// part with decisions in it, and none of it can be reached without a server on
// the other end.
var dialTLS = imapclient.DialTLS

// dial opens the account and selects the label. Both are done here because a
// run that cannot do either has nothing to do at all, and finding out halfway
// through would mean a run that captured some of a label and left the rest.
func dial(server, user, password, label string) (*mailbox, error) {
	c, err := dialTLS(server, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %v", server, err)
	}
	if err := c.Login(user, password).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("%s would not let %s in: %v (an app password, not the account password)",
			server, user, err)
	}

	all, err := allMailFolder(c)
	if err != nil {
		c.Close()
		return nil, err
	}
	sel, err := c.Select(label, nil).Wait()
	if err != nil {
		known, _ := labels(c)
		c.Close()
		return nil, fmt.Errorf("no label %q. There is: %s", label, strings.Join(known, ", "))
	}
	return &mailbox{c: c, allMail: all, holds: sel.NumMessages}, nil
}

func (m *mailbox) close() {
	m.c.Logout().Wait()
	m.c.Close()
}

// allMailFolder finds All Mail by its \All attribute and never by its name.
// It is "[Gmail]/All Mail" in an English account and "[Gmail]/Kogu meil" in an
// Estonian one, so a name would work until the interface language changed and
// then fail to find the one folder this program needs.
func allMailFolder(c *imapclient.Client) (string, error) {
	boxes, err := c.List("", "*", &imap.ListOptions{ReturnSpecialUse: true}).Collect()
	if err != nil {
		return "", fmt.Errorf("cannot list the mailboxes: %v", err)
	}
	for _, b := range boxes {
		for _, attr := range b.Attrs {
			if attr == imap.MailboxAttrAll {
				return b.Mailbox, nil
			}
		}
	}
	return "", errors.New("this account has no All Mail folder, so a label cannot be taken off without deleting the mail")
}

// labels is what to say when the named one is not there: the list the account
// actually has, so a typo is answered rather than merely refused.
func labels(c *imapclient.Client) ([]string, error) {
	boxes, err := c.List("", "*", nil).Collect()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, b := range boxes {
		if strings.HasPrefix(b.Mailbox, "[Gmail]") {
			continue // the account's own folders are not labels anyone put on
		}
		out = append(out, b.Mailbox)
	}
	return out, nil
}

// messages reads every message wearing the label, oldest first, with the four
// headers a capture is written from.
//
// The whole folder is read rather than searched: the label is the queue, and a
// mail wearing it has already been chosen — there is no criterion left to
// apply, and asking a server to answer one it may not support (ESEARCH is an
// extension) would be a question with no purpose and a way to fail.
func (m *mailbox) messages() ([]message, error) {
	if m.holds == 0 {
		return nil, nil
	}
	var all imap.SeqSet
	all.AddRange(1, m.holds)

	msgs, err := m.c.Fetch(all, &imap.FetchOptions{
		UID: true, Envelope: true,
	}).Collect()
	if err != nil {
		return nil, fmt.Errorf("cannot read the label's messages: %v", err)
	}

	out := make([]message, 0, len(msgs))
	for _, b := range msgs {
		if b.Envelope == nil {
			continue
		}
		out = append(out, message{
			UID:       b.UID,
			Subject:   b.Envelope.Subject,
			From:      b.Envelope.From,
			MessageID: b.Envelope.MessageID,
		})
	}
	return out, nil
}

// unlabel takes the label off every message named, in one command. Moving them
// to All Mail is exactly "remove this label" and nothing else: the mail is not
// deleted, not marked read, and not moved out of the account.
//
// Deleting from the folder would have been the other way, and it asks Gmail's
// own expunge setting what deleting means — a setting this program cannot see,
// and the wrong one turns taking a label off into throwing a mail away.
func (m *mailbox) unlabel(uids []imap.UID) error {
	if len(uids) == 0 {
		return nil
	}
	if _, err := m.c.Move(imap.UIDSetNum(uids...), m.allMail).Wait(); err != nil {
		return fmt.Errorf("captured, but the label could not be taken off: %v", err)
	}
	return nil
}
