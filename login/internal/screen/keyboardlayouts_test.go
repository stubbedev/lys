package screen

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/lockview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/login/internal/layouts"
)

type layoutRig struct {
	k      *KeyboardLayouts
	p      *lockview.Prompt
	steps  []int
	queued chan func()
}

func newLayoutRig(t *testing.T, canSwitch bool) *layoutRig {
	t.Helper()
	r := &layoutRig{queued: make(chan func(), 8)}
	r.p = lockview.Build(lockview.Spec{Mode: lockview.Lock, Config: config.Defaults().Login, Font: treetest.Font(t), OnSubmit: func(string) {}}).Prompt
	d := LayoutDeps{
		Invoke: func(fn func()) { r.queued <- fn },
		Label: func(names []string) ([]layouts.Layout, error) {
			out := make([]layouts.Layout, len(names))
			for i, name := range names {
				out[i] = layouts.Layout{Label: strings.ToUpper(name[:2]), Name: name}
			}
			return out, nil
		},
		Show: func([]layouts.Layout, uint32) { r.k.ShowOn(r.p) },
		Mode: "test",
	}
	if canSwitch {
		d.Next = func(n int) { r.steps = append(r.steps, n) }
	}
	r.k = NewKeyboardLayouts(d)
	return r
}

func (r *layoutRig) run() { (<-r.queued)() }

func (r *layoutRig) chip() string { return strings.Join(treetest.Labels(r.p.Layout), " ") }

func (r *layoutRig) show(labeled []layouts.Layout, active uint32) {
	r.k.labeled, r.k.keyboard.Active = labeled, active
	r.k.ShowOn(r.p)
}

var usDENames = []string{"English (US)", "German"}

func TestTheChipShowsOnceTheLayoutsAreLabeled(t *testing.T) {
	r := newLayoutRig(t, true)
	r.k.Set(app.KeyboardLayout{Names: usDENames[:1]})
	if r.p.Layout.Visible() {
		t.Error("one layout shows the chip")
	}
	r.k.Set(app.KeyboardLayout{Names: usDENames, Active: 1})
	if r.p.Layout.Visible() {
		t.Error("the chip shows before its labels")
	}
	r.run()
	if !r.p.Layout.Visible() || r.chip() != "GE" {
		t.Errorf("two layouts, labeled: chip shown %v reading %q", r.p.Layout.Visible(), r.chip())
	}
	r.k.Set(app.KeyboardLayout{Names: usDENames, Active: 0})
	if r.chip() != "EN" {
		t.Errorf("a switch to US reads %q", r.chip())
	}
	r.k.Set(app.KeyboardLayout{Names: usDENames[:1]})
	if r.p.Layout.Visible() {
		t.Error("a keymap of one layout left the chip up")
	}
}

var usDE = []layouts.Layout{{Label: "US", Name: "English (US)"}, {Label: "DE", Name: "German"}}

func TestTheLayoutChipShowsOnlyForSeveralLayouts(t *testing.T) {
	r := newLayoutRig(t, true)
	if r.p.Layout.Visible() {
		t.Error("the chip shows before any keymap")
	}
	r.show(usDE[:1], 0)
	if r.p.Layout.Visible() {
		t.Error("one layout shows a chip")
	}
	r.show(usDE, 1)
	if !r.p.Layout.Visible() || r.chip() != "DE" {
		t.Errorf("two layouts, the second active: chip shown %v reading %q", r.p.Layout.Visible(), r.chip())
	}
	r.show(usDE, 2)
	if r.p.Layout.Visible() {
		t.Error("an active layout past the keymap's leaves the chip up")
	}
	r.show(nil, 0)
	if r.p.Layout.Visible() {
		t.Error("a dropped keymap leaves the chip up")
	}
}

func TestLabelsForAnOlderKeymapAreDropped(t *testing.T) {
	r := newLayoutRig(t, true)
	label, older := r.k.d.Label, make(chan struct{})
	r.k.d.Label = func(names []string) ([]layouts.Layout, error) {
		if names[0] == usDENames[0] {
			<-older
		}
		return label(names)
	}
	r.k.Set(app.KeyboardLayout{Names: usDENames})
	r.k.Set(app.KeyboardLayout{Names: []string{"French", "German"}})
	r.run()
	close(older)
	r.run()
	if r.chip() != "FR" {
		t.Errorf("the chip reads %q, want the newer keymap's FR", r.chip())
	}
}

func TestCyclingAsksForTheNextLayout(t *testing.T) {
	r := newLayoutRig(t, true)
	r.k.Cycle()
	if len(r.steps) != 0 {
		t.Errorf("cycling with no keymap asked for %v", r.steps)
	}
	r.k.Set(app.KeyboardLayout{Names: usDENames})
	r.k.Cycle()
	if !slices.Equal(r.steps, []int{1}) {
		t.Errorf("cycling asked for %v steps", r.steps)
	}
}

func TestWithoutASwitcherNothingSwitches(t *testing.T) {
	r := newLayoutRig(t, false)
	if r.k.CanSwitch() {
		t.Error("a tracker without Next reports it can switch")
	}
	r.k.Set(app.KeyboardLayout{Names: usDENames})
	r.k.Cycle()
	r.k.Want("German")
}
