package app

import (
	"strings"
	"testing"
)

// The Settings line is read against two lists at once, and the ways it can go
// wrong are quiet ones: a parameter shown without the context that gives it a
// meaning, a # word that finds a context, a count that disagrees with what is
// on the screen. See design.md, "Filtering the remembered lists".

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
	return strings.Join(out, " ")
}

func TestNarrowNames(t *testing.T) {
	tags := []NameUse{{Name: "car"}, {Name: "carpool"}, {Name: "marathon"}}
	contexts := []NameUse{
		{Name: "home"},
		{Name: "person", Params: []NameUse{{Name: "Marju"}, {Name: "Andres"}}},
	}
	for _, c := range []struct {
		line, want   string
		shown, total int
	}{
		{"", "#car #carpool #marathon @home @person @person(Marju) @person(Andres)", 7, 7},
		{"car", "#car #carpool", 2, 7},
		{"#home", "", 0, 7},
		{"@car", "", 0, 7},
		// a parameter is shown under its context, and alone under it
		{"mar", "#marathon @person @person(Marju)", 3, 7},
		{"@mar", "@person @person(Marju)", 2, 7},
		{"@person", "@person @person(Marju) @person(Andres)", 3, 7},
		{"@person(and", "@person @person(Andres)", 2, 7},
		{"@person(Andres)", "@person @person(Andres)", 2, 7},
		// every word has to match, as on every other line
		{"car pool", "#carpool", 1, 7},
	} {
		got := NarrowNames(tags, contexts, c.line)
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

	for line, why := range map[string]string{
		"bike":          "a bare word does not say which list",
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
