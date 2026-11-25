package dm

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/drm"
	"golang.org/x/sys/unix"
)

type fakeCard struct {
	path    string
	dropErr error
	dropped bool
	closed  bool
	capErr  error
	fbs     map[uint32]uint32
	// cursors are the planes of fbs whose type is Cursor; every other
	// plane is Primary.
	cursors map[uint32]bool
}

const fakePlaneType = 7

func (c *fakeCard) Properties(id, _ uint32) (map[string]drm.Property, error) {
	kind := uint64(1)
	if c.cursors[id] {
		kind = 2
	}
	return map[string]drm.Property{"type": {ID: fakePlaneType, Value: kind}}, nil
}

func (c *fakeCard) PropertyInfo(id uint32) (drm.PropertyInfo, error) {
	if id != fakePlaneType {
		return drm.PropertyInfo{}, fmt.Errorf("no property %d", id)
	}
	return drm.PropertyInfo{Name: "type", Enums: map[string]uint64{"Overlay": 0, "Primary": 1, "Cursor": 2}}, nil
}

func (c *fakeCard) SetClientCap(uint64, uint64) error { return c.capErr }

func (c *fakeCard) Planes() ([]uint32, error) {
	return slices.Sorted(maps.Keys(c.fbs)), nil
}

func (c *fakeCard) Plane(id uint32) (drm.Plane, error) {
	return drm.Plane{ID: id, FB: c.fbs[id]}, nil
}

func (c *fakeCard) FD() int           { return -1 }
func (c *fakeCard) DropMaster() error { c.dropped = true; return c.dropErr }
func (c *fakeCard) Close() error      { c.closed = true; return nil }

func TestHeldCardsKeepEveryCardOpenWithoutMaster(t *testing.T) {
	dir := t.TempDir()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	touch("card0")
	touch("renderD128")
	opened := map[string]*fakeCard{}
	var order []string
	h := &heldCards{glob: filepath.Join(dir, "card[0-9]*"), cards: map[string]openCard{}, open: func(path string) (openCard, error) {
		c := &fakeCard{path: path, dropErr: unix.EINVAL}
		opened[path] = c
		order = append(order, filepath.Base(path))
		return c, nil
	}}
	if err := h.holdAll(); err != nil {
		t.Fatal(err)
	}
	touch("card1")
	if err := h.holdAll(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"card0", "card1"}) {
		t.Fatalf("opened %v, want each card once and no render node", order)
	}
	for path, c := range opened {
		if !c.dropped || c.closed {
			t.Errorf("%s: dropped master %t, closed %t; want it held open without master", path, c.dropped, c.closed)
		}
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	for path, c := range opened {
		if !c.closed {
			t.Errorf("%s is still open after Close", path)
		}
	}
}

func TestHeldCardsLetGoOfACardWhoseMasterTheyCannotDrop(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "card0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &fakeCard{dropErr: unix.EPERM}
	h := &heldCards{glob: filepath.Join(dir, "card[0-9]*"), cards: map[string]openCard{}, open: func(string) (openCard, error) { return c, nil }}
	if err := h.holdAll(); !errors.Is(err, unix.EPERM) {
		t.Fatalf("holdAll = %v, want the failed drop reported", err)
	}
	if !c.closed || len(h.cards) != 0 {
		t.Error("a card kept as DRM master would block the next compositor")
	}
}

func TestHeldCardsReadWhatEachPlaneScansOut(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "card0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &fakeCard{dropErr: unix.EINVAL, fbs: map[uint32]uint32{31: 40, 32: 0}}
	h := &heldCards{glob: filepath.Join(dir, "card[0-9]*"), cards: map[string]openCard{}, open: func(string) (openCard, error) { return c, nil }}
	if err := h.holdAll(); err != nil {
		t.Fatal(err)
	}
	read := h.scanouts()
	left, err := read()
	if err != nil {
		t.Fatal(err)
	}
	card := filepath.Join(dir, "card0")
	if want := (scanouts{card: {31: 40}}); !maps.EqualFunc(left, want, maps.Equal) {
		t.Fatalf("read %v, want %v: only the plane with a framebuffer", left, want)
	}
	c.fbs[32] = 50
	now, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if left.replacedIn(now) {
		t.Error("a frame lit on another plane counted as replacing the left one")
	}
	c.fbs[31] = 41
	if now, err = read(); err != nil {
		t.Fatal(err)
	}
	if !left.replacedIn(now) {
		t.Error("a flip to another framebuffer did not count as replacing the left frame")
	}
	delete(c.fbs, 31)
	if now, err = read(); err != nil {
		t.Fatal(err)
	}
	if !left.replacedIn(now) {
		t.Error("a plane turned off did not count as replacing the left frame")
	}
}

// The greeter's lys-wm turns its cursor plane off as it exits, so only
// the primary planes, which carry its last frame, are watched: a cursor
// plane going dark is not a compositor replacing the frame, and the
// console stays unbound until one does. A cursor plane is never part
// of what the greeter left.
func TestTheGreetersCursorGoingDarkIsNoTakeover(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "card0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &fakeCard{dropErr: unix.EINVAL, fbs: map[uint32]uint32{31: 40, 35: 60}, cursors: map[uint32]bool{35: true}}
	h := &heldCards{glob: filepath.Join(dir, "card[0-9]*"), cards: map[string]openCard{}, open: func(string) (openCard, error) { return c, nil }}
	if err := h.holdAll(); err != nil {
		t.Fatal(err)
	}
	read := h.scanouts()
	left, err := read()
	if err != nil {
		t.Fatal(err)
	}
	card := filepath.Join(dir, "card0")
	if want := (scanouts{card: {31: 40}}); !maps.EqualFunc(left, want, maps.Equal) {
		t.Fatalf("read %v, want %v: the primary plane alone", left, want)
	}
	c.fbs[35] = 0
	now, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if left.replacedIn(now) {
		t.Error("the greeter's cursor plane going dark counted as replacing its last frame")
	}
	c.fbs[35] = 61
	if now, err = read(); err != nil {
		t.Fatal(err)
	}
	if left.replacedIn(now) {
		t.Error("a new cursor counted as replacing the greeter's last frame")
	}
	c.fbs[31] = 41
	if now, err = read(); err != nil {
		t.Fatal(err)
	}
	if !left.replacedIn(now) {
		t.Error("a flip on the primary plane did not count as replacing the greeter's last frame")
	}
}

func TestHeldCardsLetGoOfACardThatCannotListItsPrimaryPlanes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "card0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &fakeCard{dropErr: unix.EINVAL, capErr: unix.EINVAL}
	h := &heldCards{glob: filepath.Join(dir, "card[0-9]*"), cards: map[string]openCard{}, open: func(string) (openCard, error) { return c, nil }}
	if err := h.holdAll(); !errors.Is(err, unix.EINVAL) || !strings.Contains(err.Error(), "list the primary planes of") {
		t.Fatalf("holdAll = %v, want the refused capability reported", err)
	}
	if !c.closed || len(h.cards) != 0 {
		t.Error("a card whose primary planes cannot be read is still held")
	}
}
