package app

import (
	"strings"
	"testing"
)

// The Settings line is read against three lists at once, and the ways it can
// go wrong are quiet ones: a parameter shown without the context that gives it
// a meaning, a # word that finds a context, a bare word that misses the verbs,
// a count that disagrees with what is on the screen. See design.md, "Filtering
// the remembered lists".

func cloudNames(c NameCloud) string {
	var out []string
	for _, t := range c.Tags {
		out = append(out, "#"+t.Name)
	}
	for _, ctx := range c.Contexts {
		out = append(out, "@"+ctx.Name)
		for _, p := range ctx.Params {
			out = append(out, "@"+ctx.Name+"("+p.Name+")")
		}
	}
	for _, v := range c.Verbs {
		out = append(out, v.Name)
	}
	return strings.Join(out, " ")
}

func TestNarrowNames(t *testing.T) {
	tags := []NameUse{{Name: "car"}, {Name: "carpool"}, {Name: "marathon"}}
	contexts := []NameUse{
		{Name: "home"},
		{Name: "person", Params: []NameUse{{Name: "Marju"}, {Name: "Andres"}}},
	}
	verbs := []NameUse{{Name: "call"}, {Name: "carry"}}
	for _, c := range []struct {
		line, want   string
		shown, total int
	}{
		{"", "#car #carpool #marathon @home @person @person(Marju) @person(Andres) call carry", 9, 9},
		// a bare word asks all three lists, which is what a verb is written as
		{"car", "#car #carpool carry", 3, 9},
		{"#home", "", 0, 9},
		{"@car", "", 0, 9},
		// and a sigil is the other two lists saying so: neither reaches a verb
		{"#carry", "", 0, 9},
		{"@call", "", 0, 9},
		// a parameter is shown under its context, and alone under it
		{"mar", "#marathon @person @person(Marju)", 3, 9},
		{"@mar", "@person @person(Marju)", 2, 9},
		{"@person", "@person @person(Marju) @person(Andres)", 3, 9},
		{"@person(and", "@person @person(Andres)", 2, 9},
		{"@person(Andres)", "@person @person(Andres)", 2, 9},
		// every word has to match, as on every other line
		{"car pool", "#carpool", 1, 9},
	} {
		got := NarrowNames(tags, contexts, verbs, c.line)
		if cloudNames(got) != c.want || got.Shown != c.shown || got.Total != c.total {
			t.Errorf("%q gave %q %d of %d, want %q %d of %d",
				c.line, cloudNames(got), got.Shown, got.Total, c.want, c.shown, c.total)
		}
	}
}

func TestCreateName(t *testing.T) {
	a, _ := newTestApp(t)
	for _, line := range []string{"#bike", "@garage", "@garage(Lidl)"} {
		if err := a.CreateName(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}
	if tags, _ := a.Tags(); !hasWord(tags, "bike") {
		t.Errorf("#bike was not learned: %v", tags)
	}
	if ps, _ := a.ContextParams("garage"); !hasWord(ps, "Lidl") {
		t.Errorf("@garage(Lidl) was not learned: %v", ps)
	}
	// a bare word is the third list: no sigil is how a verb is written in a
	// title, so it is how one is written here (design.md, "Inbox Zero")
	if err := a.CreateName("Позвони"); err != nil {
		t.Fatalf("Позвони: %v", err)
	}
	if vs, _ := a.Verbs(); !hasWord(vs, "позвони") {
		t.Errorf("Позвони was not learned as a verb: %v", vs)
	}

	for line, why := range map[string]string{
		"#bike(x)":      "a tag has no parameter",
		"@shop(Lidl)":   "a parameter needs its context first",
		"@Garage":       "a case variant is the drift the list prevents",
		"@garage(lidl)": "a case variant of a parameter too",
		"#a #b":         "one name at a time",
	} {
		if err := a.CreateName(line); err == nil {
			t.Errorf("%s was accepted: %s", line, why)
		}
	}
	if cs, _ := a.Contexts(); hasWord(cs, "shop") {
		t.Error("a refused @shop(Lidl) learned @shop anyway")
	}
}
