package settings

import (
	"slices"
	"testing"
)

func TestTheAudioCaptureIndicatorsAreOneRowOfTiles(t *testing.T) {
	r := pageRow(t, "audio", "modules.audio.mic-icon")
	want := []string{
		"modules.audio.mic-icon-muted", "modules.audio.capture-camera-icon", "modules.audio.capture-screen-icon",
		"modules.audio.mic-hide-when-idle", "modules.audio.capture-color",
	}
	if !slices.Equal(r.also, want) {
		t.Fatalf("the capture row also holds %q", r.also)
	}
	k := testKit(t, "modules:\n  audio:\n    mic-icon: ld-mic-symbolic\n    mic-hide-when-idle: true\n")
	c := fieldControl(k, k.store, r).(*captureTiles)
	if len(c.icons) != 4 {
		t.Fatalf("%d icon pickers, want one per state", len(c.icons))
	}
	if c.icons[0].slot.text() != "ld-mic-symbolic" || c.icons[2].slot.text() != "ld-webcam-symbolic" {
		t.Errorf("the tiles show %q and %q", c.icons[0].slot.text(), c.icons[2].slot.text())
	}
	if c.hide.Active() != 1 {
		t.Errorf("a microphone hidden while idle shows choice %d", c.hide.Active())
	}
	c.hide.SetActive(0)
	if v := k.store.value("modules.audio.mic-hide-when-idle"); v != false {
		t.Fatalf("always showing the microphone stored %v", v)
	}
	if err := k.store.set("modules.audio.mic-hide-when-idle", true); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if c.hide.Active() != 1 {
		t.Errorf("after the config hid the microphone the tile shows choice %d", c.hide.Active())
	}
	if v := k.store.value("modules.audio.mic-hide-when-idle"); v != true {
		t.Errorf("refreshing the tiles wrote the choice back: %v", v)
	}
}
