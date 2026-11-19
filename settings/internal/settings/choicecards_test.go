package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func TestEveryChoiceOfAPictureRowHasItsPicture(t *testing.T) {
	for path, pictures := range map[string]map[string]picture{
		"styling.appearance":       appearancePictures,
		"styling.theme-provider":   themeProviderPictures,
		"wallpaper.fit-mode":       fitPictures,
		"bar.shadow":               shadowPictures,
		"bar.rounding":             roundingPictures,
		"bar.button-variant":       buttonVariantPictures,
		"bar.button-icon-position": iconPositionPictures,
		"styling.rounding":         roundingPictures,
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

func paintPicture(p picture) []byte {
	const w, h = choiceCardW, choicePicH
	data := make([]byte, render.Stride(w)*h)
	p.draw(render.New(data, render.Stride(w), w, h), render.Rect{W: w, H: h})
	return data
}

func TestEachRoundingAndShadowDrawsItsOwnPicture(t *testing.T) {
	for name, pictures := range map[string]map[string]picture{"rounding": roundingPictures, "shadow": shadowPictures, "button": buttonVariantPictures, "icon position": iconPositionPictures} {
		seen := map[string]string{}
		for value, p := range pictures {
			px := string(paintPicture(p))
			if other, ok := seen[px]; ok && !(name == "rounding" && styling.RoundingRadiusPx(config.RoundingLevel(value), 1) == styling.RoundingRadiusPx(config.RoundingLevel(other), 1)) {
				t.Errorf("%s: %s and %s draw the same picture", name, value, other)
			}
			seen[px] = value
		}
	}
}

func TestTheBarBorderWidthIsASliderOverABarBordersWidths(t *testing.T) {
	k := testKit(t, "bar:\n  border-location: all\n  border-width: 20\n")
	p := layoutPage(t, k, "bar")
	c, ok := rowOf(t, p, "bar.border-width").ctl.(*pixelControl)
	if !ok {
		t.Fatalf("the border width is a %T", rowOf(t, p, "bar.border-width").ctl)
	}
	if got := c.slider.Knob.Value(); got != 8 {
		t.Errorf("a 20 px border puts the slider at %v px, want its 8 px end", got)
	}
	if got := c.number.Value(); got != 20 {
		t.Errorf("a 20 px border reads %v in the number beside the slider", got)
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
