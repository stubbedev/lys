package widget

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/xkb"

	"github.com/stubbedev/lys/ui/widget/css"
)

func recorderKeycaps(r *ShortcutRecorder) []string {
	var out []string
	walkTree(r.display, func(w Widget, _ int) {
		if l, ok := w.(*Label); ok && l.hasClass(css.Keycap) {
			out = append(out, l.Text())
		}
	})
	return out
}

func recorderPrompt(r *ShortcutRecorder) string {
	var out string
	walkTree(r.display, func(w Widget, _ int) {
		if l, ok := w.(*Label); ok && l.hasClass("prompt") {
			out = l.Text()
		}
	})
	return out
}

func TestShortcutRecorderCommitsTheFirstKeyWithItsModifiers(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "")
	var changes []string
	r.OnChanged = func(a string) { changes = append(changes, a) }
	if got := recorderPrompt(r); got != "Unassigned" {
		t.Fatalf("resting empty recorder shows %q, want Unassigned", got)
	}
	r.RecordPress(xkb.Keysym('p'), ModCtrl|ModAlt)
	if len(changes) != 0 {
		t.Fatalf("a press at rest committed %v", changes)
	}

	r.button.ClickAt(Point{})
	if !r.Recording() || recorderPrompt(r) != "Press a shortcut…" {
		t.Fatalf("a click did not start recording: recording=%v prompt=%q", r.Recording(), recorderPrompt(r))
	}
	r.RecordPress(xkb.KeyControlL, 0)
	r.RecordPress(xkb.KeyAltL, ModCtrl)
	r.RecordPress(xkb.Keysym('p'), ModCtrl|ModAlt)

	if !slices.Equal(changes, []string{"Ctrl+Alt+P"}) {
		t.Fatalf("OnChanged saw %v, want [Ctrl+Alt+P]", changes)
	}
	if r.Recording() || r.Accel() != "Ctrl+Alt+P" {
		t.Errorf("after the commit recording=%v accel=%q", r.Recording(), r.Accel())
	}
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Ctrl", "Alt", "P"}) {
		t.Errorf("keycaps %v, want [Ctrl Alt P]", got)
	}
	if !slices.Contains(r.row.Children(), Widget(r.clear)) {
		t.Error("an assigned recorder hides its clear button")
	}
}

func TestShortcutRecorderEscapeKeepsTheOldValue(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Q")
	fired := 0
	r.OnChanged = func(string) { fired++ }
	r.button.KeyAction(KeyEnter, 0)
	if !r.Recording() {
		t.Fatal("Enter did not start recording")
	}
	r.RecordPress(xkb.KeyShiftL, 0)
	r.RecordPress(xkb.KeyEscape, 0)
	if r.Recording() || r.Accel() != "Ctrl+Q" || fired != 0 {
		t.Errorf("escape: recording=%v accel=%q OnChanged fired %d times", r.Recording(), r.Accel(), fired)
	}
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Ctrl", "Q"}) {
		t.Errorf("keycaps after cancel %v, want [Ctrl Q]", got)
	}
}

func TestShortcutRecorderBackspaceAndClearUnassign(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Q")
	var changes []string
	r.OnChanged = func(a string) { changes = append(changes, a) }
	r.button.InsertRune(' ')
	if !r.Recording() {
		t.Fatal("Space did not start recording")
	}
	r.RecordPress(xkb.KeyBackSpace, 0)
	if r.Recording() || r.Accel() != "" || !slices.Equal(changes, []string{""}) {
		t.Fatalf("backspace: recording=%v accel=%q changes=%v", r.Recording(), r.Accel(), changes)
	}
	if slices.Contains(r.row.Children(), Widget(r.clear)) {
		t.Error("an unassigned recorder still offers its clear button")
	}

	r.SetAccel("Super+F9")
	if !slices.Equal(changes, []string{""}) {
		t.Errorf("SetAccel fired OnChanged: %v", changes)
	}
	r.clear.ClickAt(Point{})
	if r.Accel() != "" || !slices.Equal(changes, []string{"", ""}) {
		t.Errorf("clear button: accel=%q changes=%v", r.Accel(), changes)
	}

	r.StartRecording()
	r.RecordPress(xkb.KeyBackSpace, ModCtrl)
	if r.Accel() != "Ctrl+BackSpace" {
		t.Errorf("Ctrl+Backspace is a shortcut, not a clear: accel=%q", r.Accel())
	}
}

func TestShortcutRecorderPreviewsHeldModifiersWithoutCommitting(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "")
	fired := 0
	r.OnChanged = func(string) { fired++ }
	r.StartRecording()
	r.RecordPress(xkb.KeyControlL, 0)
	r.RecordPress(xkb.KeyShiftL, ModCtrl)
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Ctrl", "Shift"}) {
		t.Errorf("held Ctrl+Shift previews %v", got)
	}
	r.RecordRelease(xkb.KeyShiftL)
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Ctrl"}) {
		t.Errorf("after releasing Shift the preview is %v, want [Ctrl]", got)
	}
	r.RecordRelease(xkb.KeyControlL)
	if !r.Recording() || fired != 0 || r.Accel() != "" {
		t.Fatalf("modifier-only presses committed: recording=%v fired=%d accel=%q", r.Recording(), fired, r.Accel())
	}
	if got := recorderPrompt(r); got != "Press a shortcut…" {
		t.Errorf("with nothing held the recorder shows %q, want the prompt again", got)
	}
	if len(recorderKeycaps(r)) != 0 {
		t.Errorf("released modifiers still preview %v", recorderKeycaps(r))
	}
}

func TestShortcutRecorderValidateKeepsRecording(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "")
	var changes []string
	r.OnChanged = func(a string) { changes = append(changes, a) }
	r.Validate = func(a string) error {
		if a == "P" {
			return errors.New("a shortcut needs a modifier")
		}
		return nil
	}
	r.StartRecording()
	r.RecordPress(xkb.Keysym('p'), 0)
	if !r.Recording() || len(changes) != 0 {
		t.Fatalf("a refused press committed: recording=%v changes=%v", r.Recording(), changes)
	}
	if r.Error() != "a shortcut needs a modifier" || !slices.Contains(r.box.Children(), Widget(r.err)) {
		t.Errorf("the refusal is not shown: error=%q", r.Error())
	}
	r.RecordPress(xkb.Keysym('p'), ModSuper)
	if r.Recording() || !slices.Equal(changes, []string{"Super+P"}) {
		t.Fatalf("a valid press after a refusal: recording=%v changes=%v", r.Recording(), changes)
	}
	if r.Error() != "" || slices.Contains(r.box.Children(), Widget(r.err)) {
		t.Errorf("the refusal outlived the commit: %q", r.Error())
	}
}

func TestShortcutRecorderLosingFocusCancels(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Q")
	other := NewButton(NewLabel(chromeFace(t), 14, "Other", 0), 0, 0)
	root := NewBox(Column, 0, 0).Append(r, false).Append(other, false)
	router := &Router{Root: root}
	if err := router.SetFocus(r.button); err != nil {
		t.Fatal(err)
	}
	r.StartRecording()
	if !r.Recording() || router.Focused() != Widget(r.button) {
		t.Fatalf("starting to record lost focus: recording=%v focus=%T", r.Recording(), router.Focused())
	}
	if err := router.SetFocus(other); err != nil {
		t.Fatal(err)
	}
	if r.Recording() || r.Accel() != "Ctrl+Q" {
		t.Errorf("focus left: recording=%v accel=%q", r.Recording(), r.Accel())
	}
}

func TestShortcutRecorderAccessibility(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Alt+P")
	r.SetAccessibleName("Screenshot")
	st := Describe(r.button)
	if st.Role != RoleButton || st.Name != "Screenshot" || st.Description != "Ctrl+Alt+P" || !st.Focusable {
		t.Errorf("at rest: %+v", st)
	}
	r.SetAccel("Ctrl+bracketleft")
	if st := Describe(r.button); st.Description != "Ctrl+[" {
		t.Errorf("the description reads the stored key name: %q", st.Description)
	}
	gen := A11yGeneration()
	r.StartRecording()
	if A11yGeneration() == gen {
		t.Error("starting to record did not mark the accessibility tree changed")
	}
	if st := Describe(r.button); st.Description != "Recording, press a shortcut" {
		t.Errorf("recording description %q", st.Description)
	}
	r.RecordPress(xkb.KeyBackSpace, 0)
	if st := Describe(r.button); st.Description != "Unassigned" {
		t.Errorf("cleared description %q", st.Description)
	}
}

func TestAnUnclearableRecorderHasNoClearButton(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Q")
	r.SetClearable(false)
	if slices.Contains(r.row.Children(), Widget(r.clear)) {
		t.Fatal("an unclearable recorder shows its clear button")
	}
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Ctrl", "Q"}) {
		t.Errorf("keycaps %v, want [Ctrl Q]", got)
	}
	r.SetClearable(true)
	if !slices.Contains(r.row.Children(), Widget(r.clear)) {
		t.Error("a clearable recorder with a value hides its clear button")
	}
}

func TestAccelLabel(t *testing.T) {
	cases := []struct {
		sym  xkb.Keysym
		mods Mods
		want string
	}{
		{xkb.Keysym('p'), ModCtrl | ModAlt, "Ctrl+Alt+P"},
		{xkb.KeyF9, ModSuper, "Super+F9"},
		{xkb.KeyReturn, ModCtrl | ModShift, "Ctrl+Shift+Return"},
		{xkb.KeyEscape, 0, "Escape"},
	}
	for _, c := range cases {
		if got := AccelLabel(c.sym, c.mods); got != c.want {
			t.Errorf("AccelLabel(%v, %v) = %q, want %q", c.sym, c.mods, got, c.want)
		}
	}
	if IsModifierKeysym(xkb.Keysym('p')) || !IsModifierKeysym(xkb.KeySuperR) {
		t.Error("IsModifierKeysym misclassifies")
	}
}

func TestShortcutRecorderCancelKeepsTheCallersMessage(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Alt+P")
	r.SetError("Another shortcut already uses Ctrl+Alt+P.")
	r.Validate = func(a string) error {
		if a == "P" {
			return errors.New("needs a modifier")
		}
		return nil
	}
	r.StartRecording()
	r.RecordPress(xkb.Keysym('p'), 0)
	if r.Error() != "needs a modifier" {
		t.Fatalf("while recording the message is %q", r.Error())
	}
	r.RecordPress(xkb.KeyEscape, 0)
	if r.Error() != "Another shortcut already uses Ctrl+Alt+P." || r.Accel() != "Ctrl+Alt+P" {
		t.Errorf("a cancel left %q on %q, want the refusal back on the old value", r.Error(), r.Accel())
	}
	r.StartRecording()
	r.RecordPress(xkb.KeyF9, ModSuper)
	if r.Error() != "" || r.Accel() != "Super+F9" {
		t.Errorf("a new key left %q on %q, want no message", r.Error(), r.Accel())
	}
}

func tapRecorder(t *testing.T, accel string) (*ShortcutRecorder, *[]Tap, *[]string) {
	t.Helper()
	r := NewShortcutRecorder(chromeFace(t), accel)
	r.SetRecordTaps(true)
	var taps []Tap
	var changes []string
	r.OnTap = func(tap Tap) { taps = append(taps, tap) }
	r.OnChanged = func(a string) { changes = append(changes, a) }
	r.StartRecording()
	return r, &taps, &changes
}

func TestARecorderRecordsAModifierPressedAndReleasedAloneAsATap(t *testing.T) {
	r, taps, changes := tapRecorder(t, "Ctrl+Q")
	r.RecordPress(xkb.KeySuperL, 0)
	if !r.Recording() || len(*taps) != 0 {
		t.Fatalf("a press alone committed: recording=%v taps=%v", r.Recording(), *taps)
	}
	r.RecordRelease(xkb.KeySuperL)
	want := NewTap(ModSuper, 0)
	if r.Recording() || !slices.Equal(*taps, []Tap{want}) || len(*changes) != 0 {
		t.Fatalf("the release: recording=%v taps=%v changes=%v", r.Recording(), *taps, *changes)
	}
	if got, ok := r.Tap(); !ok || got != want || r.Accel() != "" {
		t.Errorf("the recorder holds tap %v %v, accel %q", got, ok, r.Accel())
	}
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Super"}) {
		t.Errorf("keycaps %v, want [Super]", got)
	}
	if st := Describe(r.button); st.Description != "Super" {
		t.Errorf("the description reads %q", st.Description)
	}
	if !slices.Contains(r.row.Children(), Widget(r.clear)) {
		t.Error("a recorder holding a tap hides its clear button")
	}
	r.StartRecording()
	r.RecordPress(xkb.KeySuperR, 0)
	r.RecordRelease(xkb.KeySuperR)
	if len(*taps) != 1 {
		t.Errorf("recording the same tap again fired OnTap: %v", *taps)
	}
}

func TestATapIsOfTheLastModifierWithTheOthersHeld(t *testing.T) {
	r, taps, _ := tapRecorder(t, "")
	r.RecordPress(xkb.KeyControlL, 0)
	r.RecordPress(xkb.KeyAltL, ModCtrl)
	r.RecordPress(xkb.KeySuperL, ModCtrl|ModAlt)
	r.RecordRelease(xkb.KeySuperL)
	want := NewTap(ModSuper, ModCtrl|ModAlt)
	if !slices.Equal(*taps, []Tap{want}) {
		t.Fatalf("taps %v, want [%v]", *taps, want)
	}
	if got := recorderKeycaps(r); !slices.Equal(got, []string{"Ctrl", "Alt", "Super"}) {
		t.Errorf("keycaps %v, want [Ctrl Alt Super]", got)
	}
	if want.String() != "Ctrl+Alt+Super" {
		t.Errorf("the tap reads %q", want.String())
	}
}

func TestAnyPressBetweenAModifiersPressAndReleaseCancelsItsTap(t *testing.T) {
	r, taps, _ := tapRecorder(t, "")
	r.RecordPress(xkb.KeySuperL, 0)
	r.RecordPress(xkb.KeyShiftL, ModSuper)
	r.RecordRelease(xkb.KeyShiftL)
	r.RecordRelease(xkb.KeySuperL)
	if !slices.Equal(*taps, []Tap{NewTap(ModShift, ModSuper)}) {
		t.Fatalf("taps %v: a modifier pressed after Super is the tap, Super's is gone", *taps)
	}

	r, taps, changes := tapRecorder(t, "")
	r.RecordPress(xkb.KeySuperL, 0)
	r.RecordPress(xkb.KeyCapsLock, ModSuper)
	r.RecordRelease(xkb.KeySuperL)
	if len(*taps) != 0 || !r.Recording() {
		t.Fatalf("Caps Lock between Super's press and release: taps %v recording %v", *taps, r.Recording())
	}

	r.RecordPress(xkb.KeySuperL, 0)
	r.Validate = func(string) error { return errors.New("refused") }
	r.RecordPress(xkb.Keysym('k'), ModSuper)
	r.RecordRelease(xkb.KeySuperL)
	if len(*taps) != 0 || len(*changes) != 0 || !r.Recording() {
		t.Fatalf("a refused key between Super's press and release: taps %v changes %v recording %v", *taps, *changes, r.Recording())
	}
	r.Validate = nil
	r.RecordPress(xkb.KeySuperL, 0)
	r.RecordPress(xkb.Keysym('k'), ModSuper)
	if len(*taps) != 0 || !slices.Equal(*changes, []string{"Super+K"}) {
		t.Fatalf("Super+K: taps %v changes %v", *taps, *changes)
	}
}

func TestARecorderRecordsNoTapUnlessAskedTo(t *testing.T) {
	r := NewShortcutRecorder(chromeFace(t), "Ctrl+Q")
	var taps []Tap
	r.OnTap = func(tap Tap) { taps = append(taps, tap) }
	r.StartRecording()
	r.RecordPress(xkb.KeyControlL, 0)
	r.RecordPress(xkb.KeySuperL, ModCtrl)
	r.RecordRelease(xkb.KeySuperL)
	r.RecordRelease(xkb.KeyControlL)
	if len(taps) != 0 || !r.Recording() || r.Accel() != "Ctrl+Q" {
		t.Fatalf("a recorder not recording taps: taps %v recording %v accel %q", taps, r.Recording(), r.Accel())
	}
	if _, ok := r.Tap(); ok {
		t.Error("a recorder not recording taps holds one")
	}
}

func TestARecordedKeyOrClearReplacesATap(t *testing.T) {
	r, _, changes := tapRecorder(t, "")
	r.SetTap(NewTap(ModSuper, ModCtrl))
	if got := recorderKeycaps(r); r.Recording() || !slices.Equal(got, []string{"Ctrl", "Super"}) {
		t.Fatalf("SetTap: recording %v keycaps %v", r.Recording(), got)
	}
	r.StartRecording()
	r.RecordPress(xkb.KeyF9, ModSuper)
	if _, ok := r.Tap(); ok || r.Accel() != "Super+F9" || !slices.Equal(*changes, []string{"Super+F9"}) {
		t.Fatalf("a key over a tap: accel %q changes %v", r.Accel(), *changes)
	}
	r.SetTap(NewTap(ModAlt, 0))
	r.clear.ClickAt(Point{})
	if _, ok := r.Tap(); ok || r.Accel() != "" || !slices.Equal(*changes, []string{"Super+F9", ""}) {
		t.Fatalf("clearing a tap: accel %q changes %v", r.Accel(), *changes)
	}
	r.SetTap(NewTap(ModAlt, 0))
	r.SetAccel("Ctrl+Q")
	if _, ok := r.Tap(); ok {
		t.Error("SetAccel kept the tap")
	}
}

func TestNewTapRefusesWhatIsNoTap(t *testing.T) {
	for name, c := range map[string]struct{ mod, held Mods }{
		"none":           {0, ModCtrl},
		"two modifiers":  {ModCtrl | ModAlt, 0},
		"caps lock":      {ModCapsLock, 0},
		"held itself":    {ModSuper, ModSuper | ModCtrl},
		"held caps lock": {ModSuper, ModCapsLock},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: NewTap(%#x, %#x) did not panic", name, c.mod, c.held)
				}
			}()
			NewTap(c.mod, c.held)
		}()
	}
}

// The recorder's face is an outline button, as tall and framed as one
// beside it; listening, it rings as a focused entry does and its prompt
// takes the accent, not the dim placeholder look of a disabled control
// (#282, #315).
func TestShortcutRecorderLooksLikeAButtonAndRingsWhileListening(t *testing.T) {
	face := chromeFace(t)
	r := NewShortcutRecorder(face, "")
	r.SetClearable(false)
	peer := NewButton(NewLabel(face, 14, "Add", 0), 0, 0)
	peer.AddClass(css.Outline)
	row := NewBox(Row, 8, 0)
	row.AppendAligned(r, false, AlignCenter)
	row.AppendAligned(peer, false, AlignCenter)
	arrangeTree(t, row, 400, 60)
	if got, want := r.button.Bounds().H, peer.Bounds().H; got != want {
		t.Errorf("recorder face %dpx tall, the outline button beside it %dpx", got, want)
	}
	rest := r.button.style(r.button)
	if borderOf(rest).Top == 0 {
		t.Error("the resting face has no border")
	}
	ring := Current().RingColor()
	if c := borderColors(rest)[0]; c == ring {
		t.Error("the resting face already wears the ring color")
	}
	r.StartRecording()
	CollectDamage(row)
	rec := r.button.style(r.button)
	if c := borderColors(rec)[0]; c != ring {
		t.Errorf("listening border %08x, want the ring color %08x", uint32(c), uint32(ring))
	}
	var prompt *Label
	walkTree(r.display, func(w Widget, _ int) {
		if l, ok := w.(*Label); ok && l.hasClass("prompt") {
			prompt = l
		}
	})
	if prompt == nil || prompt.hasClass(css.DimLabel) || !prompt.hasClass(css.Accent) {
		t.Errorf("listening prompt %v, want it in the accent, not dim", prompt)
	}
}
