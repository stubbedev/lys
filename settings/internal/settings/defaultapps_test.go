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
	w := defaultAppsWindow(t)
	if scans != 1 {
		t.Fatalf("%d scans of the applications for one page, want one", scans)
	}
	web := dropdownNamed(t, w.page, "Web browser")
	mixed := locale.T().Get("settings-default-mixed")
	if got := web.Items(); !slices.Equal(got, []string{mixed, "Chromium", "Firefox"}) || web.Selected() != 0 {
		t.Fatalf("web browsers %q selected %d, want Mixed: https opens Firefox, http Chromium", got, web.Selected())
	}
	if pdf := dropdownNamed(t, w.page, "PDF documents"); pdf.Items()[pdf.Selected()] != "Evince" {
		t.Fatalf("the only PDF viewer is not shown as the default: %q", pdf.Items())
	}
	web.SetSelected(1)
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
	if web := dropdownNamed(t, w.page, "Web browser"); !slices.Equal(web.Items(), []string{"Chromium", "Firefox"}) || web.Selected() != 0 {
		t.Fatalf("after the pick the web browsers read %q selected %d, want Chromium alone", web.Items(), web.Selected())
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

func resetButton(root widget.Widget, title string) *widget.Button {
	name := locale.T().Get("settings-default-reset-named", i18n.Str("name", title))
	for _, b := range treetest.All[*widget.Button](root) {
		if b.AccessibleName() == name {
			return b
		}
	}
	return nil
}

func defaultAppsWindow(t *testing.T) *window {
	t.Helper()
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	w.show(defaultAppsID)
	w.root.Arrange(render.Rect{W: 1400, H: 6000})
	return w
}

func TestResetGivesACategoryBackToTheSystemDefault(t *testing.T) {
	config := defaultAppsFixture(t)
	w := defaultAppsWindow(t)
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
	if dd := dropdownNamed(t, w.page, "Web browser"); dd.Items()[dd.Selected()] != "Chromium" {
		t.Errorf("after the reset the web browser reads %q, want the first that declares the types", dd.Items()[dd.Selected()])
	}
}

func TestAResetThatFailsIsSaid(t *testing.T) {
	defaultAppsFixture(t)
	before := resetAssociations
	resetAssociations = func([]string) error { return errors.New("the list is read-only") }
	t.Cleanup(func() { resetAssociations = before })
	w := defaultAppsWindow(t)
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
	w := defaultAppsWindow(t)
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
