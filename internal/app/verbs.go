package app

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// --- the verbs an action title may open with -----------------------------
//
// design.md ("Inbox Zero") asks an action's title to start with a verb, and
// the app says so by marking the box rather than by refusing the save. What
// it needs in order to say it is an answer to "is this first word a verb",
// in two languages that answer that question in completely different ways.
//
// Russian answers by shape: a title is an infinitive — "Позвонить маме",
// "Купить молоко" — and an infinitive ends -ть, -ти or -чь. That is a rule,
// it lives in the key layer beside the mark it draws, and it needs no list.
//
// English answers by nothing at all: the imperative is the bare stem, so
// there is no shape to read and only a lexicon can tell. This is that lexicon
// — and deliberately a lexicon of *your* verbs rather than of the language's.
// A part-of-speech tagger would accept "milk", which is a real English verb
// and is also design.md's own example of the title the rule exists to catch.
// The list you keep never has "milk" on it, because you never start an action
// with it, and that is exactly why the list is the better answer.
//
// It is a remembered list like the contexts and the tags, for the reason they
// are: the app is told the vocabulary rather than shipped it, so a word it
// does not know is one press from being learned and the check quietly stops
// firing as the list fills. Unlike those two it carries no sigil, and nothing
// is "carried by" it — see RemoveVerb for what that changes.

// verbSeed is the list a fresh database starts with: the verbs an action
// plausibly opens with in English, so that the first weeks are a few
// additions rather than a hundred. Seeded once and then yours — none of these
// is built in, and every one of them can be removed (see seedVerbs).
//
// English only. Russian needs no seed: the infinitive rule already accepts
// every ordinary Russian title, and seeding a language whose rule answers
// already would put a hundred words on the Settings screen to no effect.
var verbSeed = []string{
	"add", "answer", "arrange", "ask", "book", "browse", "buy",
	"call", "cancel", "change", "check", "choose", "clean", "clear", "close",
	"collect", "compare", "confirm", "connect", "cook", "copy", "create", "cut",
	"decide", "delete", "deliver", "discuss", "draft", "draw", "drop",
	"email", "empty", "fetch", "file", "fill", "find", "finish", "fix", "follow",
	"get", "give", "hang", "install", "invite", "join",
	"learn", "list", "look", "mail", "make", "measure", "meet", "merge", "move",
	"note", "open", "order", "organise", "organize", "pack", "pay", "phone",
	"pick", "plan", "post", "prepare", "print", "publish",
	"read", "rebuild", "register", "remove", "renew", "repair", "replace",
	"reply", "research", "reserve", "reset", "restore", "return", "review",
	"rewrite", "run", "schedule", "send", "set", "ship", "sign", "sketch",
	"sort", "speak", "submit", "subscribe", "sync",
	"take", "talk", "test", "throw", "tidy", "transfer", "try",
	"unsubscribe", "update", "upgrade", "upload", "verify", "visit",
	"wash", "watch", "write",
}

// verbsSeeded marks the one-time seed. Without it the seed would be a list of
// defaults rather than a starting point: every removal would be undone by the
// next start, and a list you cannot take a word off is not one you keep.
const verbsSeeded = "verbs_seeded"

func (a *App) seedVerbs() error {
	done, err := a.GetState(verbsSeeded)
	if err != nil || done != "" {
		return err
	}
	if err := a.tx(func(tx *sql.Tx) error {
		for _, v := range verbSeed {
			if _, err := tx.Exec(`INSERT INTO verbs (word) VALUES (?) ON CONFLICT DO NOTHING`, v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return a.SetState(verbsSeeded, "1")
}

// Verbs returns the remembered verb list.
func (a *App) Verbs() ([]string, error) {
	return a.stringList(`SELECT word FROM verbs ORDER BY word`)
}

// normVerb is how a verb is stored and compared: lower case, and nothing
// around it. Case is not a difference here the way it is for a name — a title
// opens with a capital and the list holds the word, so `Call` and `call` are
// the same verb and keeping both would mark neither.
func normVerb(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// plainVerb reports whether s is a single word: letters in any script, plus
// the hyphen that lives inside a few of them. Not restricted to ASCII the way
// plainName is, because half of what goes on this list is Cyrillic.
func plainVerb(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && r != '-' {
			return false
		}
	}
	return s != ""
}

// AddVerb puts a word on the remembered verb list.
func (a *App) AddVerb(word string) error {
	word = normVerb(word)
	if word == "" {
		return fmt.Errorf("a verb needs a word")
	}
	if !plainVerb(word) {
		return fmt.Errorf("%q: a verb is one word of letters", word)
	}
	_, err := a.db.Exec(`INSERT INTO verbs (word) VALUES (?) ON CONFLICT DO NOTHING`, word)
	return err
}

// RemoveVerb takes a word off the list. Never refused for being in use, which
// is where this parts company with RemoveTag: removing a tag would edit the
// items carrying it behind their back, and removing a verb edits nothing at
// all — the titles that open with it stay exactly as they are, and the box
// merely goes yellow the next time one of them is opened. Pruning a verb you
// no longer mean to start actions with is the point of the list being
// editable, so it is not the count's business to allow it.
func (a *App) RemoveVerb(word string) error {
	_, err := a.db.Exec(`DELETE FROM verbs WHERE word=?`, normVerb(word))
	return err
}

// FirstWord is the word a title opens with, as the verb list spells one:
// lower case, with whatever punctuation was written around it taken off. The
// key layer has its own copy of this, because it marks the box while the
// title is being typed and the server is not in that loop; this one is here
// to count (see VerbList).
func FirstWord(title string) string {
	f := strings.Fields(strings.TrimSpace(title))
	if len(f) == 0 {
		return ""
	}
	return normVerb(strings.TrimFunc(f[0], func(r rune) bool {
		return !unicode.IsLetter(r) && r != '-'
	}))
}

// VerbList is every verb with the number of actions whose title opens with
// it. The count is what makes the list prunable: a hundred seeded words say
// nothing about which of them are yours, and the ones still standing at zero
// after a while are the ones to take off.
func (a *App) VerbList() ([]NameUse, error) {
	words, err := a.Verbs()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	rows, err := a.db.Query(`SELECT title FROM actions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if w := FirstWord(t); w != "" {
			counts[w]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]NameUse, 0, len(words))
	for _, w := range words {
		out = append(out, NameUse{Name: w, Count: counts[w]})
	}
	return out, nil
}
