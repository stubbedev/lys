package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/tzdb"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/glibclocale"
	"github.com/stubbedev/lys/lib/glibclocale/glibclocaletest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeSysCall struct {
	method string
	args   []any
}

type fakeSys struct {
	mu     sync.Mutex
	calls  []fakeSysCall
	reads  int
	refuse string
	props  map[string]any
	conn   *dbus.Conn
	path   dbus.ObjectPath
	iface  string
}

func (f *fakeSys) refuseWith(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse = reason
}

func (f *fakeSys) record(method string, args ...any) *dbus.Error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeSysCall{method, args})
	if f.refuse != "" {
		return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{f.refuse})
	}
	return nil
}

func (f *fakeSys) snapshot() ([]fakeSysCall, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls), f.reads
}

func (f *fakeSys) set(key string, v any) {
	f.mu.Lock()
	f.props[key] = v
	f.mu.Unlock()
	_ = dbusx.PropertiesChanged.Emit(f.conn, f.path, dbusx.ChangedProperties{Interface: f.iface, Changed: map[string]dbus.Variant{key: dbus.MakeVariant(v)}, Invalidated: []string{}})
}

func (f *fakeSys) getters() dbusx.Getters {
	g := dbusx.Getters{}
	for key := range f.props {
		g[key] = func() any {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.reads++
			return f.props[key]
		}
	}
	return g
}

type fakeTimedate struct{ *fakeSys }

func (f fakeTimedate) SetNTP(on, interactive bool) *dbus.Error {
	return f.record("SetNTP", on, interactive)
}

func (f fakeTimedate) SetTime(usec int64, relative, interactive bool) *dbus.Error {
	return f.record("SetTime", usec, relative, interactive)
}

func (f fakeTimedate) SetTimezone(zone string, interactive bool) *dbus.Error {
	return f.record("SetTimezone", zone, interactive)
}

func (f fakeTimedate) ListTimezones() ([]string, *dbus.Error) {
	f.mu.Lock()
	f.reads++
	f.mu.Unlock()
	return []string{"Europe/Copenhagen", "UTC"}, nil
}

type fakeLocaled struct{ *fakeSys }

func (f fakeLocaled) SetLocale(values []string, interactive bool) *dbus.Error {
	return f.record("SetLocale", values, interactive)
}

func (f fakeLocaled) SetVConsoleKeyboard(keymap, toggle string, convert, interactive bool) *dbus.Error {
	return f.record("SetVConsoleKeyboard", keymap, toggle, convert, interactive)
}

func (f fakeLocaled) SetX11Keyboard(layout, model, variant, options string, convert, interactive bool) *dbus.Error {
	return f.record("SetX11Keyboard", layout, model, variant, options, convert, interactive)
}

func serveFake(t *testing.T, bus *dbustest.Bus, name string, path dbus.ObjectPath, f *fakeSys, methods any) {
	t.Helper()
	f.conn, f.path, f.iface = bus.Conn(t), path, name
	if _, err := dbusx.Serve(f.conn, dbusx.Service{Name: name, Path: path, Interface: name, Methods: methods, Properties: f.getters()}); err != nil {
		t.Fatal(err)
	}
}

type loop struct {
	t     *testing.T
	queue chan func()
}

func systemKit(t *testing.T, bus *dbustest.Bus) (*kit, *loop) {
	t.Helper()
	k := testKit(t, "")
	k.systemBus = func() (*dbus.Conn, error) { return dbus.Connect(bus.Address) }
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	return k, l
}

func (l *loop) until(what string, ok func() bool) {
	l.t.Helper()
	waittest.Until(l.t, func() bool {
		l.settle()
		return ok()
	}, what)
}

func (l *loop) settle() {
	for {
		select {
		case fn := <-l.queue:
			fn()
		default:
			return
		}
	}
}

const quietWindow = 200 * time.Millisecond

func (l *loop) drain() {
	deadline := time.After(quietWindow)
	for {
		select {
		case fn := <-l.queue:
			fn()
		case <-deadline:
			return
		}
	}
}

func switchNamed(t *testing.T, root widget.Widget, name string) *widget.Switch {
	t.Helper()
	for _, s := range treetest.All[*widget.Switch](root) {
		if s.AccessibleName() == name {
			return s
		}
	}
	t.Fatalf("no switch %q", name)
	return nil
}

func TestEachDateControlIsOneInteractiveCallAndARefusalRevertsIt(t *testing.T) {
	h := useHostRoot(t)
	put(t, h.path("usr/share/zoneinfo", tzdb.ZoneTableName), "DK\t+5540+01235\tEurope/Copenhagen\n")
	put(t, h.path("usr/share/zoneinfo", tzdb.CountryTableName), "DK\tDenmark\n")
	bus := dbustest.Start(t)
	td := &fakeSys{props: map[string]any{"Timezone": "UTC", "NTP": true, "CanNTP": true, "LocalRTC": false}}
	serveFake(t, bus, timedateName, timedatePath, td, fakeTimedate{td})
	k, l := systemKit(t, bus)
	footer := dateTimeLead(k).(*closingBox)
	t.Cleanup(footer.close)
	ntp := switchNamed(t, footer, "Set time automatically")
	if !ntp.On() {
		t.Fatal("the switch does not show NTP on")
	}
	_, reads := td.snapshot()
	l.drain()
	if calls, again := td.snapshot(); len(calls) != 0 || again != reads {
		t.Fatalf("an idle page made %d calls and %d more reads", len(calls), again-reads)
	}
	ntp.SetOn(false)
	l.until("SetNTP", func() bool { c, _ := td.snapshot(); return len(c) == 1 })
	calls, _ := td.snapshot()
	if calls[0].method != "SetNTP" || fmt.Sprint(calls[0].args) != "[false true]" {
		t.Fatalf("%+v, want one SetNTP(false, interactive)", calls)
	}
	td.set("NTP", false)
	l.until("the manual date enabled by the change", func() bool { return widget.IsEnabled(treetest.First[*widget.Calendar](t, footer)) })
	treetest.First[*widget.Calendar](t, footer).Select(widget.Date{Year: 2026, Month: time.March, Day: 4})
	clock := treetest.First[*clockTimeEditor](t, footer)
	clock.hour.SetSelected(slices.Index(clock.hours, 13))
	clock.minute.SetSelected(slices.Index(clock.minutes, 7))
	treetest.Button(t, footer, "Set").OnClick()
	l.until("SetTime", func() bool { c, _ := td.snapshot(); return len(c) == 2 })
	want := time.Date(2026, time.March, 4, 13, 7, 0, 0, time.UTC).UnixMicro()
	if calls, _ := td.snapshot(); calls[1].method != "SetTime" || fmt.Sprint(calls[1].args) != fmt.Sprint([]any{want, false, true}) {
		t.Fatalf("%+v, want one SetTime(%d, absolute, interactive)", calls, want)
	}
	td.refuseWith("Changing system settings via systemd is not supported on NixOS.")
	zone := treetest.First[*zoneMap](t, footer)
	zone.pick("Europe/Copenhagen")
	l.until("the refusal shown", func() bool {
		return treetest.First[*sysPanel](t, footer).problem.Visible()
	})
	calls, _ = td.snapshot()
	if len(calls) != 3 || calls[2].method != "SetTimezone" || fmt.Sprint(calls[2].args) != "[Europe/Copenhagen true]" {
		t.Fatalf("%+v", calls)
	}
	if p := treetest.First[*sysPanel](t, footer); p.problem.Text() != locale.T().Get("settings-sys-failed", i18n.Str("service", timedateName)) || !strings.Contains(p.problem.detail.Text(), "NixOS") || zone.current != "UTC" {
		t.Fatalf("problem %q over %q, zone %q: want the app's summary over the service's message and the old zone", p.problem.Text(), p.problem.detail.Text(), zone.current)
	}
}

func useLocales(t *testing.T, src glibclocale.Source) {
	before := localeSource
	localeSource = src
	t.Cleanup(func() { localeSource = before })
}

func TestWithoutLocaleDataTheRegionPageSaysWhy(t *testing.T) {
	bus := dbustest.Start(t)
	lc := &fakeSys{props: map[string]any{"Locale": []string{"LANG=en_US.UTF-8"}, "VConsoleKeymap": "us", "X11Layout": "us", "X11Model": "", "X11Variant": "", "X11Options": ""}}
	serveFake(t, bus, localeName, localePath, lc, fakeLocaled{lc})
	none := filepath.Join(t.TempDir(), "none")
	useLocales(t, glibclocale.NewSource(filepath.Join(none, "locale-archive"), none))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k, l := systemKit(t, bus)
	footer := regionFooter(k).(*closingBox)
	t.Cleanup(footer.close)
	l.until("the system controls", func() bool { return len(treetest.All[*widget.Dropdown](footer)) == len(userFormats)+1 })
	for _, lbl := range treetest.All[*widget.Label](footer) {
		if strings.Contains(lbl.Text(), `no locale "en_US.UTF-8"`) && strings.Contains(lbl.Text(), none) {
			return
		}
	}
	t.Fatal("the preview does not say the system has no data for the language")
}

func TestTheFormatsWriteLocaleConfAndChangeThePreview(t *testing.T) {
	bus := dbustest.Start(t)
	lc := &fakeSys{props: map[string]any{"Locale": []string{"LANG=en_US.UTF-8"}, "VConsoleKeymap": "us", "X11Layout": "us", "X11Model": "", "X11Variant": "", "X11Options": ""}}
	serveFake(t, bus, localeName, localePath, lc, fakeLocaled{lc})
	useLocales(t, glibclocale.NewSource("", glibclocaletest.Dirs(t)))
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	put(t, filepath.Join(config, "locale.conf"), "# mine\nLC_NUMERIC=en_US.UTF-8\n")
	h := useHostRoot(t)
	put(t, h.path("usr/share/kbd/keymaps/i386/qwerty/dk.map.gz"), "")
	put(t, h.path("usr/share/kbd/keymaps/i386/qwerty/us.map.gz"), "")
	k, l := systemKit(t, bus)
	footer := regionFooter(k).(*closingBox)
	t.Cleanup(footer.close)
	l.until("the system controls", func() bool { return len(treetest.All[*widget.Dropdown](footer)) == len(userFormats)+1 })
	preview := func() string {
		for _, lbl := range treetest.All[*widget.Label](footer) {
			if strings.Count(lbl.Text(), " · ") == 2 {
				return lbl.Text()
			}
		}
		return ""
	}
	first := preview()
	if !strings.Contains(first, "$1,234.50") {
		t.Fatalf("the preview %q is not the language's money", first)
	}
	dropdownNamed(t, footer, "LC_MONETARY").SetSelected(1 + slices.Index(glibclocaletest.Names, "en_GB.utf8"))
	if got := fileText(t, filepath.Join(config, "locale.conf")); got != "# mine\nLC_NUMERIC=en_US.UTF-8\nLC_MONETARY=en_GB.utf8\n" {
		t.Fatalf("locale.conf %q", got)
	}
	if got := preview(); !strings.Contains(got, "£1,234.50") {
		t.Fatalf("the preview did not follow LC_MONETARY: %q", got)
	}
	tr := locale.T()
	unitsLine := func() string {
		for _, lbl := range treetest.All[*widget.Label](footer) {
			if strings.Contains(lbl.Text(), "°") {
				return lbl.Text()
			}
		}
		return ""
	}
	us := tr.Get("settings-region-preview-units", i18n.Str("measurement", tr.Get("settings-region-measurement-us")), i18n.Str("paper", tr.Get("settings-region-paper-letter")))
	if got := unitsLine(); got != us {
		t.Fatalf("the units read %q, want the language's %q", got, us)
	}
	dropdownNamed(t, footer, "LC_PAPER").SetSelected(1 + slices.Index(glibclocaletest.Names, "en_GB.utf8"))
	dropdownNamed(t, footer, "LC_MEASUREMENT").SetSelected(1 + slices.Index(glibclocaletest.Names, "en_GB.utf8"))
	metric := tr.Get("settings-region-preview-units", i18n.Str("measurement", tr.Get("settings-region-measurement-metric")), i18n.Str("paper", "A4"))
	if got := unitsLine(); got != metric {
		t.Fatalf("the units did not follow LC_PAPER and LC_MEASUREMENT: %q, want %q", got, metric)
	}
	dropdownNamed(t, footer, "LC_PAPER").SetSelected(0)
	dropdownNamed(t, footer, "LC_MEASUREMENT").SetSelected(0)
	dropdownNamed(t, footer, "LC_NUMERIC").SetSelected(0)
	if got := fileText(t, filepath.Join(config, "locale.conf")); got != "# mine\nLC_MONETARY=en_GB.utf8\n" {
		t.Fatalf("locale.conf %q after the same-as-language choice", got)
	}
	notSaved := func() *inlineState {
		for _, st := range treetest.All[*inlineState](footer) {
			if st.Visible() && st.Text() == tr.Get("settings-region-format-not-saved") {
				return st
			}
		}
		return nil
	}
	if err := os.Chmod(config, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(config, 0o700) })
	dropdownNamed(t, footer, "LC_PAPER").SetSelected(1 + slices.Index(glibclocaletest.Names, "en_GB.utf8"))
	if st := notSaved(); st == nil || st.kind != stateError || !strings.Contains(st.detail.Text(), config) || st.detail.Visible() {
		t.Fatalf("a locale.conf that cannot be written says %v", st)
	}
	if err := os.Chmod(config, 0o700); err != nil {
		t.Fatal(err)
	}
	dropdownNamed(t, footer, "LC_PAPER").SetSelected(0)
	if st := notSaved(); st != nil {
		t.Fatalf("a written format still says %q", st.Text())
	}
	if calls, _ := lc.snapshot(); len(calls) != 0 {
		t.Fatalf("the user's formats called localed: %+v", calls)
	}
	dropdownNamed(t, footer, "Language").SetSelected(slices.Index(glibclocaletest.Names, "en_GB.utf8"))
	l.until("SetLocale", func() bool { c, _ := lc.snapshot(); return len(c) == 1 })
	if calls, _ := lc.snapshot(); calls[0].method != "SetLocale" || fmt.Sprint(calls[0].args) != "[[LANG=en_GB.utf8] true]" {
		t.Fatalf("%+v", calls)
	}
	keymap := treetest.First[*choiceEditor](t, footer)
	if keymap.entry.Text() != "us" || !slices.Equal(values(keymap.spec.list()), []string{"dk", "us"}) {
		t.Fatalf("the keymap shows %q of %q", keymap.entry.Text(), values(keymap.spec.list()))
	}
	keymap.pick("dk")
	l.until("SetVConsoleKeyboard", func() bool { c, _ := lc.snapshot(); return len(c) == 2 })
	if calls, _ := lc.snapshot(); calls[1].method != "SetVConsoleKeyboard" || fmt.Sprint(calls[1].args) != "[dk  false true]" {
		t.Fatalf("%+v", calls)
	}
}

func TestCopyMyLayoutSendsLysWMsKeyboardInOneCall(t *testing.T) {
	bus := dbustest.Start(t)
	lc := &fakeSys{props: map[string]any{"Locale": []string{"LANG=en_US.UTF-8"}, "VConsoleKeymap": "us", "X11Layout": "us", "X11Model": "", "X11Variant": "", "X11Options": ""}}
	serveFake(t, bus, localeName, localePath, lc, fakeLocaled{lc})
	useLocales(t, glibclocale.NewSource("", glibclocaletest.Dirs(t)))
	k, w, _, _ := wmKit(t, "input:\n  keyboard:\n    layout: us,dk\n    variant: \",nodeadkeys\"\n    options: caps:escape\n    model: pc105\n")
	k.wm = w
	k.systemBus = func() (*dbus.Conn, error) { return dbus.Connect(bus.Address) }
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	footer := regionFooter(k).(*closingBox)
	t.Cleanup(footer.close)
	l.until("the system controls", func() bool { return len(treetest.All[*widget.Dropdown](footer)) == len(userFormats)+1 })
	treetest.Button(t, footer, "Copy my layout").OnClick()
	l.until("SetX11Keyboard", func() bool { c, _ := lc.snapshot(); return len(c) == 1 })
	if calls, _ := lc.snapshot(); calls[0].method != "SetX11Keyboard" || fmt.Sprint(calls[0].args) != "[us,dk pc105 ,nodeadkeys caps:escape false true]" {
		t.Fatalf("%+v", calls)
	}
}

func TestAPageWithoutTheServiceSaysSoAndOffersNothing(t *testing.T) {
	bus := dbustest.Start(t)
	k, l := systemKit(t, bus)
	footer := dateTimeLead(k).(*closingBox)
	t.Cleanup(footer.close)
	l.drain()
	if len(treetest.All[*choiceEditor](footer)) != 0 {
		t.Fatal("controls without timedated")
	}
	found := false
	for _, lbl := range treetest.All[*widget.Label](footer) {
		found = found || (strings.Contains(lbl.Text(), "org.freedesktop.timedate1") && strings.Contains(lbl.Text(), "is not running"))
	}
	if !found {
		t.Fatal("the page does not say timedated is missing")
	}
}

func TestTheLocaleConfPathIsAbsoluteOrAnError(t *testing.T) {
	for want, env := range map[string]map[string]string{
		"/cfg/locale.conf":             {"XDG_CONFIG_HOME": "/cfg", "HOME": "/home/me"},
		"/home/me/.config/locale.conf": {"XDG_CONFIG_HOME": "cfg", "HOME": "/home/me"},
	} {
		if got, err := localeConfPath(fakeHost(t, env)); err != nil || got != want {
			t.Errorf("%v: %q, %v, want %q", env, got, err, want)
		}
	}
	for _, env := range []map[string]string{{}, {"XDG_CONFIG_HOME": "cfg", "HOME": "me"}} {
		if got, err := localeConfPath(fakeHost(t, env)); !errors.Is(err, errNoConfigHome) || !strings.Contains(err.Error(), localeConfName) {
			t.Errorf("%v: %q, %v, want an error naming %s and both variables", env, got, err, localeConfName)
		}
	}
}

func TestWithoutAHomeTheFormatsSayWhyAndTouchNoRelativeLocaleConf(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	useLocales(t, glibclocale.NewSource("", glibclocaletest.Dirs(t)))
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	footer := regionFooter(k).(*closingBox)
	t.Cleanup(footer.close)
	if labels := treetest.Labels(footer); !slices.ContainsFunc(labels, func(text string) bool { return strings.Contains(text, errNoConfigHome.Error()) }) {
		t.Fatalf("the page shows %q, not why it has no locale.conf", labels)
	}
	for _, dd := range treetest.All[*widget.Dropdown](footer) {
		if slices.Contains(userFormats, dd.AccessibleName()) {
			t.Fatalf("the format %s is offered with no locale.conf to write it to", dd.AccessibleName())
		}
	}
	if left := entries(cwd); len(left) != 0 {
		t.Fatalf("the page left %s in the working directory", left[0].Name())
	}
}

func TestTheSystemBusDialReachesTheBusAndNamesItselfOnceWhenItCannot(t *testing.T) {
	bus := dbustest.Start(t)
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", bus.Address)
	conn, err := dialSystemBus()
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.Names()) == 0 {
		t.Error("the dialled connection has no unique name")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "none"))
	k := testKit(t, "")
	k.systemBus = dialSystemBus
	footer := dateTimeLead(k).(*closingBox)
	t.Cleanup(footer.close)
	const dialing = "connect to the system bus"
	for _, text := range treetest.Labels(footer) {
		if n := strings.Count(text, dialing); n == 1 {
			return
		} else if n > 1 {
			t.Fatalf("%q says %q %d times", text, dialing, n)
		}
	}
	t.Fatalf("the page shows %q, not that the system bus cannot be reached", treetest.Labels(footer))
}

func TestThe24HourSwitchChangesOnlyTheHourOfACustomClockFormat(t *testing.T) {
	k := testKit(t, "modules:\n  clock:\n    format: \"%Y-%m-%d · %I:%M %p\"\n")
	var failed []error
	row := hourCycleRow(k, func(err error) { failed = append(failed, err) }, func() {})
	h24 := treetest.First[*widget.Switch](t, row)
	if h24.On() || !h24.Enabled() {
		t.Fatalf("a 12-hour format shows on %v, enabled %v", h24.On(), h24.Enabled())
	}
	h24.SetOn(true)
	if v := k.store.value(clockFormatPath); v != "%Y-%m-%d · %H:%M" {
		t.Fatalf("on: %q", v)
	}
	h24.SetOn(false)
	if v := k.store.value(clockFormatPath); v != "%Y-%m-%d · %I:%M %p" {
		t.Fatalf("off again: %q", v)
	}
	if len(failed) != 0 {
		t.Fatal(failed)
	}
}

func TestThe24HourSwitchIsDisabledForAClockFormatWithoutAnHour(t *testing.T) {
	for _, format := range []string{"%Y-%m-%d", "%H (%I %p)"} {
		k := testKit(t, "modules:\n  clock:\n    format: \""+format+"\"\n")
		row := hourCycleRow(k, func(err error) { t.Fatal(err) }, func() {})
		h24 := treetest.First[*widget.Switch](t, row)
		if h24.Enabled() {
			t.Errorf("%q: the switch is enabled", format)
		}
		h24.SetOn(!h24.On())
		if v := k.store.value(clockFormatPath); v != format {
			t.Errorf("%q: forcing the switch wrote %q", format, v)
		}
	}
}

func clockCardAt(t *testing.T, config, timeLocale string) *clockCard {
	t.Helper()
	useLocales(t, glibclocale.NewSource("", glibclocaletest.Dirs(t)))
	t.Setenv("LC_ALL", timeLocale)
	k := testKit(t, config)
	k.clock = fakeclock.At(time.Date(2026, time.November, 19, 14, 5, 0, 0, time.UTC))
	c := newClockCard(k)
	t.Cleanup(c.close)
	return c
}

func TestTheClockCardShowsTodayInTheTimeLocaleAndTheClocksHourCycle(t *testing.T) {
	c := clockCardAt(t, "modules:\n  clock:\n    format: \"%a %I:%M %p\"\n", "en_US.utf8")
	if c.date.Text() != "Thursday, 11/19/2026" || c.clock.Text() != "02:05 PM" {
		t.Fatalf("en_US with a 12-hour clock shows %q, %q", c.date.Text(), c.clock.Text())
	}
	if err := c.k.store.set(clockFormatPath, "%a %H:%M"); err != nil {
		t.Fatal(err)
	}
	c.retick()
	if c.clock.Text() != "14:05" {
		t.Fatalf("after the switch to 24-hour the clock shows %q", c.clock.Text())
	}
	de := clockCardAt(t, "", "de_DE.utf8")
	if de.date.Text() != "Donnerstag, 19.11.2026" {
		t.Fatalf("de_DE shows %q", de.date.Text())
	}
}

func TestTheClockCardWithAnUninstalledLocaleShowsCsNames(t *testing.T) {
	c := clockCardAt(t, "modules:\n  clock:\n    format: \"%H:%M\"\n", "xx_XX.utf8")
	if c.date.Text() != "Thursday, 11/19/26" || c.clock.Text() != "14:05" {
		t.Fatalf("an uninstalled locale shows %q, %q", c.date.Text(), c.clock.Text())
	}
}

func TestTheClockCardShowsTheZoneAndWhetherTheTimeIsSynchronized(t *testing.T) {
	c := clockCardAt(t, "", "en_US.utf8")
	if c.zone.Visible() || c.sync.Visible() {
		t.Fatal("the zone and sync lines show before timedated answered")
	}
	tr := locale.T()
	c.showSystem("Europe/Copenhagen", true, true)
	if c.zone.Text() != "Europe/Copenhagen · UTC+01:00" || c.sync.Text() != tr.Get("settings-date-sync-done") {
		t.Fatalf("synchronized: %q, %q", c.zone.Text(), c.sync.Text())
	}
	c.showSystem("Europe/Copenhagen", true, false)
	if c.sync.Text() != tr.Get("settings-date-sync-waiting") {
		t.Fatalf("not yet synchronized: %q", c.sync.Text())
	}
	c.showSystem("UTC", false, false)
	if c.zone.Text() != "UTC · UTC+00:00" || c.sync.Text() != tr.Get("settings-date-sync-manual") {
		t.Fatalf("set by hand: %q, %q", c.zone.Text(), c.sync.Text())
	}
	if c.zoneProblem.Visible() {
		t.Fatalf("a known zone says %q", c.zoneProblem.Text())
	}
	c.showSystem("Nowhere/Atlantis", true, true)
	if c.zone.Visible() || !c.zoneProblem.Visible() || c.zoneProblem.Text() != tr.Get("settings-date-zone-unknown", i18n.Str("zone", "Nowhere/Atlantis")) || !strings.Contains(c.zoneProblem.detail.Text(), `"Nowhere/Atlantis"`) || c.zoneProblem.detail.Visible() {
		t.Fatalf("an unknown zone shows the zone %v, says %q over %q", c.zone.Visible(), c.zoneProblem.Text(), c.zoneProblem.detail.Text())
	}
	c.showSystem("UTC", false, false)
	if !c.zone.Visible() || c.zoneProblem.Visible() {
		t.Fatalf("a known zone again shows the zone %v, says %q", c.zone.Visible(), c.zoneProblem.Text())
	}
}

// A date the session's time locale cannot format hides the clock and
// says so, the locale's error only in the closed detail.
func TestTheClockCardSaysADateItCannotFormatInTheAppsWords(t *testing.T) {
	dirs := glibclocaletest.Dirs(t)
	lcTime := filepath.Join(dirs, "en_US.utf8", "LC_TIME")
	raw, err := os.ReadFile(lcTime)
	if err != nil {
		t.Fatal(err)
	}
	broken := bytes.Replace(raw, []byte("%m/%d/%Y"), []byte("%5m/%d/Y"), 1)
	if bytes.Equal(broken, raw) {
		t.Fatalf("en_US's LC_TIME holds no %q date format", "%m/%d/%Y")
	}
	if err := os.WriteFile(lcTime, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	useLocales(t, glibclocale.NewSource("", dirs))
	t.Setenv("LC_ALL", "en_US.utf8")
	k := testKit(t, "")
	k.clock = fakeclock.At(time.Date(2026, time.November, 19, 14, 5, 0, 0, time.UTC))
	c := newClockCard(k)
	t.Cleanup(c.close)
	if c.clock.Visible() || c.date.Visible() || !c.problem.Visible() || c.problem.Text() != locale.T().Get("settings-date-clock-unformattable") || !strings.Contains(c.problem.detail.Text(), "%5m") || c.problem.detail.Visible() {
		t.Fatalf("an unformattable date shows clock %v date %v, says %q over %q", c.clock.Visible(), c.date.Visible(), c.problem.Text(), c.problem.detail.Text())
	}
}

func TestTheWeekStartLivesOnDateAndTimeBesideAWorldClockLink(t *testing.T) {
	if err := checkSetting(bothParts, dateTimeID, "modules.clock.calendar-weekday-start"); err != nil {
		t.Fatal(err)
	}
	if err := checkSetting(bothParts, "clock", "modules.clock.calendar-weekday-start"); err == nil {
		t.Error("the week start is still on the Clock page")
	}
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	related := dateTimePage(nil).relatedGroups()
	if opened := relatedOpens(k, related); !slices.Equal(opened, []string{"world-clock"}) {
		t.Errorf("the related links opened %q", opened)
	}
	k.nav = nil
	if _, ok := relatedSection(k, related); ok {
		t.Error("without a World Clock page the page still has related links")
	}
}

func weekStartCards(t *testing.T, yaml string) (*kit, *choiceCards) {
	t.Helper()
	k := testKit(t, yaml)
	h, ok := fieldControl(k, k.store, pageRow(t, dateTimeID, "modules.clock.calendar-weekday-start")).(*regionHint)
	if !ok {
		t.Fatal("the week start says nothing of the region")
	}
	c, ok := h.inner.(*choiceCards)
	if !ok {
		t.Fatal("the week start is not picked from cards")
	}
	return k, c
}

func cardValues(c *choiceCards) []string {
	out := make([]string, len(c.cards))
	for i, card := range c.cards {
		out[i] = card.value
	}
	return out
}

func TestTheWeekStartFollowsTheRegionOrIsMondaySundayOrSaturdayByCard(t *testing.T) {
	k, c := weekStartCards(t, "")
	if got := cardValues(c); !slices.Equal(got, []string{"auto", "monday", "sunday", "saturday"}) || c.current != "auto" {
		t.Fatalf("cards %q with %q chosen", got, c.current)
	}
	if label := c.cards[1].label; label == "monday" || label == "" {
		t.Errorf("Monday's card reads %q, not its name", label)
	}
	c.cards[1].PressAt(widget.Point{})
	if v := k.store.value("modules.clock.calendar-weekday-start"); v != "monday" || c.current != "monday" {
		t.Fatalf("stored %v, chosen %q", v, c.current)
	}
}

func TestAHandEditedWeekStartGetsItsOwnCard(t *testing.T) {
	k, c := weekStartCards(t, "modules:\n  clock:\n    calendar-weekday-start: tuesday\n")
	if got := cardValues(c); !slices.Equal(got, []string{"auto", "monday", "sunday", "saturday", "tuesday"}) || c.current != "tuesday" {
		t.Fatalf("cards %q with %q chosen", got, c.current)
	}
	if v := k.store.value("modules.clock.calendar-weekday-start"); v != "tuesday" {
		t.Fatalf("a hand-edited week start was rewritten to %v", v)
	}
}

func TestTheRegionPageLinksToKeyboardLayoutsWhenLysWMIsThere(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	related := regionPage(nil).relatedGroups()
	if opened := relatedOpens(k, related); !slices.Equal(opened, []string{"wm-keyboard"}) {
		t.Errorf("the related links opened %q", opened)
	}
	k.nav = layout(k.store.svc.Config(), parts{shell: true})
	if _, ok := relatedSection(k, related); ok {
		t.Error("without lys-wm the page still has related links")
	}
}

func useHostRoot(t *testing.T) host {
	t.Helper()
	before := systemHost
	systemHost = host{root: t.TempDir(), env: before.env}
	t.Cleanup(func() { systemHost = before })
	return systemHost
}

func TestAPaperSizeIsNamedOrMeasured(t *testing.T) {
	tr := locale.T()
	for _, c := range []struct {
		w, h uint32
		want string
	}{
		{210, 297, "A4"},
		{297, 420, "A3"},
		{216, 279, tr.Get("settings-region-paper-letter")},
		{216, 356, tr.Get("settings-region-paper-legal")},
		{297, 210, tr.Get("settings-region-paper-millimetres", i18n.Str("width", "297"), i18n.Str("height", "210"))},
		{182, 257, tr.Get("settings-region-paper-millimetres", i18n.Str("width", "182"), i18n.Str("height", "257"))},
	} {
		if got := paperName(c.w, c.h); got != c.want {
			t.Errorf("%d×%d mm = %q, want %q", c.w, c.h, got, c.want)
		}
	}
}

func TestALocaleWithoutPaperDataIsAnError(t *testing.T) {
	dir := glibclocaletest.Dirs(t)
	if err := os.Remove(filepath.Join(dir, "de_DE.utf8", "LC_PAPER")); err != nil {
		t.Fatal(err)
	}
	useLocales(t, glibclocale.NewSource("", dir))
	if _, err := previewUnits("de_DE.UTF-8", "de_DE.UTF-8"); err == nil || !strings.Contains(err.Error(), "has no LC_PAPER") {
		t.Fatalf("a locale without LC_PAPER previews with %v", err)
	}
	if got, err := previewUnits("en_US.UTF-8", "C"); err != nil || !strings.Contains(got, "A4") || !strings.Contains(got, locale.T().Get("settings-region-measurement-us")) {
		t.Fatalf("en_US measurement on C paper = %q, %v", got, err)
	}
}
