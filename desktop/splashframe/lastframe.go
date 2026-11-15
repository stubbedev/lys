package splashframe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/render"
)

// LastFramePath is where a splash that hands its screen over records its
// last frame, for as long as it runs.
const LastFramePath = "/run/lys-splash/last-frame.json"

// Logo is which logo a splash frame shows.
type Logo uint8

const (
	// LogoLys is the lys logo.
	LogoLys Logo = iota
	// LogoFirmware is the firmware's logo from the ACPI BGRT table.
	LogoFirmware
)

var logoNames = map[Logo]string{LogoLys: "lys", LogoFirmware: "firmware"}

// Output is one output the splash drew its last frame on: its connector,
// the frame's size in device pixels and the scale the grid had there.
type Output struct {
	Name  string
	Size  image.Point
	Scale int
}

// LastFrame is the splash's last frame as the greeter reproduces it: the
// background and logo of its Picture on every output, the colour the lys
// logo is drawn in, and whether the splash's theme asked for cuts instead
// of fades.
type LastFrame struct {
	Background    render.Color
	Logo          Logo
	LogoColor     render.Color
	ReducedMotion bool
	Outputs       []Output
}

type wireOutput struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Scale  int    `json:"scale"`
}

type wireFrame struct {
	Background    string       `json:"background"`
	Logo          string       `json:"logo"`
	LogoColor     string       `json:"logo-color"`
	ReducedMotion bool         `json:"reduced-motion"`
	Outputs       []wireOutput `json:"outputs"`
}

// FormatColor is c as #rrggbb.
func FormatColor(c render.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R(), c.G(), c.B())
}

// ParseColor reads an opaque #rrggbb color.
func ParseColor(s string) (render.Color, error) {
	if len(s) != 7 || s[0] != '#' {
		return 0, fmt.Errorf("%q is not #rrggbb", s)
	}
	rgb, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not #rrggbb", s)
	}
	return render.Color(0xff000000 | intconv.Must[uint32](rgb)), nil
}

// Encode is f as the record LoadLastFrame reads.
func (f LastFrame) Encode() ([]byte, error) {
	name, ok := logoNames[f.Logo]
	if !ok {
		return nil, fmt.Errorf("splashframe: record a last frame: logo %d is none of lys and firmware", f.Logo)
	}
	w := wireFrame{Background: FormatColor(f.Background), Logo: name, LogoColor: FormatColor(f.LogoColor), ReducedMotion: f.ReducedMotion, Outputs: make([]wireOutput, len(f.Outputs))}
	for i, o := range f.Outputs {
		w.Outputs[i] = wireOutput{Name: o.Name, Width: o.Size.X, Height: o.Size.Y, Scale: o.Scale}
	}
	data, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("splashframe: record a last frame: %w", err)
	}
	return append(data, '\n'), nil
}

// ParseLastFrame reads a record Encode wrote: every field present and
// valid, and at least one output.
func ParseLastFrame(data []byte) (LastFrame, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var w wireFrame
	if err := dec.Decode(&w); err != nil {
		return LastFrame{}, err
	}
	bg, err := ParseColor(w.Background)
	if err != nil {
		return LastFrame{}, fmt.Errorf("background: %w", err)
	}
	logoColor, err := ParseColor(w.LogoColor)
	if err != nil {
		return LastFrame{}, fmt.Errorf("logo-color: %w", err)
	}
	f := LastFrame{Background: bg, LogoColor: logoColor, ReducedMotion: w.ReducedMotion, Outputs: make([]Output, len(w.Outputs))}
	found := false
	for logo, name := range logoNames {
		if name == w.Logo {
			f.Logo, found = logo, true
		}
	}
	if !found {
		return LastFrame{}, fmt.Errorf("logo: %q is none of lys and firmware", w.Logo)
	}
	if len(w.Outputs) == 0 {
		return LastFrame{}, errors.New("outputs: none")
	}
	for i, o := range w.Outputs {
		if o.Name == "" || o.Width <= 0 || o.Height <= 0 || o.Scale < 1 {
			return LastFrame{}, fmt.Errorf("outputs: %+v needs a name, a positive size and a scale of at least 1", o)
		}
		f.Outputs[i] = Output{Name: o.Name, Size: image.Pt(o.Width, o.Height), Scale: o.Scale}
	}
	return f, nil
}

// SaveLastFrame records f at path, replacing what was there whole.
func SaveLastFrame(path string, f LastFrame) error {
	data, err := f.Encode()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // the greeter, another user, reads the record
		return fmt.Errorf("splashframe: create the directory of %s: %w", path, err)
	}
	if err := atomicfile.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("splashframe: record the last frame in %s: %w", path, err)
	}
	return nil
}

// LoadLastFrame reads the record at path; with no splash handing over
// there is none, and the error is fs.ErrNotExist.
func LoadLastFrame(path string) (LastFrame, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the record's path is fixed
	if err != nil {
		return LastFrame{}, fmt.Errorf("splashframe: read the splash's last frame: %w", err)
	}
	f, err := ParseLastFrame(data)
	if err != nil {
		return LastFrame{}, fmt.Errorf("splashframe: the splash's last frame in %s: %w", path, err)
	}
	return f, nil
}

// Named is the output the splash drew on under the connector name, an
// error naming the outputs it did draw on when there is none.
func (f LastFrame) Named(name string) (Output, error) {
	for _, o := range f.Outputs {
		if o.Name == name {
			return o, nil
		}
	}
	return Output{}, fmt.Errorf("splashframe: the splash drew nothing on %s, only on %v", name, f.Outputs)
}
