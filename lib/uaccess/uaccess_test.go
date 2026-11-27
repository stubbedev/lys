package uaccess

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	t     *testing.T
	roots Roots
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	r := Roots{Tags: filepath.Join(dir, "tags"), Data: filepath.Join(dir, "data"), Sys: filepath.Join(dir, "sys"), Dev: filepath.Join(dir, "dev")}
	for _, d := range []string{r.Tags, r.Data, r.Dev} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return fixture{t: t, roots: r}
}

func (f fixture) write(path, data string, mode os.FileMode) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		f.t.Fatal(err)
	}
}

func (f fixture) device(number, name, record string, mode os.FileMode) string {
	f.t.Helper()
	f.write(filepath.Join(f.roots.Tags, "c"+number), "", 0o644)
	f.write(filepath.Join(f.roots.Data, "c"+number), record, 0o644)
	f.write(filepath.Join(f.roots.Sys, "dev", "char", number, "uevent"), "MAJOR=1\nMINOR=2\nDEVNAME="+name+"\n", 0o644)
	node := filepath.Join(f.roots.Dev, name)
	f.write(node, "", mode)
	return node
}

func requireUnprivileged(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root may open every device, granted or not")
	}
}

func TestTheDevicesAreTheTaggedCharacterDevicesOfTheSeat(t *testing.T) {
	f := newFixture(t)
	udmabuf := f.device("10:125", "udmabuf", "G:uaccess\nQ:uaccess\n", 0o600)
	render := f.device("226:128", "dri/renderD128", "E:ID_SEAT=seat0\nG:uaccess\n", 0o600)
	f.device("116:3", "snd/pcmC1D0p", "E:ID_SEAT=seat1\nG:uaccess\n", 0o600)
	f.write(filepath.Join(f.roots.Tags, "+i2c:i2c-0"), "", 0o644)
	f.write(filepath.Join(f.roots.Tags, "b8:0"), "", 0o644)
	f.write(filepath.Join(f.roots.Tags, "c13:64"), "", 0o644)

	got, err := f.roots.Devices(DefaultSeat)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{render, udmabuf}; !slices.Equal(got, want) {
		t.Errorf("the devices of seat0 are %q, want %q", got, want)
	}
	other, err := f.roots.Devices("seat1")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || !strings.HasSuffix(other[0], "snd/pcmC1D0p") {
		t.Errorf("the devices of seat1 are %q, want its sound device alone", other)
	}
}

func TestASystemWithoutUaccessDevicesHasNone(t *testing.T) {
	r := Roots{Tags: filepath.Join(t.TempDir(), "absent")}
	got, err := r.Devices(DefaultSeat)
	if err != nil || len(got) != 0 {
		t.Errorf("Devices = %q, %v, want none", got, err)
	}
}

func TestADeviceWhoseUeventNamesNoNodeIsAnError(t *testing.T) {
	f := newFixture(t)
	f.device("10:125", "udmabuf", "", 0o600)
	f.write(filepath.Join(f.roots.Sys, "dev", "char", "10:125", "uevent"), "MAJOR=10\nMINOR=125\n", 0o644)
	if _, err := f.roots.Devices(DefaultSeat); err == nil || !strings.Contains(err.Error(), "names no device node") {
		t.Errorf("Devices = %v, want an error naming the uevent without a node", err)
	}
}

func TestAwaitReturnsAtOnceForGrantedOrGoneDevices(t *testing.T) {
	f := newFixture(t)
	open := f.device("10:125", "udmabuf", "", 0o600)
	if err := Await([]string{open, filepath.Join(f.roots.Dev, "gone")}, time.Hour); err != nil {
		t.Errorf("Await on a granted and a gone device: %v", err)
	}
	if err := Await(nil, time.Hour); err != nil {
		t.Errorf("Await on no devices: %v", err)
	}
}

func TestAwaitReturnsOnceTheDeviceIsGranted(t *testing.T) {
	requireUnprivileged(t)
	f := newFixture(t)
	node := f.device("10:125", "udmabuf", "", 0o000)
	done := make(chan error, 1)
	go func() { done <- Await([]string{node}, time.Hour) }()
	if err := os.Chmod(node, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Errorf("Await once the device was granted: %v", err)
	}
}

func TestAwaitOnADeviceNeverGrantedFailsNamingIt(t *testing.T) {
	requireUnprivileged(t)
	f := newFixture(t)
	closed := f.device("10:125", "udmabuf", "", 0o000)
	open := f.device("226:128", "renderD128", "", 0o600)
	err := Await([]string{closed, open}, time.Millisecond)
	if !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Await on a device never granted: %v, want ErrNotGranted", err)
	}
	if !strings.Contains(err.Error(), closed) || strings.Contains(err.Error(), open) {
		t.Errorf("the error %q does not name %s alone", err, closed)
	}
}
