package settings

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The editors built from several controls (editors/optional,
// editors/size, editors/surface_animation).

// enumLabels are the variants' labels: the Fluent key's text, or the
// raw value where the key is missing (variant_label).
func enumLabels(meta config.FieldMeta) []string {
	t := locale.T()
	labels := make([]string, len(meta.Variants))
	for i, v := range meta.Variants {
		labels[i] = v
		if key := config.EnumLabelKey(meta.Type, v); t.Has(key) {
			labels[i] = t.Get(key)
		}
	}
	return labels
}

// optionalNumber is an optional number as one spin button: empty and
// reading "Inherited" while unset, a value typed or stepped (from the
// fallback) setting it, and emptying it inheriting again.
type optionalNumber struct {
	*widget.SpinButton
	k     *kit
	slot  slot
	whole bool
}

func newOptionalNumber(k *kit, s slot, lo, hi, step, fallback float64, digits int) *optionalNumber {
	c := &optionalNumber{SpinButton: widget.NewSpinButton(k.face, k.textPx(), 0, lo, hi, step, digits), k: k, slot: s, whole: digits == 0}
	c.SetOptional(locale.T().Get("settings-inherited"))
	c.SetValue(fallback)
	c.OnValueChanged = c.write
	c.OnCleared = s.unset
	c.refresh()
	return c
}

// write stores a whole number (float fields take it as one), or the
// fraction when the spin shows digits.
func (c *optionalNumber) write(v float64) {
	if c.whole {
		_ = c.slot.set(int64(math.Round(v)))
		return
	}
	_ = c.slot.set(v)
}

func (c *optionalNumber) refresh() {
	switch n := c.slot.get().(type) {
	case int64:
		c.SetValue(float64(n))
	case float64:
		c.SetValue(n)
	default:
		c.Clear()
	}
}

// Size modes (editors/size).
const (
	sizeScale = iota
	sizePx
	// remBasePx is REM_BASE_PX, the rem a scale multiplies.
	remBasePx = 16.0
	// maxPixels bounds a size in pixels; pixelsFallback is where an optional
	// pixel length starts once stepped.
	maxPixels      = 10000
	pixelsFallback = 400
)

// sizeEditor is size_with_base: a scale-or-pixels dropdown and the
// spin. Switching the mode converts the value through the base (a
// scale of base rem), so the size stays put.
type sizeEditor struct {
	*widget.Box
	k      *kit
	slot   slot
	basePx float64
	mode   *choicePick
	spin   *widget.SpinButton
	// slide, nil for a size without one, is a slider over loPx to hiPx
	// pixels, or the scales that make them, read by the spin, which
	// takes a value past them.
	slide      *widgets.DebouncedSlider
	loPx, hiPx float64
	syncing    bool
}

func newSizeEditor(k *kit, s slot, baseRem float64) *sizeEditor {
	c := newSizeParts(k, s, baseRem)
	c.AppendAligned(c.mode, false, widget.AlignCenter)
	c.AppendAligned(c.spin, false, widget.AlignCenter)
	c.refresh()
	return c
}

// newSizeSliderEditor is newSizeEditor with a slider over loPx to hiPx
// pixels, or the scales that make them, read by the spin.
func newSizeSliderEditor(k *kit, s slot, baseRem, loPx, hiPx float64) *sizeEditor {
	c := newSizeParts(k, s, baseRem)
	c.loPx, c.hiPx = loPx, hiPx
	c.slide = k.fieldSlider(loPx/c.basePx, hiPx/c.basePx, c.spin)
	c.slide.OnValue = func(v float64) {
		if !c.syncing {
			c.spin.SetValue(v)
		}
	}
	c.slide.OnCommit = func(v float64) {
		c.spin.SetValue(v)
		c.commit()
	}
	c.AppendAligned(c.mode, false, widget.AlignCenter)
	c.Append(c.slide, true)
	c.refresh()
	return c
}

// newSizeParts is a size editor's mode dropdown and spin, not yet
// placed.
func newSizeParts(k *kit, s slot, baseRem float64) *sizeEditor {
	t := locale.T()
	c := &sizeEditor{Box: gapBox(widget.Row), k: k, slot: s, basePx: baseRem * remBasePx}
	c.mode = newChoicePick(k, []string{t.Get("settings-size-scale"), t.Get("settings-size-px")}, sizeScale)
	c.spin = widget.NewSpinButton(k.face, k.textPx(), 0, 0, maxPixels, 0.05, 2)
	c.mode.OnSelect = func(i int) {
		if c.syncing {
			return
		}
		defer c.syncSlide()
		if c.spin.Empty() {
			c.step(i == sizePx)
			return
		}
		v := c.spin.Value()
		if i == sizePx {
			v = math.Round(v * c.basePx)
		} else if c.basePx > 0 {
			v /= c.basePx
		}
		c.configure(i == sizePx, v)
		c.commit()
	}
	c.spin.OnValueChanged = func(float64) {
		c.commit()
		c.syncSlide()
	}
	return c
}

// configure sets the spin for a mode (configure_spin_for_mode): whole
// pixels, or a scale in hundredths, then shows v.
func (c *sizeEditor) configure(px bool, v float64) {
	c.step(px)
	c.spin.SetValue(v)
}

// step sets the spin's step and digits for a mode: whole pixels, or a
// scale in hundredths.
func (c *sizeEditor) step(px bool) {
	if px {
		c.spin.SetStep(1)
		c.spin.SetDigits(0)
		return
	}
	c.spin.SetStep(0.05)
	c.spin.SetDigits(2)
}

func (c *sizeEditor) commit() {
	v := c.spin.Value()
	if c.mode.Selected() == sizePx {
		_ = c.slot.set(strconv.FormatFloat(math.Round(v), 'f', 0, 64) + "px")
		return
	}
	_ = c.slot.set(v)
}

// refresh shows the stored size: a number is a scale, a "<n>px" string
// pixels.
func (c *sizeEditor) refresh() {
	c.syncing = true
	defer func() {
		c.syncing = false
		c.syncSlide()
	}()
	switch v := c.slot.get().(type) {
	case nil:
		c.mode.show(sizeScale)
		c.configure(false, 1)
		c.spin.Clear()
	case float64:
		c.mode.show(sizeScale)
		c.configure(false, v)
	case int64:
		c.mode.show(sizeScale)
		c.configure(false, float64(v))
	case string:
		px, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
		if err != nil {
			return
		}
		c.mode.show(sizePx)
		c.configure(true, math.Round(px))
	}
}

// syncSlide spans the slider over the mode's range, in pixels or as a
// scale, its knob at the spin's value, which it leaves as it is.
func (c *sizeEditor) syncSlide() {
	if c.slide == nil {
		return
	}
	lo, hi := c.loPx, c.hiPx
	if c.mode.Selected() != sizePx {
		lo, hi = lo/c.basePx, hi/c.basePx
	}
	was := c.syncing
	c.syncing = true
	defer func() { c.syncing = was }()
	c.slide.SetRange(lo, hi)
	c.slide.Set(c.spin.Value())
}

// pixelLength is a pixel length ("400px") as the whole pixels a spin edits.
func pixelLength(s slot) slot {
	return slot{
		get: func() any {
			text, ok := s.get().(string)
			if !ok {
				return nil
			}
			px, err := strconv.ParseFloat(strings.TrimSuffix(text, "px"), 64)
			if err != nil {
				log.Printf("settings: %q is no pixel length", text)
				return nil
			}
			return int64(math.Round(px))
		},
		set: func(v any) error {
			px, ok := v.(int64)
			if !ok {
				return fmt.Errorf("settings: %v is no whole number of pixels", v)
			}
			return s.set(strconv.FormatInt(px, 10) + "px")
		},
		unset: s.unset,
	}
}

// sizeBase is size_with_base for a row: the base rem a scale of 1 is.
func sizeBase(rem float64) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
		return newSizeEditor(k, s, rem)
	})
}

// sizeSlider is sizeBase with a slider over loPx to hiPx pixels.
func sizeSlider(rem, loPx, hiPx float64) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
		return newSizeSliderEditor(k, s, rem, loPx, hiPx)
	})
}

const motionPageID = "motion"

var surfaceAnimationLink = pageLink{motionPageID, "settings-link-surface-animation"}

func animationSurfaces() []string {
	var surfaces []string
	for _, p := range config.LeafPaths() {
		if _, ok := config.Field(p + ".enter-duration"); ok && strings.HasPrefix(p, "animations.") {
			surfaces = append(surfaces, p)
		}
	}
	return surfaces
}

func surfaceAnimationTable() rowSpec {
	surfaces := animationSurfaces()
	rows := make([]gridRow, len(surfaces))
	for i, s := range surfaces {
		key, ok := config.I18nKey(s)
		if !ok {
			log.Panicf("settings: the animated surface %s has no label", s)
		}
		rows[i] = gridRow{key: key, cells: surfaceAnimationRows(s)}
	}
	return cellGridRow("settings-surface-animations", []string{"settings-animations-enter", "settings-animations-exit", "settings-animations-enter-duration", "settings-animations-exit-duration"}, rows...)
}

func surfaceAnimationRows(prefix string) []rowSpec {
	return []rowSpec{
		field(prefix+".enter", withKey("settings-animations-enter")),
		field(prefix+".exit", withKey("settings-animations-exit")),
		field(prefix+".enter-duration", withKey("settings-animations-enter-duration"), animationDuration),
		field(prefix+".exit-duration", withKey("settings-animations-exit-duration"), animationDuration),
	}
}

const (
	durationStepMS     = 10
	durationFallbackMS = 200
)

var animationDuration = withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
	if meta.Optional {
		return newOptionalNumber(k, s, meta.Min, meta.Max, durationStepMS, durationFallbackMS, 0)
	}
	return newSoftRange(k, s, meta, 0, animationSliderMS, durationStepMS, unitMilliseconds)
})

// animationSliderMS is the end of an animation duration's slider, past
// every duration that still reads as motion rather than a wait.
const animationSliderMS = 2000

// newOptionalSize is dropdown_size's size_control: the size editor over
// an optional size, its spin empty and reading "Inherited" while unset; a
// value typed or stepped (from a scale of 1) sets it, emptying it unsets.
func newOptionalSize(k *kit, s slot, baseRem float64) *sizeEditor {
	c := newSizeEditor(k, s, baseRem)
	c.spin.SetOptional(locale.T().Get("settings-inherited"))
	c.spin.OnCleared = s.unset
	c.refresh()
	return c
}

// dropdownSizeRows are dropdown_size_rows for the dropdown at path:
// its optional width and height.
func dropdownSizeRows(path string) []rowSpec {
	size := withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newOptionalSize(k, s, 1) })
	return []rowSpec{
		field(path+".width", withKey("settings-dropdown-width"), size),
		field(path+".height", withKey("settings-dropdown-height")),
	}
}
