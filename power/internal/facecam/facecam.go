// Package facecam switches a camera on for the attention check: a few
// luma frames at its smallest size, each judged in memory and cleared,
// then the camera off. Infrared cameras come first, as their own
// illuminator shows a face in the dark. Nothing is kept, written or sent.
package facecam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/v4l2"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/internal/presence"
)

var (
	ErrNone = errors.New("no camera")
	ErrBusy = errors.New("the camera is in use")
)

type Device interface {
	Infrared() (bool, error)
	Start() error
	Luma(timeout time.Duration, dst *image.Gray) error
	Close() error
}

type Devices interface {
	Capture(path string) bool
	Open(path string) (Device, error)
}

type Params struct {
	SettleFrames int
	Frames       int
	Frame        time.Duration
}

type Camera struct {
	path     string
	infrared bool
	devices  Devices
	judge    presence.Judge
	params   Params
}

func (c Camera) String() string {
	if c.infrared {
		return c.path + " (infrared)"
	}
	return c.path
}

func Find(dir string, devices Devices, judge presence.Judge, p Params) ([]Camera, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list camera nodes in %s: %w", dir, err)
	}
	var nodes []int
	for _, entry := range entries {
		digits, ok := strings.CutPrefix(entry.Name(), "video")
		if n, err := strconv.Atoi(digits); ok && err == nil {
			nodes = append(nodes, n)
		}
	}
	slices.Sort(nodes)
	var cameras []Camera
	for _, n := range nodes {
		path := filepath.Join(dir, "video"+strconv.Itoa(n))
		if !devices.Capture(path) {
			continue
		}
		infrared, err := probe(devices, path)
		if err != nil {
			return nil, err
		}
		cameras = append(cameras, Camera{path: path, infrared: infrared, devices: devices, judge: judge, params: p})
	}
	if len(cameras) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNone, dir)
	}
	slices.SortStableFunc(cameras, func(a, b Camera) int {
		switch {
		case a.infrared == b.infrared:
			return 0
		case a.infrared:
			return -1
		}
		return 1
	})
	return cameras, nil
}

func probe(devices Devices, path string) (infrared bool, err error) {
	d, err := devices.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	return d.Infrared()
}

func (c Camera) Look(ctx context.Context) (look presence.Look, err error) {
	d, err := c.devices.Open(c.path)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	if err := d.Start(); err != nil {
		if errors.Is(err, unix.EBUSY) {
			return 0, fmt.Errorf("%w: %s: %w", ErrBusy, c.path, err)
		}
		return 0, err
	}
	var frame image.Gray
	defer func() { clear(frame.Pix) }()
	for range c.params.SettleFrames {
		if err := c.next(ctx, d, &frame); err != nil {
			return 0, err
		}
	}
	looks := make([]presence.Look, 0, c.params.Frames)
	for range c.params.Frames {
		if err := c.next(ctx, d, &frame); err != nil {
			return 0, err
		}
		look := c.judge.Look(&frame)
		if look == presence.Face {
			return look, nil
		}
		looks = append(looks, look)
	}
	return presence.Of(looks), nil
}

func (c Camera) next(ctx context.Context, d Device, frame *image.Gray) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("look through %s: %w", c.path, err)
	}
	return d.Luma(c.params.Frame, frame)
}

type V4L2 struct{}

func (V4L2) Capture(path string) bool {
	_, ok := v4l2.CaptureCard(path)
	return ok
}

func (V4L2) Open(path string) (Device, error) { return v4l2.OpenMeter(path) }
