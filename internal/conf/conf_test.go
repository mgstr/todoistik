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
doing.show_nav = true

  doing.show_keybar=FALSE   # a comment can trail a value too
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !c.DoingShowsNav {
		t.Error("doing.show_nav = true was not read")
	}
	if c.DoingShowsKeybar {
		t.Error("doing.show_keybar = FALSE was not read (values are case-insensitive)")
	}
}

func TestOmittedKeyKeepsItsDefault(t *testing.T) {
	c, err := Load(write(t, "doing.show_keybar = true\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.DoingShowsNav != Defaults().DoingShowsNav {
		t.Error("a key left out of the file did not keep its default")
	}
}

// The three ways a settings file can be wrong all have to stop startup, since
// the file is read once and a quietly-ignored line looks set forever.
func TestBadLinesAreRefused(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"unknown key", "doing.show_everything = true\n", "unknown setting"},
		{"no equals", "doing.show_nav true\n", "not a key = value line"},
		{"not a bool", "doing.show_nav = sometimes\n", "wants true or false"},
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
