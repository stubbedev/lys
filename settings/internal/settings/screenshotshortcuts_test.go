package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

func screenshotChord(t *testing.T, s *screenshotShortcuts, sub string) *actionChord {
	t.Helper()
	i := slices.IndexFunc(screenshotCaptures, func(c screenshotCapture) bool { return c.sub == sub })
	if i < 0 {
		t.Fatalf("no capture %q", sub)
	}
	return s.chords[i]
}

func TestTheScreenshotPageRecordsTheChordOfEachCapture(t *testing.T) {
	k, w, _, files := wmKit(t, "binds:\n  - {chord: SUPER + Print, action: exec lys-shell screenshot region}\n  - {chord: SUPER + Q, action: kill}\n")
	s := fieldControl(k, w, screenshotShortcutsRow()).(*screenshotShortcuts)
	if len(s.chords) != len(screenshotCaptures) {
		t.Fatalf("%d recorders for %d captures", len(s.chords), len(screenshotCaptures))
	}
	region, window, text := screenshotChord(t, s, "region"), screenshotChord(t, s, "window"), screenshotChord(t, s, "text")
	if region.recorder.Accel() != "Super+Print" || window.recorder.Accel() != "" || region.clash.Visible() {
		t.Fatalf("the region reads %q and the window %q", region.recorder.Accel(), window.recorder.Accel())
	}
	window.recorder.StartRecording()
	window.recorder.RecordPress(xkb.Keysym('W'), widget.ModSuper)
	got := fileText(t, files.runtime)
	if !strings.Contains(got, "exec lys-shell screenshot window") || !strings.Contains(got, "SUPER + W") || !strings.Contains(got, "exec lys-shell screenshot region") {
		t.Fatalf("binding the window capture wrote %q", got)
	}
	text.recorder.StartRecording()
	text.recorder.RecordPress(xkb.Keysym('Q'), widget.ModSuper)
	s.refresh()
	if !text.clash.Visible() || !strings.Contains(unisolated(text.clash.Text()), "Close window") {
		t.Fatalf("a chord another bind holds reads %v %q", text.clash.Visible(), text.clash.Text())
	}
	region.recorder.StartRecording()
	region.recorder.RecordPress(xkb.KeyBackSpace, 0)
	if got := fileText(t, files.runtime); strings.Contains(got, "screenshot region") || !strings.Contains(got, "kill") || !strings.Contains(got, "screenshot window") {
		t.Fatalf("unbinding the region capture wrote %q", got)
	}
}

func TestTheScreenshotShortcutsLinkToEveryShortcutOnlyWhereThatPageIs(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	var opened []string
	k.openPage = func(id string) { opened = append(opened, id) }
	buttons := treetest.All[*widget.Button](fieldControl(k, w, screenshotShortcutsRow()))
	if len(buttons) != 1 {
		t.Fatalf("%d buttons beside the captures, want the link", len(buttons))
	}
	buttons[0].OnClick()
	if !slices.Equal(opened, []string{wmBindsID}) {
		t.Errorf("opened %q", opened)
	}
	k.nav = nil
	if buttons := treetest.All[*widget.Button](fieldControl(k, w, screenshotShortcutsRow())); len(buttons) != 0 {
		t.Errorf("without the shortcuts page %d buttons remain", len(buttons))
	}
}

func TestTheScreenshotShortcutsAreLysWMsAndLeaveWithoutIt(t *testing.T) {
	section := func(installed parts) (sectionSpec, bool) {
		p, ok := pageByID(layout(config.Defaults(), installed), "screenshot")
		if !ok {
			t.Fatal("no screenshot page")
		}
		i := slices.IndexFunc(p.sections, func(s sectionSpec) bool { return s.title == "settings-section-screenshot-shortcuts" })
		if i < 0 {
			return sectionSpec{}, false
		}
		return p.sections[i], true
	}
	s, ok := section(bothParts)
	if !ok || len(s.rows) != 1 || !s.rows[0].wm || s.rows[0].path != bindsKey {
		t.Fatalf("with lys-wm the shortcuts section is %+v", s)
	}
	if _, ok := section(parts{shell: true}); ok {
		t.Error("without lys-wm the screenshot page still offers its binds")
	}
}

func TestTheScreenshotButtonOffersEveryCapture(t *testing.T) {
	var got []string
	for _, c := range actionChoices("screenshot") {
		got = append(got, c.command)
	}
	want := []string{"lys-shell screenshot region", "lys-shell screenshot output", "lys-shell screenshot window", "lys-shell screenshot text", "lys-shell screenshot qr"}
	if !slices.Equal(got, want) {
		t.Errorf("the screenshot button offers %q, want %q", got, want)
	}
}
