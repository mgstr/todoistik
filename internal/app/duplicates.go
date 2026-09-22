package app

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Matching a capture against what already exists (design.md, "Matches while
// processing").
//
// This is not the duplicate collapse capture does. That one compares the
// whole text, exactly, against the open inbox, and it *drops* what it finds —
// which is why it has to be exact and why it never looks at history (design.md,
// "Duplicate captures"). This one looks at everything that was ever committed
// to, it decides nothing, and it shows what it found beside a capture that is
// being looked at deliberately. A rule that acts by itself has to be certain;
// a rule that only speaks can afford to be generous, because the person is
// already reading the screen when it does.

// The three ways of comparing, mirroring conf's `duplicates.match`. The
// domain keeps its own names rather than importing the settings package: the
// rule is a fact about how two titles are compared, and it would still be
// that if nothing ever read it out of a file.
const (
	MatchNone    = "none"
	MatchOverlap = "overlap"
	MatchSimilar = "similar"
)

// DupRule is the whole of what the settings file says about this question,
// handed in by the caller for the same reason the someday review period is
// (see SetSomedayReviewDays): internal/app does not read files.
type DupRule struct {
	Method  string // MatchNone, MatchOverlap or MatchSimilar
	Overlap int    // percent of the shorter title's words, when MatchOverlap
	Similar int    // percent of the character pairs, when MatchSimilar
}

// MatchLimit is how many of each half are shown. Nine because the finished
// ones are pressed by the digits and there are nine of those (keys.md, "The
// map") — and the open half takes the same cap rather than one of its own,
// since a list long enough to scroll past the answers below it would be
// answering a different question than "have I written this before".
const MatchLimit = 9

// Match is one thing the capture looks like: an action or a project, open or
// finished, with the score that found it.
type Match struct {
	Action  *Action  `json:"action,omitempty"`
	Project *Project `json:"project,omitempty"`
	Score   int      `json:"score"` // percent, and what the list is ordered by
}

// Title is the matched item's own title — what was compared.
func (m *Match) Title() string {
	if m.Action != nil {
		return m.Action.Title
	}
	return m.Project.Title
}

// Kind is "action" or "project", which is both what the row says and which
// form a copy of it opens (design.md, "Copying a finished one").
func (m *Match) Kind() string {
	if m.Action != nil {
		return "action"
	}
	return "project"
}

// ID is the matched item's own id, within its kind.
func (m *Match) ID() int64 {
	if m.Action != nil {
		return m.Action.ID
	}
	return m.Project.ID
}

// Done reports a finished commitment: the half the digits act on.
func (m *Match) Done() bool { return m.at() != nil }

// When is the moment the row dates itself by — finished then for a completed
// item, written then for an open one. Both are the same question: when did I
// last have this thought.
func (m *Match) When() time.Time {
	if at := m.at(); at != nil {
		return *at
	}
	if m.Action != nil {
		return m.Action.CreatedAt
	}
	return m.Project.CreatedAt
}

// Home is the project a matched action sits under, empty for a standalone one
// and for a project match. The archive leaves a finished action inside a
// finished project unlisted, because it is a step and not a commitment
// (design.md, "Archive") — here it is listed and says whose step it was, since
// the question being asked is "have I written this before" and a step is
// something that was written.
func (m *Match) Home() string {
	if m.Action != nil {
		return m.Action.ProjectTitle
	}
	return ""
}

func (m *Match) at() *time.Time {
	if m.Action != nil {
		return m.Action.CompletedAt
	}
	return m.Project.CompletedAt
}

// Matches is what a capture looks like, in two lists: what is still open and
// what is finished. Both are ordered by score and capped at MatchLimit, with
// the count before capping returned alongside — a cap that does not announce
// itself reads as a complete list (design.md, "Views").
//
// The first line only, stripped of its notation, is what is compared. The body
// is never read, for the reason the branch forms never read it: it arrived
// from wherever the capture came from and was not written to identify anything
// (design.md, "Inbox item").
func (a *App) Matches(text string, rule DupRule) (open, done []*Match, openTotal, doneTotal int, err error) {
	if rule.Method == MatchNone || rule.Method == "" {
		return nil, nil, 0, 0, nil
	}
	line, _ := SplitCapture(text)
	subject := StripNotation(line)
	if subject == "" {
		// a capture that is nothing but notation has no title to compare, and
		// matching it on the empty string would match everything
		return nil, nil, 0, 0, nil
	}
	acts, err := a.loadActions(`1=1`)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	// false: the completed sort that flag asks for reads CompletedAt on every
	// project, and half of these are open. They are sorted below, by score
	projects, err := a.projectsWhere(`1=1`, Filters{}, false)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	score := scorer(subject, rule)
	var all []*Match
	for _, act := range acts {
		if n := score(act.Title); n > 0 {
			all = append(all, &Match{Action: act, Score: n})
		}
	}
	for _, p := range projects {
		if n := score(p.Title); n > 0 {
			all = append(all, &Match{Project: p, Score: n})
		}
	}
	// the best first, and the most recent of equals: two captures of the same
	// chore score the same, and the one that happened last is the one worth
	// copying from
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].When().After(all[j].When())
	})
	for _, m := range all {
		if m.Done() {
			doneTotal++
			if len(done) < MatchLimit {
				done = append(done, m)
			}
			continue
		}
		openTotal++
		if len(open) < MatchLimit {
			open = append(open, m)
		}
	}
	return open, done, openTotal, doneTotal, nil
}

// CopySeed is what a branch's form starts from when a finished match is
// copied rather than the capture read (design.md, "Copying a finished one").
// It is the same shape for both kinds, because the two forms differ in which
// fields they have and not in where the words come from.
//
// It is built here and not in the web layer for the ordinary reason: what a
// copy of a finished item consists of is a domain question — which fields
// describe the work and which described an occasion that has passed — and the
// handler's job is to put the answer in boxes.
type CopySeed struct {
	Kind        string     // "action" or "project", which says which form this is
	Title       string     //
	Meta        string     // the branch's meta line, occasion fields dropped
	DOD         string     // a project's definition of done, empty for an action
	Description string     // an action's own; a project keeps none
	Actions     []CopySeed // a project's actions, as the drafts its form holds
	At          time.Time  // when it was finished, for the line that says so
}

// CopyOf is the seed for one finished item, or nil when there is nothing to
// copy — no such item, or one that is not finished. Nil and not an error: the
// only way to ask about either is a hand-written or a stale link, and the
// screen has nothing to say about one beyond carrying on with the capture.
func (a *App) CopyOf(kind string, id int64) (*CopySeed, error) {
	switch kind {
	case "action":
		act, err := a.Action(id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if act.CompletedAt == nil {
			return nil, nil
		}
		return &CopySeed{
			Kind: kind, Title: act.Title, Meta: act.CopyMeta(),
			Description: act.Description, At: *act.CompletedAt,
		}, nil
	case "project":
		p, err := a.Project(id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if p.CompletedAt == nil {
			return nil, nil
		}
		// the DOD comes with it, which no other route to that form may do:
		// this sentence is the definition of done you wrote for this outcome
		// and then met, not prose that happened to be in a capture
		seed := &CopySeed{Kind: kind, Title: p.Title, Meta: p.CopyMeta(), DOD: p.DOD, At: *p.CompletedAt}
		// every action it was completed with, in the order it held them: what
		// a finished plan turned out to be is most of what makes it worth
		// copying, so the done ones come too
		for _, act := range p.Actions {
			seed.Actions = append(seed.Actions, CopySeed{
				Kind: "action", Title: act.Title, Meta: act.CopyMeta(), Description: act.Description,
			})
		}
		return seed, nil
	}
	return nil, nil
}

// scorer is the comparison the rule asks for, as a function of one title. The
// capture is taken apart once and every candidate is measured against that,
// rather than both being taken apart per pair.
func scorer(subject string, rule DupRule) func(string) int {
	if rule.Method == MatchSimilar {
		want, pairs := rule.Similar, bigrams(subject)
		return func(title string) int {
			if n := dice(pairs, bigrams(StripNotation(title))); n >= want {
				return n
			}
			return 0
		}
	}
	want, words := rule.Overlap, wordSet(subject)
	return func(title string) int {
		if n := shared(words, wordSet(StripNotation(title))); n >= want {
			return n
		}
		return 0
	}
}

// StripNotation takes the meta notation out of a line: every `@name`,
// `@name(param)` and `#name`, and the `due:` and `snooze:` a line can carry.
//
// It asks no vocabulary, unlike the readers that seed a form (see
// MetaFromText). That is deliberate and it is the difference between the two
// jobs: seeding a form has to leave an unknown `#hoem` in the title, because
// moving a name the app does not know into a box would be inventing one. Here
// there is no box and nothing is moved — a `#` word is notation whether or not
// the app has heard of it, and leaving the unknown ones in would make a
// capture match *less* the more notation was written on it, which is the
// opposite of what the notation is for.
//
// A title is stripped too, not only the capture. Nothing writes notation into
// a title today, but a title that once carried some would otherwise be
// compared with it still attached.
func StripNotation(s string) string {
	out := tokenRe.ReplaceAllString(s, "$1")
	out = dateRe.ReplaceAllString(out, "$1")
	// a token taken out of the middle of a line leaves the space that was in
	// front of it beside the space that was behind it. Neither rule below
	// cares — words are cut at every space and character pairs collapse a run
	// to one — but a function that says it takes the notation out should hand
	// back the line a person would have written without it
	return strings.Join(strings.Fields(out), " ")
}

// wordSet is a title as the set of words it is made of: folded to lower case
// and cut at everything that is not a letter or a digit, so that punctuation,
// quotes and a trailing question mark are not part of what a word is. A set
// and not a list, because "pay the rent the" says the same thing twice and
// should not count twice.
func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		out[w] = true
	}
	return out
}

// shared is the overlap score: how much of the shorter title the two have in
// common, as a percentage. The shorter one is the denominator so that a short
// capture can match a long title — "Call dentist" against "Call dentist about
// the crown" is the commonest real repeat there is, and measuring it against
// the longer side would score it 40% and throw it away.
func shared(a, b map[string]bool) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	small, big := a, b
	if len(big) < len(small) {
		small, big = big, small
	}
	n := 0
	for w := range small {
		if big[w] {
			n++
		}
	}
	return n * 100 / len(small)
}

// bigrams is a title as the multiset of adjacent character pairs it is made
// of, over the letters and digits alone. Pairs rather than whole words is what
// makes this rule see a typo: "passport" and "passpport" share every pair but
// two, while as words they are simply different.
func bigrams(s string) map[string]int {
	var runes []rune
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			runes = append(runes, r)
		} else if len(runes) > 0 && runes[len(runes)-1] != ' ' {
			runes = append(runes, ' ')
		}
	}
	out := map[string]int{}
	for i := 0; i+1 < len(runes); i++ {
		out[string(runes[i:i+2])]++
	}
	return out
}

// dice is the similarity score: twice the pairs both titles hold, over the
// pairs they hold between them, as a percentage. It is symmetric — neither
// side is the one being searched — and it is 100 only for two titles made of
// the same characters in the same order.
func dice(a, b map[string]int) int {
	total := 0
	for _, n := range a {
		total += n
	}
	for _, n := range b {
		total += n
	}
	if total == 0 {
		return 0
	}
	common := 0
	for pair, n := range a {
		if m := b[pair]; m < n {
			common += m
		} else {
			common += n
		}
	}
	return 2 * common * 100 / total
}
