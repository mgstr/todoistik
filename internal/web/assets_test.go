package web

import (
	"regexp"
	"strings"
	"testing"
)

// Every page links its static files through asset, so none can be served
// stale by a cache that kept the old file under the same name.
func TestStaticLinksCarryAVersion(t *testing.T) {
	for _, name := range []string{"_layout.html", "login.html"} {
		b, err := templateFS.ReadFile("templates/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if m := regexp.MustCompile(`"/static/[^"]*"`).FindAllString(string(b), -1); len(m) > 0 {
			t.Errorf("%s links %v by bare name; use {{asset ...}}", name, m)
		}
	}
	for _, f := range []string{"style.css", "app.js", "htmx.min.js"} {
		if got := asset(f); !strings.HasPrefix(got, "/static/"+f+"?v=") || len(got) <= len("/static/"+f+"?v=") {
			t.Errorf("asset(%q) = %q", f, got)
		}
	}
}
