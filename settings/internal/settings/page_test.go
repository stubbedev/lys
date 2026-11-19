package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestPageColumnIsCappedAndCentred(t *testing.T) {
	k := testKit(t, "styling:\n  scale: 1.5\n")
	spec, _ := pageByID(layout(k.store.svc.Config(), bothParts), "fonts")
	p := buildPage(k, spec)
	p.Arrange(render.Rect{W: 3000, H: 3000})
	clamp, ok := p.Children()[0].(*widget.Clamp)
	if !ok {
		t.Fatalf("the page scrolls a %T, want its clamp", p.Children()[0])
	}
	content := clamp.Children()[0].(widget.Boundser).Bounds()
	if want := pageMaxWidth * 3 / 2; content.W != want {
		t.Errorf("the column is %dpx wide, want %dpx at scale 1.5", content.W, want)
	}
	if left, right := content.X, 3000-content.X-content.W; left-right > 1 || right-left > 1 {
		t.Errorf("the column sits %dpx from the left and %dpx from the right", left, right)
	}
	p.Arrange(render.Rect{W: 600, H: 3000})
	if w := clamp.Children()[0].(widget.Boundser).Bounds().W; w != 600 {
		t.Errorf("a narrow window gives the column %dpx, want all 600", w)
	}
}

func TestASectionShowsItsDescriptionOnlyWhenItHasOne(t *testing.T) {
	k, _, _, _ := wmKit(t, "")
	spec := wmPage("wm-test", "ld-monitor-symbolic",
		sectionSpec{title: "wm-section-gaps", rows: []rowSpec{wmField("gaps.inner")}},
		sectionSpec{title: "settings-title", rows: []rowSpec{wmField("tearing")}})
	notes := treetest.WithClass(buildPage(k, spec), "settings-section-description")
	if len(notes) != 1 {
		t.Fatalf("%d section notes, want the gaps section's alone", len(notes))
	}
	if got := notes[0].(*widget.Label).Text(); got != description("wm-section-gaps") || got == "" {
		t.Errorf("the note reads %q", got)
	}
}

func TestTheThemePageShowsOnlyTheActiveProvidersSettings(t *testing.T) {
	k := testKit(t, "")
	spec, _ := pageByID(layout(k.store.svc.Config(), bothParts), "theme")
	p := buildPage(k, spec)
	visible := func() []string {
		var out []string
		for _, c := range p.sections {
			if c.box.Visible() {
				out = append(out, treetest.Labels(c.box)[0])
			}
		}
		return out
	}
	tr := locale.T()
	if got := visible(); len(got) != 1 || got[0] != tr.Get("settings-section-palette") {
		t.Fatalf("with lys-shell's theme the provider sections shown are %v", got)
	}
	provider := slices.IndexFunc(p.rows, func(r *settingRow) bool { return r.path == "styling.theme-provider" })
	if err := p.rows[provider].be.slot("styling.theme-provider").set("matugen"); err != nil {
		t.Fatal(err)
	}
	if got := visible(); len(got) != 1 || got[0] != tr.Get("settings-section-matugen") {
		t.Errorf("after choosing matugen the provider sections shown are %v", got)
	}
	i := slices.IndexFunc(p.targets, func(r *settingRow) bool { return r != nil && r.path == "styling.wallust-palette" })
	p.reveal(i, nil)
	if got := visible(); !slices.Contains(got, tr.Get("settings-section-wallust")) {
		t.Errorf("revealing a wallust setting left its section hidden: %v", got)
	}
}
