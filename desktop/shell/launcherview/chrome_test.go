package launcherview

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestMessageMarkupFallsBackToTheLiteral(t *testing.T) {
	if got := messageMarkup(`<span color="#ff0000">late</span>`); !strings.Contains(got, "late") || !strings.Contains(got, "<span") {
		t.Errorf("valid markup = %q, want it translated", got)
	}
	got := messageMarkup("<b>broken")
	if strings.Contains(got, "<b>") || !strings.Contains(got, "broken") {
		t.Errorf("broken markup = %q, want the escaped literal", got)
	}
}

func TestApostropheInATitleIsMarkupTheLabelReads(t *testing.T) {
	l := widget.NewRichLabel(treetest.Font(t), 14, string(AppendEscaped(nil, "it's")), 0)
	if !strings.Contains(l.Text(), "it's") {
		t.Errorf("label reads %q", l.Text())
	}
}

func TestTheChromeShowsWhatItIsGivenAndHidesWhatIsEmpty(t *testing.T) {
	c := NewChrome(treetest.Font(t), nil, config.Defaults(), nil)
	if c.Chip.Visible() || c.Message.Visible() || c.Footer.Visible() || c.Sep.Visible() {
		t.Error("a fresh chrome shows a chip, message, footer or separator")
	}
	c.SetChip("drun")
	c.SetMessage("<b>note</b>")
	if !c.Chip.Visible() || c.Chip.Text() != "drun" || !c.Message.Visible() || c.Message.Text() != "note" {
		t.Errorf("chip %q message %q", c.Chip.Text(), c.Message.Text())
	}
	c.SetChip("")
	c.SetMessage("")
	if c.Chip.Visible() || c.Message.Visible() {
		t.Error("an empty chip or message still shows")
	}
	c.ShowHints(FooterHint{"Enter", "Open <it>"}, FooterHint{"Esc", "Close"})
	if got := c.Hints.Text(); !strings.Contains(got, "Enter Open <it>") || !strings.Contains(got, "Esc Close") {
		t.Errorf("hints read %q", got)
	}
	c.SetMasked(true)
	if c.Entry.Echo() != widget.EchoPassword {
		t.Error("a masked entry echoes its text")
	}
	c.SetMasked(false)
	if c.Entry.Echo() != widget.EchoNormal {
		t.Error("an unmasked entry hides its text")
	}
}
