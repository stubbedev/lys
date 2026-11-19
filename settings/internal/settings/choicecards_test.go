package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

func TestEveryChoiceOfAPictureRowHasItsPicture(t *testing.T) {
	for path, pictures := range map[string]map[string]picture{
		"styling.appearance":     appearancePictures,
		"styling.theme-provider": themeProviderPictures,
		"wallpaper.fit-mode":     fitPictures,
		"styling.rounding":       roundingPictures,
	} {
		meta, ok := config.Field(path)
		if !ok {
			t.Fatalf("no %s", path)
		}
		for _, v := range meta.Variants {
			if p, ok := pictures[v]; !ok || p.draw == nil && p.icon == "" {
				t.Errorf("%s: %s has no picture", path, v)
			}
		}
	}
}

func TestAPictureChoiceWritesTheCardChosen(t *testing.T) {
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("styling.appearance", pictureChoice(appearancePictures))).(*choiceCards)
	var dark *choiceCard
	for _, card := range c.cards {
		if card.value == string(config.AppearanceDark) {
			dark = card
		}
	}
	dark.PressAt(widget.Point{})
	if v := k.store.value("styling.appearance"); v != "dark" || c.current != "dark" {
		t.Fatalf("pressing Dark stored %v, shows %q", v, c.current)
	}
	c.cards[0].KeyAction(widget.KeyEnter, 0)
	if v := k.store.value("styling.appearance"); v != c.cards[0].value {
		t.Errorf("Enter on the first card stored %v", v)
	}
}

func TestAPictureChoiceMissingAPictureRefusesToBuild(t *testing.T) {
	k := testKit(t, "")
	defer func() {
		if recover() == nil {
			t.Error("a picture choice without a picture for every value built")
		}
	}()
	fieldControl(k, k.store, field("styling.appearance", pictureChoice(map[string]picture{"light": {icon: "ld-sun-symbolic"}})))
}

func TestTheRoundingPicturesRoundMoreFromNoneToFull(t *testing.T) {
	const h = 40
	order := []config.RoundingLevel{config.RoundingNone, config.RoundingSm, config.RoundingMd, config.RoundingLg, config.RoundingFull}
	for i := 1; i < len(order); i++ {
		if roundingRadius(order[i], h) <= roundingRadius(order[i-1], h) {
			t.Errorf("%s rounds %d px, no more than %s's %d", order[i], roundingRadius(order[i], h), order[i-1], roundingRadius(order[i-1], h))
		}
	}
	if roundingRadius(config.RoundingNone, h) != 0 || roundingRadius(config.RoundingFull, h) != h/2 {
		t.Error("none is not square or full is not a pill")
	}
	if r := roundingRadius(config.RoundingDefault, h); r < 0 || r > h/2 {
		t.Errorf("the theme default rounds %d px on a %d px button", r, h)
	}
	defer func() {
		if recover() == nil {
			t.Error("an unknown rounding drew a radius")
		}
	}()
	roundingRadius("huge", h)
}
