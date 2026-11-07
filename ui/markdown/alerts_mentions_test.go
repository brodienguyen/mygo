package markdown

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestViewAlert(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { View(c, "> [!WARNING]\n> Mind the gap.", nil) }, 600, 400)
	for _, s := range []string{"Warning", "Mind the gap."} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	if tt.HasText("[!WARNING]") {
		t.Errorf("the marker shows: %q", tt.Texts())
	}
}

// resolver knows octocat and issue 12 of the document's repository, as
// paths of the app, and whom it knows once told.
type resolver struct{ known []string }

func (r *resolver) resolve(in parser.Inline) (string, bool) {
	switch in.Kind {
	case parser.Mention:
		if slices.Contains(r.known, in.Login()) {
			return "/users/" + in.Login(), true
		}
	case parser.Reference:
		if repo, n := in.Ref(); repo == "" && n == 12 {
			return "/pulls/12", true
		}
	}
	return "", false
}

func TestViewMentions(t *testing.T) {
	r := &resolver{known: []string{"octocat"}}
	var opened []string
	o := &Options{Resolve: r.resolve, OnLink: func(url string) { opened = append(opened, url) }}
	src := "Thanks @octocat and @nobody, see #12 and #13."
	tt := ui.NewTester(func(c *ui.Context) { View(c, src, o) }, 600, 400)
	for _, s := range []string{"@octocat", "#12"} {
		if err := tt.Click(s); err != nil {
			t.Fatalf("%s: %v in %q", s, err, tt.Texts())
		}
	}
	if !slices.Equal(opened, []string{"/users/octocat", "/pulls/12"}) {
		t.Errorf("opened %q", opened)
	}
	// Those it does not know stay text.
	if err := tt.Click("@nobody"); err == nil {
		t.Error("@nobody is a link")
	}
	// Once the app knows them, they link.
	r.known = append(r.known, "nobody")
	tt.Frame()
	if err := tt.Click("@nobody"); err != nil || opened[len(opened)-1] != "/users/nobody" {
		t.Errorf("@nobody once known: %v, opened %q", err, opened)
	}
	// Without Resolve, they are text.
	tt = ui.NewTester(func(c *ui.Context) { View(c, src, nil) }, 600, 400)
	if !tt.HasText(src) {
		t.Errorf("not one text: %q", tt.Texts())
	}
}
