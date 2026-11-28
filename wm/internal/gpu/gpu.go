// Package gpu binds libgbm through purego for buffer allocation, and holds the
// dmabuf and YUV types the renderers share and the loop thread's pinning.
package gpu

import (
	"errors"
	"fmt"
	"image"

	"github.com/ebitengine/purego"
	"github.com/stubbedev/lys/lib/dmabuf"
	"github.com/stubbedev/lys/lib/fourcc"
)

const GBMEnv = "LYS_WM_LIBGBM"

var ErrUnsupported = errors.New("gpu: unsupported")

var defaultGBM = "libgbm.so.1"

type Paths struct {
	GBM string
}

func PathsFromEnv(getenv func(string) string) Paths {
	if p := getenv(GBMEnv); p != "" {
		return Paths{GBM: p}
	}
	return Paths{GBM: defaultGBM}
}

func open(path, env string) (uintptr, error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return 0, fmt.Errorf("gpu: load %s (set %s to its path): %w", path, env, err)
	}
	return h, nil
}

type symbol struct {
	fn   any
	name string
}

func bind(lib uintptr, path string, syms []symbol) error {
	for _, s := range syms {
		if _, err := purego.Dlsym(lib, s.name); err != nil {
			return fmt.Errorf("gpu: %s lacks %s: %w", path, s.name, err)
		}
		if raw, ok := s.fn.(*uintptr); ok {
			*raw, _ = purego.Dlsym(lib, s.name)
			continue
		}
		purego.RegisterLibFunc(s.fn, lib, s.name)
	}
	return nil
}

type Plane struct {
	FD     int
	Offset uint32
	Stride uint32
}

type DMABUF struct {
	Size     image.Point
	Format   fourcc.Format
	Modifier fourcc.Modifier
	Planes   []Plane
	YUV      YUVEncoding
}

func (d DMABUF) Dmabuf() dmabuf.Buffer {
	planes := make([]dmabuf.Plane, len(d.Planes))
	for i, p := range d.Planes {
		planes[i] = dmabuf.Plane{FD: p.FD, Offset: p.Offset, Stride: p.Stride}
	}
	return dmabuf.Buffer{Width: d.Size.X, Height: d.Size.Y, Format: d.Format, Modifier: d.Modifier, Planes: planes}
}

func FromDmabuf(b dmabuf.Buffer) DMABUF {
	planes := make([]Plane, len(b.Planes))
	for i, p := range b.Planes {
		planes[i] = Plane{FD: p.FD, Offset: p.Offset, Stride: p.Stride}
	}
	return DMABUF{Size: image.Pt(b.Width, b.Height), Format: b.Format, Modifier: b.Modifier, Planes: planes}
}

type Coefficients uint8

const (
	CoefficientsUnspecified Coefficients = iota
	BT601
	BT709
	BT2020
)

type Range uint8

const (
	RangeUnspecified Range = iota
	LimitedRange
	FullRange
)

type Siting uint8

const (
	SitingUnspecified Siting = iota
	SitedAt0
	SitedAtHalf
	SitedAt1
)

type ChromaSiting struct {
	Horizontal, Vertical Siting
}

type YUVEncoding struct {
	Coefficients Coefficients
	Range        Range
	Chroma       ChromaSiting
}

func DefaultYUV(size image.Point) YUVEncoding {
	if size.Y >= 720 {
		return YUVEncoding{Coefficients: BT709, Range: LimitedRange}
	}
	return YUVEncoding{Coefficients: BT601, Range: LimitedRange}
}

type FormatModifier struct {
	Format   fourcc.Format
	Modifier fourcc.Modifier
}

type ScanoutTarget struct {
	Device  uint64
	Formats []FormatModifier
}
