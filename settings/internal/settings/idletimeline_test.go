package settings

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"

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
		return tr.Get("settings-idle-timeline-mark", i18n.Str("stage", tr.Get("settings-section-idle-"+string(stage))), i18n.Str("after", durationWords(d)))
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
