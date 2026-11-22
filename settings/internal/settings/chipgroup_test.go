package settings

import (
	"math"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/ui/widget"
)

// pressChip is the user's press on chip i of g.
func pressChip(g *widget.ChoiceChips, i int) { g.Chip(i).ClickAt(widget.Point{}) }

// presetRow is a shell row on a page whose control is preset chips.
type presetRow struct {
	page string
	spec rowSpec
}

func presetRows(t *testing.T) []presetRow {
	t.Helper()
	k := testKit(t, "")
	var out []presetRow
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			if p.wm {
				continue
			}
			for _, sec := range p.sections {
				for _, r := range sec.flatRows() {
					if r.wm || r.editor == nil {
						continue
					}
					switch fieldControl(k, k.store, r).(type) {
					case *presetPicker[int64], *presetPicker[float64]:
						out = append(out, presetRow{page: p.id, spec: r})
					}
				}
			}
		}
	}
	return out
}

// Every preset picker on every page shows the stored value as the chip
// that is checked, painted apart from the others; a value off the
// presets checks Custom… and shows it in the input; and picking a
// preset after that hides the input and checks that chip alone.
func TestEveryPresetPickerShowsTheStoredValueAsTheCheckedChip(t *testing.T) {
	rows := presetRows(t)
	pages := map[string]bool{}
	for _, r := range rows {
		pages[r.page] = true
	}
	for _, want := range []string{"fonts", "lock", "recorder", "privacy", "notifications"} {
		if !pages[want] {
			t.Errorf("the sweep found no preset chips on %s", want)
		}
	}
	for _, r := range rows {
		t.Run(r.spec.path, func(t *testing.T) {
			k := testKit(t, "")
			switch p := fieldControl(k, k.store, r.spec).(type) {
			case *presetPicker[int64]:
				checkPresetPicker(t, k, p)
			case *presetPicker[float64]:
				checkPresetPicker(t, k, p)
			}
		})
	}
}

func checkPresetPicker[T presetValue](t *testing.T, k *kit, p *presetPicker[T]) {
	t.Helper()
	apptheme.New(k.store.svc.Config()).Attach(p.ChoiceChips)
	for i, pr := range p.presets {
		if err := p.slot.set(pr.value); err != nil {
			t.Fatalf("storing preset %d: %v", i, err)
		}
		p.refresh()
		if !slices.Equal(p.Checked(), []int{i}) || p.typed.Visible() {
			t.Fatalf("preset %d stored checks %v with the input shown %v", i, p.Checked(), p.typed.Visible())
		}
		on, off := p.Chip(i), p.Chip(len(p.presets))
		if !on.HasState(widget.StateChecked) || widget.StyleBackground(on) == widget.StyleBackground(off) {
			t.Fatalf("the checked chip %d paints as an unchecked one", i)
		}
	}
	off := offPreset(t, p)
	if err := p.slot.set(off); err != nil {
		t.Fatalf("storing %v: %v", off, err)
	}
	p.refresh()
	custom := len(p.presets)
	if !slices.Equal(p.Checked(), []int{custom}) || !p.typed.Visible() {
		t.Fatalf("%v, off the presets, checks %v with the input shown %v", off, p.Checked(), p.typed.Visible())
	}
	want, _ := p.custom.slot.get().(float64)
	if got := p.custom.Value(); math.Abs(got-want) > 0.01 {
		t.Errorf("the input shows %v for %v, want %v", got, off, want)
	}
	pressChip(p.ChoiceChips, 0)
	if !slices.Equal(p.Checked(), []int{0}) || p.typed.Visible() {
		t.Errorf("picking the first preset checks %v with the input shown %v", p.Checked(), p.typed.Visible())
	}
	if v, _ := p.slot.get().(T); v != p.presets[0].value {
		t.Errorf("picking the first preset stored %v, want %v", v, p.presets[0].value)
	}
}

// offPreset is a value no preset is: halfway between the two largest.
func offPreset[T presetValue](t *testing.T, p *presetPicker[T]) T {
	t.Helper()
	values := make([]T, len(p.presets))
	for i, pr := range p.presets {
		values[i] = pr.value
	}
	slices.Sort(values)
	if len(values) < 2 {
		t.Fatalf("%d presets, too few to choose between", len(values))
	}
	return values[len(values)-2] + (values[len(values)-1]-values[len(values)-2])/2
}

// The chip rows that were hand-built toggle buttons are choice chips.
func TestNoChipRowIsBuiltFromToggleButtons(t *testing.T) {
	toggle := regexp.MustCompile(`widget\.NewToggleButton\(`)
	for _, file := range []string{"presetchips.go", "barstates.go", "namechips.go", "editors_enumset.go", "editors_schedule.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if toggle.Match(src) {
			t.Errorf("%s builds a chip from widget.NewToggleButton", file)
		}
	}
}
