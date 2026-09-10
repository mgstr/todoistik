package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// A run against a real account cannot be had here, so it is had against an
// in-memory one: the label folder, an All Mail folder wearing the \All
// attribute, and the app's capture endpoint. What that pins is the whole of
// what this program decides — that the label is what is read, that All Mail is
// found by its attribute rather than its name, and above all that a captured
// mail loses its label by *moving* and is still in the account afterwards.
//
// Losing a mail is the one mistake here that cannot be undone, so the test
// that matters most is the last assertion: All Mail still holds it.

const (
	testUser  = "you@example.com"
	testPass  = "app-password"
	testLabel = "todoistik"
	// deliberately not "[Gmail]/All Mail": the name is localised, and finding
	// the folder by name is exactly the bug this arrangement would catch
	testAllMail = "[Gmail]/Kogu meil"
)

// allMailSession is imapmemserver's session with one thing added: a LIST that
// says which folder is All Mail. The memory server has no way to set a
// special-use attribute on a mailbox, and the attribute is the thing under
// test.
type allMailSession struct {
	*imapmemserver.UserSession
}

func (s *allMailSession) List(w *imapserver.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	for _, name := range []string{testLabel, testAllMail} {
		data := &imap.ListData{Mailbox: name, Delim: '/'}
		if name == testAllMail {
			data.Attrs = []imap.MailboxAttr{imap.MailboxAttrAll}
		}
		if err := w.WriteList(data); err != nil {
			return err
		}
	}
	return nil
}

// imapFor stands up a server holding the given messages on the label, and
// points the program's dialler at it for the duration of the test.
func imapFor(t *testing.T, msgs ...string) (*imapmemserver.User, string) {
	t.Helper()

	user := imapmemserver.NewUser(testUser, testPass)
	if err := user.Create(testLabel, nil); err != nil {
		t.Fatal(err)
	}
	if err := user.Create(testAllMail, nil); err != nil {
		t.Fatal(err)
	}
	for _, raw := range msgs {
		if _, err := user.Append(testLabel, strings.NewReader(raw), &imap.AppendOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	mem := imapmemserver.New()
	mem.AddUser(user)
	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return &allMailSession{imapmemserver.NewUserSession(user)}, nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapMove: {}, imap.CapSpecialUse: {}},
		InsecureAuth: true, // the real one is TLS; this one is a loopback socket in a test
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)

	old := dialTLS
	dialTLS = imapclient.DialInsecure
	t.Cleanup(func() {
		dialTLS = old
		srv.Close()
	})
	return user, ln.Addr().String()
}

// appFor is the capture API and nothing else: it records what arrived and
// answers the way the real one does.
func appFor(t *testing.T, status string) (*httptest.Server, *[]string) {
	t.Helper()
	var (
		mu   sync.Mutex
		got  []string
		srv  *httptest.Server
		body struct {
			Text string `json:"text"`
		}
	)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		json.Unmarshal(payload, &body)
		mu.Lock()
		got = append(got, body.Text)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":%q}`, status)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func mail(subject, from, id string) string {
	return "From: " + from + "\r\nSubject: " + subject + "\r\nMessage-ID: " + id +
		"\r\nDate: Tue, 8 Sep 2026 14:30:00 +0300\r\n\r\nthe body of the mail, which is not captured\r\n"
}

func count(t *testing.T, user *imapmemserver.User, mailbox string) uint32 {
	t.Helper()
	st, err := user.Status(mailbox, &imap.StatusOptions{NumMessages: true})
	if err != nil {
		t.Fatal(err)
	}
	return *st.NumMessages
}

func cfg(app *httptest.Server, addr string) config {
	return config{
		label: testLabel, user: testUser, password: testPass,
		server: addr, base: app.URL,
	}
}

func TestARunCapturesTheLabelAndTakesItOff(t *testing.T) {
	user, addr := imapFor(t, mail("Re: winter tyre quote", "Marju Tamm <marju@example.com>", "<CAF9x2k@mail.gmail.com>"))
	app, captured := appFor(t, "accepted")

	left, err := run(cfg(app, addr))
	if err != nil || left != 0 {
		t.Fatalf("run() = %d, %v", left, err)
	}

	want := "Re: winter tyre quote\n" +
		"from Marju Tamm <marju@example.com>\n" +
		"https://mail.google.com/mail/u/0/#search/rfc822msgid:CAF9x2k@mail.gmail.com"
	if len(*captured) != 1 || (*captured)[0] != want {
		t.Fatalf("captured %q", *captured)
	}
	if n := count(t, user, testLabel); n != 0 {
		t.Errorf("the label still holds %d: it is the queue, and a captured mail leaves it", n)
	}
	// the assertion this whole arrangement exists for
	if n := count(t, user, testAllMail); n != 1 {
		t.Errorf("All Mail holds %d: taking a label off must never lose the mail", n)
	}
}

// The identical capture is already in the inbox, so the mail has nothing left
// to carry — which is what makes running this again safe after a run that was
// cut short between capturing and unlabelling.
func TestADuplicateLosesItsLabelToo(t *testing.T) {
	user, addr := imapFor(t, mail("Re: winter tyre quote", "marju@example.com", "<abc@x>"))
	app, _ := appFor(t, "duplicate")

	if left, err := run(cfg(app, addr)); err != nil || left != 0 {
		t.Fatalf("run() = %d, %v", left, err)
	}
	if n := count(t, user, testLabel); n != 0 {
		t.Errorf("a duplicate kept its label: the next run would capture it forever")
	}
	if n := count(t, user, testAllMail); n != 1 {
		t.Errorf("All Mail holds %d", n)
	}
}

// A dry run is asked what it would do, so nothing on either side may move.
func TestADryRunTouchesNeitherSide(t *testing.T) {
	user, addr := imapFor(t, mail("Re: winter tyre quote", "marju@example.com", "<abc@x>"))
	app, captured := appFor(t, "accepted")

	c := cfg(app, addr)
	c.dry = true
	if left, err := run(c); err != nil || left != 0 {
		t.Fatalf("run() = %d, %v", left, err)
	}
	if len(*captured) != 0 {
		t.Errorf("a dry run captured %q", *captured)
	}
	if n := count(t, user, testLabel); n != 1 {
		t.Errorf("a dry run moved the mail off the label")
	}
}

// A label the account does not have is answered with the ones it does, so a
// typo is told what it should have said rather than merely refused.
func TestAnUnknownLabelSaysWhichOnesThereAre(t *testing.T) {
	_, addr := imapFor(t)
	app, _ := appFor(t, "accepted")

	c := cfg(app, addr)
	c.label = "todoistk"
	_, err := run(c)
	if err == nil || !strings.Contains(err.Error(), testLabel) {
		t.Fatalf("err = %v, want it to name %q", err, testLabel)
	}
}

// A run with nothing on the label is the quiet outcome, not an error.
func TestAnEmptyLabelIsNotAFailure(t *testing.T) {
	_, addr := imapFor(t)
	app, captured := appFor(t, "accepted")

	if left, err := run(cfg(app, addr)); err != nil || left != 0 {
		t.Fatalf("run() = %d, %v", left, err)
	}
	if len(*captured) != 0 {
		t.Fatalf("captured %q from an empty label", *captured)
	}
}
