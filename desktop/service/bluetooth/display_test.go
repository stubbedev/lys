package bluetooth

import "testing"

func classOf(major, minor uint32) *uint32 {
	c := major<<8 | minor<<2
	return &c
}

func TestDeviceIconFromHintAndClass(t *testing.T) {
	for _, c := range []struct {
		hint  *string
		class *uint32
		want  string
	}{
		{new("audio-headphones"), nil, "ld-headphones-symbolic"},
		{new("audio-headset"), nil, "ld-headphones-symbolic"},
		{new("audio-speakers"), nil, "ld-speaker-symbolic"},
		{new("input-keyboard"), nil, "ld-keyboard-symbolic"},
		{new("input-mouse"), nil, "ld-mouse-symbolic"},
		{new("input-gaming"), nil, "ld-gamepad-2-symbolic"},
		{new("video-display"), nil, "ld-monitor-symbolic"},
		{new("phone"), nil, "ld-smartphone-symbolic"},
		{nil, classOf(MajorComputer, 0), "ld-monitor-symbolic"},
		{nil, classOf(MajorComputer, 0x03), "ld-laptop-symbolic"},
		{nil, classOf(MajorPhone, 0), "ld-smartphone-symbolic"},
		{nil, classOf(MajorAV, 0x06), "ld-headphones-symbolic"},
		{nil, classOf(MajorAV, 0x09), "ld-tv-symbolic"},
		{nil, classOf(MajorAV, 0x05), "ld-speaker-symbolic"},
		{nil, classOf(MajorAV, 0x04), "ld-mic-symbolic"},
		{nil, classOf(MajorPeripheral, 0x01<<4), "ld-keyboard-symbolic"},
		{nil, classOf(MajorPeripheral, 0x02<<4), "ld-mouse-symbolic"},
		{nil, classOf(MajorPeripheral, 0x02), "ld-gamepad-2-symbolic"},
		{nil, classOf(MajorWearable, 0), "ld-watch-symbolic"},
		// The hint wins over the class; an unknown hint falls through.
		{new("phone"), classOf(MajorComputer, 0), "ld-smartphone-symbolic"},
		{new("unknown-device"), classOf(MajorPhone, 0), "ld-smartphone-symbolic"},
		{nil, nil, "ld-bluetooth-symbolic"},
		{new("alien-device"), new(uint32(0xFF00)), "ld-bluetooth-symbolic"},
	} {
		if got := DeviceIcon(c.hint, c.class); got != c.want {
			t.Errorf("DeviceIcon(%v, %v) = %q, want %q", c.hint, c.class, got, c.want)
		}
	}
}

func TestFormatPasskeyPads(t *testing.T) {
	if FormatPasskey(42) != "000042" || FormatPasskey(999999) != "999999" {
		t.Error("passkeys are six zero-padded digits")
	}
}

func TestAnAudioDeviceIsKnownByItsHintOrClass(t *testing.T) {
	headset, keyboard := "audio-headset", "input-keyboard"
	av, phone := uint32(MajorAV<<8), uint32(MajorPhone<<8)
	for _, c := range []struct {
		hint  *string
		class *uint32
		want  bool
	}{
		{&headset, nil, true}, {&keyboard, &av, false}, {nil, &av, true}, {nil, &phone, false}, {nil, nil, false},
	} {
		if got := IsAudio(c.hint, c.class); got != c.want {
			t.Errorf("IsAudio(%v, %v) = %v, want %v", c.hint, c.class, got, c.want)
		}
	}
}
