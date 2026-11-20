package settings

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const screenshotNamePath = "modules.screenshot.filename-format"

func TestEveryFilenameTheSimpleFormWritesReadsBackAsItsParts(t *testing.T) {
	for _, prefix := range []string{"", "Screenshot_", "shot-"} {
		for _, d := range filenameDates {
			for _, c := range filenameTimes {
				p := filenameParts{prefix: prefix, date: d, clock: c}
				if prefix == "" && p.numbered() {
					continue
				}
				got, ok := parseFilename(p.format())
				if !ok || got != p {
					t.Errorf("%q reads as %+v %v, want %+v", p.format(), got, ok, p)
				}
			}
		}
	}
	if p, ok := parseFilename(shellDefault(screenshotNamePath)); !ok || p != (filenameParts{"Screenshot_", "%Y-%m-%d", "%H-%M-%S"}) {
		t.Errorf("the default reads as %+v %v", p, ok)
	}
}

func TestAFilenameTheSimpleFormCannotShowIsNotParts(t *testing.T) {
	for _, format := range []string{"shot-%j.png", "Screenshot.jpg", ".png", "%Y/%m/shot.png", "100%%_%Y-%m-%d.png", "%H-%M-%S_%Y-%m-%d.png"} {
		if p, ok := parseFilename(format); ok {
			t.Errorf("%q reads as %+v", format, p)
		}
	}
}

func filenameControl(t *testing.T, k *kit) *filenameEditor {
	t.Helper()
	e, ok := fieldControl(k, k.store, pageRow(t, "screenshot", screenshotNamePath)).(*filenameEditor)
	if !ok {
		t.Fatal("the screenshot filename is not the filename form")
	}
	return e
}

func TestTheFilenameFormWritesItsPartsAndPreviewsTheFullPath(t *testing.T) {
	k := testKit(t, "modules:\n  screenshot:\n    output-directory: /shots\n")
	e := filenameControl(t, k)
	tr := locale.T()
	if !e.simple.Visible() || e.notice.Visible() || e.custom.Visible() {
		t.Fatal("the default name is not shown as parts")
	}
	e.date.SetSelected(slices.Index(filenameDates, "%Y%m%d"))
	e.write()
	if v := k.store.value(screenshotNamePath); v != "Screenshot_%Y%m%d_%H-%M-%S.png" {
		t.Fatalf("stored %v", v)
	}
	if want := tr.Get("settings-filename-path", i18n.Str("path", filepath.Join("/shots", "Screenshot_20260314_15-09-26.png"))); e.path.Text() != want {
		t.Fatalf("previews %q, want %q", e.path.Text(), want)
	}
	e.date.SetSelected(0)
	e.clock.SetSelected(0)
	e.write()
	if v := k.store.value(screenshotNamePath); v != "Screenshot_.png" {
		t.Fatalf("stored %v", v)
	}
	if want := tr.Get("settings-filename-path-numbered", i18n.Str("path", filepath.Join("/shots", "Screenshot_.png"))); e.path.Text() != want {
		t.Fatalf("a numbered name previews %q", e.path.Text())
	}
}

func TestTheFilenameFormRefusesANameNoFileCanHave(t *testing.T) {
	k := testKit(t, "")
	e := filenameControl(t, k)
	before := k.store.value(screenshotNamePath)
	tr := locale.T()
	for prefix, key := range map[string]string{"shots/": "settings-filename-slash", "100%": "settings-filename-percent"} {
		e.prefix.SetText(prefix)
		if !e.problem.Visible() || e.problem.Text() != tr.Get(key) {
			t.Errorf("%q shows %q", prefix, e.problem.Text())
		}
		if v := k.store.value(screenshotNamePath); v != before {
			t.Errorf("%q stored %v", prefix, v)
		}
	}
	e.syncing = true
	e.date.SetSelected(0)
	e.clock.SetSelected(0)
	e.syncing = false
	e.prefix.SetText("")
	if e.problem.Text() != tr.Get("settings-filename-empty") || k.store.value(screenshotNamePath) != before {
		t.Fatalf("an empty name shows %q and stored %v", e.problem.Text(), k.store.value(screenshotNamePath))
	}
}

func TestAFilenameBeyondTheFormOpensAsAFormatUntouched(t *testing.T) {
	k := testKit(t, "modules:\n  screenshot:\n    filename-format: \"shot-%j.png\"\n")
	e := filenameControl(t, k)
	if e.simple.Visible() || !e.notice.Visible() || !e.custom.Visible() || e.toggle.Visible() {
		t.Fatal("a custom name is not shown as its format")
	}
	if v := k.store.value(screenshotNamePath); v != "shot-%j.png" {
		t.Fatalf("a custom name was rewritten to %v", v)
	}
}
