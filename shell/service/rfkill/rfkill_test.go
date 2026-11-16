package rfkill

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func deviceFile(t *testing.T, events ...Event) string {
	t.Helper()
	var b []byte
	for _, e := range events {
		w := e.Encode()
		b = append(b, w[:]...)
	}
	path := filepath.Join(t.TempDir(), "rfkill")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func written(t *testing.T, path string) []Event {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b)%EventSize != 0 {
		t.Fatalf("the device holds %d bytes, not whole events", len(b))
	}
	var out []Event
	for off := 0; off < len(b); off += EventSize {
		out = append(out, Decode([EventSize]byte(b[off:off+EventSize])))
	}
	return out
}

func TestAnEventSurvivesItsWireForm(t *testing.T) {
	for _, e := range []Event{
		{Index: 7, Type: TypeBluetooth, Op: OpChange, Soft: true},
		{Index: 1 << 30, Type: TypeWWAN, Op: OpAdd, Hard: true},
		{Type: TypeWLAN, Op: OpChangeAll},
	} {
		if got := Decode(e.Encode()); got != e {
			t.Errorf("Decode(Encode(%+v)) = %+v", e, got)
		}
	}
	b := Event{Index: 1, Type: TypeNFC, Op: OpChangeAll, Soft: true}.Encode()
	if b[4] != byte(TypeNFC) || b[5] != byte(OpChangeAll) || b[6] != 1 || b[7] != 0 {
		t.Errorf("the wire form %v puts the type, op and flags out of place", b)
	}
}

func TestReadFoldsTheDeviceEventsIntoTheSwitches(t *testing.T) {
	path := deviceFile(t,
		Event{Index: 0, Type: TypeWLAN, Op: OpAdd, Soft: true},
		Event{Index: 1, Type: TypeBluetooth, Op: OpAdd},
		Event{Index: 2, Type: TypeGPS, Op: OpAdd},
		Event{Index: 3, Type: TypeWWAN, Op: OpAdd},
		Event{Index: 1, Type: TypeBluetooth, Op: OpChange, Soft: true},
		Event{Index: 3, Op: OpDel},
	)
	snap, err := New(path).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Radio{{Index: 0, Type: TypeWLAN, Soft: true}, {Index: 1, Type: TypeBluetooth, Soft: true}, {Index: 2, Type: TypeGPS}}
	if !slices.Equal(snap.Radios, want) {
		t.Errorf("radios = %+v, want %+v", snap.Radios, want)
	}
	if !snap.Available() || !snap.Airplane() {
		t.Errorf("every transmitter soft-blocked, a receiver not: available %v, airplane %v; want both", snap.Available(), snap.Airplane())
	}
}

func TestAirplaneModeNeedsEveryTransmitterSoftBlocked(t *testing.T) {
	for _, tc := range []struct {
		name                string
		radios              []Radio
		available, airplane bool
	}{
		{"no switches", nil, false, false},
		{"only receivers", []Radio{{Type: TypeGPS, Soft: true}, {Index: 1, Type: TypeFM}}, false, false},
		{"one transmitter unblocked", []Radio{{Type: TypeWLAN, Soft: true}, {Index: 1, Type: TypeBluetooth}}, true, false},
		{"hard-blocked only", []Radio{{Type: TypeWLAN, Hard: true}}, true, false},
		{"all soft-blocked", []Radio{{Type: TypeWLAN, Soft: true}, {Index: 1, Type: TypeWWAN, Soft: true, Hard: true}}, true, true},
	} {
		s := Snapshot{Radios: tc.radios}
		if s.Available() != tc.available || s.Airplane() != tc.airplane {
			t.Errorf("%s: available %v airplane %v, want %v %v", tc.name, s.Available(), s.Airplane(), tc.available, tc.airplane)
		}
	}
}

func TestReadRefusesATruncatedEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rfkill")
	if err := os.WriteFile(path, []byte{1, 0, 0, 0, 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := New(path).Read(context.Background())
	if err == nil || !strings.Contains(err.Error(), "5-byte event") || !strings.Contains(err.Error(), path) {
		t.Errorf("a truncated event read as %v, want an error naming the size and the device", err)
	}
}

func TestReadOfAMissingDeviceNamesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	_, err := New(path).Read(context.Background())
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Errorf("a missing device read as %v, want not-exist naming %s", err, path)
	}
}

func TestSetAirplaneSwitchesEveryTransmitterClass(t *testing.T) {
	for _, on := range []bool{true, false} {
		path := deviceFile(t)
		if err := New(path).SetAirplane(context.Background(), on); err != nil {
			t.Fatal(err)
		}
		var want []Event
		for _, ty := range AirplaneTypes {
			want = append(want, Event{Type: ty, Op: OpChangeAll, Soft: on})
		}
		if got := written(t, path); !slices.Equal(got, want) {
			t.Errorf("airplane %v wrote %+v, want %+v", on, got, want)
		}
	}
	for _, ty := range AirplaneTypes {
		if ty == TypeGPS || ty == TypeFM || ty == TypeAll {
			t.Errorf("airplane mode switches %d, which is not a transmitter class", ty)
		}
	}
	for _, ty := range []Type{TypeWLAN, TypeBluetooth, TypeWWAN} {
		if !slices.Contains(AirplaneTypes, ty) {
			t.Errorf("airplane mode leaves radio type %d alone", ty)
		}
	}
}

func TestARefusedAirplaneSwitchSaysWhy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	err := New(path).SetAirplane(context.Background(), true)
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Errorf("switching through a missing device = %v, want not-exist naming %s", err, path)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New(deviceFile(t)).SetAirplane(ctx, true); !errors.Is(err, context.Canceled) {
		t.Errorf("switching under a done context = %v, want context.Canceled", err)
	}
}

func fifoDevice(t *testing.T) (string, *os.File) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rfkill")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return path, w
}

func TestSubscribeTicksOnEachDeviceEventAndClosesOnStop(t *testing.T) {
	path, w := fifoDevice(t)
	ticks, stop, err := New(path).Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ev := Event{Index: 2, Type: TypeBluetooth, Op: OpChange, Soft: true}.Encode()
	for range 2 {
		if _, err := w.Write(ev[:]); err != nil {
			t.Fatal(err)
		}
		if _, ok := <-ticks; !ok {
			t.Fatal("the ticks closed on an event")
		}
	}
	if err := stop(); err != nil {
		t.Errorf("stop = %v", err)
	}
	if _, ok := <-ticks; ok {
		t.Error("a tick after stop")
	}
	if err := stop(); err != nil {
		t.Errorf("a second stop = %v", err)
	}
}

func TestSubscribeEndsWithItsContext(t *testing.T) {
	path, _ := fifoDevice(t)
	ctx, cancel := context.WithCancel(context.Background())
	ticks, stop, err := New(path).Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range ticks {
		t.Error("a tick without an event")
	}
	if err := stop(); err != nil {
		t.Errorf("stop after the context ended = %v", err)
	}
}

func TestSubscribeToAMissingDeviceFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	if _, _, err := New(path).Subscribe(context.Background()); !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Errorf("following a missing device = %v, want not-exist naming %s", err, path)
	}
}
