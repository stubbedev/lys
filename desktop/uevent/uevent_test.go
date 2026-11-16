package uevent

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
)

func TestParseReadsAKernelUevent(t *testing.T) {
	ev, err := Parse([]byte("change@/devices/pci0000:00/0000:00:02.0/drm/card1\x00ACTION=change\x00DEVPATH=/devices/pci0000:00/0000:00:02.0/drm/card1\x00SUBSYSTEM=drm\x00HOTPLUG=1\x00SEQNUM=4242\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != Change || ev.Subsystem != "drm" || ev.Sysname != "card1" || ev.Props["HOTPLUG"] != "1" {
		t.Errorf("event = %+v", ev)
	}
	for raw, want := range map[string]Action{"add": Add, "remove": Remove, "change": Change, "bind": Other, "move": Other} {
		ev, err := Parse([]byte(raw + "@/x/b\x00ACTION=" + raw + "\x00SUBSYSTEM=backlight\x00"))
		if err != nil || ev.Action != want {
			t.Errorf("ACTION=%s parsed to %v, %v; want %v", raw, ev.Action, err, want)
		}
	}
}

func TestParseRejectsWhatIsNoKernelUevent(t *testing.T) {
	for name, bad := range map[string][]byte{
		"libudev framing": []byte("libudev\x00\xfe\xed\xca\xfe"),
		"no action":       []byte("add@/x\x00SUBSYSTEM=drm\x00"),
		"no subsystem":    []byte("add@/x\x00ACTION=add\x00"),
		"empty":           nil,
	} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%s: parsed, want an error", name)
		}
	}
}

func TestCloseUnblocksNext(t *testing.T) {
	reader, err := Listen()
	if err != nil {
		t.Fatalf("no uevent netlink socket: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		for {
			if _, err := reader.Next(); err != nil {
				done <- err
				return
			}
		}
	}()
	time.Sleep(20 * time.Millisecond)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waittest.For(t, done, "Close to unblock Next"); err == nil {
		t.Error("Next after Close returned no error")
	}
}
