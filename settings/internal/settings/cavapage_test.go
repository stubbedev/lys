package settings

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheCavaCutoffsCannotCross(t *testing.T) {
	k := testKit(t, "modules:\n  cava:\n    low-cutoff: 50\n    high-cutoff: 17000\n")
	c := pageRange(t, k, "cava", "modules.cava.low-cutoff")
	if !c.spec.axis.log {
		t.Error("the frequencies are not on a logarithmic axis")
	}
	c.move(0, 20000)
	if v := k.store.value("modules.cava.low-cutoff"); v != int64(16999) {
		t.Errorf("a low cutoff past the high one stored %v", v)
	}
	c.move(1, 10)
	if v := k.store.value("modules.cava.high-cutoff"); v != int64(17000) {
		t.Errorf("a high cutoff under the low one stored %v, want it held at 17000", v)
	}
	c.thumbs[1].KeyAction(widget.KeyRight, widget.ModShift)
	if v, _ := thresholdNum(k.store.value("modules.cava.high-cutoff")); v <= 17000 {
		t.Errorf("shift-right on the high cutoff stored %v", v)
	}
}

func TestTheCavaRangeFollowsAnOutsideChange(t *testing.T) {
	k := testKit(t, "")
	c := pageRange(t, k, "cava", "modules.cava.low-cutoff")
	if err := k.store.set("modules.cava.high-cutoff", int64(400)); err != nil {
		t.Fatal(err)
	}
	if err := k.store.set("modules.cava.low-cutoff", int64(300)); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if c.value(0) != 300 || c.value(1) != 400 {
		t.Fatalf("the range shows %v-%v, want 300-400", c.value(0), c.value(1))
	}
	if !strings.Contains(c.thumbs[1].AccessibleName(), "400") {
		t.Errorf("the high thumb reads %q after the change", c.thumbs[1].AccessibleName())
	}
	if _, hi := c.bounds(0); hi != 399 {
		t.Errorf("the low cutoff reaches %v", hi)
	}
}

func TestTheCavaPageDrawsItsLookAndFoldsItsSmoothing(t *testing.T) {
	k := testKit(t, "")
	p := layoutPage(t, k, "cava")
	for _, path := range []string{"modules.cava.style", "modules.cava.direction"} {
		if _, ok := rowOf(t, p, path).ctl.(*choiceCards); !ok {
			t.Errorf("%s is not chosen by picture", path)
		}
	}
	spec, _ := pageByID(layout(k.store.svc.Config(), bothParts), "cava")
	for _, s := range spec.sections {
		for _, r := range s.rows {
			if r.path == "modules.cava.framerate" && !s.advanced {
				t.Error("the framerate is not folded away with the smoothing")
			}
			if r.path == "modules.cava.style" && s.advanced {
				t.Error("the style is folded away")
			}
		}
	}
	fps, ok := rowOf(t, p, "modules.cava.framerate").ctl.(*presetPicker)
	if !ok {
		t.Fatal("the framerate is not chips")
	}
	if got, want := chipLabels(fps)[1], locale.T().Get("settings-framerate", i18n.Int("n", 60)); got != want {
		t.Errorf("the second framerate chip reads %q, want %q", got, want)
	}
}
