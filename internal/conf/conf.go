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
	// shown beside it. The default only — ctrl-t flips it while the mode is up.
	DoingShowsTimer bool
	// DoingTimerFormat: how that number is written. "auto", or a pattern of
	// H/HH/M/MM with anything else taken literally.
	DoingTimerFormat string
}

// TimerAuto is the format that is not a pattern: minutes up to an hour, then
// hours and minutes. It is the default because it is the only rendering that
// is short while the answer is short and still right after an hour, which no
// single pattern can be.
const TimerAuto = "auto"

// Defaults are what the app runs with when there is no file at all, and what
// any key left out of the file falls back to.
//
// The rail goes and the bar stays: doing mode exists to take away the list of
// other places you could be, which is exactly what the rail is, while the bar
// in that mode says `c done` and `esc back` and nothing else — the two keys
// that are the whole of the mode. The timer is off, because a clock on the
// wall is a thing you ask for. All three are one line away from the opposite.
func Defaults() Config {
	return Config{
		DoingShowsNav:    false,
		DoingShowsKeybar: true,
		DoingShowsTimer:  false,
		DoingTimerFormat: TimerAuto,
	}
}

// bools maps a key in the file to the field it sets. Adding a setting is
// adding a line here (or to strs below); nothing else in this file knows any
// key's name.
func (c *Config) bools() map[string]*bool {
	return map[string]*bool{
		"doing.show_nav":    &c.DoingShowsNav,
		"doing.show_keybar": &c.DoingShowsKeybar,
		"doing.show_timer":  &c.DoingShowsTimer,
	}
}

// strs are the settings that take words rather than true/false. Each brings
// its own check, because a string setting with no check is a typo that reaches
// the screen — which for a value read once at startup means it stays there.
func (c *Config) strs() map[string]*string {
	return map[string]*string{
		"doing.timer_format": &c.DoingTimerFormat,
	}
}

var checks = map[string]func(string) error{
	"doing.timer_format": checkTimerFormat,
}

// checkTimerFormat: "auto", or a pattern. In a pattern, uppercase `H`/`HH` is
// the hours and `M`/`MM` the minutes — within the hour if the pattern also
// asks for hours, and the whole elapsed time if it does not. Everything else
// is literal, so `H:MM`, `HH:MM` and `M` all work, and so does `H h MM`.
//
// Every other capital is refused. A capital in a pattern reads as a field, and
// silently printing `HH:NN` because `N` is not one would be exactly the kind
// of wrong a settings file read once must not be.
func checkTimerFormat(v string) error {
	if strings.EqualFold(v, TimerAuto) {
		return nil
	}
	if len(v) > 24 {
		return fmt.Errorf("a format is at most 24 characters, got %d", len(v))
	}
	fields := 0
	for _, r := range v {
		switch {
		case r == 'H' || r == 'M':
			fields++
		case r >= 'A' && r <= 'Z':
			return fmt.Errorf("%q is not a field: the fields are H, HH, M and MM, and %q is capitalised like one", string(r), string(r))
		}
	}
	if fields == 0 {
		return fmt.Errorf("a format needs an H or an M in it, or the word %q", TimerAuto)
	}
	return nil
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

	bools, strs := c.bools(), c.strs()
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
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if field, ok := strs[key]; ok {
			if err := checks[key](val); err != nil {
				return c, fmt.Errorf("%s:%d: %s: %v", path, n, key, err)
			}
			*field = val
			continue
		}
		field, ok := bools[key]
		if !ok {
			return c, fmt.Errorf("%s:%d: unknown setting %q (known: %s)", path, n, key, strings.Join(Keys(), ", "))
		}
		switch strings.ToLower(val) {
		case "true":
			*field = true
		case "false":
			*field = false
		default:
			return c, fmt.Errorf("%s:%d: %s wants true or false, got %q", path, n, key, val)
		}
	}
	return c, sc.Err()
}

// Keys lists every setting name, sorted, for the message a wrong one gets.
func Keys() []string {
	var c Config
	out := make([]string, 0, len(c.bools())+len(c.strs()))
	for k := range c.bools() {
		out = append(out, k)
	}
	for k := range c.strs() {
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
