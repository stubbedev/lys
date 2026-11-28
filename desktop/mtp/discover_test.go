package mtp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/udevmonitor/udevmonitortest"

	"github.com/stubbedev/lys/desktop/mtp/usb"
)

const sysfs = "usb/testdata/devices"

type fakeProber struct {
	mu     sync.Mutex
	answer map[string]bool
	fail   map[string]error
	asked  []string
}

func (p *fakeProber) probe(_ context.Context, d usb.Device) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.asked = append(p.asked, d.Name)
	if err := p.fail[d.Name]; err != nil {
		return false, err
	}
	return p.answer[d.Name], nil
}

func deviceNames(devices []usb.Device) []string {
	out := make([]string, len(devices))
	for i, d := range devices {
		out[i] = d.Name
	}
	return out
}

func TestFindKeepsVendorInterfacesTheirDescriptorsCallMTP(t *testing.T) {
	t.Parallel()
	p := &fakeProber{answer: map[string]bool{"1-5": true}}
	f := newFinder(sysfs, p.probe)
	got, probeErr, err := f.find(t.Context(), "")
	if err != nil || probeErr != nil {
		t.Fatal(err, probeErr)
	}
	if n := deviceNames(got); !slices.Equal(n, []string{"1-2", "1-3", "1-5", "2-1"}) {
		t.Fatalf("find gave %v", n)
	}
	if got[2].Protocol != usb.MTP {
		t.Fatalf("the probed device speaks %v", got[2].Protocol)
	}
	if _, _, err := f.find(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.asked, []string{"1-5"}) {
		t.Fatalf("the prober was asked %v, want 1-5 once", p.asked)
	}
}

func TestFindLeavesOutVendorInterfacesThatAreNotMTP(t *testing.T) {
	t.Parallel()
	p := &fakeProber{}
	got, _, err := newFinder(sysfs, p.probe).find(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if n := deviceNames(got); !slices.Equal(n, []string{"1-2", "1-3", "2-1"}) {
		t.Fatalf("find gave %v", n)
	}
}

func TestFindReportsAProbeThatFailedAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	broken := errors.New("timed out")
	p := &fakeProber{fail: map[string]error{"1-5": broken}}
	got, probeErr, err := newFinder(sysfs, p.probe).find(t.Context(), "")
	if err != nil || !errors.Is(probeErr, broken) {
		t.Fatalf("find answered %v, %v", err, probeErr)
	}
	if n := deviceNames(got); !slices.Equal(n, []string{"1-2", "1-3", "2-1"}) {
		t.Fatalf("find gave %v", n)
	}
}

func TestFindFailsWithoutSysfs(t *testing.T) {
	t.Parallel()
	if _, _, err := newFinder(filepath.Join(t.TempDir(), "none"), (&fakeProber{}).probe).find(t.Context(), ""); err == nil {
		t.Fatal("find without sysfs succeeded")
	}
}

type snapshot struct {
	names []string
	err   error
}

func startWatch(t *testing.T, root string) (*udevmonitortest.Group, <-chan snapshot, context.CancelFunc, <-chan error) {
	t.Helper()
	g := udevmonitortest.New(t)
	l, err := g.Listen([]string{"usb"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	snapshots := make(chan snapshot, 16)
	done := make(chan error, 1)
	p := &fakeProber{}
	go func() {
		done <- watch(ctx, l, newFinder(root, p.probe), func(d []usb.Device, err error) {
			snapshots <- snapshot{deviceNames(d), err}
		})
	}()
	return g, snapshots, cancel, done
}

func next(t *testing.T, snapshots <-chan snapshot) snapshot {
	t.Helper()
	select {
	case s := <-snapshots:
		return s
	case <-time.After(5 * time.Second):
		t.Fatal("no snapshot came")
	}
	return snapshot{}
}

func TestWatchFollowsPlugAndUnplug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(sysfs)); err != nil {
		t.Fatal(err)
	}
	g, snapshots, cancel, done := startWatch(t, root)
	if s := next(t, snapshots); !slices.Equal(s.names, []string{"1-2", "1-3", "2-1"}) || s.err != nil {
		t.Fatalf("the first snapshot is %+v", s)
	}

	g.Send("ACTION=remove", "SUBSYSTEM=usb", "DEVTYPE=usb_interface", "DEVPATH=/devices/pci0000:00/0000:00:14.0/usb1/1-3/1-3:1.0")
	if s := next(t, snapshots); !slices.Equal(s.names, []string{"1-2", "2-1"}) {
		t.Fatalf("after unplugging 1-3, while sysfs still shows it, the snapshot is %+v", s)
	}

	g.Send("ACTION=change", "SUBSYSTEM=usb", "DEVTYPE=usb_device", "DEVPATH=/devices/pci0000:00/0000:00:14.0/usb1/1-2")
	g.Send("ACTION=add", "SUBSYSTEM=usb", "DEVTYPE=usb_interface", "DEVPATH=/devices/pci0000:00/0000:00:14.0/usb1/1-3/1-3:1.0")
	if s := next(t, snapshots); !slices.Equal(s.names, []string{"1-2", "1-3", "2-1"}) {
		t.Fatalf("after plugging 1-3 back the snapshot is %+v", s)
	}

	if err := os.RemoveAll(filepath.Join(root, "1-2:1.0")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "1-2")); err != nil {
		t.Fatal(err)
	}
	g.Send("ACTION=add", "SUBSYSTEM=usb", "DEVTYPE=usb_interface", "DEVPATH=/devices/pci0000:00/0000:00:14.0/usb1/1-4/1-4:1.0")
	if s := next(t, snapshots); !slices.Equal(s.names, []string{"1-3", "2-1"}) {
		t.Fatalf("a rescan after 1-2 went gives %+v", s)
	}

	g.Send("ACTION=add", "SUBSYSTEM=usb", "DEVTYPE=usb_interface", "DEVPATH=/devices/pci0000:00/0000:00:14.0/usb1/1-4/1-4:1.0")
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("watch ended with %v", err)
	}
	select {
	case s := <-snapshots:
		t.Fatalf("an event that changed nothing gave the snapshot %+v", s)
	default:
	}
}
