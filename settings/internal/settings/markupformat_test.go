package settings

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

// Launcher modes' Display Format split its Pango markup into literal
// chips ("[<span weight='light' size='small'><i>(") and previewed the
// tags as text. A format the config marks as Pango markup is edited as
// text, and its preview is styled by the markup, no tag read as text;
// markup that does not parse previews as the failure it is.
func TestAMarkupFormatPreviewsStyledNotAsTags(t *testing.T) {
	const path = "launcher.drun.display-format"
	if meta, ok := config.Field(path); !ok || !meta.Markup {
		t.Fatalf("the config does not mark %s as Pango markup", path)
	}
	if meta, _ := config.Field("modules.clock.format"); meta.Markup {
		t.Fatal("the clock's format reads as Pango markup")
	}
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, field(path)).(*formatEditor)
	if !ok {
		t.Fatal("a markup format is not edited as text")
	}
	p := c.preview
	if p.sample.Visible() || !p.styled.Visible() {
		t.Fatalf("the preview shows the plain sample %v, the styled one %v", p.sample.Visible(), p.styled.Visible())
	}
	runs, ok := widget.ParseMarkup(p.styled.Markup())
	var text strings.Builder
	for _, r := range runs {
		text.WriteString(r.Text)
	}
	if !ok || strings.Contains(text.String(), "span") || !strings.Contains(text.String(), "Firefox") {
		t.Errorf("the preview reads %q", text.String())
	}
	c.showPreview("{name} <span weight='light'")
	if p.styled.Visible() || !p.invalid.Visible() {
		t.Error("markup that does not parse still previews styled")
	}
}
