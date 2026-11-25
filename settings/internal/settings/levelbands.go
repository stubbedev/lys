package settings

import (
	"math"
	"strconv"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	levelIconPx  = 18
	levelRowH    = levelIconPx + 6
	levelGap     = 6
	levelPercent = 100.0
)

// levelScale names the keys a module draws its level icons from: the
// list dividing 0–100 % evenly, the icon shown muted instead of any
// level, and the floor the level never goes below; an empty path is a
// key the module does not have.
type levelScale struct{ icons, muted, floor string }

// levelBands is a level scale as the threshold bar draws it: each
// level icon above its share of 0–100 %, the muted icon at the left
// end and the region under the floor greyed.
type levelBands struct {
	k     *kit
	scale levelScale
	icons []*widget.Icon
	muted *widget.Icon
	floor float64
}

func levelThresholdList(scale levelScale) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newThresholdEditor(k, s, &levelBands{k: k, scale: scale})
		}
	}
}

func (l *levelBands) refresh() {
	names, _ := l.k.store.value(l.scale.icons).([]any)
	l.icons = l.icons[:0]
	for i, v := range names {
		name, _ := v.(string)
		icon := widget.NewThemeIcon(name, levelIconPx)
		lo, hi := levelBand(i, len(names))
		icon.SetTooltip(locale.T().Get("settings-level-band", i18n.Int("from", int64(math.Round(lo))), i18n.Int("to", int64(math.Round(hi)))))
		l.icons = append(l.icons, icon)
	}
	l.muted = nil
	if l.scale.muted != "" {
		name, _ := l.k.store.value(l.scale.muted).(string)
		l.muted = widget.NewThemeIcon(name, levelIconPx)
		l.muted.SetTooltip(locale.T().Get("settings-level-muted"))
	}
	l.floor = 0
	if l.scale.floor != "" {
		l.floor, _ = numeric(l.k.store.value(l.scale.floor))
	}
}

func levelBand(i, n int) (lo, hi float64) {
	return levelPercent * float64(i) / float64(n), levelPercent * float64(i+1) / float64(n)
}

func (l *levelBands) widgets() []widget.Widget {
	out := make([]widget.Widget, 0, len(l.icons)+1)
	if l.muted != nil {
		out = append(out, l.muted)
	}
	for _, icon := range l.icons {
		out = append(out, icon)
	}
	return out
}

func (l *levelBands) leftPad() int {
	if l.muted == nil {
		return 0
	}
	return levelIconPx + levelGap
}

func (l *levelBands) arrange(b *thresholdBar) {
	t := b.track()
	top := b.Bounds().Y + (levelRowH-levelIconPx)/2
	if l.muted != nil {
		l.muted.Arrange(render.Rect{X: t.X - l.leftPad() - thresholdStopR, Y: t.Y + (t.H-levelIconPx)/2, W: levelIconPx, H: levelIconPx})
	}
	for i, icon := range l.icons {
		lo, hi := levelBand(i, len(l.icons))
		mid := (b.xOf(lo) + b.xOf(hi)) / 2
		icon.Arrange(render.Rect{X: mid - levelIconPx/2, Y: top, W: levelIconPx, H: levelIconPx})
	}
}

// levelMark is a value the bands mark under the track, and its text.
type levelMark struct {
	value float64
	text  string
}

// marks are the values the bands mark: the floor, then each boundary
// between two levels.
func (l *levelBands) marks() []levelMark {
	var out []levelMark
	if l.floor > 0 {
		out = append(out, levelMark{l.floor, locale.T().Get("settings-level-floor-mark", i18n.Str("n", strconv.FormatFloat(l.floor, 'f', -1, 64)))})
	}
	for i := 1; i < len(l.icons); i++ {
		lo, _ := levelBand(i, len(l.icons))
		out = append(out, levelMark{lo, strconv.FormatFloat(math.Round(lo), 'f', -1, 64)})
	}
	return out
}

func (l *levelBands) paint(b *thresholdBar, cv *render.Canvas) {
	th := widget.Current()
	t := b.track()
	if l.floor > 0 {
		x := b.xOf(min(l.floor, levelPercent))
		cv.RoundedRect(render.Rect{X: t.X, Y: t.Y, W: max(1, x-t.X), H: t.H}, t.H/2, th.DisabledSurface())
	}
	for i := 1; i < len(l.icons); i++ {
		lo, _ := levelBand(i, len(l.icons))
		cv.FillRect(render.Rect{X: b.xOf(lo), Y: t.Y - levelGap/2, W: 1, H: t.H + levelGap}, th.Text)
	}
	for _, w := range l.widgets() {
		w.Paint(cv)
	}
}
