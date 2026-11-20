package settings

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/shell/service/sounds"

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
	e := shellControl(t, k, "modules.notifications.sound-default", soundNameRow).(*alertSoundEditor)
	e.play.OnClick()
	plays := srv.Plays()
	if len(asked) != 1 || asked[0] != "bell" || len(plays) != 1 {
		t.Fatalf("looked up %q, played %+v", asked, plays)
	}
	if plays[0].Volume != native.LinearVolume(0.5) || plays[0].Props["event.id"] != "bell" || e.problem.Visible() {
		t.Fatalf("played at %#x as %q, problem shown %v", plays[0].Volume, plays[0].Props["event.id"], e.problem.Visible())
	}
	if got := len(srv.Samples()[plays[0].Name].Data); got != 2*800 {
		t.Fatalf("the uploaded sample holds %d bytes, want the file's %d", got, 2*800)
	}
}

func TestASoundTheThemeLacksSaysWhyAndPlaysNothing(t *testing.T) {
	srv := soundServer(t)
	useSoundServer(t, srv)
	useAlertSounds(t, func(string) (string, error) { return "", sounds.ErrNoSound })
	k := testKit(t, "modules:\n  notifications:\n    sound-default: missing\n")
	e := shellControl(t, k, "modules.notifications.sound-default", soundNameRow).(*alertSoundEditor)
	e.play.OnClick()
	if !e.problem.Visible() || !strings.HasPrefix(e.problem.Text(), locale.T().Get("settings-sound-alert-play-failed")) || !strings.Contains(e.problem.Text(), sounds.ErrNoSound.Error()) {
		t.Fatalf("the problem reads %q, shown %v", e.problem.Text(), e.problem.Visible())
	}
	if plays := srv.Plays(); len(plays) != 0 {
		t.Fatalf("a missing sound played %+v", plays)
	}
}
