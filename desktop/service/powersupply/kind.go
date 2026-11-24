package powersupply

import (
	"errors"
	"fmt"
	"io/fs"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DeviceKind is what a peripheral is, told by the input devices its
// battery belongs to.
type DeviceKind uint8

// Device kinds, in the order a device with several inputs is named by:
// a mouse with macro keys is a mouse.
const (
	KindUnknown DeviceKind = iota
	KindKeyboard
	KindMouse
	KindTouchpad
	KindGamingInput
	KindPen
)

const (
	keyEsc        = 1
	keyD          = 32
	btnMouse      = 0x110
	btnJoystick   = 0x120
	btnGamepad    = 0x130
	btnToolPen    = 0x140
	btnToolFinger = 0x145
	btnStylus     = 0x14b
	relX          = 0
	relY          = 1
	absX          = 0
)

// kindOf names the peripheral whose supply sits at dir by the input
// devices of the device it belongs to; KindUnknown without any.
func kindOf(dir string) (DeviceKind, error) {
	inputs, err := filepath.Glob(filepath.Join(dir, "device", "input", "input*", "capabilities"))
	if err != nil {
		return KindUnknown, fmt.Errorf("powersupply: list the inputs of %s: %w", dir, err)
	}
	kind := KindUnknown
	for _, caps := range inputs {
		k, err := inputKind(caps)
		if err != nil {
			return KindUnknown, err
		}
		kind = max(kind, k)
	}
	return kind, nil
}

func inputKind(caps string) (DeviceKind, error) {
	var maps [3]bitmap
	for i, name := range [...]string{"key", "rel", "abs"} {
		m, err := readBitmap(filepath.Join(caps, name))
		if err != nil {
			return KindUnknown, err
		}
		maps[i] = m
	}
	key, rel, abs := maps[0], maps[1], maps[2]
	switch {
	case key.has(btnToolPen) || key.has(btnStylus):
		return KindPen, nil
	case key.has(btnJoystick) || key.has(btnGamepad):
		return KindGamingInput, nil
	case key.has(btnToolFinger) && abs.has(absX):
		return KindTouchpad, nil
	case key.has(btnMouse) && rel.has(relX) && rel.has(relY):
		return KindMouse, nil
	case key.hasAll(keyEsc, keyD):
		return KindKeyboard, nil
	}
	return KindUnknown, nil
}

// bitmap is a capability bitmap as the kernel prints it: words of the
// kernel's long, most significant first.
type bitmap []uint

func readBitmap(path string) (bitmap, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is a supply under Dir and its device's capabilities
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("powersupply: read %s: %w", path, err)
	}
	fields := strings.Fields(string(raw))
	words := make(bitmap, len(fields))
	for i, f := range fields {
		w, err := strconv.ParseUint(f, 16, bits.UintSize)
		if err != nil {
			return nil, fmt.Errorf("%w: %s is %q", ErrMalformed, path, raw)
		}
		words[len(fields)-1-i] = uint(w)
	}
	return words, nil
}

func (b bitmap) has(bit int) bool {
	word := bit / bits.UintSize
	return word < len(b) && b[word]&(1<<(bit%bits.UintSize)) != 0
}

func (b bitmap) hasAll(from, to int) bool {
	for bit := from; bit <= to; bit++ {
		if !b.has(bit) {
			return false
		}
	}
	return true
}
