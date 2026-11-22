package settings

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/shell/service/sounds"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func writeWAV(t *testing.T, frames int) string {
	t.Helper()
	pcm := pcm(8000, frames)
	header := make([]byte, 0, 44)
	header = append(header, "RIFF"...)
	header = binary.LittleEndian.AppendUint32(header, uint32(36+len(pcm)))
	header = append(header, "WAVEfmt "...)
	header = binary.LittleEndian.AppendUint32(header, 16)
	header = binary.LittleEndian.AppendUint16(header, 1)
	header = binary.LittleEndian.AppendUint16(header, 1)
	header = binary.LittleEndian.AppendUint32(header, 8000)
	header = binary.LittleEndian.AppendUint32(header, 16000)
	header = binary.LittleEndian.AppendUint16(header, 2)
	header = binary.LittleEndian.AppendUint16(header, 16)
	header = append(header, "data"...)
	header = binary.LittleEndian.AppendUint32(header, uint32(len(pcm)))
	path := filepath.Join(t.TempDir(), "bell.wav")
	if err := os.WriteFile(path, append(header, pcm...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func useAlertSounds(t *testing.T, lookup func(string) (string, error)) {
	t.Helper()
	before := lookupAlertSound
	lookupAlertSound = lookup
	t.Cleanup(func() { lookupAlertSound = before })
}

const soundDefaultPath = "modules.notifications.sound-default"

func alertList(t *testing.T, k *kit, names ...string) *alertSoundList {
	t.Helper()
	listed := make([]choice, len(names))
	for i, n := range names {
		listed[i] = choice{value: n, label: n}
	}
	return newAlertSoundList(k, k.store.slot(soundDefaultPath), fixedChoices(listed))
}

func (e *alertSoundList) sound(t *testing.T, name string) alertSound {
	t.Helper()
	i := slices.IndexFunc(e.sounds, func(s alertSound) bool { return s.name == name })
	if i < 0 {
		t.Fatalf("no row for %q", name)
	}
	return e.sounds[i]
}

func (e *alertSoundList) checked() []string {
	var out []string
	for _, s := range e.sounds {
		if s.radio.Checked() {
			out = append(out, s.name)
		}
	}
	return out
}

func TestTheAlertSoundIsARadioListOfEverySound(t *testing.T) {
	k := testKit(t, "modules:\n  notifications:\n    sound-default: bell\n")
	if _, ok := shellControl(t, k, soundDefaultPath, soundNameRow).(*alertSoundList); !ok {
		t.Fatal("the alert sound row is not the sound list")
	}
	e := alertList(t, k, "bell", "complete", "message-new-instant")
	if got := e.checked(); !slices.Equal(got, []string{"bell"}) || len(e.sounds) != 3 {
		t.Fatalf("%d rows, checked %q", len(e.sounds), got)
	}
	e.sound(t, "complete").radio.Toggle()
	if got := k.store.value(soundDefaultPath); got != "complete" {
		t.Errorf("stored %v, want complete", got)
	}
	if got := e.checked(); !slices.Equal(got, []string{"complete"}) {
		t.Errorf("checked %q after picking complete", got)
	}
	e.sound(t, "complete").radio.Toggle()
	if got := e.checked(); !slices.Equal(got, []string{"complete"}) {
		t.Errorf("a second click on the checked sound left %q", got)
	}
}

// A click on a sound's row, its name included, picks the sound as its
// radio does; the play button previews without picking.
func TestClickingASoundsRowPicksIt(t *testing.T) {
	k := testKit(t, "modules:\n  notifications:\n    sound-default: bell\n")
	e := alertList(t, k, "bell", "complete")
	rows := treetest.All[*settingRow](e.rows)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want one per sound", len(rows))
	}
	rows[1].ClickAt(widget.Point{})
	if got := k.store.value(soundDefaultPath); got != "complete" {
		t.Errorf("clicking the complete row stored %v", got)
	}
	if got := e.checked(); !slices.Equal(got, []string{"complete"}) {
		t.Errorf("checked %q after clicking the complete row", got)
	}
}

func TestASoundNoThemeHoldsKeepsItsOwnCheckedRow(t *testing.T) {
	k := testKit(t, "modules:\n  notifications:\n    sound-default: gone\n")
	e := alertList(t, k, "bell")
	if len(e.sounds) != 2 || e.sounds[0].name != "gone" || !slices.Equal(e.checked(), []string{"gone"}) {
		t.Fatalf("rows %+v, checked %q", e.sounds, e.checked())
	}
	k = testKit(t, "modules:\n  notifications:\n    sound-default: ''\n")
	e = alertList(t, k)
	if len(e.sounds) != 0 || len(treetest.All[*settingRow](e.rows)) != 0 || e.state.kind != stateEmpty {
		t.Fatalf("without sounds %d rows, %d other rows, the state %v", len(e.sounds), len(treetest.All[*settingRow](e.rows)), e.state.kind)
	}
}

func TestPlayPreviewsTheChosenAlertSoundAtTheAlertVolume(t *testing.T) {
	srv := soundServer(t)
	useSoundServer(t, srv)
	wav := writeWAV(t, 800)
	var asked []string
	useAlertSounds(t, func(name string) (string, error) {
		asked = append(asked, name)
		return wav, nil
	})
	k := testKit(t, "modules:\n  notifications:\n    sound-default: bell\n    sound-volume: 50\n")
	e := alertList(t, k, "bell", "complete")
	e.sound(t, "complete").play.OnClick()
	plays := srv.Plays()
	if len(asked) != 1 || asked[0] != "complete" || len(plays) != 1 {
		t.Fatalf("looked up %q, played %+v", asked, plays)
	}
	if plays[0].Volume != native.LinearVolume(0.5) || plays[0].Props["event.id"] != "complete" || e.problem.Visible() {
		t.Fatalf("played at %#x as %q, problem shown %v", plays[0].Volume, plays[0].Props["event.id"], e.problem.Visible())
	}
	if got := len(srv.Samples()[plays[0].Name].Data); got != 2*800 {
		t.Fatalf("the uploaded sample holds %d bytes, want the file's %d", got, 2*800)
	}
	if got := k.store.value(soundDefaultPath); got != "bell" {
		t.Errorf("playing a sound chose it: %v", got)
	}
}

func TestASoundTheThemeLacksSaysWhyAndPlaysNothing(t *testing.T) {
	srv := soundServer(t)
	useSoundServer(t, srv)
	useAlertSounds(t, func(string) (string, error) { return "", sounds.ErrNoSound })
	k := testKit(t, "modules:\n  notifications:\n    sound-default: missing\n")
	e := alertList(t, k, "bell")
	e.sound(t, "missing").play.OnClick()
	if !e.problem.Visible() || !strings.HasPrefix(e.problem.Text(), locale.T().Get("settings-sound-alert-play-failed")) || !strings.Contains(e.problem.Text(), sounds.ErrNoSound.Error()) {
		t.Fatalf("the problem reads %q, shown %v", e.problem.Text(), e.problem.Visible())
	}
	if plays := srv.Plays(); len(plays) != 0 {
		t.Fatalf("a missing sound played %+v", plays)
	}
}
