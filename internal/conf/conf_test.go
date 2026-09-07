package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "todoistik.conf")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileIsTheDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nothing-here.conf"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if c != Defaults() {
		t.Errorf("missing file gave %+v, want the defaults %+v", c, Defaults())
	}
}

func TestReadsOverTheDefaults(t *testing.T) {
	c, err := Load(write(t, `
# the rail stays put while doing
doing.hide_nav = false

  doing.hide_keybar=TRUE   # a comment can trail a value too
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.DoingHidesNav {
		t.Error("doing.hide_nav = false was not read")
	}
	if !c.DoingHidesKeybar {
		t.Error("doing.hide_keybar = TRUE was not read (values are case-insensitive)")
	}
}

func TestOmittedKeyKeepsItsDefault(t *testing.T) {
	c, err := Load(write(t, "doing.hide_keybar = true\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.DoingHidesNav != Defaults().DoingHidesNav {
		t.Error("a key left out of the file did not keep its default")
	}
}

// The three ways a settings file can be wrong all have to stop startup, since
// the file is read once and a quietly-ignored line looks set forever.
func TestBadLinesAreRefused(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"unknown key", "doing.hide_everything = true\n", "unknown setting"},
		{"no equals", "doing.hide_nav true\n", "not a key = value line"},
		{"not a bool", "doing.hide_nav = sometimes\n", "wants true or false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not say %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), ":1:") {
				t.Errorf("error %q does not point at the line", err)
			}
		})
	}
}
