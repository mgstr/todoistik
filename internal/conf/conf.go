// Package conf reads the settings file: the handful of choices that are not
// items and not screen state, and that a person (or an agent) sets by editing
// a file rather than by pressing anything.
//
// One `key = value` per line, `#` starts a comment anywhere on a line, blank
// lines are ignored.
// The format is deliberately the least there can be: no sections, no nesting,
// no types beyond what a key declares, so that changing a setting is one line
// found by grep and rewritten by anything.
package conf

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config is the whole of it. Every field is also a key in the file, and the
// zero value of this struct is not the default set — Defaults() is.
type Config struct {
	// DoingShowsNav: the nav rail stays on the screen in doing mode.
	DoingShowsNav bool
	// DoingShowsKeybar: so does the key bar.
	DoingShowsKeybar bool
	// DoingShowsTimer: the minutes since this action went on the screen are
	// shown beside it.
	DoingShowsTimer bool
}

// Defaults are what the app runs with when there is no file at all, and what
// any key left out of the file falls back to.
//
// The rail goes and the bar stays: doing mode exists to take away the list of
// other places you could be, which is exactly what the rail is, while the bar
// in that mode says `c done` and `esc back` and nothing else — the two keys
// that are the whole of the mode. The timer is off, because a clock on the
// wall is a thing you ask for. All three are one line away from the opposite.
func Defaults() Config {
	return Config{DoingShowsNav: false, DoingShowsKeybar: true, DoingShowsTimer: false}
}

// bools maps a key in the file to the field it sets. Adding a setting is
// adding a line here; nothing else in this file knows any key's name.
func (c *Config) bools() map[string]*bool {
	return map[string]*bool{
		"doing.show_nav":    &c.DoingShowsNav,
		"doing.show_keybar": &c.DoingShowsKeybar,
		"doing.show_timer":  &c.DoingShowsTimer,
	}
}

// Load reads path over the defaults. A missing file is not an error — running
// with no settings file is the ordinary case, and the defaults are the app.
//
// Anything else is: an unknown key, a line that is not `key = value`, or a
// value that is not `true` or `false` fails startup naming the file, the line
// number and what was wrong. A settings file is read once (see the package
// comment), so a typo that was quietly ignored would be a setting that looks
// set for as long as the process lives — the one failure this format could
// have, and the reason it fails loudly instead.
func Load(path string) (Config, error) {
	c := Defaults()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()

	keys := c.bools()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		// a comment runs to the end of the line, and there is nothing a value
		// could be that contains a "#" — so the whole line is cut at the first
		// one before anything else looks at it
		line, _, _ := strings.Cut(sc.Text(), "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return c, fmt.Errorf("%s:%d: not a key = value line: %q", path, n, line)
		}
		key = strings.TrimSpace(key)
		field, ok := keys[key]
		if !ok {
			return c, fmt.Errorf("%s:%d: unknown setting %q (known: %s)", path, n, key, strings.Join(Keys(), ", "))
		}
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true":
			*field = true
		case "false":
			*field = false
		default:
			return c, fmt.Errorf("%s:%d: %s wants true or false, got %q", path, n, key, strings.TrimSpace(val))
		}
	}
	return c, sc.Err()
}

// Keys lists every setting name, sorted, for the message a wrong one gets.
func Keys() []string {
	var c Config
	out := make([]string, 0, len(c.bools()))
	for k := range c.bools() {
		out = append(out, k)
	}
	// small and fixed, so an insertion sort is the whole of it
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
