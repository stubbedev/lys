package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Resample scales srcRect of src into a fresh dstW x dstH RGBA with the
// CatmullRom kernel. Cubic resampling is what keeps fractional and
// non-integer down- and upscales from the blockiness of a
// nearest-neighbor pass (stubbedev/lys-ui#14); the canvas's own image
// drawing is nearest-neighbor and must never see unresampled pixels.
//
// Memory stays bounded by the rows in flight, never the image: a
// downscale by two or more first box-averages whole factor blocks (one
// source row read at a time), and the cubic pass keeps only the
// horizontally filtered rows its vertical taps span. Any source model
// reads through a row converter, YCbCr and NRGBA without a per-pixel
// interface call.
func Resample(src image.Image, srcRect image.Rectangle, dstW, dstH int) *image.RGBA {
	if dstW <= 0 || dstH <= 0 {
		return image.NewRGBA(image.Rectangle{})
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	resample(dst, src, srcRect)
	return dst
}

// ResampleTo is Resample into an image the caller owns, pixels its
// memory holds (reused, or off the Go heap): srcRect of src scaled
// over all of dst, at dst's own stride and origin, so a sub-image
// takes it too. It writes every pixel of dst, the same bytes Resample
// would, and refuses an empty dst or one its pixels cannot hold.
func ResampleTo(dst *image.RGBA, src image.Image, srcRect image.Rectangle) error {
	r := dst.Rect
	if r.Empty() {
		return fmt.Errorf("render: resampling into an empty image %v", r)
	}
	if dst.Stride < 4*r.Dx() || len(dst.Pix) < dst.PixOffset(r.Max.X-1, r.Max.Y-1)+4 {
		return fmt.Errorf("render: a %v image of stride %d does not fit its %d pixel bytes", r, dst.Stride, len(dst.Pix))
	}
	resample(dst, src, srcRect)
	return nil
}

// resample scales srcRect of src over all of dst, a fully transparent
// dst when the two do not overlap.
func resample(dst *image.RGBA, src image.Image, srcRect image.Rectangle) {
	r := srcRect.Intersect(src.Bounds())
	if r.Empty() {
		for y := dst.Rect.Min.Y; y < dst.Rect.Max.Y; y++ {
			at := dst.PixOffset(dst.Rect.Min.X, y)
			clear(dst.Pix[at : at+4*dst.Rect.Dx()])
		}
		return
	}
	rows := boxReduce(src, r, max(1, min(r.Dx()/dst.Rect.Dx(), r.Dy()/dst.Rect.Dy())))
	cubicScale(dst, rows)
}

// rowSource yields a source's premultiplied RGBA rows, w pixels wide
// and h tall.
type rowSource struct {
	w, h int
	row  func(y int) []uint8
}

// boxReduce reads r of src as premultiplied RGBA rows, each k x k
// block averaged into one pixel (k of 1 converts only).
func boxReduce(src image.Image, r image.Rectangle, k int) rowSource {
	read := rowReader(src, r)
	if k == 1 {
		return rowSource{w: r.Dx(), h: r.Dy(), row: read}
	}
	w, h := (r.Dx()+k-1)/k, (r.Dy()+k-1)/k
	sums := make([]uint64, w*4)
	out := make([]uint8, w*4)
	return rowSource{w: w, h: h, row: func(y int) []uint8 {
		clear(sums)
		y0, y1 := y*k, min((y+1)*k, r.Dy())
		for sy := y0; sy < y1; sy++ {
			line := read(sy)
			for x := range w {
				block := line[x*k*4 : min((x+1)*k, r.Dx())*4]
				var s0, s1, s2, s3 uint64
				for len(block) >= 4 {
					p := block[:4:4]
					s0, s1, s2, s3 = s0+uint64(p[0]), s1+uint64(p[1]), s2+uint64(p[2]), s3+uint64(p[3])
					block = block[4:]
				}
				acc := sums[x*4 : x*4+4 : x*4+4]
				acc[0], acc[1], acc[2], acc[3] = acc[0]+s0, acc[1]+s1, acc[2]+s2, acc[3]+s3
			}
		}
		for x := range w {
			n := uint64(max(1, (min((x+1)*k, r.Dx())-x*k)*(y1-y0)))
			for c := range 4 {
				out[x*4+c] = uint8(min((sums[x*4+c]+n/2)/n, 255))
			}
		}
		return out
	}}
}

// rowReader returns row y (0 at r's top) of src over r as premultiplied
// RGBA, in a buffer reused between calls: an RGBA source's, still or
// versioned too, read in place.
func rowReader(src image.Image, r image.Rectangle) func(y int) []uint8 {
	w := r.Dx()
	if s := rgbaOf(src); s != nil {
		return func(y int) []uint8 {
			o := s.PixOffset(r.Min.X, r.Min.Y+y)
			return s.Pix[o : o+w*4]
		}
	}
	buf := make([]uint8, w*4)
	switch s := src.(type) {
	case *image.NRGBA:
		return func(y int) []uint8 {
			o := s.PixOffset(r.Min.X, r.Min.Y+y)
			for x := range w {
				p := s.Pix[o+x*4 : o+x*4+4]
				a := uint32(p[3])
				buf[x*4] = uint8(min((uint32(p[0])*a+127)/255, 255))
				buf[x*4+1] = uint8(min((uint32(p[1])*a+127)/255, 255))
				buf[x*4+2] = uint8(min((uint32(p[2])*a+127)/255, 255))
				buf[x*4+3] = p[3]
			}
			return buf
		}
	case *image.YCbCr:
		return func(y int) []uint8 {
			ys := s.Y[s.YOffset(r.Min.X, r.Min.Y+y):]
			switch s.SubsampleRatio {
			case image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio422:
				ci := s.COffset(r.Min.X, r.Min.Y+y)
				parity := r.Min.X & 1
				var chroma ycbcrChroma
				last := -1
				for x := range w {
					if c := ci + (x+parity)>>1; c != last {
						chroma, last = newYCbCrChroma(s.Cb[c], s.Cr[c]), c
					}
					p := buf[x*4 : x*4+4 : x*4+4]
					p[0], p[1], p[2] = chroma.rgb(ys[x])
					p[3] = 0xff
				}
			default:
				for x := range w {
					ci := s.COffset(r.Min.X+x, r.Min.Y+y)
					buf[x*4], buf[x*4+1], buf[x*4+2] = color.YCbCrToRGB(ys[x], s.Cb[ci], s.Cr[ci])
					buf[x*4+3] = 0xff
				}
			}
			return buf
		}
	case *image.Gray:
		return func(y int) []uint8 {
			o := s.PixOffset(r.Min.X, r.Min.Y+y)
			for x := range w {
				v := s.Pix[o+x]
				buf[x*4], buf[x*4+1], buf[x*4+2], buf[x*4+3] = v, v, v, 0xff
			}
			return buf
		}
	}
	return func(y int) []uint8 {
		for x := range w {
			cr, cg, cb, ca := src.At(r.Min.X+x, r.Min.Y+y).RGBA()
			buf[x*4], buf[x*4+1], buf[x*4+2], buf[x*4+3] = uint8(cr>>8&0xff), uint8(cg>>8&0xff), uint8(cb>>8&0xff), uint8(ca>>8&0xff)
		}
		return buf
	}
}

// ycbcrChroma is the chroma half of color.YCbCrToRGB, computed once for
// the pixels that share a chroma sample.
type ycbcrChroma struct{ r, g, b int32 }

func newYCbCrChroma(cb, cr uint8) ycbcrChroma {
	cb1, cr1 := int32(cb)-128, int32(cr)-128
	return ycbcrChroma{r: 91881 * cr1, g: -22554*cb1 - 46802*cr1, b: 116130 * cb1}
}

// rgb is color.YCbCrToRGB(y, cb, cr) for c's cb and cr.
func (c ycbcrChroma) rgb(y uint8) (uint8, uint8, uint8) {
	yy1 := int32(y) * 0x10101
	return clampYCbCr(yy1 + c.r), clampYCbCr(yy1 + c.g), clampYCbCr(yy1 + c.b)
}

func clampYCbCr(v int32) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 0xffffff:
		return 0xff
	}
	return uint8(v >> 16)
}

// taps are one output pixel's source indices and their weights.
type taps struct {
	first int
	w     []float32
}

// catmullRom is the cubic kernel at distance t (B 0, C 0.5).
func catmullRom(t float64) float64 {
	t = math.Abs(t)
	switch {
	case t < 1:
		return float64(1.5*t*t*t) - float64(2.5*t*t) + 1
	case t < 2:
		return float64(-0.5*t*t*t) + float64(2.5*t*t) - float64(4*t) + 2
	}
	return 0
}

// kernelTaps are the normalized CatmullRom taps of every one of dst
// output pixels over src source pixels, the kernel widened by the
// downscale so every source pixel contributes; indices past the edges
// clamp onto the edge pixels.
func kernelTaps(src, dst int) []taps {
	scale := float64(src) / float64(dst)
	support := 2 * max(scale, 1)
	stretch := max(scale, 1)
	out := make([]taps, dst)
	width := int(math.Ceil(2*support)) + 1
	backing := make([]float32, 0, dst*width)
	for d := range dst {
		center := float64((float64(d)+0.5)*scale) - 0.5
		first := int(math.Floor(center-support)) + 1
		last := int(math.Floor(center + support))
		start := len(backing)
		sum := 0.0
		for i := first; i <= last; i++ {
			wgt := catmullRom((float64(i) - center) / stretch)
			backing = append(backing, float32(wgt))
			sum += wgt
		}
		ws := backing[start:len(backing):len(backing)]
		for i := range ws {
			ws[i] = float32(float64(ws[i]) / sum)
		}
		out[d] = taps{first: first, w: ws}
	}
	return out
}

// cubicScale fills dst from rows with the CatmullRom kernel, separably:
// each source row is filtered horizontally once, kept in a ring as
// long as the vertical taps reach it.
func cubicScale(dst *image.RGBA, rows rowSource) {
	dw, dh := dst.Rect.Dx(), dst.Rect.Dy()
	xt, yt := kernelTaps(rows.w, dw), kernelTaps(rows.h, dh)
	span := 0
	for _, t := range yt {
		span = max(span, len(t.w))
	}
	ring := make([][]float32, span)
	ringRow := make([]int, span)
	for i := range ring {
		ring[i] = make([]float32, dw*4)
		ringRow[i] = -1
	}
	clampRow := func(y int) int { return min(max(y, 0), rows.h-1) }
	line := make([]float32, rows.w*4)
	filtered := func(y int) []float32 {
		slot := y % span
		if ringRow[slot] == y {
			return ring[slot]
		}
		out := ring[slot]
		for i, v := range rows.row(y) {
			line[i] = float32(v)
		}
		for dx, t := range xt {
			var a0, a1, a2, a3 float32
			if t.first >= 0 && t.first+len(t.w) <= rows.w {
				px := line[t.first*4:]
				for _, wgt := range t.w {
					p := px[:4:4]
					a0 += float32(wgt * p[0])
					a1 += float32(wgt * p[1])
					a2 += float32(wgt * p[2])
					a3 += float32(wgt * p[3])
					px = px[4:]
				}
			} else {
				for i, wgt := range t.w {
					o := min(max(t.first+i, 0), rows.w-1) * 4
					p := line[o : o+4 : o+4]
					a0 += float32(wgt * p[0])
					a1 += float32(wgt * p[1])
					a2 += float32(wgt * p[2])
					a3 += float32(wgt * p[3])
				}
			}
			q := out[dx*4 : dx*4+4 : dx*4+4]
			q[0], q[1], q[2], q[3] = a0, a1, a2, a3
		}
		ringRow[slot] = y
		return out
	}
	sums := make([]float32, dw*4)
	for dy, t := range yt {
		at := dst.PixOffset(dst.Rect.Min.X, dst.Rect.Min.Y+dy)
		row := dst.Pix[at : at+dw*4]
		clear(sums)
		for i, wgt := range t.w {
			line := filtered(clampRow(t.first + i))
			acc := sums[:len(line)]
			for x, v := range line {
				acc[x] += float32(wgt * v)
			}
		}
		for x := 0; x+3 < dw*4; x += 4 {
			v, p := sums[x:x+4:x+4], row[x:x+4:x+4]
			a := byteOf(v[3])
			p[3] = a
			p[0] = min(byteOf(v[0]), a)
			p[1] = min(byteOf(v[1]), a)
			p[2] = min(byteOf(v[2]), a)
		}
	}
}

// byteOf rounds an accumulated channel to the nearest byte, half up,
// clamped.
func byteOf(v float32) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return uint8(float64(v) + 0.5)
}
