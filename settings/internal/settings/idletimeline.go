package settings

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	timelineH     = 16
	timelineMinW  = 160
	timelineTrack = 4
	timelineDot   = 12
)

// hairline is the width of a card's border and of its rows' own.
const hairline = 1

// controlEdgeInset is how far in from a card's outer edge its rows'
// controls end: the card's border, the row's border and its padding.
func controlEdgeInset() int { return 2*hairline + cardInsets().Right }

type idleMark struct {
	stage idleStage
	after time.Duration
}

func idleSequence(times map[idleStage]idleTime) []idleMark {
	marks := make([]idleMark, 0, len(idleStages))
	for _, stage := range idleStages {
		if t := times[stage]; !t.never {
			marks = append(marks, idleMark{stage, t.after})
		}
	}
	slices.SortStableFunc(marks, func(a, b idleMark) int { return cmp.Compare(a.after, b.after) })
	return marks
}

// idleSequenceText reads marks in order, each mark's words held
// together so a line wraps only between marks.
func idleSequenceText(marks []idleMark) string {
	t := locale.T()
	if len(marks) == 0 {
		return t.Get("settings-idle-timeline-never")
	}
	parts := make([]string, len(marks))
	for i, m := range marks {
		mark := t.Get("settings-idle-timeline-mark", i18n.Str("stage", m.stage.name()), i18n.Str("after", durationWords(m.after)))
		parts[i] = strings.ReplaceAll(mark, " ", "\u00a0")
	}
	return strings.Join(parts, " → ")
}

// idleSpan is the time a timeline's track spans: the latest of marks'
// last steps, at least a minute.
func idleSpan(marks ...[]idleMark) time.Duration {
	span := time.Minute
	for _, m := range marks {
		if len(m) > 0 {
			span = max(span, m[len(m)-1].after)
		}
	}
	return span
}

func stageColor(stage idleStage) render.Color {
	return widget.Current().ChartColor(slices.Index(idleStages, stage))
}

type timelineBar struct {
	widget.Base
	marks []idleMark
	span  time.Duration
}

func (b *timelineBar) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(timelineMinW)
	}
	return widget.FixedSizes(timelineH)
}

func (b *timelineBar) HitTest(widget.Point) widget.Widget { return nil }

func (b *timelineBar) Paint(cv *render.Canvas) {
	th := widget.Current()
	r := b.Bounds()
	inner := render.Rect{X: r.X + timelineDot/2, Y: r.Y, W: r.W - timelineDot, H: r.H}
	cv.RoundedRect(render.Rect{X: inner.X, Y: r.Y + (r.H-timelineTrack)/2, W: inner.W, H: timelineTrack}, timelineTrack/2, th.Border)
	if len(b.marks) == 0 {
		return
	}
	for _, m := range b.marks {
		x := inner.X + b.markX(m, inner.W)
		cv.RoundedRect(render.Rect{X: x - timelineDot/2, Y: r.Y + (r.H-timelineDot)/2, W: timelineDot, H: timelineDot}, timelineDot/2, stageColor(m.stage))
	}
}

// markX is how far along a track w wide m sits, on the bar's span.
func (b *timelineBar) markX(m idleMark, w int) int {
	return int(float64(w) * float64(m.after) / float64(max(b.span, m.after, time.Minute)))
}

type idleTimelineLine struct {
	*widget.Box
	source idleSource
	bar    *timelineBar
	text   *inlineState
}

// show draws times on the line, or says they could not be read, err
// why.
func (line *idleTimelineLine) show(times map[idleStage]idleTime, err error) {
	line.bar.marks = nil
	if err == nil {
		line.bar.marks = idleSequence(times)
	}
	line.bar.Invalidate()
	if err != nil {
		line.text.Note(locale.T().Get("settings-idle-times-unreadable"), err)
		return
	}
	line.text.Note(idleSequenceText(line.bar.marks), nil)
}

type idleTimeline struct {
	*widget.Box
	k       *kit
	lines   []*idleTimelineLine
	off     *widget.Label
	battery bool
}

// idleTimelinePreview is a line per source, its icon beside its track
// over the steps it reads, the caption starting where the track does.
// Every track spans the same time, so one step sits at one place on
// each, and ends on the page's control edge.
func idleTimelinePreview(k *kit) widget.Widget {
	t := locale.T()
	tl := &idleTimeline{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, battery: true}
	tl.SetPadding(render.Insets{Right: controlEdgeInset()})
	tl.SetAccessibleName(t.Get("settings-idle-timeline"))
	tl.off = k.note(t.Get("settings-idle-timeline-off"), "settings-preview-state")
	tl.Append(tl.off, false)
	for _, source := range idleSources {
		line := &idleTimelineLine{Box: gapBox(widget.Row), source: source, bar: &timelineBar{Base: widget.NewBase()}}
		line.text = k.inlineState()
		caption := widget.NewBox(widget.Column, 0, 0)
		caption.SetPadding(render.Insets{Left: timelineDot / 2})
		caption.Append(line.text, false)
		track := widget.NewBox(widget.Column, 0, 0)
		track.Append(line.bar, false)
		track.Append(caption, false)
		ic := k.icon(source.icon())
		ic.SetAccessibleName(source.name())
		line.AppendAligned(ic, false, widget.AlignStart)
		line.Append(track, true)
		tl.lines = append(tl.lines, line)
		tl.Append(line, false)
	}
	tl.configChanged()
	k.whenBatteryKnown(func(present bool) {
		tl.battery = present
		tl.configChanged()
	})
	return tl
}

func (tl *idleTimeline) configChanged() {
	on := tl.k.store.svc.Config().Idle.Enable
	tl.off.SetVisible(!on)
	shown := make([][]idleMark, 0, len(tl.lines))
	for _, line := range tl.lines {
		line.SetVisible(on && (line.source == idleOnAC || tl.battery))
		line.show(idleTimesOn(tl.k.store, line.source))
		if line.Visible() {
			shown = append(shown, line.bar.marks)
		}
	}
	span := idleSpan(shown...)
	for _, line := range tl.lines {
		line.bar.span = span
	}
}
