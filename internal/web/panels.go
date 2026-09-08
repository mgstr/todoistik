package web

import (
	"net/url"
	"strings"

	"todoistik/internal/conf"
)

// The chrome around a view is three panels — the title bar, the nav rail and
// the key bar — and every one of them can be taken off the screen. What is on
// is remembered the way the ages flag and the per-view filter sets are
// remembered: in app_state, on the server, because it is screen state of
// exactly the same kind and a single-user app has one place for that (see
// implementation.md, "Panels").
const panelsState = "panels"

// panelState is what is stored. The three flags are what is on in the ordinary
// way of working; Zen is the separate answer "none of it, for now", which is
// why the three survive it untouched and are simply not obeyed while it is on.
type panelState struct {
	Title  bool
	Nav    bool
	Keybar bool
	Zen    bool
	// Auto: zen was turned on by the screen (conf's zen.views) rather than by
	// hand, which is what makes it turn itself off again on the way out. A zen
	// that was asked for is nobody's business but the person who asked.
	Auto bool
	// At: the zen screen currently being looked at, so that the screen only
	// gets its say on arrival. Without it, turning zen off by hand on a screen
	// the settings file names would be undone by that screen's next render —
	// stage two of processing, or a reload — and "it can be toggled manually"
	// would be false exactly where the setting applies.
	At string
}

// panels is what a template gets: what is actually on the screen. Zen is not
// a fourth panel, it is the answer to all three at once, so it is resolved
// here rather than in nine templates.
type panels struct {
	Title  bool
	Nav    bool
	Keybar bool
	Zen    bool
}

func defaultPanels() panelState {
	return panelState{Title: true, Nav: true, Keybar: true}
}

// panelNames are the three, in the order the visibility dialog lists them and
// in the order their keys read. The name is the key it answers to as well.
var panelNames = []string{"title", "nav", "keybar"}

func (p panelState) encode() string {
	q := url.Values{}
	q.Set("title", bit(p.Title))
	q.Set("nav", bit(p.Nav))
	q.Set("keybar", bit(p.Keybar))
	q.Set("zen", bit(p.Zen))
	q.Set("auto", bit(p.Auto))
	q.Set("at", p.At)
	return q.Encode()
}

func decodePanels(s string) panelState {
	p := defaultPanels()
	q, err := url.ParseQuery(s)
	if err != nil || len(q) == 0 {
		return p
	}
	p.Title, p.Nav, p.Keybar = q.Get("title") == "1", q.Get("nav") == "1", q.Get("keybar") == "1"
	p.Zen, p.Auto = q.Get("zen") == "1", q.Get("auto") == "1"
	p.At = q.Get("at")
	return p
}

func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (s *Server) panelState() panelState {
	v, err := s.app.GetState(panelsState)
	if err != nil || v == "" {
		return defaultPanels()
	}
	return decodePanels(v)
}

func (s *Server) savePanels(p panelState) error {
	return s.app.SetState(panelsState, p.encode())
}

// shown resolves the stored answer against zen: zen is all three off, and the
// three keep saying what they said, which is what gives them back untouched
// when it is turned off again.
func (p panelState) shown() panels {
	if p.Zen {
		return panels{Zen: true}
	}
	return panels{Title: p.Title, Nav: p.Nav, Keybar: p.Keybar, Zen: false}
}

// toggle answers one press in the visibility dialog.
//
// Asking for a single panel while zen is on ends zen, because zen means none
// of them: "show me the title bar" and "show me nothing" cannot both be true,
// and the one just asked for is the newer answer. The other two come back as
// they were, so the way out of zen through a panel key is the same way out as
// through the zen key, plus the panel you asked for.
func (p panelState) toggle(which string) panelState {
	if which == "zen" {
		p.Zen, p.Auto = !p.Zen, false
		return p
	}
	if p.Zen {
		p.Zen, p.Auto = false, false
	}
	switch which {
	case "title":
		p.Title = !p.Title
	case "nav":
		p.Nav = !p.Nav
	case "keybar":
		p.Keybar = !p.Keybar
	}
	return p
}

// forScreen is zen.views doing its work. A screen the settings file names
// opens with everything off; the panels come back when you leave it — but only
// if this zen was the app's idea. Zen that was asked for by hand is left
// exactly where it was found, on the way in and on the way out, because it was
// an answer about how you want to work rather than about this screen.
//
// The screen only gets its say on arrival, which is what At is for: while you
// are still on it, zen is yours to turn off and it stays off. screen is "" for
// a screen the settings file does not name.
func (p panelState) forScreen(screen string) (panelState, bool) {
	if screen == "" {
		switch {
		case p.Auto:
			p.Zen, p.Auto, p.At = false, false, ""
			return p, true
		case p.At != "":
			p.At = ""
			return p, true
		}
		return p, false
	}
	if p.At == screen {
		return p, false
	}
	p.At = screen
	if !p.Zen {
		p.Zen, p.Auto = true, true
	}
	return p, true
}

// zenScreen is the name the settings file matched, if any step of the trail is
// one it names. Any step, so that the stages of processing inherit the answer
// given for "processing" — a screen reached from inside a zen screen is still
// the middle of the same one thing, and the two stages are then one arrival
// rather than two.
func zenScreen(trail []crumb, c conf.Config) string {
	for _, cr := range trail {
		for _, name := range c.ZenViews {
			if cr.Slug != "" && cr.Slug == name {
				return name
			}
		}
	}
	return ""
}

// checkZenViews refuses a settings file that names a screen this app does not
// have. conf checks the shape of the list and stops there, because the list of
// screens lives here — but the check has to happen at startup like every other
// settings error, or a typo is a setting that looks set forever (see
// internal/conf, package comment).
func checkZenViews(c conf.Config) error {
	for _, name := range c.ZenViews {
		if _, ok := screenNames()[name]; !ok {
			return &badSetting{key: "zen.views", val: name, known: strings.Join(sortedScreens(), ", ")}
		}
	}
	return nil
}

// screenNames is every name a trail step can carry — the views, plus the two
// screens that are not views but are places you are in: processing and doing.
// It is derived from the help table, so a view cannot exist without being
// nameable here and the two lists cannot drift.
func screenNames() map[string]bool {
	out := map[string]bool{"doing": true}
	for slug := range viewHelp {
		out[slug] = true
	}
	return out
}

func sortedScreens() []string {
	names := make([]string, 0, len(screenNames()))
	for n := range screenNames() {
		names = append(names, n)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

type badSetting struct{ key, val, known string }

func (e *badSetting) Error() string {
	return e.key + ": " + e.val + " is not a screen (known: " + e.known + ")"
}
