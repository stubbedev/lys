package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fontStore struct {
	files    map[string]string
	families map[string]string
}

func (s fontStore) fontFile(family string) (string, error) {
	if path, ok := s.files[family]; ok {
		return path, nil
	}
	return "", errors.New("not installed")
}

func (s fontStore) fileFamily(path string) (string, error) {
	if family, ok := s.families[path]; ok {
		return family, nil
	}
	return "", errors.New("not a font lys-wm reads")
}

func tabFontKit(t *testing.T, config string) (*kit, *wmBackend, *fakePickers) {
	t.Helper()
	k, w, _, _ := wmKit(t, config)
	store := fontStore{
		files:    map[string]string{"Inter": "/fonts/Inter-Regular.ttf", "Noto Sans CJK": "/fonts/NotoSansCJK.ttc"},
		families: map[string]string{"/fonts/Inter-Regular.ttf": "Inter", "/home/me/.fonts/Mine.ttf": "Mine"},
	}
	k.fontFile, k.fileFamily = store.fontFile, store.fileFamily
	f := &fakePickers{}
	k.pickers = f
	return k, w, f
}

func TestTheTabFontIsPickedByFamilyAndWrittenAsItsFile(t *testing.T) {
	k, w, f := tabFontKit(t, "")
	c := fieldControl(k, w, wmField("tabs.font")).(*wmFontEditor)
	if got := c.file.Text(); got != locale.T().Get("wm-tabs-font-built-in") {
		t.Errorf("an unset tab font reads %q, want lys-wm's built-in font", got)
	}
	c.picker.open()
	if f.fontFn == nil || f.fontMono {
		t.Fatal("the font dialog did not open for every family")
	}
	f.fontFn("Inter")
	if got := w.value("tabs.font"); got != "/fonts/Inter-Regular.ttf" {
		t.Fatalf("picking Inter wrote %v, want its regular face's file", got)
	}
	c.refresh()
	if c.picker.label.Text() != "Inter" || c.file.Text() != "/fonts/Inter-Regular.ttf" || c.problem.Visible() {
		t.Errorf("the picked font reads %q from %q, problem shown %v", c.picker.label.Text(), c.file.Text(), c.problem.Visible())
	}
}

func TestATabFontLysWMCannotReadIsNotWritten(t *testing.T) {
	k, w, f := tabFontKit(t, "tabs:\n  font: /fonts/Inter-Regular.ttf\n")
	c := fieldControl(k, w, wmField("tabs.font")).(*wmFontEditor)
	tr := locale.T()
	for family, key := range map[string]string{"Noto Sans CJK": "wm-tabs-font-unreadable", "Nowhere Sans": "wm-tabs-font-unresolved"} {
		c.picker.open()
		f.fontFn(family)
		if got := w.value("tabs.font"); got != "/fonts/Inter-Regular.ttf" {
			t.Errorf("picking %s wrote %v", family, got)
		}
		if want := tr.Get(key, i18n.Str("family", family)); !c.problem.Visible() || !strings.HasPrefix(c.problem.Text(), want) {
			t.Errorf("picking %s says %q, want it to start %q", family, c.problem.Text(), want)
		}
	}
}

func TestATabFontFileOutsideTheFamiliesIsBrowsed(t *testing.T) {
	k, w, f := tabFontKit(t, "")
	c := fieldControl(k, w, wmField("tabs.font")).(*wmFontEditor)
	c.browse.ClickAt(widget.Point{})
	if f.fileFn == nil {
		t.Fatal("no file chooser")
	}
	f.fileFn("/home/me/.fonts/Mine.ttf")
	c.refresh()
	if got := w.value("tabs.font"); got != "/home/me/.fonts/Mine.ttf" || c.picker.label.Text() != "Mine" {
		t.Errorf("browsing wrote %v reading %q", got, c.picker.label.Text())
	}
}

func TestAFontFilesFamilyIsReadFromTheFile(t *testing.T) {
	dir := t.TempDir()
	font, text := filepath.Join(dir, "Go-Regular.ttf"), filepath.Join(dir, "readme.ttf")
	if err := os.WriteFile(font, goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(text, []byte("not a font"), 0o600); err != nil {
		t.Fatal(err)
	}
	if family, err := readFontFamily(font); err != nil || family != "Go" {
		t.Errorf("Go Regular reads as %q, %v", family, err)
	}
	for _, path := range []string{text, filepath.Join(dir, "missing.ttf")} {
		if family, err := readFontFamily(path); err == nil {
			t.Errorf("%s reads as the family %q", path, family)
		}
	}
}
