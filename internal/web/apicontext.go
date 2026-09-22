package web

import (
	"io"
	"net/http"
	"strings"

	"todoistik/internal/app"
)

// The bundle: every view in one answer (design.md, "The read API").
//
// It is a stapler and not a query. Handing something the whole situation
// through /api/view means eight or eleven calls stapled together by hand at
// the other end, and the staple is the only part that was missing — so the
// bundle is exactly the concatenation of ordinary reads, each labelled with
// the view and the filter that produced it, in the order the navigation rail
// has them. Nothing in it is a list the app could not show, which is the
// property design.md asks of every answer the read API gives: a bundle that
// blended or de-duplicated its views would be showing a screen that does not
// exist, and the argument would have to be made a second time.
//
// The same action really does appear on Next, on the Calendar and under its
// project, on screen and therefore here. The overlap is not noise to be
// cleaned up: it is what the views are, and `::41` in three sections is the
// same item said three times, which is legible precisely because the id is
// there to say so.

// bundleArchive is how far back the Archive section reaches when the caller
// says nothing.
//
// The Archive is the one view with no bottom — everything ever finished is in
// it — so it is the one view a bundle cannot simply take whole. "3months" is
// the month you are in and the two before it: a calendar period, like every
// other value `completed:` takes, and wide enough that the bundle does not
// become nearly empty on the first of a month, which a bare "month" would.
const bundleArchive = "3months"

// apiContext answers every view at once.
func (s *Server) apiContext(w http.ResponseWriter, r *http.Request) {
	format, ok := apiFormat(w, r)
	if !ok {
		return
	}
	// The one parameter, and it narrows the one view that needs narrowing.
	// Everything else is answered whole: a bundle is the situation by
	// definition, and a caller that wants a filtered view asks for that view,
	// where it can say so per view instead of in one line that would have to
	// name which view each token was meant for.
	window := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("archive")))
	switch window {
	case "":
		window = bundleArchive
	case "none":
	default:
		if _, _, valid := app.CompletedRange(window, s.app.Today()); !valid {
			jsonError(w, http.StatusBadRequest, "archive: "+window+" is not a window; a date, a day name, today/yesterday, week/month/year, 2weeks/3months/2years, or none")
			return
		}
	}

	var answers []apiAnswer
	for _, name := range apiViews {
		f := app.Filters{}
		if name == "archive" {
			if window == "none" {
				continue
			}
			f.Completed = window
		}
		data, err, _ := s.readView(name, f)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, name+": "+err.Error())
			return
		}
		answers = append(answers, apiAnswer{View: name, Filters: f, Items: data})
	}

	if format == "text" {
		// Each section is exactly what ?format=text gives for that view,
		// header and all — so any one of them can be cut out of the paste and
		// still be a complete answer that says which view it is and which day
		// it was given on. That is worth eleven repetitions of one short line.
		var out []string
		for _, a := range answers {
			out = append(out, strings.TrimRight(s.textAnswer(a.View, a.Filters, nil, a.Items), "\n"))
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, strings.Join(out, "\n\n")+"\n")
		return
	}
	writeJSON(w, map[string]any{"today": s.app.Today(), "views": answers})
}
