package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheIdleSequenceIsTheStagesInTheOrderTheyHappen(t *testing.T) {
	at := func(d time.Duration) idleTime { return idleTime{after: d} }
	got := idleSequence(map[idleStage]idleTime{idleKeyboard: {never: true}, idleDim: at(10 * time.Minute), idleLock: at(5 * time.Minute), idleScreenOff: at(5 * time.Minute), idleSuspend: {never: true}})
	want := []idleMark{{idleLock, 5 * time.Minute}, {idleScreenOff, 5 * time.Minute}, {idleDim, 10 * time.Minute}}
	if !slices.Equal(got, want) {
		t.Fatalf("sequence %v, want %v", got, want)
	}
	never := idleTime{never: true}
	if got := idleSequence(map[idleStage]idleTime{idleKeyboard: never, idleDim: never, idleLock: never, idleScreenOff: never, idleSuspend: never}); len(got) != 0 {
		t.Fatalf("all never gives %v", got)
	}
	if got := idleSequenceText(nil); got != locale.T().Get("settings-idle-timeline-never") {
		t.Errorf("an empty sequence reads %q", got)
	}
}

func idleTimelineOf(t *testing.T, k *kit) *idleTimeline {
	t.Helper()
	tl, ok := idleTimelinePreview(k).(*idleTimeline)
	if !ok {
		t.Fatal("the idle preview is not the timeline")
	}
	return tl
}

func TestTheIdleTimelineFollowsTheTimesOfEachSource(t *testing.T) {
	k := testKit(t, "idle:\n  dim: {ac: 2m, battery: 1m}\n  lock: {ac: 5m, battery: 2m}\n  screen-off: {ac: 10m, battery: 3m}\n  suspend: {ac: never, battery: 15m}\n")
	tl := idleTimelineOf(t, k)
	tr := locale.T()
	mark := func(stage idleStage, d time.Duration) string {
		return strings.ReplaceAll(tr.Get("settings-idle-timeline-mark", i18n.Str("stage", tr.Get("settings-section-idle-"+string(stage))), i18n.Str("after", durationWords(d))), " ", "\u00a0")
	}
	ac, battery := tl.lines[0], tl.lines[1]
	if want := strings.Join([]string{mark(idleDim, 2*time.Minute), mark(idleLock, 5*time.Minute), mark(idleScreenOff, 10*time.Minute)}, " → "); ac.text.Text() != want {
		t.Fatalf("plugged in reads %q, want %q", ac.text.Text(), want)
	}
	if len(battery.bar.marks) != 5 || battery.bar.marks[0] != (idleMark{idleKeyboard, time.Minute}) || battery.bar.marks[4] != (idleMark{idleSuspend, 15 * time.Minute}) || !battery.Visible() || tl.off.Visible() {
		t.Fatalf("on battery marks %v, shown %v, off note shown %v", battery.bar.marks, battery.Visible(), tl.off.Visible())
	}
	if err := k.store.set("idle.lock.ac", "1m"); err != nil {
		t.Fatal(err)
	}
	tl.configChanged()
	if ac.bar.marks[0] != (idleMark{idleLock, time.Minute}) {
		t.Fatalf("after moving the lock to 1 minute the sequence starts %v", ac.bar.marks[0])
	}
	ac.show(nil, errors.New("read idle.dim.ac: not a duration"))
	wantClosedFailure(t, "a line over unreadable times", ac.text.stateParts, ac.text.Text(), tr.Get("settings-idle-times-unreadable"), "not a duration")
	if ac.bar.marks != nil {
		t.Errorf("a line over unreadable times marks %v", ac.bar.marks)
	}
}

func TestTheIdleTimelineSaysWhenAnotherProgramHandlesIdle(t *testing.T) {
	k := testKit(t, "idle:\n  enable: false\n")
	tl := idleTimelineOf(t, k)
	if !tl.off.Visible() || tl.lines[0].Visible() || tl.lines[1].Visible() {
		t.Fatalf("with idle off the note shows %v, lines %v %v", tl.off.Visible(), tl.lines[0].Visible(), tl.lines[1].Visible())
	}
}

func TestWithoutABatteryTheTimelineShowsPluggedInAlone(t *testing.T) {
	k := testKit(t, "")
	k.battery = &batteryPresence{settled: true, present: false}
	tl := idleTimelineOf(t, k)
	if !tl.lines[0].Visible() || tl.lines[1].Visible() {
		t.Fatalf("without a battery: plugged in shown %v, battery shown %v", tl.lines[0].Visible(), tl.lines[1].Visible())
	}
}

// The When Idle timeline stood 8px in from its section's header, on no
// column of the page, the padding of a framed preview kept on one drawn
// straight onto the page. It starts on the header's column.
func TestTheIdleTimelineStartsOnItsSectionsColumn(t *testing.T) {
	win := laidOutPage(t, "", "idle", sweepW)
	tl := treetest.First[*idleTimeline](t, win.page)
	header := treetest.LabelReading(t, win.page, locale.T().Get("settings-section-idle-when")).Bounds()
	if x := tl.lines[0].Bounds().X; x != header.X {
		t.Errorf("the timeline starts at x=%d, its section's header at %d", x, header.X)
	}
}

// A caption broke inside a step, leaving "15 minutes" alone on its
// line. A caption breaks only between its steps.
func TestAnIdleCaptionBreaksOnlyBetweenSteps(t *testing.T) {
	marks := []idleMark{{idleKeyboard, time.Minute}, {idleDim, 2 * time.Minute}, {idleSuspend, 15 * time.Minute}}
	pieces := strings.Split(idleSequenceText(marks), " ")
	if len(pieces) != 2*len(marks)-1 {
		t.Fatalf("the caption breaks into %q, want %d steps between arrows", pieces, len(marks))
	}
	for i, p := range pieces {
		if (i%2 == 1) != (p == "→") {
			t.Errorf("piece %d of the caption is %q", i, p)
		}
	}
}

// The two timelines spanned their own last step each, so Dim after 4
// minutes sat right of Screens Off after 5; their captions started under
// the icon, left of the tracks; the tracks ran past the control edge.
// Both tracks span the same time, a caption starts where its track
// does, and the tracks end on the edge the page's controls end on.
func TestTheIdleTimelinesShareOneScaleAndTheControlColumn(t *testing.T) {
	win := laidOutPage(t, "idle:\n  dim: {ac: 4m, battery: 2m}\n  screen-off: {ac: 10m, battery: 5m}\n  suspend: {ac: never, battery: 15m}\n", "idle", sweepW)
	tl := treetest.First[*idleTimeline](t, win.page)
	ac, battery := tl.lines[0], tl.lines[1]
	if ac.bar.span != battery.bar.span || ac.bar.span != 15*time.Minute {
		t.Fatalf("the tracks span %v and %v, want both the latest step's 15m", ac.bar.span, battery.bar.span)
	}
	dim := ac.bar.markX(idleMark{idleDim, 4 * time.Minute}, ac.bar.Bounds().W)
	off := battery.bar.markX(idleMark{idleScreenOff, 5 * time.Minute}, battery.bar.Bounds().W)
	if dim >= off {
		t.Errorf("Dim after 4 minutes sits at %d, Screens Off after 5 at %d", dim, off)
	}
	edge := -1
	for _, r := range win.page.rows {
		if shown(r) && shown(r.ctl) && r.ControlFit() != widget.ItemControlWide {
			b := r.ctl.(widget.Boundser).Bounds()
			edge = b.X + b.W
		}
	}
	if edge < 0 {
		t.Fatal("the idle page shows no control")
	}
	for _, line := range tl.lines {
		bar, text := line.bar.Bounds(), line.text.Bounds()
		if track := bar.X + timelineDot/2; text.X != track {
			t.Errorf("%s: the caption starts at x=%d, the track at %d", line.source, text.X, track)
		}
		if end := bar.X + bar.W; end != edge {
			t.Errorf("%s: the track ends at x=%d, the controls at %d", line.source, end, edge)
		}
	}
}
