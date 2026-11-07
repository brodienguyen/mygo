package markdown

import (
	"testing"

	"github.com/egoist/mygo/ui"
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
