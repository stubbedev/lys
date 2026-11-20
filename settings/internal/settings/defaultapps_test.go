package settings

import (
	"errors"
	"os"
	"path/filepath"
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

func dropdownNamed(t *testing.T, root widget.Widget, name string) *widget.Dropdown {
	t.Helper()
	for _, d := range treetest.All[*widget.Dropdown](root) {
		if d.AccessibleName() == name {
			return d
		}
	}
	t.Fatalf("no dropdown %q", name)
	return nil
}

func tileNamed(t *testing.T, root widget.Widget, name string) *appTile {
	t.Helper()
	for _, tile := range treetest.All[*appTile](root) {
		if tile.button.AccessibleName() == name {
			return tile
		}
	}
	t.Fatalf("no app tile %q", name)
	return nil
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
	if resetButton(w.page, terminal) != nil {
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
	resetButton(w.page, terminal).OnClick()
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

func resetButton(root widget.Widget, title string) *widget.Button {
	name := locale.T().Get("settings-default-reset-named", i18n.Str("name", title))
	for _, b := range treetest.All[*widget.Button](root) {
		if b.AccessibleName() == name {
			return b
		}
	}
	return nil
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
	if resetButton(w.page, "PDF documents") != nil {
		t.Fatal("a category the user never chose for offers a reset")
	}
	web := resetButton(w.page, "Web browser")
	if web == nil {
		t.Fatal("the web browser the user chose offers no reset")
	}
	web.OnClick()
	got := fileText(t, filepath.Join(config, "mimeapps.list"))
	if strings.Contains(got, "x-scheme-handler/https") || !strings.Contains(got, "image/png=chromium.desktop;") {
		t.Fatalf("after the reset the user's list reads %q", got)
	}
	if resetButton(w.page, "Web browser") != nil {
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
	resetButton(w.page, "Web browser").OnClick()
	if labels := strings.Join(treetest.Labels(w.page), "|"); !strings.Contains(labels, "the list is read-only") {
		t.Fatalf("a failed reset left the page reading %q", labels)
	}
	if resetButton(w.page, "Web browser") == nil {
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
	if resetButton(w.page, "Web browser") != nil {
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
