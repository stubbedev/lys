package filebrowser

import (
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

// qlState is what Quick Look's frame shows: whether it is mapped and
// open, how far its card is revealed, and the file its card holds.
type qlState struct {
	visible, open bool
	progress      float64
	path          string
}

func (hs *harness) qlState() qlState {
	var s qlState
	hs.do(func() {
		f := hs.c.ui.quicklook
		s = qlState{visible: f.shown, open: hs.c.quicklookOpen(), progress: f.rev.Progress(), path: hs.c.ui.ql.Path()}
	})
	return s
}

// animated gives hs the default animations with the toolkit's motion
// on, so Quick Look plays its enter and exit.
func animated(t *testing.T, hs *harness) {
	t.Helper()
	t.Cleanup(anim.SetInstant(false))
	anim.SetReduced(false)
	hs.do(func() { hs.c.d.Animations = config.DefaultsAnimations })
}

// notes writes note0.txt to note2.txt in docs and is the folder.
func notes(t *testing.T, hs *harness) string {
	t.Helper()
	dir := filepath.Join(hs.root, "docs")
	for _, n := range []string{"note0.txt", "note1.txt", "note2.txt"} {
		writeFile(t, filepath.Join(dir, n), []byte(n))
	}
	return dir
}

// Quick Look enters as the launcher does, the card showing while it
// fades in; Escape plays the exit, and only once it ends is the card
// hidden and what it showed let go. Stepping while it shows swaps the
// file without playing the enter again.
func TestQuicklookEntersAndLeavesWithTheSurfacesAnimation(t *testing.T) {
	hs := newHarness(t, "")
	animated(t, hs)
	dir := notes(t, hs)
	hs.quicklookOn(t, dir, "note0.txt")
	if s := hs.qlState(); !s.visible || !s.open || s.progress >= 1 {
		t.Fatalf("after Space Quick Look is %+v, want it shown and entering", s)
	}
	hs.do(func() { hs.c.ui.quicklook.rev.Finish() })
	hs.key(xkb.KeyRight, 0, nil)
	hs.waitPreview(t, "note1.txt")
	if s := hs.qlState(); !s.open || s.progress != 1 {
		t.Errorf("stepping left Quick Look %+v, want it shown whole without a new enter", s)
	}
	hs.key(xkb.KeyEscape, 0, nil)
	if s := hs.qlState(); !s.visible || s.open || filepath.Base(s.path) != "note1.txt" {
		t.Fatalf("while leaving Quick Look is %+v, want it still showing note1.txt", s)
	}
	hs.do(func() { hs.c.ui.quicklook.rev.Finish() })
	if s := hs.qlState(); s.visible || s.open || s.path != "" {
		t.Errorf("after the exit Quick Look is %+v, want it hidden and empty", s)
	}
	hs.hide()
}

// Space while Quick Look leaves opens it again: the exit's end then
// neither hides it nor lets go of its file.
func TestQuicklookReopenedWhileLeavingStays(t *testing.T) {
	hs := newHarness(t, "")
	animated(t, hs)
	dir := notes(t, hs)
	hs.quicklookOn(t, dir, "note0.txt")
	hs.do(func() { hs.c.ui.quicklook.rev.Finish() })
	hs.key(xkb.KeySpace, 0, nil)
	hs.key(xkb.KeySpace, 0, nil)
	hs.do(func() { hs.c.ui.quicklook.rev.Finish() })
	if s := hs.qlState(); !s.visible || !s.open || s.progress != 1 || filepath.Base(s.path) != "note0.txt" {
		t.Errorf("reopened while leaving, Quick Look is %+v", s)
	}
	hs.hide()
}

// With the toolkit's motion off or reduced, or the animations
// disabled, Quick Look opens and closes at once.
func TestQuicklookInstantMotionLandsAtOnce(t *testing.T) {
	for _, mode := range []string{"instant", "reduced", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			hs := newHarness(t, "")
			animated(t, hs)
			switch mode {
			case "instant":
				t.Cleanup(anim.SetInstant(true))
			case "reduced":
				anim.SetReduced(true)
				t.Cleanup(func() { anim.SetReduced(false) })
			case "disabled":
				hs.do(func() {
					hs.c.d.Animations = func() config.AnimationsConfig {
						a := config.DefaultsAnimations()
						a.Enabled = false
						return a
					}
				})
			}
			dir := notes(t, hs)
			hs.quicklookOn(t, dir, "note0.txt")
			if s := hs.qlState(); !s.visible || !s.open || s.progress != 1 {
				t.Errorf("Space opened Quick Look as %+v, want it whole at once", s)
			}
			hs.key(xkb.KeyEscape, 0, nil)
			if s := hs.qlState(); s.visible || s.path != "" {
				t.Errorf("Escape left Quick Look %+v, want it hidden and empty at once", s)
			}
			hs.hide()
		})
	}
}

// A click beside the open card closes Quick Look; one on the card does
// not, and once it closed presses reach the views again.
func TestAClickBesideQuicklookClosesIt(t *testing.T) {
	hs := newHarness(t, "")
	dir := notes(t, hs)
	hs.quicklookOn(t, dir, "note0.txt")
	click := func(at func(card, frame render.Rect) widget.Point) widget.Widget {
		var hit widget.Widget
		hs.do(func() {
			lay(hs.cfg.Root, 1400, 1000)
			f := hs.c.ui.quicklook
			p := at(f.rev.Bounds(), f.Bounds())
			hit = hs.cfg.Root.HitTest(p)
			r := &widget.Router{Root: hs.cfg.Root}
			r.Move(p)
			r.Press(widget.BTNLeft, p, 0)
			r.Release(widget.BTNLeft, p)
		})
		return hit
	}
	inside := func(card, _ render.Rect) widget.Point {
		return widget.Point{X: card.X + card.W/2, Y: card.Y + card.H/2}
	}
	beside := func(card, frame render.Rect) widget.Point {
		return widget.Point{X: frame.X + 4, Y: card.Y + card.H - 4}
	}
	click(inside)
	if s := hs.qlState(); !s.open {
		t.Fatalf("a click on the card closed Quick Look: %+v", s)
	}
	if hit := click(beside); hit != widget.Widget(hs.c.ui.quicklook) {
		t.Errorf("a click beside the card hit %T, want the frame", hit)
	}
	if s := hs.qlState(); s.open || s.visible {
		t.Errorf("a click beside the card left Quick Look %+v", s)
	}
	if hit := click(beside); hit == widget.Widget(hs.c.ui.quicklook) {
		t.Error("with Quick Look closed a press still hits its frame")
	}
	hs.hide()
}
