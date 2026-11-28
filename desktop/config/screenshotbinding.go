package config

import (
	"fmt"
	"strings"
)

// ScreenshotMode is what a screenshot captures: a drag-selected region,
// one output (the focused one without a target), the whole
// multi-monitor space composited, or the active window.
type ScreenshotMode string

// ScreenshotMode values.
const (
	ScreenshotRegion ScreenshotMode = "region"
	ScreenshotOutput ScreenshotMode = "output"
	ScreenshotScreen ScreenshotMode = "screen"
	ScreenshotWindow ScreenshotMode = "window"
)

// ScreenshotModes are every ScreenshotMode.
var ScreenshotModes = []ScreenshotMode{ScreenshotRegion, ScreenshotOutput, ScreenshotScreen, ScreenshotWindow}

// ParseScreenshotMode reads a mode named on the command line, the bus or
// in a binding.
func ParseScreenshotMode(s string) (ScreenshotMode, error) {
	return parseVariant("screenshot mode", ScreenshotModes, s)
}

// ScreenReading is what a reading of a dragged region looks for: its
// text, or the QR code in it.
type ScreenReading string

// ScreenReading values.
const (
	ScreenReadText ScreenReading = "text"
	ScreenReadQR   ScreenReading = "qr"
)

// ScreenReadings are every ScreenReading.
var ScreenReadings = []ScreenReading{ScreenReadText, ScreenReadQR}

// ParseScreenReading reads a reading named on the command line, the bus
// or in a binding.
func ParseScreenReading(s string) (ScreenReading, error) {
	return parseVariant("screen reading", ScreenReadings, s)
}

// ScreenshotBinding is a `lys-shell screenshot <mode> [<output>]`
// binding the running shell captures in process. Output is the
// connector output mode captures, empty for the focused output, and
// always empty in every other mode.
type ScreenshotBinding struct {
	Mode   ScreenshotMode
	Output string
}

const screenshotCommand = "lys-shell screenshot"

// screenshotBindingUsage is what a `lys-shell screenshot` binding may say.
const screenshotBindingUsage = screenshotCommand + " region|output [<output>]|screen|window|text|qr"

// parseScreenshotClick reads a `lys-shell screenshot` command line into
// its capture or reading; ok is false for any other command line. A
// mode the shell has not, or an argument the mode takes none of, does
// not parse.
func parseScreenshotClick(s string) (a ClickAction, ok bool, err error) {
	f := strings.Fields(s)
	if len(f) < 2 || f[0]+" "+f[1] != screenshotCommand {
		return ClickAction{}, false, nil
	}
	args := f[2:]
	if len(args) == 0 {
		return ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotRegion}}, true, nil
	}
	if r, err := ParseScreenReading(args[0]); err == nil {
		if len(args) > 1 {
			return ClickAction{}, true, fmt.Errorf("invalid value: %q, expected %s: %s takes no argument", s, screenshotBindingUsage, r)
		}
		return ClickAction{Kind: ClickScreenRead, Reading: r}, true, nil
	}
	mode, err := ParseScreenshotMode(args[0])
	if err != nil {
		return ClickAction{}, true, fmt.Errorf("invalid value: %q, expected %s: %w", s, screenshotBindingUsage, err)
	}
	b := ScreenshotBinding{Mode: mode}
	switch {
	case len(args) == 2 && mode == ScreenshotOutput:
		b.Output = args[1]
	case len(args) > 1:
		return ClickAction{}, true, fmt.Errorf("invalid value: %q, expected %s: %s takes no further argument", s, screenshotBindingUsage, mode)
	}
	return ClickAction{Kind: ClickScreenshot, Screenshot: b}, true, nil
}

// String is the binding's `lys-shell screenshot` command line.
func (b ScreenshotBinding) String() string {
	s := screenshotCommand + " " + string(b.Mode)
	if b.Output != "" {
		s += " " + b.Output
	}
	return s
}
