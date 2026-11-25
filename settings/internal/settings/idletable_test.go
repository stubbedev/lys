package settings

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func idleTableOf(t *testing.T, k *kit) *idleTable {
	t.Helper()
	g, ok := fieldControl(k, k.store, idleTimesRow()).(*idleTable)
	if !ok {
		t.Fatal("the idle times are not a table")
	}
	return g
}

func (g *idleTable) cell(t *testing.T, stage idleStage, source idleSource) *idleCell {
	t.Helper()
	i := slices.IndexFunc(g.cells, func(c *idleCell) bool { return c.stage == stage && c.source == source })
	if i < 0 {
		t.Fatalf("the table has no %s cell %s", stage, source)
	}
	return g.cells[i]
}

func TestTheIdleTableHasACellPerStageAndSource(t *testing.T) {
	k := testKit(t, "")
	spec := idleTimesRow()
	paths := append([]string{spec.path}, spec.also...)
	if len(paths) != len(idleStages)*len(idleSources) {
		t.Fatalf("the table edits %v", paths)
	}
	g := idleTableOf(t, k)
	if len(g.cells) != len(paths) {
		t.Fatalf("%d cells for %d times", len(g.cells), len(paths))
	}
	labels := treetest.Labels(g)
	for _, want := range []string{idleOnAC.name(), idleOnBattery.name(), idleDim.name(), idleSuspend.name(), locale.T().Get("settings-duration-never")} {
		if !slices.Contains(labels, want) {
			t.Errorf("the table lacks %q: %q", want, labels)
		}
	}
}

func TestAnIdleChipWritesItsDuration(t *testing.T) {
	k := testKit(t, "")
	g := idleTableOf(t, k)
	lock := g.cell(t, idleLock, idleOnBattery).chips
	pressChip(lock.ChoiceChips, len(idleChoices)-1)
	if v := k.store.value("idle.lock.battery"); v != "never" {
		t.Errorf("choosing Never on battery stored %v", v)
	}
	pressChip(lock.ChoiceChips, 0)
	if v := k.store.value("idle.lock.battery"); v != "1m" {
		t.Errorf("choosing 1 minute on battery stored %v", v)
	}
	pressChip(g.cell(t, idleSuspend, idleOnAC).chips.ChoiceChips, 6)
	if v := k.store.value("idle.suspend.ac"); v != "1h" {
		t.Errorf("choosing 1 hour plugged in stored %v", v)
	}
	if v := k.store.value("idle.lock.ac"); v != "5m" {
		t.Errorf("another cell's chip changed the lock plugged in to %v", v)
	}
}

func TestAnIdleTimeOffTheChipsIsCustomMinutes(t *testing.T) {
	k := testKit(t, "idle:\n  lock:\n    ac: 90s\n")
	c := idleTableOf(t, k).cell(t, idleLock, idleOnAC).chips
	if c.active() != len(idleChoices) || !c.typed.Visible() {
		t.Fatalf("90s shows chip %d, custom shown %v", c.active(), c.typed.Visible())
	}
	if got := c.custom.Value(); got != 1.5 {
		t.Errorf("90s reads %v minutes", got)
	}
	c.custom.OnValueChanged(7)
	if v := k.store.value("idle.lock.ac"); v != "7m" {
		t.Errorf("typing 7 minutes stored %v", v)
	}
	if lo, _ := c.custom.Range(); lo <= 0 {
		t.Errorf("a custom time can be %v minutes, which is no idle time", lo)
	}
}

func TestAnIdleTimeIsRefusedOutsideMinutes(t *testing.T) {
	k := testKit(t, "")
	s := idleMinutes(k.store.slot("idle.dim.ac"))
	if err := s.set("5m"); err == nil {
		t.Error("a text idle time was taken as minutes")
	}
	if v := k.store.value("idle.dim.ac"); v != "4m" {
		t.Errorf("a refused idle time left %v", v)
	}
}

func TestAnIdleTimeFieldAnywhereIsChips(t *testing.T) {
	k := testKit(t, "")
	meta, ok := config.Field("idle.screen-off.battery")
	if !ok {
		t.Fatal("no idle.screen-off.battery field")
	}
	c, ok := semanticEditor(k, k.store.slot("idle.screen-off.battery"), "idle.screen-off.battery", meta)
	if _, chips := c.(*presetPicker[float64]); !ok || !chips {
		t.Fatalf("an idle time is edited by %T", c)
	}
}

func TestTheIdleTableWarnsWhenALaterStageComesFirst(t *testing.T) {
	k := testKit(t, "idle:\n  dim:\n    ac: 10m\n    battery: 1m\n  screen-off:\n    ac: 5m\n    battery: 5m\n")
	g := idleTableOf(t, k)
	ac, battery := g.cell(t, idleDim, idleOnAC), g.cell(t, idleDim, idleOnBattery)
	want := locale.T().Get("settings-idle-overtaken", i18n.Str("source", "ac"), i18n.Str("then", "screen-off"))
	if !ac.state.Visible() || ac.state.Text() != want || ac.state.kind != stateError {
		t.Fatalf("plugged in the dim cell warns %v (%v): %q", ac.state.Visible(), ac.state.kind, ac.state.Text())
	}
	if battery.state.Visible() {
		t.Error("on battery, dimming before the screens turn off is warned about")
	}
	if err := k.store.set("idle.dim.ac", "2m"); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	if ac.state.Visible() {
		t.Error("the warning stays once the order is fixed")
	}
	ac.showTimes(nil, errors.New("read idle.dim.ac: not a duration"))
	wantClosedFailure(t, "a cell over unreadable times", ac.state.stateParts, ac.state.Text(), locale.T().Get("settings-idle-times-unreadable"), "not a duration")
	g.refresh()
	if ac.state.Visible() {
		t.Errorf("read again, the cell still says %q", ac.state.Text())
	}
}

// The warning that a stage never comes widened its column to the
// sentence's width, so the battery column moved and its chips wrapped
// differently. A warning wraps inside the column the chips set, and
// moves no chip.
func TestAnIdleWarningMovesNoChip(t *testing.T) {
	quiet := laidOutPage(t, "", "idle", windowWidth)
	warned := laidOutPage(t, "idle:\n  screen-off:\n    ac: 1m\n    battery: 1m\n", "idle", windowWidth)
	cells := func(win *window) []*idleCell { return treetest.All[*idleCell](win.page) }
	q, w := cells(quiet), cells(warned)
	if len(q) == 0 || len(q) != len(w) {
		t.Fatalf("%d cells quiet, %d warned", len(q), len(w))
	}
	shown := 0
	for i := range q {
		if w[i].state.Visible() {
			shown++
		}
		if qb, wb := q[i].chips.Bounds(), w[i].chips.Bounds(); qb.X != wb.X || qb.W != wb.W || qb.H != wb.H {
			t.Errorf("%s %s: chips at %+v quiet, %+v beside a warning", q[i].stage, q[i].source, qb, wb)
		}
	}
	if shown != 2 {
		t.Fatalf("%d cells warn, want the two dim cells", shown)
	}
}

// One stage's chips followed the last stage's at the pitch of the chips
// inside a stage, so the stages ran together. The gap between stages is
// wider than the chips' own.
func TestIdleStagesArePartedWiderThanTheirChips(t *testing.T) {
	win := laidOutPage(t, "", "idle", windowWidth)
	cells := treetest.All[*idleCell](win.page)
	pitch := widget.Current().Spacing
	for i, c := range cells {
		if c.stage == idleKeyboard {
			continue
		}
		above := cells[i-len(idleSources)].Bounds()
		if gap := c.Bounds().Y - (above.Y + above.H); gap <= pitch {
			t.Errorf("%s %s sits %dpx under the stage above, the chips inside one %dpx apart", c.stage, c.source, gap, pitch)
		}
	}
}

func TestTheBatteryColumnLeavesWithoutABattery(t *testing.T) {
	desktop := laptop()
	desktop.state.battery.IsPresent = false
	for name, c := range map[string]struct {
		power func() (powerControl, error)
		shown bool
	}{
		"a laptop":          {func() (powerControl, error) { return laptop(), nil }, true},
		"a desktop":         {func() (powerControl, error) { return desktop, nil }, false},
		"no battery to ask": {func() (powerControl, error) { return nil, errors.New("no battery") }, true},
	} {
		k := testKit(t, "")
		asked := 0
		k.power = func() (powerControl, error) { asked++; return c.power() }
		g := idleTableOf(t, k)
		idleTimelineOf(t, k)
		if asked != 1 {
			t.Errorf("%s: the table and timeline asked for the battery %d times", name, asked)
		}
		battery := slices.ContainsFunc(g.cells, func(c *idleCell) bool { return c.source == idleOnBattery })
		labels := treetest.Labels(g)
		if battery != c.shown || slices.Contains(labels, idleOnBattery.name()) != c.shown {
			t.Errorf("%s: the battery column shown %v (heading %v), want %v", name, battery, slices.Contains(labels, idleOnBattery.name()), c.shown)
		}
		if columns := len(g.cells) / len(idleStages); len(g.Children()) != columns+len(idleStages)+len(g.cells) {
			t.Errorf("%s: %d widgets left in the table for %d cells", name, len(g.Children()), len(g.cells))
		}
	}
}

func TestTheBatteryTimeHasItsOwnSettingLink(t *testing.T) {
	if err := checkSetting(bothParts, "idle", "idle.suspend.battery"); err != nil {
		t.Error(err)
	}
}

func TestAStageReachedOnlyAfterALaterOneIsOvertaken(t *testing.T) {
	at := func(d time.Duration) idleTime { return idleTime{after: d} }
	never := idleTime{never: true}
	ordered := map[idleStage]idleTime{idleDim: at(time.Minute), idleLock: at(5 * time.Minute), idleScreenOff: at(3 * time.Minute), idleSuspend: at(time.Hour)}
	for _, stage := range idleStages {
		if then, ok := idleOvertaken(ordered, stage); ok {
			t.Errorf("in order, %s is overtaken by %s", stage, then)
		}
	}
	for name, c := range map[string]struct {
		times       map[idleStage]idleTime
		stage, then idleStage
	}{
		"dim with the screens off":  {map[idleStage]idleTime{idleDim: at(10 * time.Minute), idleScreenOff: at(10 * time.Minute), idleSuspend: never}, idleDim, idleScreenOff},
		"dim after suspend":         {map[idleStage]idleTime{idleDim: at(20 * time.Minute), idleScreenOff: never, idleSuspend: at(15 * time.Minute)}, idleDim, idleSuspend},
		"lock after suspend":        {map[idleStage]idleTime{idleLock: at(time.Hour), idleSuspend: at(15 * time.Minute)}, idleLock, idleSuspend},
		"screens off after suspend": {map[idleStage]idleTime{idleScreenOff: at(30 * time.Minute), idleSuspend: at(30 * time.Minute)}, idleScreenOff, idleSuspend},
	} {
		if then, ok := idleOvertaken(c.times, c.stage); !ok || then != c.then {
			t.Errorf("%s: overtaken by %q (%v), want %q", name, then, ok, c.then)
		}
	}
	if _, ok := idleOvertaken(map[idleStage]idleTime{idleDim: never, idleScreenOff: at(time.Minute), idleSuspend: never}, idleDim); ok {
		t.Error("a stage that never runs is overtaken")
	}
	if _, ok := idleOvertaken(map[idleStage]idleTime{idleLock: at(time.Hour), idleSuspend: never}, idleLock); ok {
		t.Error("a stage is overtaken by one that never runs")
	}
}

func TestIdleRowsFollowTheBuiltInIdleSwitch(t *testing.T) {
	k := testKit(t, "idle:\n  enable: false\n")
	p := buildPage(k, idlePage(nil))
	for _, r := range p.rows {
		if r.path == "idle.enable" || r.path == "lid_action" {
			if !widget.IsEnabled(r) {
				t.Errorf("%s, which does not hang on the built-in idle, is disabled", r.path)
			}
			continue
		}
		if widget.IsEnabled(r) {
			t.Errorf("%s is enabled while the built-in idle is off", r.path)
		}
	}
	if err := k.store.set("idle.enable", true); err != nil {
		t.Fatal(err)
	}
	p.showSections()
	for _, r := range p.rows {
		if !widget.IsEnabled(r) {
			t.Errorf("%s stays disabled with the built-in idle on", r.path)
		}
	}
}
