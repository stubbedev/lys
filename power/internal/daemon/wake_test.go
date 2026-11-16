package daemon

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

func TestTouchesOnBattery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		changed dbusx.ChangedProperties
		want    bool
	}{
		{"changed", dbusx.ChangedProperties{Interface: upowerName, Changed: map[string]dbus.Variant{"OnBattery": dbus.MakeVariant(true)}}, true},
		{"invalidated", dbusx.ChangedProperties{Interface: upowerName, Changed: map[string]dbus.Variant{}, Invalidated: []string{"OnBattery"}}, true},
		{"lid only", dbusx.ChangedProperties{Interface: upowerName, Changed: map[string]dbus.Variant{"LidIsClosed": dbus.MakeVariant(true)}}, false},
		{"nothing", dbusx.ChangedProperties{Interface: upowerName}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := touchesOnBattery(tt.changed); got != tt.want {
				t.Errorf("touchesOnBattery() = %v, want %v", got, tt.want)
			}
		})
	}
}

const quietWindow = 200 * time.Millisecond

func expectQuiet(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
		t.Fatal("unexpected wakeup")
	case <-time.After(quietWindow):
	}
}

func TestWallTimer(t *testing.T) {
	t.Parallel()
	timer, err := newWallTimer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = timer.Close() }()

	if err := timer.Arm(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, timer.wakes(), "the timer to wake")

	if err := timer.Arm(time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, timer.wakes(), "the timer to wake")

	if err := timer.Arm(time.Now().Add(quietWindow / 4)); err != nil {
		t.Fatal(err)
	}
	if err := timer.Arm(time.Time{}); err != nil {
		t.Fatal(err)
	}
	expectQuiet(t, timer.wakes())
}

func TestWatchDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	watch, err := watchDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = watch.Close() }()

	flag := filepath.Join(dir, "full-now")
	if err := os.WriteFile(flag, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, watch.wakes(), "the watch to wake")

	if err := os.Remove(flag); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, watch.wakes(), "the watch to wake")
}

func TestWatchDirMissing(t *testing.T) {
	t.Parallel()
	if _, err := watchDir(filepath.Join(t.TempDir(), "absent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("watchDir() error = %v, want not-exist", err)
	}
}

func TestNilWatchNeverWakes(t *testing.T) {
	t.Parallel()
	var watch *fdWake
	if watch.wakes() != nil {
		t.Error("nil watch returned a channel, want nil so select never fires")
	}
}
