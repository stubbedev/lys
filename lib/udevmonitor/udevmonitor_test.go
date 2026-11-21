package udevmonitor

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestHashIsLibudevs(t *testing.T) {
	t.Parallel()
	for s, want := range map[string]uint32{
		"":             0,
		"ab":           0x1aa14063,
		"drm":          0x16664179,
		"mem":          0xc365cd83,
		"input":        0xc1a28470,
		"sound":        0xd196ab6e,
		"power_supply": 0xb1acbcca,
	} {
		if got := hash(s); got != want {
			t.Errorf("hash(%q) = %#x, want %#x", s, got, want)
		}
	}
}

func filtered(t *testing.T, subsystems []string) (int, int) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(unix.Close(fds[0]), unix.Close(fds[1])); err != nil {
			t.Error(err)
		}
	})
	if err := Filter(fds[0], subsystems); err != nil {
		t.Fatal(err)
	}
	return fds[0], fds[1]
}

func received(t *testing.T, fd int) [][]byte {
	t.Helper()
	var got [][]byte
	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(fd, buf)
		if errors.Is(err, unix.EAGAIN) {
			return got
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, slices.Clone(buf[:n]))
	}
}

func TestFilterPassesOnlyTheSubsystemsAsked(t *testing.T) {
	t.Parallel()
	in, out := filtered(t, []string{"drm", "input", "power_supply"})
	drm := Frame("ACTION=change", "SUBSYSTEM=drm", "HOTPLUG=1")
	input := Frame("ACTION=add", "SUBSYSTEM=input")
	supply := Frame("ACTION=change", "SUBSYSTEM=power_supply")
	wrongMagic := Frame("ACTION=change", "SUBSYSTEM=drm")
	wrongMagic[magicOffset] ^= 0xff
	for _, msg := range [][]byte{
		Frame("ACTION=change", "SUBSYSTEM=mem"),
		drm,
		Frame("ACTION=add", "SUBSYSTEM=sound"),
		[]byte("change@/devices/virtual/drm/card0\x00ACTION=change\x00SUBSYSTEM=drm\x00"),
		wrongMagic,
		input,
		[]byte("libudev"),
		Frame("ACTION=change"),
		supply,
	} {
		if _, err := unix.Write(out, msg); err != nil {
			t.Fatal(err)
		}
	}
	got := received(t, in)
	if want := [][]byte{drm, input, supply}; !slices.EqualFunc(got, want, bytes.Equal) {
		t.Fatalf("the filter passed %q, want %q", got, want)
	}
}

func TestFilterForNoSubsystemIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := program(nil); err == nil {
		t.Fatal("a filter for no subsystem was built")
	}
	if _, err := Open(nil); err == nil {
		t.Fatal("a socket for no subsystem opened")
	}
}

func TestPropertiesOfAFrame(t *testing.T) {
	t.Parallel()
	msg := Frame("ACTION=change", "SUBSYSTEM=drm")
	props, err := Properties(msg)
	if err != nil || string(props) != "ACTION=change\x00SUBSYSTEM=drm\x00" {
		t.Fatalf("properties %q, %v", props, err)
	}
	if _, err := Properties([]byte("change@/devices/x\x00ACTION=change\x00")); !errors.Is(err, ErrNotUdev) {
		t.Fatalf("a kernel uevent: %v", err)
	}
	bad := slices.Clone(msg)
	bad[magicOffset] = 0
	if _, err := Properties(bad); !errors.Is(err, ErrNotUdev) {
		t.Fatalf("a wrong magic: %v", err)
	}
	long := slices.Clone(msg)
	long[propertiesLength] = 0xff
	if _, err := Properties(long); err == nil || errors.Is(err, ErrNotUdev) {
		t.Fatalf("properties past the end: %v", err)
	}
}

func TestOpenJoinsTheUdevGroup(t *testing.T) {
	t.Parallel()
	fd, err := Open([]string{"drm"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := unix.Getsockname(fd)
	if err != nil {
		t.Fatal(err)
	}
	if nl, ok := sa.(*unix.SockaddrNetlink); !ok || nl.Groups != Group {
		t.Errorf("bound to %+v, want group %d", sa, Group)
	}
	if err := unix.Close(fd); err != nil {
		t.Fatal(err)
	}
}

func TestParseReadsAnEvent(t *testing.T) {
	t.Parallel()
	ev, err := Parse(Frame("ACTION=change", "DEVPATH=/devices/pci0000:00/0000:00:02.0/drm/card1", "SUBSYSTEM=drm", "HOTPLUG=1"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != Change || ev.Subsystem != "drm" || ev.Props["HOTPLUG"] != "1" {
		t.Errorf("event = %+v", ev)
	}
	for raw, want := range map[string]Action{"add": Add, "remove": Remove, "change": Change, "bind": Other, "move": Other} {
		ev, err := Parse(Frame("ACTION="+raw, "SUBSYSTEM=backlight"))
		if err != nil || ev.Action != want {
			t.Errorf("ACTION=%s parsed to %v, %v; want %v", raw, ev.Action, err, want)
		}
	}
}

func TestParseRejectsWhatIsNoEvent(t *testing.T) {
	t.Parallel()
	for name, bad := range map[string][]byte{
		"a kernel uevent":      []byte("add@/x\x00ACTION=add\x00SUBSYSTEM=drm\x00"),
		"no action":            Frame("SUBSYSTEM=drm"),
		"no subsystem":         Frame("ACTION=add"),
		"a property without =": Frame("ACTION=add", "SUBSYSTEM=drm", "HOTPLUG"),
		"empty":                nil,
	} {
		if ev, err := Parse(bad); err == nil {
			t.Errorf("%s: parsed to %+v, want an error", name, ev)
		}
	}
}

func TestListenerReadsOnlyTheSubsystemsAskedUntilClosed(t *testing.T) {
	t.Parallel()
	listen, send := filtered(t, []string{"backlight"})
	dup, err := unix.Dup(listen)
	if err != nil {
		t.Fatal(err)
	}
	l := NewListener(dup)
	for _, msg := range [][]byte{
		Frame("ACTION=change", "SUBSYSTEM=mem"),
		Frame("ACTION=change", "SUBSYSTEM=drm", "HOTPLUG=1"),
		Frame("ACTION=add", "SUBSYSTEM=backlight", "DEVPATH=/devices/virtual/backlight/acpi_video0"),
	} {
		if _, err := unix.Write(send, msg); err != nil {
			t.Fatal(err)
		}
	}
	ev, err := l.Next()
	if err != nil || ev.Action != Add || ev.Subsystem != "backlight" {
		t.Fatalf("the first event read is %+v, %v; want the backlight's add", ev, err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := l.Next()
		done <- err
	}()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, os.ErrClosed) {
		t.Errorf("Next after Close: %v, want %v", err, os.ErrClosed)
	}
}
