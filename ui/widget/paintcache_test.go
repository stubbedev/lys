package widget

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
)

func TestTwoTreesPaintOneStylesheetImageAtOnce(t *testing.T) {
	dir := t.TempDir()
	const rounds = 12
	paths := make([]string, rounds)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("image-%d.png", i))
		if i%2 == 1 {
			continue
		}
		if err := os.WriteFile(paths[i], []byte(redPNG(t)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const css = `.i { background-image: url("%s"); background-color: #0000ff; }`
	atOnce(t, 2, func(int) error {
		sheet := NewStylesheet(bytesReplaceOne(css, paths[0]), StylePriorityApplication)
		w := &boxLayersLeaf{}
		w.AddClass("i")
		w.AttachStylesheet(sheet)
		var errs []error
		for i, path := range paths {
			sheet.Load(bytesReplaceOne(css, path))
			SizeWithin(w, Size{W: 30, H: 30})
			w.Arrange(render.Rect{W: 30, H: 30})
			want := render.RGB(255, 0, 0)
			if i%2 == 1 {
				want = render.RGB(0, 0, 255)
			}
			if got := centerOf(paintTree(w, 30, 30), 30, 30); got != want {
				errs = append(errs, fmt.Errorf("round %d: center %v, want %v", i, got, want))
			}
		}
		return errors.Join(errs...)
	})
}

func TestTwoTreesPaintTranslucentSwatchesAtOnce(t *testing.T) {
	surface, border := render.RGB(0x10, 0x20, 0x31), render.RGB(0xe0, 0xd0, 0xc1)
	th := *DarkTheme()
	th.Surface, th.Border = surface, border
	SetTheme(&th)
	t.Cleanup(func() { SetTheme(nil) })
	atOnce(t, 2, func(int) error {
		seeThrough, solid := NewColorSwatch(render.RGBA(0, 0, 0, 0)), NewColorSwatch(render.RGB(0, 200, 0))
		var errs []error
		for i := range 20 {
			for _, s := range []*ColorSwatch{seeThrough, solid} {
				SizeWithin(s, Size{W: 24, H: 24})
				s.Arrange(render.Rect{W: 24, H: 24})
			}
			got := centerOf(paintTree(seeThrough, 24, 24), 24, 24)
			if got != surface && got != border {
				errs = append(errs, fmt.Errorf("round %d: a clear swatch's center %v is neither checkerboard shade", i, got))
			}
			if got := centerOf(paintTree(solid, 24, 24), 24, 24); got != render.RGB(0, 200, 0) {
				errs = append(errs, fmt.Errorf("round %d: an opaque swatch's center %v, want its own green over no checkerboard", i, got))
			}
		}
		return errors.Join(errs...)
	})
}

type heroTree struct {
	clock       anim.Clock
	root, small *Box
}

func newHeroTree(t *testing.T, class string) *heroTree {
	t.Helper()
	h := &heroTree{root: NewBox(Column, 0, 0)}
	t.Cleanup(Host(h.root, &h.clock))
	h.small = NewBox(Column, 0, 0)
	h.small.AddClass(class)
	h.small.SetSharedElement("hero")
	return h
}

func (h *heroTree) frame() {
	SizeWithin(h.root, Size{W: 120, H: 120})
	h.root.Arrange(render.Rect{W: 120, H: 120})
	CollectDamage(h.root)
	paintTree(h.root, 120, 120)
}

func (h *heroTree) morph() error {
	h.root.Clear()
	h.root.Append(h.small, false)
	h.frame()
	from := h.small.Bounds()
	h.root.Remove(h.small)
	big := NewBox(Column, 0, 0)
	big.AddClass("big")
	big.SetSharedElement("hero")
	h.root.Append(big, false)
	h.frame()
	if big.shared == nil {
		return errors.New("the big hero did not morph out of the small one")
	}
	if got := big.shared.from.rect; got != from {
		return fmt.Errorf("the morph starts at %v, want its own tree's small hero at %v", got, from)
	}
	return nil
}

func heroCSS(t *testing.T) {
	loadCSS(t, `.near { min-width: 20px; min-height: 20px; margin: 10px; background-color: #ff0000; }
		.far { min-width: 30px; min-height: 30px; margin: 50px; background-color: #ff0000; }
		.big { min-width: 60px; min-height: 60px; margin: 5px; background-color: #0000ff; }`)
	t.Cleanup(sharedShots.reset)
}

func TestTwoApplicationsMorphTheirOwnSharedElementsAtOnce(t *testing.T) {
	heroCSS(t)
	trees := [2]*heroTree{newHeroTree(t, "near"), newHeroTree(t, "far")}
	atOnce(t, len(trees), func(w int) error {
		var errs []error
		for i := range 10 {
			if err := trees[w].morph(); err != nil {
				errs = append(errs, fmt.Errorf("round %d: %w", i, err))
			}
		}
		return errors.Join(errs...)
	})
}

func TestASharedElementMorphsOnlyOutOfItsOwnClocksTrees(t *testing.T) {
	heroCSS(t)
	t.Cleanup(SetSurfaceOriginResolver(func(Widget) (Point, bool) { return Point{}, true }))
	shown := newHeroTree(t, "near")
	shown.root.Append(shown.small, false)
	shown.frame()

	other := &heroTree{root: NewBox(Column, 0, 0)}
	t.Cleanup(Host(other.root, &other.clock))
	stranger := NewBox(Column, 0, 0)
	stranger.AddClass("big")
	stranger.SetSharedElement("hero")
	other.root.Append(stranger, false)
	if SharedMorphPending(other.root, &other.clock) {
		t.Error("a tree on another clock reports a morph pending out of the first clock's hero")
	}
	other.frame()
	if stranger.shared != nil {
		t.Error("a hero on another clock morphed out of the first clock's hero")
	}

	sibling := NewBox(Column, 0, 0)
	t.Cleanup(Host(sibling, &shown.clock))
	heir := NewBox(Column, 0, 0)
	heir.AddClass("big")
	heir.SetSharedElement("hero")
	sibling.Append(heir, false)
	if !SharedMorphPending(sibling, &shown.clock) {
		t.Error("a second surface on the first clock reports no morph pending")
	}
	SizeWithin(sibling, Size{W: 120, H: 120})
	sibling.Arrange(render.Rect{W: 120, H: 120})
	paintTree(sibling, 120, 120)
	if heir.shared == nil {
		t.Error("a hero on a second surface of the first clock did not morph out of its hero")
	}
}

func TestTwoTabViewsOpenAndClosePagesAtOnce(t *testing.T) {
	atOnce(t, 2, func(int) error {
		v := NewTabView()
		var errs []error
		for i := range 50 {
			p := v.Append(NewSpacer(1, 1), fmt.Sprintf("page %d", i))
			if got := tabPageByID(p.id); got != p {
				errs = append(errs, fmt.Errorf("page %d: its id finds %v", i, got))
			}
			if !v.ClosePage(p) {
				errs = append(errs, fmt.Errorf("page %d: did not close", i))
			}
			if got := tabPageByID(p.id); got != nil {
				errs = append(errs, fmt.Errorf("page %d: a closed page's id still finds it", i))
			}
		}
		return errors.Join(errs...)
	})
}

func TestTwoLoopsAnnounceAtOnceAndEachBridgeTakesItsOwn(t *testing.T) {
	const each = 40
	loops := [2]*testLoop{newTestLoop(t), newTestLoop(t)}
	var speakers [2]*Box
	for i, l := range loops {
		l.do(t, func() {
			SetAnnouncementsHeard(true)
			speakers[i] = NewBox(Row, 0, 0)
		})
		t.Cleanup(func() { l.do(t, func() { SetAnnouncementsHeard(false) }) })
	}
	var wg sync.WaitGroup
	for i, l := range loops {
		wg.Go(func() {
			l.do(t, func() {
				for n := range each {
					Announce(speakers[i], fmt.Sprintf("loop %d says %d", i, n), Polite)
				}
			})
		})
	}
	wg.Wait()
	for i, l := range loops {
		var got, left []Announcement
		l.do(t, func() {
			got = TakeAnnouncements(nil)
			left = TakeAnnouncements(nil)
		})
		if len(got) != 1 || got[0].Widget != Widget(speakers[i]) || got[0].Text != fmt.Sprintf("loop %d says %d", i, each-1) {
			t.Errorf("loop %d took %v, want its own widget's latest announcement alone", i, got)
		}
		if len(left) != 0 {
			t.Errorf("loop %d: a second take found %v, want the queue emptied", i, left)
		}
	}

	var stoppedLeft, heard []Announcement
	loops[0].do(t, func() {
		Announce(speakers[0], "dropped", Polite)
		SetAnnouncementsHeard(false)
		stoppedLeft = TakeAnnouncements(nil)
		Announce(speakers[0], "unheard", Polite)
		stoppedLeft = TakeAnnouncements(stoppedLeft)
	})
	loops[1].do(t, func() {
		Announce(speakers[1], "still heard", Assertive)
		heard = TakeAnnouncements(nil)
	})
	if len(stoppedLeft) != 0 {
		t.Errorf("the loop whose bridge stopped kept %v", stoppedLeft)
	}
	if len(heard) != 1 || heard[0].Text != "still heard" {
		t.Errorf("after the other loop's bridge stopped, this one took %v, want its announcement", heard)
	}
}
