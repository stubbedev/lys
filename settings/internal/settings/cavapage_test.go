package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func cavaRange(t *testing.T, k *kit) *frequencyRange {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "cava", "modules.cava.low-cutoff")).(*frequencyRange)
	if !ok {
		t.Fatal("the cutoffs are not one range")
	}
	return c
}

func TestTheCavaCutoffsCannotCross(t *testing.T) {
	k := testKit(t, "modules:\n  cava:\n    low-cutoff: 50\n    high-cutoff: 17000\n")
	c := cavaRange(t, k)
	if _, hi := c.low.Range(); hi != 16999 {
		t.Errorf("the low cutoff reaches %v under a 17000 Hz high one", hi)
	}
	c.low.SetValue(20000)
	c.low.OnValueChanged(c.low.Value())
	if v := k.store.value("modules.cava.low-cutoff"); v != int64(16999) {
		t.Errorf("a low cutoff past the high one stored %v", v)
	}
	c.high.SetValue(18000)
	c.high.OnValueChanged(c.high.Value())
	if lo, hi := c.low.Range(); lo != cavaLowestHz || hi != 17999 {
		t.Errorf("after raising the high cutoff the low one spans %v-%v", lo, hi)
	}
	if lo, _ := c.high.Range(); lo != 17000 {
		t.Errorf("the high cutoff reaches down to %v over a 16999 Hz low one", lo)
	}
}

func TestTheCavaRangeFollowsAnOutsideChange(t *testing.T) {
	k := testKit(t, "")
	c := cavaRange(t, k)
	if err := k.store.set("modules.cava.high-cutoff", int64(400)); err != nil {
		t.Fatal(err)
	}
	if err := k.store.set("modules.cava.low-cutoff", int64(300)); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if c.low.Value() != 300 || c.high.Value() != 400 {
		t.Fatalf("the range shows %v-%v, want 300-400", c.low.Value(), c.high.Value())
	}
	if _, hi := c.low.Range(); hi != 399 {
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
