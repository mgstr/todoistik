package main

import "testing"

// The server will not start without a token, and this is the rule that says so.
// It is worth a test although it is four lines, because what it prevents is
// silent: a build that lost it would start, serve, and answer everyone, and
// nothing about the running app would look different.
func TestTheServerWillNotStartWithoutAToken(t *testing.T) {
	for _, raw := range []string{"", "   ", "\n", "\t \n"} {
		if tok, err := checkedToken(raw); err == nil {
			t.Errorf("checkedToken(%q) returned %q and no error; want a refusal", raw, tok)
		}
	}

	// a token out of a file or an editor arrives with a newline on it, and it
	// is the same token — trimmed here rather than failing every login later
	for raw, want := range map[string]string{
		"s3cret":     "s3cret",
		"s3cret\n":   "s3cret",
		"  s3cret  ": "s3cret",
	} {
		tok, err := checkedToken(raw)
		if err != nil || tok != want {
			t.Errorf("checkedToken(%q) = %q, %v; want %q, nil", raw, tok, err, want)
		}
	}
}
