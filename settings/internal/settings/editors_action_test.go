package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func everyActionChoice() []actionChoice {
	out := workspaceChoices()
	for _, p := range modulePages(config.Defaults()) {
		out = append(out, actionChoices(p.id)...)
	}
	return out
}

func TestEveryActionPresetIsAMessage(t *testing.T) {
	tr := locale.T()
	for _, ch := range everyActionChoice() {
		if !tr.Has(ch.key) {
			t.Errorf("the preset %q has no message %q", ch.command, ch.key)
		}
	}
}

func TestTheActionMenuReadsItsPresetsInTheLanguage(t *testing.T) {
	k := testKit(t, "")
	c := newActionEditor(k, k.store.slot("modules.media.left-click"), actionChoices("media"))
	items := c.drop.Items()
	tr := locale.T()
	want := []string{tr.Get("settings-action-media-play-pause"), tr.Get("settings-action-none"), tr.Get("settings-action-custom")}
	for _, w := range want {
		if !slices.Contains(items, w) {
			t.Errorf("the menu %q lacks %q", items, w)
		}
	}
	for _, it := range items {
		if strings.HasPrefix(it, "settings-") {
			t.Errorf("the menu shows the message id %q", it)
		}
	}
}
