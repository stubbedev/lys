package settings

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/keys"
)

// shortcutPageBinds are lys-wm binds on the keys lys-wm's defaults use
// whose stored names are no keycap's, the screenshot captures and the
// launcher among them, two on one chord so a clash says it.
var shortcutPageBinds = []struct{ chord, action string }{
	{"ALT + grave", "exec foot"},
	{"SUPER + BackSpace", "kill"},
	{"SUPER + equal", "workspace 1"},
	{"SUPER + minus", "workspace 2"},
	{"SUPER + mouse:272", "exec foot"},
	{"SUPER + mouse:273", "kill"},
	{"SUPER + CTRL + mouse_down", "workspace 3"},
	{"SUPER + mouse_up", "workspace 4"},
	{"SUPER + Page_Up", "workspace 5"},
	{"SUPER + KP_Enter", "exec lys-shell screenshot region"},
	{"CTRL + bracketleft", "exec lys-shell screenshot window"},
	{"CTRL + bracketleft", "kill"},
	{"SUPER + grave", "global dev.stubbe.lys.shell:launcher"},
}

// storedKeyNames are the key names the chords store that read
// otherwise on a keycap: lys-wm's pointer names and every key name of
// the binds and of the launcher's defaults whose keycap differs, longer
// than two letters, a shorter one ("at") being a word as often as a key.
func storedKeyNames(t *testing.T) []string {
	t.Helper()
	names := []string{"mouse:", "mouse_down", "mouse_up"}
	add := func(form keys.Form, text string) {
		c, err := form.Parse(text)
		if err != nil {
			return
		}
		k, ok := c.Trigger.(keys.Key)
		if !ok {
			return
		}
		parts := strings.Split(text, "+")
		name := strings.TrimSpace(parts[len(parts)-1])
		if label, _ := widget.KeyLabel(k.Sym); len(name) > 2 && label != name && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, b := range shortcutPageBinds {
		add(keys.WM, b.chord)
	}
	for _, b := range config.LauncherDefaultKeybindings {
		for key := range strings.SplitSeq(b.Keys, ",") {
			add(keys.Rofi, key)
		}
	}
	for _, want := range []string{"grave", "BackSpace", "bracketleft", "KP_Enter", "ISO_Left_Tab", "Page_Up"} {
		if !slices.Contains(names, want) {
			t.Fatalf("the stored names %q leave out %s", names, want)
		}
	}
	return names
}

func TestNoShortcutPageShowsAStoredKeyName(t *testing.T) {
	var declared strings.Builder
	declared.WriteString("binds:\n")
	for _, b := range shortcutPageBinds {
		declared.WriteString("  - {chord: " + b.chord + ", action: " + b.action + "}\n")
	}
	k, w, _, _ := wmKit(t, declared.String())
	k.nav = layout(k.store.svc.Config(), bothParts)
	names := storedKeyNames(t)
	stored := make([]*regexp.Regexp, len(names))
	for i, n := range names {
		stored[i] = regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(n) + `($|[^A-Za-z0-9_])`)
	}
	for _, spec := range []pageSpec{fakeWMRows(w, wmBindsPage()), launcherPage(nil), screenshotPage(nil)} {
		texts := shownTexts(themed(k, buildPage(k, spec)))
		if len(texts) == 0 {
			t.Fatalf("%s shows no label", spec.id)
		}
		for _, text := range texts {
			for i, re := range stored {
				if re.MatchString(text) {
					t.Errorf("%s shows %q, the stored key name %s", spec.id, text, names[i])
				}
			}
		}
	}
}

// fakeWMRows is spec without the lys-wm rows naming a key the fake
// lys-wm's schema lacks.
func fakeWMRows(w *wmBackend, spec pageSpec) pageSpec {
	sections := slices.Clone(spec.sections)
	for i := range sections {
		sections[i].rows = slices.DeleteFunc(slices.Clone(sections[i].rows), func(r rowSpec) bool {
			_, ok := w.field(r.path)
			return r.wm && !ok
		})
	}
	spec.sections = sections
	return spec
}

func TestAClashSaysTheChordAsItReads(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: CTRL + bracketleft, action: kill}\n  - {chord: ctrl + bracketleft, action: exec foot}\n")
	got := unisolated(c.cards[1].clashMsg.Text())
	if !strings.HasPrefix(got, "Ctrl+[ is also bound by") {
		t.Fatalf("the clash reads %q", got)
	}
}

func TestAChordNothingReadsStillShowsAsKeycaps(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: HYPER + grave, action: kill}\n")
	aside := c.cards[0].aside.(*bindAside)
	if got := treetest.Labels(aside.keys); !slices.Equal(got, []string{"HYPER", "`"}) {
		t.Fatalf("the keycaps read %q", got)
	}
	if len(treetest.WithClass(aside.keys, css.Keycap)) != 2 {
		t.Fatal("the parts are not keycaps")
	}
}

// keycapLook paints the first keycap under w alone: the pixel it fills
// most of, and the pixel of its text farthest from that fill.
func keycapLook(t *testing.T, w widget.Widget) (fill, ink [4]byte) {
	t.Helper()
	caps := treetest.WithClass(w, css.Keycap)
	if len(caps) == 0 {
		t.Fatalf("%T draws no keycap", w)
	}
	l := caps[0].(*widget.Label)
	b := l.Bounds()
	cw, ch := b.X+b.W+1, b.Y+b.H+1
	data := make([]byte, render.Stride(cw)*ch)
	l.Paint(render.NewScaled(data, render.Stride(cw), cw, ch, 1, 1))
	counts := map[[4]byte]int{}
	var pixels [][4]byte
	for y := b.Y; y < b.Y+b.H; y++ {
		for x := b.X; x < b.X+b.W; x++ {
			i := y*render.Stride(cw) + 4*x
			p := [4]byte(data[i : i+4])
			counts[p]++
			pixels = append(pixels, p)
		}
	}
	for _, p := range pixels {
		if counts[p] > counts[fill] {
			fill = p
		}
	}
	distance := func(p [4]byte) int {
		d := 0
		for c := range 3 {
			d += (int(p[c]) - int(fill[c])) * (int(p[c]) - int(fill[c]))
		}
		return d
	}
	for _, p := range pixels {
		if distance(p) > distance(ink) {
			ink = p
		}
	}
	return fill, ink
}

func TestBindKeycapsAndRecorderKeycapsLookTheSame(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: SUPER + Q, action: kill}\n")
	k := c.k
	rec := k.shortcutRecorder("probe", unsetClears)
	rec.SetAccel("Super+Q")
	host := widget.NewBox(widget.Column, 0, 0)
	host.AddClass("settings-page")
	host.Append(c, false)
	host.Append(rec, false)
	themed(k, host)
	cardFill, cardInk := keycapLook(t, c.cards[0].aside.(*bindAside).keys)
	recFill, recInk := keycapLook(t, rec)
	if cardFill != recFill || cardInk != recInk {
		t.Errorf("a bind card's keycap reads %v on %v, the recorder's %v on %v", cardInk, cardFill, recInk, recFill)
	}
	fg := uint32(namedColor(host, "window_fg_color"))
	if got := uint32(cardInk[2])<<16 | uint32(cardInk[1])<<8 | uint32(cardInk[0]); got != fg&0xffffff {
		t.Errorf("the keycaps' text is %#06x, not the window's text %#06x", got, fg&0xffffff)
	}
	if bg := widget.StyleBackground(treetest.WithClass(rec, css.Keycap)[0]); bg.A() == 0 || bg != widget.StyleBackground(treetest.WithClass(c, css.Keycap)[0]) {
		t.Errorf("the keycaps' backgrounds differ or are none: %#x", uint32(bg))
	}
}

func TestEveryRecorderIsNamedAndExplained(t *testing.T) {
	k := testKit(t, "")
	rec := k.shortcutRecorder("Capture a region", unsetClears)
	if !slices.ContainsFunc(widget.DescribeTree(rec), func(st widget.A11yState) bool { return st.Name == "Capture a region" && st.Focusable }) {
		t.Errorf("the recorder is not named: %+v", widget.DescribeTree(rec))
	}
	if rec.TooltipText() != "Capture a region" {
		t.Errorf("the recorder's tooltip is %q", rec.TooltipText())
	}
}

// clearButtons are the shown clear buttons of the recorders under root.
func clearButtons(root widget.Widget) int {
	n := 0
	for _, b := range treetest.All[*widget.Button](root) {
		if widget.Describe(b).Name == "Clear shortcut" {
			n++
		}
	}
	return n
}

func TestALauncherKeyResetsInsteadOfClearing(t *testing.T) {
	const path = "launcher.keybindings"
	k := testKit(t, "launcher:\n  keybindings:\n    cancel: \"Control+q\"\n")
	c := fieldControl(k, k.store, field(path, launcherKeysRow)).(*launcherBindings)
	if n := clearButtons(c); n != 0 {
		t.Fatalf("%d clear buttons on the launcher's keys, want the resets alone", n)
	}
	i := slices.IndexFunc(config.LauncherDefaultKeybindings, func(b config.LauncherBinding) bool { return b.Action == "cancel" })
	rec := treetest.All[*widget.ShortcutRecorder](c)[i]
	if rec.Accel() != "Ctrl+Q" || !c.resets[i].shown() {
		t.Fatalf("the override reads %q, reset shown %v", rec.Accel(), c.resets[i].shown())
	}
	rec.StartRecording()
	rec.RecordPress(xkb.KeyBackSpace, 0)
	if got, _ := k.store.value(path).(map[string]any); len(got) != 0 {
		t.Fatalf("Backspace left %v, want the default back", got)
	}
	if rec.Accel() != "Escape" || c.resets[i].shown() {
		t.Errorf("after Backspace the key reads %q, reset shown %v", rec.Accel(), c.resets[i].shown())
	}
}

func TestALauncherKeyLysShellCannotReadSaysSo(t *testing.T) {
	k := testKit(t, "launcher:\n  keybindings:\n    cancel: \"Hyper+bracketleft\"\n")
	c := fieldControl(k, k.store, field("launcher.keybindings", launcherKeysRow)).(*launcherBindings)
	i := slices.IndexFunc(config.LauncherDefaultKeybindings, func(b config.LauncherBinding) bool { return b.Action == "cancel" })
	rec := treetest.All[*widget.ShortcutRecorder](c)[i]
	if rec.Accel() != "" || unisolated(rec.Error()) != "lys-shell does not read Hyper+[ as a key" {
		t.Errorf("an unreadable key reads %q with %q", rec.Accel(), unisolated(rec.Error()))
	}
}

func TestEveryLauncherDefaultKeyIsReadAndHasAKeycap(t *testing.T) {
	for _, b := range config.LauncherDefaultKeybindings {
		for key := range strings.SplitSeq(b.Keys, ",") {
			c, err := keys.Rofi.Parse(key)
			if err != nil {
				t.Errorf("%s: %v", b.Action, err)
				continue
			}
			sym := c.Trigger.(keys.Key).Sym
			if label, ok := widget.KeyLabel(sym); !ok {
				t.Errorf("%s: %s has no keycap, it reads %q", b.Action, key, label)
			}
		}
	}
}

func TestTheWayToEveryShortcutEndsBothShortcutLists(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	for _, row := range []rowSpec{launcherShortcutRow(), screenshotShortcutsRow()} {
		b := fieldControl(k, w, row).(*bindRecorders)
		rows := treetest.All[*settingRow](b)
		links := treetest.All[*pageLinkRow](b)
		if len(rows) != len(b.chords) || len(links) != 1 || links[0].page != wmBindsID {
			t.Errorf("%s holds %d rows for %d recorders and the links %v, want one to every shortcut", row.key, len(rows), len(b.chords), links)
		}
	}
	k.nav = layout(k.store.svc.Config(), parts{shell: true})
	if links := treetest.All[*pageLinkRow](fieldControl(k, w, launcherShortcutRow())); len(links) != 0 {
		t.Errorf("without the Keyboard Shortcuts page the recorders still link to it: %v", links)
	}
}
