// Package camlight estimates the room's illuminance with a camera. A
// camera's auto exposure evens out how bright its frames look, so their
// luma alone does not follow the room: each sample lets auto exposure
// settle, then holds the exposure manually at the camera's default gain
// and brackets it until a frame is neither crushed nor clipped, and
// takes luma over exposure as the estimate. The controls are put back
// as they were afterwards. Frames are only averaged in memory: none is
// kept, written or sent anywhere.
package camlight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/v4l2"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/internal/ambient"
)

// ErrNone is a system without a camera that has exposure controls.
var ErrNone = errors.New("no camera with exposure controls")

// Params tune one sample.
type Params struct {
	SettleFrames  int
	DiscardFrames int
	Brackets      int
	Frame         time.Duration
	MaxExposure   int32
	LumaLow       float64
	LumaTarget    float64
	LumaHigh      float64
	LuxScale      float64
}

// meter is the part of a v4l2.Meter a sample uses.
type meter interface {
	Infrared() (bool, error)
	Start() error
	QueryControl(id v4l2.ControlID) (v4l2.ControlRange, error)
	Control(id v4l2.ControlID) (int32, error)
	SetControl(id v4l2.ControlID, value int32) error
	MeanLuma(timeout time.Duration) (float64, error)
	Close() error
}

// Camera samples one capture node.
type Camera struct {
	device string
	params Params
	open   func(device string) (meter, error)
}

func openMeter(device string) (meter, error) { return v4l2.OpenMeter(device) }

// Find returns the first capture node in dir, in number order, that has
// manual exposure and is no infrared camera, whose frames show its own
// illuminator rather than the room; ErrNone when there is none. It reads
// formats and controls only, so no camera switches on.
func Find(dir string, p Params) (Camera, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Camera{}, fmt.Errorf("list camera nodes in %s: %w", dir, err)
	}
	var nodes []int
	for _, entry := range entries {
		n, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "video"))
		if err == nil && strings.HasPrefix(entry.Name(), "video") {
			nodes = append(nodes, n)
		}
	}
	slices.Sort(nodes)
	for _, n := range nodes {
		device := filepath.Join(dir, "video"+strconv.Itoa(n))
		if _, ok := v4l2.CaptureCard(device); !ok {
			continue
		}
		c := Camera{device: device, params: p, open: openMeter}
		ok, err := c.measures()
		if err != nil {
			return Camera{}, err
		}
		if ok {
			return c, nil
		}
	}
	return Camera{}, fmt.Errorf("%w in %s", ErrNone, dir)
}

func (c Camera) measures() (ok bool, err error) {
	m, err := c.open(c.device)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.Close()) }()
	if infrared, err := m.Infrared(); err != nil || infrared {
		return false, err
	}
	for _, id := range []v4l2.ControlID{v4l2.ControlExposureAuto, v4l2.ControlExposureAbsolute} {
		if _, err := m.QueryControl(id); errors.Is(err, v4l2.ErrNoControl) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	return true, nil
}

// String is the camera's node.
func (c Camera) String() string { return c.device }

// Sample switches the camera on, estimates the illuminance and switches
// it off. A camera another application streams is ambient.ErrNoSample,
// its controls untouched.
func (c Camera) Sample(ctx context.Context) (lux ambient.Lux, err error) {
	m, err := c.open(c.device)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, m.Close()) }()
	if err := m.Start(); err != nil {
		if errors.Is(err, unix.EBUSY) {
			return 0, fmt.Errorf("%w: %s is in use: %w", ambient.ErrNoSample, c.device, err)
		}
		return 0, err
	}
	return c.measure(ctx, m)
}

// measure brackets a streaming camera's exposure, restoring every
// control it set in reverse order whatever the outcome.
func (c Camera) measure(ctx context.Context, m meter) (lux ambient.Lux, err error) {
	exposure, err := m.QueryControl(v4l2.ControlExposureAbsolute)
	if err != nil {
		return 0, err
	}
	bracket, err := ambient.NewBracket(max(exposure.Min, 1), min(exposure.Max, c.params.MaxExposure), c.params.LumaLow, c.params.LumaTarget, c.params.LumaHigh)
	if err != nil {
		return 0, fmt.Errorf("camera %s: %w", c.device, err)
	}
	gain, err := m.QueryControl(v4l2.ControlGain)
	hasGain := err == nil
	if err != nil && !errors.Is(err, v4l2.ErrNoControl) {
		return 0, err
	}

	var restores []func() error
	defer func() {
		for _, restore := range slices.Backward(restores) {
			err = errors.Join(err, restore())
		}
	}()
	set := func(id v4l2.ControlID, value int32) error {
		before, err := m.Control(id)
		if err != nil {
			return err
		}
		if err := m.SetControl(id, value); err != nil {
			return err
		}
		restores = append(restores, func() error { return m.SetControl(id, before) })
		return nil
	}

	if err := c.frames(ctx, m, c.params.SettleFrames); err != nil {
		return 0, err
	}
	settled, err := m.Control(v4l2.ControlExposureAbsolute)
	if err != nil {
		return 0, err
	}
	if err := set(v4l2.ControlExposureAuto, v4l2.ExposureManual); err != nil {
		return 0, err
	}
	if hasGain {
		if err := set(v4l2.ControlGain, gain.Default); err != nil {
			return 0, err
		}
	}
	at := bracket.Clamp(settled)
	if err := set(v4l2.ControlExposureAbsolute, at); err != nil {
		return 0, err
	}
	for i := range c.params.Brackets {
		if i > 0 {
			if err := m.SetControl(v4l2.ControlExposureAbsolute, at); err != nil {
				return 0, err
			}
		}
		if err := c.frames(ctx, m, c.params.DiscardFrames); err != nil {
			return 0, err
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		luma, err := m.MeanLuma(c.params.Frame)
		if err != nil {
			return 0, err
		}
		next, done := bracket.Next(at, luma)
		if done {
			return ambient.CameraLux(c.params.LuxScale, luma, at), nil
		}
		at = next
	}
	return 0, fmt.Errorf("%w: camera %s found no exposure in %d tries", ambient.ErrNoSample, c.device, c.params.Brackets)
}

// frames drops n frames.
func (c Camera) frames(ctx context.Context, m meter, n int) error {
	for range n {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := m.MeanLuma(c.params.Frame); err != nil {
			return err
		}
	}
	return nil
}
