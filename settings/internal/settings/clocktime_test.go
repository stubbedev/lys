package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestAClockTimeIsPickedAsHourAndMinute(t *testing.T) {
	k := testKit(t, "modules:\n  clock:\n    format: \"%H:%M\"\n")
	var held any = "22:00"
	e := newClockTimeEditor(k, memorySlot(&held, nil), "From", anyClockTime)
	if e.hour.Selected() != 22 || e.minutes[e.minute.Selected()] != 0 || e.problem.Visible() {
		t.Fatalf("22:00 shows hour %d minute %d", e.hour.Selected(), e.minute.Selected())
	}
	if e.hour.Items()[7] != "07" {
		t.Errorf("a 24-hour clock names hour 7 %q", e.hour.Items()[7])
	}
	e.hour.SetSelected(7)
	if held != "07:00" {
		t.Fatalf("picking hour 7 stored %v", held)
	}
	e.minute.SetSelected(slices.Index(e.minutes, 45))
	if held != "07:45" {
		t.Fatalf("picking minute 45 stored %v", held)
	}
}

func TestAClockTimeOffTheFiveMinuteStepsKeepsItsMinute(t *testing.T) {
	k := testKit(t, "")
	var held any = "06:07"
	e := newClockTimeEditor(k, memorySlot(&held, nil), "To", anyClockTime)
	if !slices.Contains(e.minutes, 7) || e.minutes[e.minute.Selected()] != 7 {
		t.Fatalf("06:07 offers %v with %d picked", e.minutes, e.minute.Selected())
	}
	e.hour.SetSelected(8)
	if held != "08:07" {
		t.Errorf("a new hour lost the minute: %v", held)
	}
	if got := minuteChoices(15); len(got) != 12 {
		t.Errorf("a minute on the steps is offered twice: %v", got)
	}
}

func TestATwelveHourClockNamesTheHoursSo(t *testing.T) {
	k := testKit(t, "modules:\n  clock:\n    format: \"%I:%M %p\"\n")
	var held any = "22:00"
	e := newClockTimeEditor(k, memorySlot(&held, nil), "From", anyClockTime)
	if got := e.hour.Items()[22]; got != "10 PM" {
		t.Errorf("a 12-hour clock names hour 22 %q", got)
	}
}

func TestAClockTimeThatIsNotOneSaysSo(t *testing.T) {
	k := testKit(t, "")
	var held any = "7 o'clock"
	e := newClockTimeEditor(k, memorySlot(&held, nil), "From", anyClockTime)
	if !e.problem.Visible() || widget.IsEnabled(e.hour) || widget.IsEnabled(e.minute) {
		t.Fatalf("a malformed time shows problem %v, pickers enabled %v %v", e.problem.Visible(), widget.IsEnabled(e.hour), widget.IsEnabled(e.minute))
	}
	e.hour.SetSelected(3)
	if held != "7 o'clock" {
		t.Errorf("an hour picked over a malformed time stored %v", held)
	}
	held = "09:00"
	r := newClockTimeEditor(k, memorySlot(&held, func(any) bool { return true }), "From", anyClockTime)
	r.hour.SetSelected(10)
	if !r.problem.Visible() || r.problem.Text() != "refused" || r.hour.Selected() != 9 {
		t.Errorf("a refused time shows %q (%v), hour %d", r.problem.Text(), r.problem.Visible(), r.hour.Selected())
	}
}

func TestAScheduleWindowSaysWhenItRunsIntoTheNextDay(t *testing.T) {
	tr := locale.T()
	overnight := scheduleTitle(map[string]any{"from": "22:00", "to": "07:00"}, 0)
	if want := tr.Get("settings-schedule-range", i18n.Str("from", "22:00"), i18n.Str("to", "07:00"), i18n.Str("overnight", "true")); overnight != want {
		t.Errorf("22:00-07:00 reads %q, want %q", overnight, want)
	}
	day := scheduleTitle(map[string]any{"from": "09:00", "to": "17:00"}, 0)
	if want := tr.Get("settings-schedule-range", i18n.Str("from", "09:00"), i18n.Str("to", "17:00"), i18n.Str("overnight", "false")); day != want || day == overnight {
		t.Errorf("09:00-17:00 reads %q, want %q", day, want)
	}
}

func TestTheScheduleTimesArePickers(t *testing.T) {
	k := testKit(t, "modules:\n  notifications:\n    dnd:\n      schedule:\n        - {from: \"22:00\", to: \"07:00\"}\n")
	c := fieldControl(k, k.store, pageRow(t, "notifications", "modules.notifications.dnd.schedule"))
	editors := treetest.All[*clockTimeEditor](c)
	if len(editors) != 2 {
		t.Fatalf("the schedule window has %d time pickers", len(editors))
	}
	from, to := editors[0], editors[1]
	if slices.Contains(to.hours, 22) || slices.Contains(from.hours, 7) || len(to.hours) != hoursPerDay-1 {
		t.Fatalf("the end offers %v, the start %v: a window that ends as it starts is on offer", to.hours, from.hours)
	}
	to.hour.SetSelected(slices.Index(to.hours, 6))
	items, _ := k.store.value("modules.notifications.dnd.schedule").([]any)
	if len(items) != 1 || items[0].(map[string]any)["to"] != "06:00" {
		t.Fatalf("picking 6 for the end stored %v", items)
	}
	from.refresh()
	if !slices.Contains(from.hours, 7) || slices.Contains(from.hours, 6) {
		t.Errorf("after the end moved to 06:00 the start offers %v", from.hours)
	}
}

func TestAClockTimeAvoidingAnotherOffersEverythingElse(t *testing.T) {
	k := testKit(t, "")
	var held, other any = "07:30", "07:00"
	e := newClockTimeEditor(k, memorySlot(&held, nil), "To", clockTimeOf(memorySlot(&other, nil)))
	if slices.Contains(e.minutes, 0) || !slices.Contains(e.minutes, 5) || len(e.hours) != hoursPerDay {
		t.Fatalf("07:30 avoiding 07:00 offers minutes %v, hours %v", e.minutes, e.hours)
	}
	free := newClockTimeEditor(k, memorySlot(&held, nil), "To", anyClockTime)
	if !slices.Contains(free.minutes, 0) {
		t.Errorf("a time avoiding nothing leaves out minute 0: %v", free.minutes)
	}
}
