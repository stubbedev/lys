package settings

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/power/pause"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

// undoToasts records every toast k's removals offer, in order.
func undoToasts(k *kit) *[]*widget.Toast {
	var shown []*widget.Toast
	next := k.toast
	k.toast = func(text string, life time.Duration) (*widget.Toast, error) {
		toast, err := next(text, life)
		if err == nil {
			shown = append(shown, toast)
		}
		return toast, err
	}
	return &shown
}

// lastUndo is the latest toast offered, failing without one.
func lastUndo(t *testing.T, shown *[]*widget.Toast) *widget.Toast {
	t.Helper()
	if len(*shown) == 0 {
		t.Fatal("the removal offered no undo")
	}
	return (*shown)[len(*shown)-1]
}

// pressUndo presses the toast's Undo as its click does: the action runs,
// then the toast goes.
func pressUndo(t *testing.T, toast *widget.Toast) {
	t.Helper()
	if toast.Action() != locale.T().Get("settings-list-undo") || toast.OnAction == nil {
		t.Fatalf("the toast %q offers %q, want Undo", toast.Text(), toast.Action())
	}
	toast.OnAction()
	toast.Dismiss()
}

// A removal's toast says what went and its Undo restores it once; the
// toast's own life is undoLife.
func TestAnUndoRestoresTheRemovalWhileItShows(t *testing.T) {
	k := testKit(t, "")
	var life time.Duration
	next := k.toast
	k.toast = func(text string, d time.Duration) (*widget.Toast, error) { life = d; return next(text, d) }
	shown := undoToasts(k)
	restored := 0
	k.offerUndo("firefox", func() { restored++ })
	toast := lastUndo(t, shown)
	if toast.Text() != locale.T().Get("settings-list-removed") || toast.Description() != "firefox" || life != undoLife {
		t.Fatalf("the toast reads %q over %q for %v", toast.Text(), toast.Description(), life)
	}
	pressUndo(t, toast)
	toast.OnAction()
	if restored != 1 || !toast.Closed() {
		t.Fatalf("undo restored %d times, the toast closed %v", restored, toast.Closed())
	}
}

// A toast that goes without its Undo leaves the removal as it is.
func TestAnExpiredUndoLeavesTheRemoval(t *testing.T) {
	k := testKit(t, "")
	shown := undoToasts(k)
	k.offerUndo("firefox", func() { t.Fatal("an expired undo restored") })
	toast := lastUndo(t, shown)
	toast.Dismiss()
	if !toast.Closed() {
		t.Fatal("the toast outlived its expiry")
	}
	k.offerUndo("next", func() {})
}

// A second removal while an undo shows takes the toast's place: the
// first loses its Undo and goes, its removal final, and the second's
// Undo restores the second alone.
func TestALaterRemovalMakesTheEarlierFinal(t *testing.T) {
	k := testKit(t, "")
	shown := undoToasts(k)
	var restored []string
	k.offerUndo("a", func() { restored = append(restored, "a") })
	first := lastUndo(t, shown)
	stale := first.OnAction
	k.offerUndo("b", func() { restored = append(restored, "b") })
	second := lastUndo(t, shown)
	if first.Action() != "" || !first.Closed() {
		t.Fatalf("the earlier toast still offers %q, closed %v", first.Action(), first.Closed())
	}
	stale()
	pressUndo(t, second)
	if !slices.Equal(restored, []string{"b"}) {
		t.Fatalf("restored %q, want the later removal alone", restored)
	}
}

// With no window to show a toast on, the removal stands and nothing is
// restored.
func TestWithoutAToastTheRemovalStands(t *testing.T) {
	k := testKit(t, "")
	k.toast = func(string, time.Duration) (*widget.Toast, error) { return nil, errors.New("no window") }
	k.offerUndo("a", func() { t.Fatal("restored without an undo") })
	if k.undoing != nil {
		t.Fatal("an undo is held with no toast")
	}
}

func memSlot(v *any) slot {
	return slot{get: func() any { return *v }, set: func(nv any) error { *v = nv; return nil }, unset: func() { *v = nil }}
}

// A string list's removed word comes back at its place on Undo, rows and
// config alike; expired, it stays gone; two removals keep the first.
func TestAStringListRemovalIsUndoneInPlace(t *testing.T) {
	k := testKit(t, "")
	shown := undoToasts(k)
	var stored any = []any{"a", "b", "c", "d"}
	c := newListEditor(k, memSlot(&stored), stringListSpec())
	c.remove(c.items[1])
	if got := stored.([]any); !slices.Equal(got, []any{"a", "c", "d"}) {
		t.Fatalf("removing b stored %v", got)
	}
	if d := lastUndo(t, shown).Description(); d != "b" {
		t.Errorf("the undo names %q, want b", d)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := stored.([]any); !slices.Equal(got, []any{"a", "b", "c", "d"}) || !slices.Equal(c.values(), got) {
		t.Fatalf("undo stored %v showing %v, want b back at its place", got, c.values())
	}

	c.remove(c.items[0])
	lastUndo(t, shown).Dismiss()
	if got := stored.([]any); !slices.Equal(got, []any{"b", "c", "d"}) || len(c.items) != 3 {
		t.Fatalf("after the undo expired: stored %v, %d rows", got, len(c.items))
	}

	c.remove(c.items[0])
	first := lastUndo(t, shown)
	c.remove(c.items[1])
	if first.Action() != "" {
		t.Fatal("the first removal's undo survived the second")
	}
	pressUndo(t, lastUndo(t, shown))
	if got := stored.([]any); !slices.Equal(got, []any{"c", "d"}) || !slices.Equal(c.values(), got) {
		t.Fatalf("undoing the second of two removals stored %v showing %v, want d back and b gone", got, c.values())
	}
}

// A card comes back on Undo with its exact item at its place.
func TestACardRemovalIsUndoneWithItsExactItem(t *testing.T) {
	k := testKit(t, "")
	shown := undoToasts(k)
	items := []any{
		map[string]any{"above": int64(10)},
		map[string]any{"above": int64(20), "below": int64(40), "icon-color": "red"},
		map[string]any{"above": int64(60)},
	}
	var stored any = slices.Clone(items)
	c := newCardList(k, memSlot(&stored), thresholdCards)
	title := c.cards[1].title.Text()
	c.removeItem(1)
	if got := stored.([]any); len(got) != 2 || len(c.cards) != 2 {
		t.Fatalf("removing the second card stored %v with %d cards", got, len(c.cards))
	}
	if d := lastUndo(t, shown).Description(); d != title {
		t.Errorf("the undo names %q, want the card's title %q", d, title)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := stored.([]any); !reflect.DeepEqual(got, items) || len(c.cards) != 3 {
		t.Fatalf("undo stored %v with %d cards, want %v", got, len(c.cards), items)
	}
	c.removeItem(0)
	lastUndo(t, shown).Dismiss()
	if got := stored.([]any); !reflect.DeepEqual(got, items[1:]) {
		t.Errorf("after the undo expired stored %v", got)
	}
}

// An app list's removed app comes back on Undo at its row and in the
// config; two removals keep the first gone.
func TestAnAppListRemovalIsUndoneInPlace(t *testing.T) {
	stored := []string{"firefox", "org.gnome.Calculator", "org.gnome.Nautilus"}
	l, _ := testAppList(t, &stored)
	shown := undoToasts(l.k)
	l.remove("org.gnome.Calculator")
	if d := lastUndo(t, shown).Description(); d != "Calculator" {
		t.Errorf("the undo names %q, want the app's name", d)
	}
	pressUndo(t, lastUndo(t, shown))
	want := []string{"firefox", "org.gnome.Calculator", "org.gnome.Nautilus"}
	if !slices.Equal(stored, want) || !slices.Equal(l.rows, want) {
		t.Fatalf("undo stored %q showing %q", stored, l.rows)
	}
	l.remove("firefox")
	l.remove("org.gnome.Nautilus")
	pressUndo(t, lastUndo(t, shown))
	want = []string{"org.gnome.Calculator", "org.gnome.Nautilus"}
	if !slices.Equal(stored, want) || !slices.Equal(l.rows, want) {
		t.Fatalf("undoing the second of two removals stored %q showing %q", stored, l.rows)
	}
}

// A notification app's Undo puts it back on every rule's list it was on,
// at its place, and on no other.
func TestANotificationAppIsRestoredToEachRuleItWasOn(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  notifications:\n    blocklist: [a, firefox, b]\n    sound-mute-apps: [firefox]\n")
	shown := undoToasts(k)
	c := fieldControl(k, k.store, notifyAppsRow()).(*notifyApps)
	c.remove("firefox")
	if got := storedNames(t, k, notifyBlockPath); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("removing firefox left the blocklist %v", got)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := storedNames(t, k, notifyBlockPath); !slices.Equal(got, []string{"a", "firefox", "b"}) {
		t.Errorf("undo left the blocklist %v", got)
	}
	if got := storedNames(t, k, notifyMutePath); !slices.Equal(got, []string{"firefox"}) {
		t.Errorf("undo left the silenced %v", got)
	}
	if got := storedNames(t, k, notifyFocusPath); slices.Contains(got, "firefox") {
		t.Errorf("undo put firefox on a rule it was not on: %v", got)
	}
}

// A napping app's Undo puts it back at its level.
func TestANappingAppIsRestoredAtItsLevel(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "")
	shown := undoToasts(k)
	level := string(config.NapLevels()[len(config.NapLevels())-1])
	if err := k.store.set(appNapPath, map[string]any{"firefox": level}); err != nil {
		t.Fatal(err)
	}
	c := fieldControl(k, k.store, appNapRow()).(*napApps)
	c.remove("firefox")
	if got := storedNapLevels(k.store.slot(appNapPath)); len(got) != 0 {
		t.Fatalf("removing firefox left %v", got)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := storedNapLevels(k.store.slot(appNapPath)); got["firefox"] != level || len(got) != 1 {
		t.Fatalf("undo stored %v, want firefox at %s", got, level)
	}
}

// A strip's dismissed app comes back on Undo at its place.
func TestAnAppStripRemovalIsUndoneInPlace(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "")
	shown := undoToasts(k)
	var written []string
	c := newAppStrip(k, pinnedSpec, []string{"firefox", "org.gnome.Calculator", "org.gnome.Nautilus"}, func(ids []string) { written = ids })
	c.drop(1)
	if !slices.Equal(written, []string{"firefox", "org.gnome.Nautilus"}) {
		t.Fatalf("dismissing Calculator wrote %q", written)
	}
	if d := lastUndo(t, shown).Description(); d != "Calculator" {
		t.Errorf("the undo names %q", d)
	}
	pressUndo(t, lastUndo(t, shown))
	if want := []string{"firefox", "org.gnome.Calculator", "org.gnome.Nautilus"}; !slices.Equal(written, want) || c.chips.Len() != 3 {
		t.Fatalf("undo wrote %q with %d chips", written, c.chips.Len())
	}
}

// A bar layout, a zone's item and a group's module each come back on
// Undo at their place.
func TestABarLayoutsRemovalsAreUndoneInPlace(t *testing.T) {
	k := testKit(t, "")
	shown := undoToasts(k)
	c := newLayoutEditor(k, k.store.slot("bar.layout"))
	stored := func() any { return config.Plain(k.store.value("bar.layout")) }
	before := stored()
	zone := func() *[]config.BarItem { return c.zone(0, zoneLeft) }
	if len(*zone()) < 2 {
		t.Fatalf("the default layout's left zone holds %d items", len(*zone()))
	}
	box := func() *widget.ChipBox {
		for b, at := range c.zones {
			if at == (zoneRef{0, zoneLeft}) {
				return b
			}
		}
		t.Fatal("no left zone shown")
		return nil
	}
	chip := func(i int) *widget.Chip { return box().ChipAt(i) }
	c.removeItem(chip(1))
	if reflect.DeepEqual(stored(), before) {
		t.Fatal("removing an item wrote nothing")
	}
	pressUndo(t, lastUndo(t, shown))
	if got := stored(); !reflect.DeepEqual(got, before) {
		t.Fatalf("undoing the item stored %v, want %v", got, before)
	}

	c.layouts = append(c.layouts, config.BarLayout{Monitor: "DP-9", Show: true})
	c.commit()
	c.rebuild()
	withTwo := stored()
	c.removeLayout(0)
	if len(c.layouts) != 1 {
		t.Fatalf("removing the first layout left %d", len(c.layouts))
	}
	pressUndo(t, lastUndo(t, shown))
	if got := stored(); !reflect.DeepEqual(got, withTwo) || c.at != 0 {
		t.Fatalf("undoing the layout stored %v showing %d, want %v showing it", got, c.at, withTwo)
	}

	*zone() = append(*zone(), config.BarItem{Group: &config.BarGroup{Name: "g", Modules: []config.BarItem{{Module: config.BuiltinModules[0]}, {Module: config.BuiltinModules[1]}}}})
	c.commit()
	c.showCard()
	withGroup := stored()
	c.removeGroupModule(chip(len(*zone())-1), 0)
	if g := (*zone())[len(*zone())-1].Group; len(g.Modules) != 1 {
		t.Fatalf("removing the group's module left %v", g.Modules)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := stored(); !reflect.DeepEqual(got, withGroup) {
		t.Fatalf("undoing the group's module stored %v, want %v", got, withGroup)
	}
}

// A dock display not connected comes back on Undo at its place.
func TestADockDisplayRemovalIsUndoneInPlace(t *testing.T) {
	k := testKit(t, "dock:\n  monitors: [DP-9, DP-1]\n")
	shown := undoToasts(k)
	fakeHeads(t, []displays.Head{head("DP-1", 0)}, nil)
	c := fieldControl(k, k.store, field("dock.monitors", monitorSetRow)).(*monitorSet)
	treetest.First[*widget.Button](t, c.away).OnClick()
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"DP-1"}) {
		t.Fatalf("removing DP-9 stored %v", got)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"DP-9", "DP-1"}) {
		t.Fatalf("undo stored %v", got)
	}
	if labels := treetest.Labels(c.away); !slices.Contains(labels, "DP-9") {
		t.Errorf("the restored display is not listed: %v", labels)
	}
}

// A disconnected display's wallpaper comes back on Undo with its entry.
func TestADisplayWallpaperRemovalIsUndoneWithItsEntry(t *testing.T) {
	k := testKit(t, "wallpaper:\n  monitors:\n    - {name: DP-9, wallpaper: /walls/old.png, fit-mode: fit}\n")
	shown := undoToasts(k)
	fakeHeads(t, []displays.Head{head("DP-1", 0)}, nil)
	c := fieldControl(k, k.store, field("wallpaper.monitors", monitorWallpaperRow)).(*monitorWallpapers)
	before := k.store.value("wallpaper.monitors")
	treetest.First[*widget.Button](t, c.away).OnClick()
	if _, ok := monitorEntry(k.store.value("wallpaper.monitors"), "DP-9"); ok {
		t.Fatal("removing DP-9 kept it")
	}
	pressUndo(t, lastUndo(t, shown))
	if got := k.store.value("wallpaper.monitors"); !reflect.DeepEqual(got, before) {
		t.Fatalf("undo stored %v, want %v", got, before)
	}
}

// A forgotten device's Undo sets each of its keys back.
func TestAForgottenDeviceIsRestoredByUndo(t *testing.T) {
	k := deviceKit(t, deviceConfig+`    "Combo":
      natural_scroll: true
      accel_speed: 0.25
`, someDevices)
	shown := undoToasts(k)
	c := devicesOf(t, k, wmPointerPage())
	c.forget(cardNamed(t, c, "Combo"))
	acceptedByLysWM(t, k)
	if got := k.wm.valueAt([]string{"input", "devices", "Combo", "accel_speed"}); got != nil {
		t.Fatalf("forgetting left accel_speed %v", got)
	}
	pressUndo(t, lastUndo(t, shown))
	acceptedByLysWM(t, k)
	if got := k.wm.valueAt([]string{"input", "devices", "Combo", "accel_speed"}); got != 0.25 {
		t.Errorf("undo left accel_speed %v, want 0.25", got)
	}
	if got := k.wm.valueAt([]string{"input", "devices", "Combo", "natural_scroll"}); got != true {
		t.Errorf("undo left natural_scroll %v, want true", got)
	}
	if !slices.Contains(cardNames(c), "Combo (absent)") {
		t.Errorf("the restored device is not listed: %v", cardNames(c))
	}
}

// A column width comes back on Undo at its place.
func TestAColumnWidthRemovalIsUndoneInPlace(t *testing.T) {
	k := lysWMKit(t, "scroll:\n  presets: [0.25, 0.5, 0.75]\n")
	shown := undoToasts(k)
	c := presetsOf(t, layoutPage(t, k, "wm-tiling"))
	c.remove(1)
	if got := k.wm.value(columnPresetsPath); !reflect.DeepEqual(got, []any{0.25, 0.75}) {
		t.Fatalf("removing 0.5 wrote %v", got)
	}
	pressUndo(t, lastUndo(t, shown))
	if got := k.wm.value(columnPresetsPath); !reflect.DeepEqual(got, []any{0.25, 0.5, 0.75}) {
		t.Fatalf("undo wrote %v", got)
	}
}

// A rule's removed effect comes back on Undo with its value.
func TestARulesEffectRemovalIsUndone(t *testing.T) {
	k, w, _, _ := wmKit(t, "rules:\n  - {app_id: foot, float: true}\n")
	shown := undoToasts(k)
	c := fieldControl(k, w, wmRules(windowRules)).(*cardList)
	_, effects := openRule(t, c, 0)
	effects.removeEffect("float")
	if slices.Contains(effects.shown, "float") {
		t.Fatal("removing float kept it shown")
	}
	pressUndo(t, lastUndo(t, shown))
	if !slices.Equal(effects.shown, []string{"float"}) || effects.field("float").get() != true {
		t.Fatalf("undo shows %v with float %v", effects.shown, effects.field("float").get())
	}
}

// A pause pick's removal is picked again on Undo.
func TestAPausePickRemovalIsUndone(t *testing.T) {
	docker := sysUnit(t, "docker.service")
	timer := sysUnit(t, "nix-gc.timer")
	f := &fakePausing{st: pause.State{Picks: []pause.Unit{docker, timer}}, cands: []pauseCandidate{{unit: docker}, {unit: timer}}}
	s := pausingOf(t, f)
	shown := undoToasts(s.k)
	buttonsTitled(s.rows, locale.T().Get("settings-power-pause-remove"))[0].OnClick()
	pressUndo(t, lastUndo(t, shown))
	if n := len(f.system); n != 2 || !slices.Equal(f.system[1], []string{"docker.service", "nix-gc.timer"}) {
		t.Fatalf("the picks sent %q, want docker picked again", f.system)
	}
}

// A label's removed part comes back on Undo where it stood, the text
// around it split again; changed since, the undo leaves it.
func TestALabelPartRemovalIsUndoneWhileTheLabelIsUnchanged(t *testing.T) {
	const path = "modules.battery.format"
	ps := config.Placeholders(path)
	label := "a " + ps[0].Token + " b"
	k := testKit(t, "modules:\n  battery:\n    format: \""+label+"\"\n")
	shown := undoToasts(k)
	b := shellControl(t, k, path).(*labelBuilder)
	b.removeSegment(1)
	if v := k.store.value(path); v != "a  b" {
		t.Fatalf("removing the placeholder stored %v", v)
	}
	b.refresh()
	pressUndo(t, lastUndo(t, shown))
	if v := k.store.value(path); v != label || b.chips.Len() != 3 {
		t.Fatalf("undo stored %v with %d chips", v, b.chips.Len())
	}
	b.removeSegment(1)
	if err := k.store.set(path, "other"); err != nil {
		t.Fatal(err)
	}
	pressUndo(t, lastUndo(t, shown))
	if v := k.store.value(path); v != "other" {
		t.Errorf("an undo over a label changed since stored %v", v)
	}
}

// A mail condition comes back on Undo where it stood.
func TestAMailConditionRemovalIsUndoneInPlace(t *testing.T) {
	const path = "modules.mail.query"
	k := testKit(t, "modules:\n  mail:\n    query: \"tag:unread and not tag:spam and tag:inbox\"\n")
	shown := undoToasts(k)
	e := fieldControl(k, k.store, field(path, mailQueryRow)).(*mailQueryEditor)
	e.removeTerm(1)
	if v := k.store.value(path); v != "tag:unread and tag:inbox" {
		t.Fatalf("removing a condition stored %v", v)
	}
	pressUndo(t, lastUndo(t, shown))
	if v := k.store.value(path); v != "tag:unread and not tag:spam and tag:inbox" {
		t.Fatalf("undo stored %v", v)
	}
}
