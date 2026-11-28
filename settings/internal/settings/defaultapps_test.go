package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func defaultAppsFixture(t *testing.T) (config string) {
	t.Helper()
	root := t.TempDir()
	data, config := filepath.Join(root, "data"), filepath.Join(root, "config")
	app := func(id, name, mimes string) {
		put(t, filepath.Join(data, "applications", id), "[Desktop Entry]\nType=Application\nName="+name+"\nExec=true\nIcon="+name+"\nMimeType="+mimes+"\n")
	}
	app("firefox.desktop", "Firefox", "x-scheme-handler/http;x-scheme-handler/https;text/html;")
	app("chromium.desktop", "Chromium", "x-scheme-handler/http;x-scheme-handler/https;text/html;image/png;")
	app("evince.desktop", "Evince", "application/pdf;")
	put(t, filepath.Join(config, "mimeapps.list"), "# mine\n[Added Associations]\nimage/png=chromium.desktop;\n\n[Default Applications]\nx-scheme-handler/https=firefox.desktop;\n")
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "none"))
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "none"))
	t.Setenv("XDG_CURRENT_DESKTOP", "lys-wm")
	return config
}

func named[W widget.Widget](root widget.Widget, want string, name func(W) string) (W, bool) {
	for _, w := range treetest.All[W](root) {
		if name(w) == want {
			return w, true
		}
	}
	var zero W
	return zero, false
}

func mustNamed[W widget.Widget](t testing.TB, root widget.Widget, kind, want string, name func(W) string) W {
	t.Helper()
	w, ok := named(root, want, name)
	if !ok {
		t.Fatalf("no %s %q", kind, want)
	}
	return w
}

func dropdownName(d *widget.Dropdown) string { return d.AccessibleName() }

func tileName(tile *appTile) string { return tile.button.AccessibleName() }

func buttonName(b *widget.Button) string { return b.AccessibleName() }

func dropdownNamed(t testing.TB, root widget.Widget, name string) *widget.Dropdown {
	t.Helper()
	return mustNamed(t, root, "dropdown", name, dropdownName)
}

func tileNamed(t testing.TB, root widget.Widget, name string) *appTile {
	t.Helper()
	return mustNamed(t, root, "app tile", name, tileName)
}

func defaultResetName(title string) string {
	return locale.T().Get("settings-default-reset-named", i18n.Str("name", title))
}

func hasDefaultResetButton(root widget.Widget, title string) bool {
	_, ok := named(root, defaultResetName(title), buttonName)
	return ok
}

func defaultResetButton(t testing.TB, root widget.Widget, title string) *widget.Button {
	t.Helper()
	return mustNamed(t, root, "reset button", defaultResetName(title), buttonName)
}

func pickFromTile(t *testing.T, w *window, f *fakePickers, title, app string) []string {
	t.Helper()
	tileNamed(t, w.page, title).button.OnClick()
	if f.content == nil {
		t.Fatalf("the %s tile opened no picker", title)
	}
	flow := treetest.First[*widget.FlowBox](t, f.content)
	offered := shownLabels(flow)
	if app != "" {
		i := slices.Index(offered, app)
		if i < 0 {
			t.Fatalf("the %s picker offers %q, not %q", title, offered, app)
		}
		flow.OnActivate(i)
	}
	f.content = nil
	return offered
}

func TestPickingABrowserWritesItsThreeKeysAtOnceFromOneScan(t *testing.T) {
	config := defaultAppsFixture(t)
	scans := 0
	before := scanApps
	scanApps = func(dirs []string) []desktopentry.App { scans++; return before(dirs) }
	t.Cleanup(func() { scanApps = before })
	var writes [][]string
	beforeSet := setDefaults
	setDefaults = func(id string, types []string) error { writes = append(writes, types); return beforeSet(id, types) }
	t.Cleanup(func() { setDefaults = beforeSet })
	w, f := defaultAppsWindow(t, "")
	if scans != 1 {
		t.Fatalf("%d scans of the applications for one page, want one", scans)
	}
	mixed := locale.T().Get("settings-default-mixed")
	if got := tileNamed(t, w.page, "Web browser").app.Text(); got != mixed {
		t.Fatalf("the web browser tile reads %q, want Mixed: https opens Firefox, http Chromium", got)
	}
	if got := tileNamed(t, w.page, "PDF documents").app.Text(); got != "Evince" {
		t.Fatalf("the only PDF viewer is not shown as the default: %q", got)
	}
	if offered := pickFromTile(t, w, f, "Web browser", "Chromium"); !slices.Equal(offered, []string{"Chromium", "Firefox"}) {
		t.Fatalf("the web browser picker offered %q", offered)
	}
	if len(writes) != 1 || !slices.Equal(writes[0], []string{"x-scheme-handler/https", "x-scheme-handler/http", "text/html"}) {
		t.Fatalf("writes %v, want the three browser types in one", writes)
	}
	got := fileText(t, filepath.Join(config, "mimeapps.list"))
	if !strings.HasPrefix(got, "# mine\n[Added Associations]\nimage/png=chromium.desktop;\n") {
		t.Fatalf("the rest of the file changed: %q", got)
	}
	for _, mimeType := range []string{"x-scheme-handler/http", "x-scheme-handler/https", "text/html"} {
		if a, ok := desktopentry.Default(desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists(), mimeType); !ok || a.ID != "chromium.desktop" {
			t.Errorf("%s resolves to %q, want chromium.desktop", mimeType, a.ID)
		}
	}
	if scans != 1 {
		t.Fatalf("a pick rescanned the applications: %d scans", scans)
	}
	if got := tileNamed(t, w.page, "Web browser").app.Text(); got != "Chromium" {
		t.Fatalf("after the pick the web browser tile reads %q, want Chromium", got)
	}
	if tile := tileNamed(t, w.page, "Email"); widget.IsEnabled(tile.button) || tile.app.Text() != locale.T().Get("settings-default-none") {
		t.Errorf("a category no app opens reads %q, enabled %v", tile.app.Text(), widget.IsEnabled(tile.button))
	}
	filter := treetest.First[*widget.SearchEntry](t, w.page)
	filter.SetText("pdf")
	filter.OnSearchChanged("pdf")
	var types []string
	for _, dd := range treetest.All[*widget.Dropdown](w.page) {
		if strings.Contains(dd.AccessibleName(), "/") {
			types = append(types, dd.AccessibleName())
		}
	}
	if !slices.Equal(types, []string{"application/pdf"}) {
		t.Fatalf("the pdf filter shows the types %q", types)
	}
}

func TestTheTerminalIsATileWritingTheLaunchersTerminal(t *testing.T) {
	defaultAppsFixture(t)
	data := os.Getenv("XDG_DATA_HOME")
	bin := t.TempDir()
	for _, program := range []string{"foot", "footclient"} {
		put(t, filepath.Join(bin, program), "")
		if err := os.Chmod(filepath.Join(bin, program), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	put(t, filepath.Join(data, "applications", "foot.desktop"), "[Desktop Entry]\nType=Application\nName=Foot\nExec=foot --server\nIcon=foot\nCategories=System;TerminalEmulator;\n")
	put(t, filepath.Join(data, "applications", "footclient.desktop"), "[Desktop Entry]\nType=Application\nName=Foot Client\nExec=footclient\nNoDisplay=true\nCategories=System;TerminalEmulator;\n")
	w, f := defaultAppsWindow(t, "")
	tr := locale.T()
	terminal := tr.Get("settings-default-terminal")
	if got := tileNamed(t, w.page, terminal).app.Text(); got != tr.Get("settings-default-terminal-auto") {
		t.Fatalf("an unset terminal reads %q", got)
	}
	if hasDefaultResetButton(w.page, terminal) {
		t.Fatal("the automatic terminal offers a reset")
	}
	if offered := pickFromTile(t, w, f, terminal, "Foot"); !slices.Equal(offered, []string{"Foot"}) {
		t.Fatalf("the terminal picker offered %q: a hidden or non-terminal app is on offer", offered)
	}
	if v := w.k.store.value(terminalPath); v != "foot" {
		t.Fatalf("picking Foot stored %v", v)
	}
	if got := tileNamed(t, w.page, terminal).app.Text(); got != "Foot" {
		t.Fatalf("after the pick the terminal reads %q", got)
	}
	defaultResetButton(t, w.page, terminal).OnClick()
	if v := w.k.store.value(terminalPath); v != "" {
		t.Fatalf("resetting the terminal left %v", v)
	}
	wmOnly := testKit(t, "")
	wmOnly.parts = parts{wm: true}
	for _, tile := range treetest.All[*appTile](defaultAppsFooter(wmOnly)) {
		if tile.button.AccessibleName() == terminal {
			t.Error("without lys-shell Default Apps offers lys-shell's terminal")
		}
	}
	other, _ := defaultAppsWindow(t, "launcher:\n  terminal: wezterm\n")
	if got := tileNamed(t, other.page, terminal).app.Text(); got != "wezterm" {
		t.Errorf("a terminal no app declares reads %q, want its command", got)
	}
}

func defaultAppsWindow(t *testing.T, config string) (*window, *fakePickers) {
	t.Helper()
	k := testKit(t, config)
	f := &fakePickers{}
	k.pickers = f
	w := newWindow(k, k.store.svc.Config(), func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	w.show(defaultAppsID)
	w.root.Arrange(render.Rect{W: 1400, H: 6000})
	return w, f
}

func TestResetGivesACategoryBackToTheSystemDefault(t *testing.T) {
	config := defaultAppsFixture(t)
	w, _ := defaultAppsWindow(t, "")
	if hasDefaultResetButton(w.page, "PDF documents") {
		t.Fatal("a category the user never chose for offers a reset")
	}
	defaultResetButton(t, w.page, "Web browser").OnClick()
	got := fileText(t, filepath.Join(config, "mimeapps.list"))
	if strings.Contains(got, "x-scheme-handler/https") || !strings.Contains(got, "image/png=chromium.desktop;") {
		t.Fatalf("after the reset the user's list reads %q", got)
	}
	if hasDefaultResetButton(w.page, "Web browser") {
		t.Error("the reset category still offers a reset")
	}
	if got := tileNamed(t, w.page, "Web browser").app.Text(); got != "Chromium" {
		t.Errorf("after the reset the web browser reads %q, want the first that declares the types", got)
	}
}

func TestAResetThatFailsIsSaid(t *testing.T) {
	defaultAppsFixture(t)
	before := resetAssociations
	resetAssociations = func([]string) error { return errors.New("the list is read-only") }
	t.Cleanup(func() { resetAssociations = before })
	w, _ := defaultAppsWindow(t, "")
	defaultResetButton(t, w.page, "Web browser").OnClick()
	if labels := strings.Join(treetest.Labels(w.page), "|"); !strings.Contains(labels, "the list is read-only") {
		t.Fatalf("a failed reset left the page reading %q", labels)
	}
	if !hasDefaultResetButton(w.page, "Web browser") {
		t.Error("a failed reset took the reset away")
	}
}

func TestAMalformedUserListIsSaidAndOffersNoReset(t *testing.T) {
	config := defaultAppsFixture(t)
	put(t, filepath.Join(config, "mimeapps.list"), "x-scheme-handler/https=firefox.desktop;\n")
	w, _ := defaultAppsWindow(t, "")
	if labels := strings.Join(treetest.Labels(w.page), "|"); !strings.Contains(labels, "mimeapps.list") {
		t.Fatalf("a malformed list left the page reading %q", labels)
	}
	if hasDefaultResetButton(w.page, "Web browser") {
		t.Error("a malformed list offers a reset")
	}
}

func TestAFileTypeReadsAsItsDescription(t *testing.T) {
	defaultAppsFixture(t)
	data := os.Getenv("XDG_DATA_HOME")
	put(t, filepath.Join(data, "mime", "application", "pdf.xml"), `<mime-type type="application/pdf"><comment>PDF document</comment><comment xml:lang="zz">ZZ</comment></mime-type>`)
	ix := indexApps(desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists(), mimeDescriber())
	if got := ix.names["application/pdf"]; got != "PDF document" {
		t.Fatalf("application/pdf reads %q", got)
	}
	if got := ix.names["image/png"]; got != "image/png" {
		t.Errorf("an undescribed type reads %q, want its own name", got)
	}
}

type fatalRecorder struct {
	testing.TB
	fatal string
}

func (r *fatalRecorder) Helper() {}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.fatal = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func fatalOf(t *testing.T, look func(testing.TB)) (fatal string, panicked any) {
	t.Helper()
	r := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { panicked = recover() }()
		look(r)
	}()
	<-done
	return r.fatal, panicked
}

func TestALookupThatMissesFailsNamingWhatIsMissing(t *testing.T) {
	defaultAppsFixture(t)
	w, _ := defaultAppsWindow(t, "")
	for _, c := range []struct {
		want string
		look func(testing.TB)
	}{
		{`no app tile "Fax"`, func(tb testing.TB) { tileNamed(tb, w.page, "Fax").button.OnClick() }},
		{`no dropdown "Fax"`, func(tb testing.TB) { dropdownNamed(tb, w.page, "Fax").SetSelected(0) }},
		{fmt.Sprintf("no reset button %q", defaultResetName("PDF documents")), func(tb testing.TB) { defaultResetButton(tb, w.page, "PDF documents").OnClick() }},
		{"", func(tb testing.TB) { tileNamed(tb, w.page, "Web browser") }},
		{"", func(tb testing.TB) { defaultResetButton(tb, w.page, "Web browser") }},
	} {
		fatal, panicked := fatalOf(t, c.look)
		if panicked != nil {
			t.Errorf("a lookup panicked with %v, want the failure %q", panicked, c.want)
		}
		if fatal != c.want {
			t.Errorf("a lookup failed with %q, want %q", fatal, c.want)
		}
	}
}

func TestTheDefaultAppsPageTurnsRemovableDriveMountingOffAndOn(t *testing.T) {
	k := testKit(t, "")
	sw := automountToggle(t, k)
	if !sw.On() {
		t.Fatal("the removable drives switch starts off, want on as drives.automount is by default")
	}
	sw.Toggle()
	if got := k.store.value("drives.automount"); got != false {
		t.Fatalf("turning the switch off stored %v, want false", got)
	}
	sw.Toggle()
	if got := k.store.value("drives.automount"); got != true {
		t.Errorf("turning the switch on again stored %v, want true", got)
	}
}

func TestTheRemovableDrivesSwitchReadsAnAutomountTurnedOff(t *testing.T) {
	k := testKit(t, "drives:\n  automount: false\n")
	if automountToggle(t, k).On() {
		t.Error("the removable drives switch is on with drives.automount: false")
	}
}

func automountToggle(t *testing.T, k *kit) *toggle {
	t.Helper()
	ctl := fieldControl(k, k.store, pageRow(t, defaultAppsID, "drives.automount"))
	c, ok := ctl.(*toggle)
	if !ok {
		t.Fatalf("drives.automount on the Default Apps page is %T, want a switch", ctl)
	}
	return c
}
