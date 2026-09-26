package web

import (
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// `r` on the processing question opens a form now rather than pressing the
// answer: the material is filed with the area it is about, which is the
// thinking that answer was already doing (design.md, "Inbox Zero"). The
// capture is only consumed when the form is submitted.
func TestTheReferenceBranchOpensAFormAndKeepsWhatItIsGiven(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("house"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("The boiler is a Vaillant ecoTEC 832 #house\nthe manual is in the hall cupboard", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()
	id := inbox[0].ID

	// stage one offers the branch as a link to its form, not as a form of its own
	question := getPage(t, s, "/process")
	if !strings.Contains(question, "as=reference") {
		t.Fatalf("the question does not lead to the reference form:\n%s", question)
	}

	// the form arrives seeded: the whole capture in the one box, the tag read
	// off the line and onto the meta line
	form := getPage(t, s, "/process?item="+itoa(id)+"&as=reference")
	if !strings.Contains(form, "the manual is in the hall cupboard") {
		t.Fatalf("the body was not seeded into the box:\n%s", form)
	}
	if !strings.Contains(form, `value="#house"`) {
		t.Fatalf("the tag was not read onto the meta line:\n%s", form)
	}
	// and the capture is still in the inbox until the form is submitted
	if left, _ := a.Inbox(); len(left) != 1 {
		t.Fatalf("opening the form consumed the capture: %d left", len(left))
	}

	postForm(t, s, "/process/"+itoa(id)+"/reference", url.Values{
		"text": {"The boiler is a Vaillant ecoTEC 832\nthe manual is in the hall cupboard"},
		"meta": {"#house"},
	})
	items, err := a.ReferenceItems(app.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].Tags) != 1 || items[0].Tags[0] != "house" {
		t.Fatalf("filed as %+v", items)
	}
	if left, _ := a.Inbox(); len(left) != 0 {
		t.Fatalf("the capture is still in the inbox: %d", len(left))
	}
}

// The view has two controls and no more: the line that narrows the pile, and
// the delete that takes a row out of it (design.md, "Reference"). Everything
// else about an item is on its own page.
func TestTheReferenceViewFiltersAndDeletes(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("house"); err != nil {
		t.Fatal(err)
	}
	keep := func(text string, tags ...string) *app.ReferenceItem {
		t.Helper()
		if _, _, err := a.Capture(text, app.SourceApp); err != nil {
			t.Fatal(err)
		}
		inbox, _ := a.Inbox()
		it, err := a.ProcessReference(inbox[len(inbox)-1].ID, app.ReferenceFields{Text: text, Tags: tags})
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	boiler := keep("The boiler is a Vaillant ecoTEC 832", "house")
	keep("An article on soldering")

	body := getPage(t, s, "/reference")
	if !strings.Contains(body, "Vaillant") || !strings.Contains(body, "soldering") {
		t.Fatalf("the view is missing what is kept:\n%s", body)
	}
	// no completing and no picking for today: nothing here is a commitment
	if strings.Contains(body, "kb-complete") || strings.Contains(body, "kb-pick") {
		t.Fatalf("the rows carry commitment controls:\n%s", body)
	}
	if !strings.Contains(body, `action="/referenceitem/`+itoa(boiler.ID)+`/delete"`) {
		t.Fatalf("the row carries no delete:\n%s", body)
	}

	// the line narrows by tag and by name, and says how much it is hiding
	tagged := getPage(t, s, "/reference?"+url.Values{"q": {"#house"}}.Encode())
	if strings.Contains(tagged, "soldering") {
		t.Fatalf("#house did not narrow the pile:\n%s", tagged)
	}

	postForm(t, s, "/referenceitem/"+itoa(boiler.ID)+"/delete", url.Values{})
	left, err := a.ReferenceItems(app.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].Text != "An article on soldering" {
		t.Fatalf("after the delete: %+v", left)
	}
}

// The item's own page is where the material is corrected and added to, with
// the same two boxes it was filed in — and it leaves by being deleted, never
// by going back to the inbox, because there is no decision left to make about
// it (design.md, "Reference").
func TestTheReferenceItemPageSavesAndDeletes(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("The boiler", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()
	it, err := a.ProcessReference(inbox[0].ID, app.ReferenceFields{Text: "The boiler"})
	if err != nil {
		t.Fatal(err)
	}

	page := getPage(t, s, "/referenceitem/"+itoa(it.ID))
	if !strings.Contains(page, `data-key="s"`) {
		t.Fatalf("no Save on the page:\n%s", page)
	}
	// no Inbox button: a someday item has one because there is a decision left
	// to make about it, and this has none
	if strings.Contains(page, `data-key="i"`) || strings.Contains(page, "/referenceitem/"+itoa(it.ID)+"/inbox") {
		t.Fatalf("the page offers a way back to the inbox:\n%s", page)
	}
	if !strings.Contains(page, "/referenceitem/"+itoa(it.ID)+"/delete") {
		t.Fatalf("no Delete on the page:\n%s", page)
	}

	postForm(t, s, "/referenceitem/"+itoa(it.ID), url.Values{
		"text": {"The boiler is a Vaillant ecoTEC 832"},
		"meta": {""},
	})
	got, err := a.ReferenceItem(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "The boiler is a Vaillant ecoTEC 832" {
		t.Fatalf("the save did not take: %+v", got)
	}
}

// The read API answers this view like any other, in both spellings — and the
// text one says what it is: material that is kept and never reviewed.
func TestTheReadAPIAnswersReference(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("house"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("The boiler", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()
	if _, err := a.ProcessReference(inbox[0].ID, app.ReferenceFields{
		Text: "The boiler is a Vaillant ecoTEC 832\nservice code 3-2-1", Tags: []string{"house"}}); err != nil {
		t.Fatal(err)
	}

	text := getPage(t, s, "/api/view/reference?format=text")
	if !strings.Contains(text, "reference — 1 item") {
		t.Fatalf("header:\n%s", text)
	}
	if !strings.Contains(text, "#house") || !strings.Contains(text, "kept:") {
		t.Fatalf("the line does not say what it carries:\n%s", text)
	}
	if strings.Contains(text, "reviewed:") {
		t.Fatalf("material is never reviewed, so no row says when it was:\n%s", text)
	}
	if !strings.Contains(text, "service code 3-2-1") {
		t.Fatalf("the body is gone rather than deferred — text has no second page:\n%s", text)
	}

	// and the bundle carries it, in the order the rail has the views
	bundle := getPage(t, s, "/api/context?format=text")
	i, j := strings.Index(bundle, "someday"), strings.Index(bundle, "reference")
	if i < 0 || j < 0 || j < i {
		t.Fatalf("reference is missing from the bundle or out of order (someday %d, reference %d)", i, j)
	}
}
