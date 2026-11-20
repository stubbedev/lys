package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/styling"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func previewedDesktop(t *testing.T, p *settingsPage) *themeDesktop {
	t.Helper()
	found := treetest.All[*themeDesktop](onlyPreview(t, p))
	if len(found) != 1 {
		t.Fatalf("the theme page previews %d desktops, want one: %v", len(found), treetest.Labels(onlyPreview(t, p)))
	}
	return found[0]
}

func TestTheThemePagePreviewsTheDraftsPalette(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, themePage(nil))
	want, err := paletteColors(config.Defaults().Styling.ActivePalette())
	if err != nil {
		t.Fatal(err)
	}
	if got := previewedDesktop(t, p).colors; !slices.Equal(got, want) {
		t.Errorf("the default palette previews %v, want %v", got, want)
	}
	set(t, k, p, "styling.palette.primary", "#ff0000")
	red, _ := styling.ParseHex("#ff0000")
	if got := previewedDesktop(t, p).colors[slices.Index(paletteKeys, "primary")]; got != red {
		t.Errorf("primary #ff0000 previews %#08x", got)
	}

	set(t, k, p, "styling.theme-provider", string(config.ThemeProviderMatugen))
	preview := onlyPreview(t, p)
	if len(treetest.All[*themeDesktop](preview)) != 0 || !slices.Contains(treetest.Labels(preview), locale.T().Get("settings-theme-preview-generated")) {
		t.Errorf("a generated palette previews %v, want the note that it has none to show", treetest.Labels(preview))
	}
}
