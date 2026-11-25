package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func TestEveryChoiceOfAPictureRowHasItsPicture(t *testing.T) {
	for path, pictures := range map[string]map[string]picture{
		"styling.appearance":                appearancePictures,
		"styling.theme-provider":            themeProviderPictures,
		"wallpaper.fit-mode":                fitPictures,
		"bar.shadow":                        shadowPictures,
		"bar.rounding":                      roundingPictures,
		"bar.button-variant":                buttonVariantPictures,
		"bar.button-icon-position":          iconPositionPictures,
		"styling.rounding":                  roundingPictures,
		"login.background-mode":             lockBackgroundPictures,
		"modules.notifications.popup-stack": popupStackPictures,
		"osd.text-align":                    textAlignPictures,
		"modules.minimized-windows.scope":   windowScopePictures,
		"animations.transition":             transitionPictures,
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
	last := c.cards[len(c.cards)-1]
	last.KeyAction(widget.KeySpace, 0)
	if v := k.store.value("styling.appearance"); v != last.value {
		t.Errorf("Space on the last card stored %v", v)
	}
}

func paintPicture(p picture) []byte {
	const w, h = choiceCardW, choicePicH
	data := make([]byte, render.Stride(w)*h)
	p.draw(render.New(data, render.Stride(w), w, h), render.Rect{W: w, H: h})
	return data
}

func TestEachRoundingAndShadowDrawsItsOwnPicture(t *testing.T) {
	for name, pictures := range map[string]map[string]picture{"rounding": roundingPictures, "shadow": shadowPictures, "button": buttonVariantPictures, "icon position": iconPositionPictures, "transition": transitionPictures} {
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
	c, ok := rowOf(t, p, "bar.border-width").ctl.(*softRange)
	if !ok {
		t.Fatalf("the border width is a %T", rowOf(t, p, "bar.border-width").ctl)
	}
	if got := c.Knob.Value(); got != 8 {
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

// Media's "Application Mapped" broke onto two lines whose descenders
// touched the selected card's ring, and a later fix squeezed two-line
// names below the height the face draws a line at, so they vanished:
// most Theme presets showed no name. Every card's name shows every line
// it is broken onto, ending half the picture's inset above the card's
// bottom edge, clear of its ring.
func TestACardsNameShowsEveryLineClearOfItsRing(t *testing.T) {
	for _, id := range []string{"media", "theme"} {
		win := laidOutPage(t, "", id, sweepW)
		px := win.k.textPx()
		twoLines := 0
		for _, c := range treetest.All[*choiceCards](win.page) {
			for _, card := range c.cards {
				b := card.Bounds()
				if b.Empty() {
					continue
				}
				lines := card.labelLines()
				if len(lines) == 2 {
					twoLines++
				}
				stride := render.Stride(b.X + b.W)
				buf := make([]byte, stride*(b.Y+b.H))
				card.Paint(render.New(buf, stride, b.X+b.W, b.Y+b.H))
				for _, line := range lines {
					if line.text == "" || !inked(buf, stride, line.box) {
						t.Errorf("%s: %q shows no %q in %v (a line is %dpx tall)", id, card.label, line.text, line.box, win.k.face.Shape(line.text, px).LineHeight())
					}
				}
				last := lines[len(lines)-1].box
				if room := b.Y + b.H - (last.Y + last.H); room < choiceCardGap/2 {
					t.Errorf("%s: %q ends %dpx above its card's edge, want %d", id, card.label, room, choiceCardGap/2)
				}
			}
		}
		if twoLines == 0 {
			t.Fatalf("the %s page shows no card whose name takes two lines", id)
		}
	}
}

// inked reports whether any pixel of r in buf differs from r's first.
func inked(buf []byte, stride int, r render.Rect) bool {
	at := func(x, y int) [4]byte { return [4]byte(buf[y*stride+4*x:]) }
	bg := at(r.X, r.Y)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if at(x, y) != bg {
				return true
			}
		}
	}
	return false
}
