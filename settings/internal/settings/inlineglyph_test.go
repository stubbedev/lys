package settings

import (
	"errors"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

// An inline failure set its summary in red beside a grey triangle. The
// glyph takes the summary's color, red for a failure and dim for a note.
func TestAnInlineStatesGlyphTakesItsSummarysColor(t *testing.T) {
	k := testKit(t, "")
	s := k.inlineState()
	host := themed(k, s)
	for _, show := range []func(){
		func() { s.Error("Could not save", errors.New("disk full"), nil) },
		func() { s.Note("Saved elsewhere", errors.New("moved")) },
	} {
		show()
		host.Arrange(host.Bounds())
		glyph, summary := widget.CascadeColor(s.glyph), widget.CascadeColor(s.summary)
		if glyph != summary {
			t.Errorf("%q reads in %#08x beside a glyph in %#08x", s.Text(), uint32(summary), uint32(glyph))
		}
	}
}
