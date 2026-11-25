package widget

import (
	"fmt"
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"

	"github.com/stubbedev/lys/ui/widget/css"
)

// shortcutRecorderSize is the recorder's text size.
const shortcutRecorderSize = 14

// Tap is a modifier pressed and released alone, with other modifiers
// held: Super tapped with Ctrl held reads "Ctrl+Super". No accelerator
// writes one; a ShortcutRecorder that records taps records one. The
// zero Tap is none.
type Tap struct{ mod, held Mods }

// tapModifiers are the modifier bits a tap is made of, the
// accelerator's.
var tapModifiers = func() Mods {
	var all Mods
	for mod := range AccelModifiers() {
		all |= mod
	}
	return all
}()

// NewTap is the tap of mod with held held. It panics unless mod is
// exactly one accelerator modifier and held is other accelerator
// modifiers only.
func NewTap(mod, held Mods) Tap {
	if mod == 0 || mod&(mod-1) != 0 || mod&tapModifiers != mod {
		panic(fmt.Sprintf("widget.NewTap: %#x is not one modifier", mod))
	}
	if held&tapModifiers != held || held&mod != 0 {
		panic(fmt.Sprintf("widget.NewTap: %#x is not modifiers held beside %#x", held, mod))
	}
	return Tap{mod: mod, held: held}
}

// Mod is the modifier tapped, zero for no tap.
func (t Tap) Mod() Mods { return t.mod }

// Held are the modifiers held while it is tapped.
func (t Tap) Held() Mods { return t.held }

// Labels are its keycaps: the held modifiers in label order, then the
// tapped one.
func (t Tap) Labels() []string {
	return append(modifierNames(t.held), modifierNames(t.mod)...)
}

// String is t in the label form, its Labels joined by "+": "Ctrl+Super".
func (t Tap) String() string { return strings.Join(t.Labels(), "+") }

// ShortcutRecorder is a control that records a key combination (GNOME
// Settings' "Set Shortcut", KDE's KKeySequenceWidget). At rest it
// shows the accelerator or tap as keycaps, or "Unassigned", beside a
// clear button. Activated (a click, Enter or Space) it records: it
// takes the window's raw key stream as a KeyRecorder, previews held
// modifiers live, and commits the first non-modifier key with its
// modifiers. A recorder that records taps (SetRecordTaps) also commits
// a modifier released with no key pressed since it went down, as a Tap
// of it with the other modifiers held. Escape cancels, keeping the old
// value and the message shown under it, Backspace alone clears it, and
// losing focus cancels. A Validate error shows under the control while
// it keeps recording. It styles as the `shortcutrecorder` element, its
// face an outline button, ringed as a focused entry is and its
// .prompt in the accent while it records (.recording), the message an
// .error label.
type ShortcutRecorder struct {
	composite
	face      render.Font
	box       *Box
	row       *Box
	button    *recorderButton
	display   *Box
	clear     *Button
	err       *Label
	value     string
	tap       Tap
	clearable bool
	recording bool
	taps      bool
	held      Mods
	// pending is the tap a release of its modifier commits: the last
	// modifier pressed while no press came after it.
	pending Tap
	// shown is the message under the control when recording started,
	// back on a cancel.
	shown string

	// OnChanged fires when a press, Backspace or the clear button
	// changed the value to an accelerator, the new one in the label form
	// ("Ctrl+Alt+P"), or to nothing, empty. SetAccel does not fire it.
	OnChanged func(accel string)
	// OnTap fires when a recorder that records taps recorded a tap other
	// than its value. SetTap does not fire it.
	OnTap func(Tap)
	// Validate, when set, judges a recorded accelerator before it
	// commits: an error is shown under the control and recording goes
	// on. A tap commits unjudged.
	Validate func(accel string) error
}

// recorderButton is the recorder's focusable face: the widget that
// holds focus, so the one the window hands its key stream to. Its type
// names no element, so it names itself a `button`.
type recorderButton struct {
	*Button
	r *ShortcutRecorder
}

// NewShortcutRecorder returns a recorder at rest showing accel, in
// the label form ("Ctrl+Alt+P"); empty shows it unassigned.
func NewShortcutRecorder(face render.Font, accel string) *ShortcutRecorder {
	face = requireFace("widget.NewShortcutRecorder", face)
	r := &ShortcutRecorder{face: face, box: NewBox(Column, 4, 0), row: NewBox(Row, 6, 0), display: NewBox(Row, 0, 0), value: accel, clearable: true}
	r.SetElement("shortcutrecorder")
	r.button = &recorderButton{Button: NewButton(r.display, 0, 0), r: r}
	r.button.SetElement("button")
	r.button.AddClass(css.Outline)
	r.button.OnClick = r.StartRecording
	r.button.SetAccessibleName(Tr("Shortcut"))
	r.button.SetOnFocusChanged(func(focused bool) {
		if !focused && r.recording {
			r.cancel()
		}
	})
	r.clear = NewButton(NewThemeIcon("edit-clear-symbolic", 16), 6, 4)
	r.clear.SetAccessibleName(Tr("Clear shortcut"))
	r.clear.OnClick = func() {
		r.stop("")
		r.commit("")
	}
	r.err = NewLabel(face, shortcutRecorderSize, "", 0)
	r.err.AddClass(css.Error)
	r.err.SetWrap(true)
	r.row.Append(r.button, true)
	r.box.Append(r.row, false)
	r.initComposite(r, r.box)
	r.refresh()
	return r
}

// Accel returns the accelerator in the label form, empty while
// unassigned or holding a tap.
func (r *ShortcutRecorder) Accel() string { return r.value }

// SetAccel shows accel, ending any recording, without firing
// OnChanged: the caller's own value, not the user's edit.
func (r *ShortcutRecorder) SetAccel(accel string) {
	r.value, r.tap = accel, Tap{}
	r.stop("")
}

// Tap returns the tap the recorder holds, false while it holds an
// accelerator or nothing.
func (r *ShortcutRecorder) Tap() (Tap, bool) { return r.tap, r.tap.mod != 0 }

// SetTap shows t, ending any recording, without firing OnTap: the
// caller's own value, not the user's edit. The zero Tap shows the
// recorder unassigned.
func (r *ShortcutRecorder) SetTap(t Tap) {
	r.value, r.tap = "", t
	r.stop("")
}

// SetRecordTaps sets whether recording commits a modifier pressed and
// released alone as a Tap; off, the default, such a press only
// previews.
func (r *ShortcutRecorder) SetRecordTaps(on bool) { r.taps = on }

// SetClearable sets whether the clear button shows beside a value; a
// recorder whose row goes back to a default through a reset of its own
// has none. Backspace still clears it while it records.
func (r *ShortcutRecorder) SetClearable(on bool) {
	r.clearable = on
	r.refresh()
}

// SetAccessibleName names the control for assistive technology; the
// name lands on the focusable button.
func (r *ShortcutRecorder) SetAccessibleName(name string) { r.button.SetAccessibleName(name) }

// SetError shows msg under the control and marks it invalid; empty
// clears both. A caller reports a refusal that arrives after the
// commit (a compositor denying the key) through it.
func (r *ShortcutRecorder) SetError(msg string) {
	r.err.SetText(msg)
	r.button.SetClass(css.Error, msg != "")
	r.box.Remove(r.err)
	if msg != "" {
		r.box.Append(r.err, false)
	}
	MarkA11yChanged()
	r.InvalidateLayout()
}

// Error returns the message shown under the control, empty while
// none is.
func (r *ShortcutRecorder) Error() string { return r.err.Text() }

// Recording reports whether the recorder is taking key presses.
func (r *ShortcutRecorder) Recording() bool { return r.recording }

// StartRecording enters recording. A disabled recorder stays at rest.
func (r *ShortcutRecorder) StartRecording() {
	if r.recording || !IsEnabled(r.button) {
		return
	}
	r.recording, r.held, r.pending, r.shown = true, 0, Tap{}, r.err.Text()
	r.refresh()
}

// RecordPress implements KeyRecorder: a modifier key joins the live
// preview and, recording taps, becomes the tap its release commits;
// Escape alone cancels, Backspace alone clears, and any other key with
// its modifiers is validated and committed.
func (r *ShortcutRecorder) RecordPress(sym xkb.Keysym, mods Mods) {
	if !r.recording {
		return
	}
	r.pending = Tap{}
	if bit, ok := modifierOf(sym); ok {
		r.held = mods | bit
		if r.taps && bit != 0 {
			r.pending = NewTap(bit, mods&tapModifiers&^bit)
		}
		r.refresh()
		return
	}
	switch {
	case sym == xkb.KeyEscape && mods == 0:
		r.cancel()
		return
	case sym == xkb.KeyBackSpace && mods == 0:
		r.stop("")
		r.commit("")
		return
	}
	accel := AccelLabel(sym, mods)
	if r.Validate != nil {
		if err := r.Validate(accel); err != nil {
			r.SetError(err.Error())
			return
		}
	}
	r.stop("")
	r.commit(accel)
}

// RecordRelease implements KeyRecorder: the release of the pending
// tap's modifier commits the tap, any other released modifier leaves
// the live preview, and with none held the recorder prompts again.
func (r *ShortcutRecorder) RecordRelease(sym xkb.Keysym) {
	bit, ok := modifierOf(sym)
	if !r.recording || !ok {
		return
	}
	if t := r.pending; t.mod != 0 && t.mod == bit {
		r.stop("")
		r.commitTap(t)
		return
	}
	r.held &^= bit
	r.refresh()
}

// stop leaves recording, showing msg under the control.
func (r *ShortcutRecorder) stop(msg string) {
	r.recording, r.held, r.pending = false, 0, Tap{}
	r.SetError(msg)
	r.refresh()
}

// cancel leaves recording with the value and the message it started
// with.
func (r *ShortcutRecorder) cancel() { r.stop(r.shown) }

// commit makes accel the value, firing OnChanged when it differs.
func (r *ShortcutRecorder) commit(accel string) {
	if accel == r.value && r.tap.mod == 0 {
		return
	}
	r.value, r.tap = accel, Tap{}
	r.refresh()
	if r.OnChanged != nil {
		r.OnChanged(accel)
	}
}

// commitTap makes t the value, firing OnTap when it differs.
func (r *ShortcutRecorder) commitTap(t Tap) {
	if t == r.tap {
		return
	}
	r.value, r.tap = "", t
	r.refresh()
	if r.OnTap != nil {
		r.OnTap(t)
	}
}

// shownValue is the value in the label form, empty while unassigned.
func (r *ShortcutRecorder) shownValue() string {
	if r.tap.mod != 0 {
		return r.tap.String()
	}
	return r.value
}

// refresh rebuilds the button's content and the clear button for the
// state: keycaps of the value or the held modifiers, or a dim prompt.
func (r *ShortcutRecorder) refresh() {
	r.display.Clear()
	switch {
	case r.recording && r.held != 0:
		r.display.AppendAligned(NewShortcutLabel(r.face, strings.Join(modifierNames(r.held), "+")), false, AlignCenter)
	case r.recording:
		r.display.AppendAligned(r.promptLabel(Tr("Press a shortcut…"), css.Accent), false, AlignCenter)
	case r.shownValue() != "":
		r.display.AppendAligned(NewShortcutLabel(r.face, r.shownValue()), false, AlignCenter)
	default:
		r.display.AppendAligned(r.promptLabel(Tr("Unassigned"), css.DimLabel), false, AlignCenter)
	}
	r.button.SetClass("recording", r.recording)
	r.row.Remove(r.clear)
	if r.clearable && r.shownValue() != "" && !r.recording {
		r.row.AppendAligned(r.clear, false, AlignCenter)
	}
	MarkA11yChanged()
	r.InvalidateLayout()
}

// promptLabel is a prompt in place of keycaps, colored by class: dim
// while unassigned, the accent while it listens.
func (r *ShortcutRecorder) promptLabel(text, class string) *Label {
	l := NewLabel(r.face, shortcutRecorderSize, text, 0)
	l.AddClass("prompt", class)
	return l
}

// description is what assistive technology reads after the name: the
// recording state, else the current value.
func (r *ShortcutRecorder) description() string {
	switch {
	case r.recording:
		return Tr("Recording, press a shortcut")
	case r.shownValue() == "":
		return Tr("Unassigned")
	}
	return accelText(r.shownValue())
}

// Recording implements KeyRecorder for the focused button.
func (b *recorderButton) Recording() bool { return b.r.Recording() }

// RecordPress implements KeyRecorder for the focused button.
func (b *recorderButton) RecordPress(sym xkb.Keysym, mods Mods) { b.r.RecordPress(sym, mods) }

// RecordRelease implements KeyRecorder for the focused button.
func (b *recorderButton) RecordRelease(sym xkb.Keysym) { b.r.RecordRelease(sym) }

// HitTest returns the button itself, so a click focuses the
// recorder's face rather than the embedded Button.
func (b *recorderButton) HitTest(p Point) Widget { return b.HitLeaf(b, p) }

// a11yDescription is the recorder's description on its button.
func (b *recorderButton) a11yDescription() string { return b.r.description() }
