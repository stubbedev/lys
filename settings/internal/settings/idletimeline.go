package settings

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	timelineH     = 16
	timelineMinW  = 160
	timelineTrack = 4
	timelineDot   = 12
)

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

func idleSequenceText(marks []idleMark) string {
	t := locale.T()
	if len(marks) == 0 {
		return t.Get("settings-idle-timeline-never")
	}
	parts := make([]string, len(marks))
	for i, m := range marks {
		parts[i] = t.Get("settings-idle-timeline-mark", i18n.Str("stage", m.stage.name()), i18n.Str("after", durationWords(m.after)))
	}
	return strings.Join(parts, " → ")
}

func stageColor(stage idleStage) render.Color {
	return widget.Current().ChartColor(slices.Index(idleStages, stage))
}

type timelineBar struct {
	widget.Base
	marks []idleMark
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
	span := max(b.marks[len(b.marks)-1].after, time.Minute)
	for _, m := range b.marks {
		x := inner.X + int(float64(inner.W)*float64(m.after)/float64(span))
		cv.RoundedRect(render.Rect{X: x - timelineDot/2, Y: r.Y + (r.H-timelineDot)/2, W: timelineDot, H: timelineDot}, timelineDot/2, stageColor(m.stage))
	}
}

type idleTimelineLine struct {
	*widget.Box
	source idleSource
	bar    *timelineBar
	text   *widget.Label
}

type idleTimeline struct {
	*widget.Box
	k       *kit
	lines   []*idleTimelineLine
	off     *widget.Label
	battery bool
}

func idleTimelinePreview(k *kit) widget.Widget {
	t := locale.T()
	tl := &idleTimeline{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, widget.Current().Padding), k: k, battery: true}
	tl.SetAccessibleName(t.Get("settings-idle-timeline"))
	tl.off = wrapped(k.label(t.Get("settings-idle-timeline-off"), "settings-preview-state", css.Caption, css.DimLabel))
	tl.Append(tl.off, false)
	for _, source := range idleSources {
		line := &idleTimelineLine{Box: widget.NewBox(widget.Column, 0, 0), source: source, bar: &timelineBar{Base: widget.NewBase()}}
		line.text = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
		head := gapBox(widget.Row)
		ic := k.icon(source.icon())
		ic.SetAccessibleName(source.name())
		head.AppendAligned(ic, false, widget.AlignCenter)
		head.Append(line.bar, true)
		line.Append(head, false)
		line.Append(line.text, false)
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
	for _, line := range tl.lines {
		line.SetVisible(on && (line.source == idleOnAC || tl.battery))
		times, err := idleTimesOn(tl.k.store, line.source)
		if err != nil {
			line.bar.marks = nil
			line.text.SetText(err.Error())
			continue
		}
		line.bar.marks = idleSequence(times)
		line.bar.Invalidate()
		line.text.SetText(idleSequenceText(line.bar.marks))
	}
}
