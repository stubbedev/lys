package render

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestModulateMatchesItsPerChannelRounding(t *testing.T) {
	for scale := range uint32(256) {
		for ch := range uint32(256) {
			col := Color(ch<<24 | ch<<16 | (255-ch)<<8 | ch/2)
			if got, want := modulate(col, scale), modulateRef(col, scale); got != want {
				t.Fatalf("modulate(%#08x, %d) = %#08x, want %#08x", col, scale, got, want)
			}
		}
	}
}

func TestMixMatchesThePerChannelCrossFade(t *testing.T) {
	for w := range uint64(256) {
		for a := range uint32(256) {
			for b := range uint32(256) {
				ref := (a*(255-uint32(w)) + b*uint32(w) + 127) / 255
				want := Color(ref<<24 | ref<<16 | ref<<8 | ref)
				if got := mix(Color(a*0x01010101), Color(b*0x01010101), w); got != want {
					t.Fatalf("mix(%d, %d, %d) = %#08x, want %#08x", a, b, w, got, want)
				}
			}
		}
	}
}

// The per-pixel passes the row kernels replaced, kept as the reference
// the kernels must match byte for byte.

func refCompositeShift(c *Canvas, l *Layer, src Rect, dx, dy int, alpha float64) {
	src = src.Intersect(l.cv.Rect())
	prev := c.PushAlpha(alpha)
	defer c.PopAlpha(prev)
	if c.alphaScale == 0 {
		return
	}
	dst := Translate(float64(dx), float64(dy)).MapBounds(src).Intersect(c.clip).Intersect(c.Rect())
	for y := dst.Y; y < dst.Y+dst.H; y++ {
		for x := dst.X; x < dst.X+dst.W; x++ {
			sx, sy := x-dx, y-dy
			if sx < src.X || sy < src.Y || sx >= src.X+src.W || sy >= src.Y+src.H {
				continue
			}
			if col := l.cv.get(sx, sy); col != 0 {
				c.blend(x, y, col)
			}
		}
	}
}

func refCrossFade(l, other *Layer, region Rect, t float64) {
	region = region.Intersect(l.cv.Rect()).Intersect(other.cv.Rect())
	t = min(1, max(0, t))
	w := uint32(math.Round(t * 255))
	for y := region.Y; y < region.Y+region.H; y++ {
		for x := region.X; x < region.X+region.W; x++ {
			a, b := l.cv.get(x, y), other.cv.get(x, y)
			if a == b {
				continue
			}
			mix := func(ca, cb uint8) uint32 { return (uint32(ca)*(255-w) + uint32(cb)*w + 127) / 255 }
			l.cv.set(x, y, Color(mix(a.A(), b.A())<<24|mix(a.R(), b.R())<<16|mix(a.G(), b.G())<<8|mix(a.B(), b.B())))
		}
	}
}

func refDrawLayer(c *Canvas, l *Layer, src Rect, mode BlendMode) {
	src = src.Intersect(l.cv.Rect()).Intersect(c.clip)
	for y := src.Y; y < src.Y+src.H; y++ {
		for x := src.X; x < src.X+src.W; x++ {
			col := l.cv.get(x, y)
			if col == 0 {
				continue
			}
			switch c.alphaScale {
			case 0:
				return
			case 255:
			default:
				col = modulate(col, c.alphaScale)
			}
			if mode == BlendNormal {
				c.set(x, y, col.over(c.get(x, y)))
				continue
			}
			c.set(x, y, blendPixel(mode, col, c.get(x, y)))
		}
	}
}

func refCompositeCrossFade(c *Canvas, from, to *Layer, region Rect, t float64) {
	mix := c.Layer(nil, region)
	defer mix.Free()
	refDrawLayer(mix.cv, from, region, BlendNormal)
	refCrossFade(mix, to, region, t)
	refCompositeShift(c, mix, region, 0, 0, 1)
}

// premulNoise is a random premultiplied color drawn from what pages
// hold: mostly transparent, opaque or one shared color, some translucent.
func premulNoise(r *rand.Rand, shared Color) Color {
	switch r.IntN(5) {
	case 0:
		return 0
	case 1:
		return shared
	case 2:
		return RGB(uint8(r.UintN(256)), uint8(r.UintN(256)), uint8(r.UintN(256)))
	}
	return RGBA(uint8(r.UintN(256)), uint8(r.UintN(256)), uint8(r.UintN(256)), uint8(r.UintN(256)))
}

// compositeCase is a canvas with a noisy background and two noisy
// layers over it, under a clip, an opacity and a brightness.
type compositeCase struct {
	name         string
	clip         Rect
	alpha        float64
	bright       float64
	region       Rect
	dx, dy       int
	t            float64
	seed         uint64
	from, to, bg []Color
}

const compositeW, compositeH = 37, 23

func compositeCases() []compositeCase {
	var cases []compositeCase
	r := rand.New(rand.NewPCG(7, 11))
	for i := range 48 {
		cc := compositeCase{
			clip:   Rect{X: r.IntN(8), Y: r.IntN(6), W: 20 + r.IntN(20), H: 12 + r.IntN(12)},
			alpha:  []float64{1, 0, 0.5, 0.31, 1}[i%5],
			bright: []float64{0, 1, 1.4, 0.6}[i%4],
			region: Rect{X: r.IntN(10) - 2, Y: r.IntN(8) - 2, W: 10 + r.IntN(30), H: 6 + r.IntN(20)},
			dx:     r.IntN(21) - 10,
			dy:     r.IntN(13) - 6,
			t:      []float64{0, 1, 0.5, 0.137, -0.2, 1.3, 0.999}[i%7],
			seed:   uint64(i),
		}
		shared := premulNoise(r, RGB(30, 40, 50))
		for _, p := range []*[]Color{&cc.from, &cc.to, &cc.bg} {
			*p = make([]Color, compositeW*compositeH)
			for j := range *p {
				(*p)[j] = premulNoise(r, shared)
			}
		}
		cc.name = fmt.Sprintf("case %d %+v alpha %v bright %v d(%d,%d) t %v", i, cc.region, cc.alpha, cc.bright, cc.dx, cc.dy, cc.t)
		cases = append(cases, cc)
	}
	return cases
}

// canvases builds the case's canvas and layers afresh.
func (cc *compositeCase) canvases() (*Canvas, *Layer, *Layer) {
	c := New(make([]byte, Stride(compositeW)*compositeH), Stride(compositeW), compositeW, compositeH)
	whole := c.Rect()
	from, to := c.Layer(nil, whole), c.Layer(nil, whole)
	for y := range compositeH {
		for x := range compositeW {
			i := y*compositeW + x
			c.set(x, y, cc.bg[i])
			from.cv.set(x, y, cc.from[i])
			to.cv.set(x, y, cc.to[i])
		}
	}
	c.touched = 0
	c.clip = cc.clip.Intersect(whole)
	c.bright = cc.bright
	c.PushAlpha(cc.alpha)
	return c, from, to
}

func assertSamePixels(t *testing.T, name string, got, want *Canvas) {
	t.Helper()
	if !bytes.Equal(got.data, want.data) {
		for y := range got.h {
			for x := range got.w {
				if g, w := got.get(x, y), want.get(x, y); g != w {
					t.Fatalf("%s: (%d,%d) = %#08x, want %#08x", name, x, y, g, w)
				}
			}
		}
	}
	if got.touched != want.touched {
		t.Fatalf("%s: touched %d pixels, want %d", name, got.touched, want.touched)
	}
}

func TestRowPassesMatchThePerPixelPasses(t *testing.T) {
	for _, cc := range compositeCases() {
		t.Run("composite translated", func(t *testing.T) {
			got, gl, _ := cc.canvases()
			want, wl, _ := cc.canvases()
			got.Composite(gl, cc.region, Translate(float64(cc.dx), float64(cc.dy)), 0.8)
			refCompositeShift(want, wl, cc.region, cc.dx, cc.dy, 0.8)
			assertSamePixels(t, cc.name, got, want)
		})
		t.Run("cross-fade", func(t *testing.T) {
			_, got, gto := cc.canvases()
			_, want, wto := cc.canvases()
			got.CrossFade(gto, cc.region, cc.t)
			refCrossFade(want, wto, cc.region, cc.t)
			assertSamePixels(t, cc.name, got.cv, want.cv)
		})
		t.Run("draw layer", func(t *testing.T) {
			for _, mode := range []BlendMode{BlendNormal, BlendMultiply} {
				got, gl, _ := cc.canvases()
				want, wl, _ := cc.canvases()
				got.DrawLayer(gl, cc.region, mode)
				refDrawLayer(want, wl, cc.region, mode)
				assertSamePixels(t, fmt.Sprintf("%s mode %v", cc.name, mode), got, want)
			}
		})
		t.Run("composite cross-fade", func(t *testing.T) {
			got, gfrom, gto := cc.canvases()
			want, wfrom, wto := cc.canvases()
			got.CompositeCrossFade(gfrom, gto, cc.region, cc.t)
			refCompositeCrossFade(want, wfrom, wto, cc.region, cc.t)
			assertSamePixels(t, cc.name, got, want)
		})
	}
}

func TestCompositeCrossFadeRecordsTheScratchLayerPasses(t *testing.T) {
	rec := NewRecorder(compositeW, compositeH, 120, 120)
	whole := rec.Rect()
	from, to := rec.Layer(nil, whole), rec.Layer(nil, whole)
	from.Canvas().FillRect(Rect{W: 20, H: 10}, RGB(200, 10, 10))
	to.Canvas().FillRect(Rect{X: 10, Y: 5, W: 20, H: 10}, RGBA(10, 10, 200, 128))
	rec.CompositeCrossFade(from, to, whole, 0.4)
	l := rec.Recording()
	defer l.End()
	var kinds []opKind
	for _, o := range l.ops {
		kinds = append(kinds, o.kind)
	}
	want := []opKind{opScratch, opDrawLayer, opCrossFade, opComposite, opRelease}
	if len(kinds) < len(want) || !slices.Equal(kinds[len(kinds)-len(want):], want) {
		t.Fatalf("recorded %v, want it to end in the scratch layer passes %v", kinds, want)
	}
	got := New(make([]byte, Stride(compositeW)*compositeH), Stride(compositeW), compositeW, compositeH)
	l.Paint(got)
	ref := New(make([]byte, Stride(compositeW)*compositeH), Stride(compositeW), compositeW, compositeH)
	rf, rt := ref.Layer(nil, whole), ref.Layer(nil, whole)
	rf.Canvas().FillRect(Rect{W: 20, H: 10}, RGB(200, 10, 10))
	rt.Canvas().FillRect(Rect{X: 10, Y: 5, W: 20, H: 10}, RGBA(10, 10, 200, 128))
	ref.CompositeCrossFade(rf, rt, whole, 0.4)
	if !bytes.Equal(got.data, ref.data) {
		t.Error("the recorded cross-fade played back differs from the raster one")
	}
}

func TestCompositeCrossFadeRefusesALayerOfAnotherSize(t *testing.T) {
	c := New(make([]byte, Stride(8)*8), Stride(8), 8, 8)
	small := New(make([]byte, Stride(4)*4), Stride(4), 4, 4)
	defer func() {
		if recover() == nil {
			t.Error("a 4x4 layer cross-faded onto an 8x8 canvas did not panic")
		}
	}()
	c.CompositeCrossFade(LayerOf(small), c.Layer(nil, c.Rect()), c.Rect(), 0.5)
}

// BenchmarkCompositeCrossFade fades two 2880x1800 pages that share
// their background, as a Stack's pages do, over an opaque canvas.
func BenchmarkCompositeCrossFade(b *testing.B) {
	const w, h = 2880, 1800
	c := New(make([]byte, Stride(w)*h), Stride(w), w, h)
	c.ClearDevice(c.Rect(), RGB(250, 250, 250))
	from, to := c.Layer(nil, c.Rect()), c.Layer(nil, c.Rect())
	for _, l := range []*Layer{from, to} {
		l.Canvas().ClearDevice(l.Canvas().Rect(), RGB(240, 240, 240))
	}
	for y := 0; y < h; y += 60 {
		from.Canvas().ClearDevice(Rect{X: 300, Y: y, W: 1800, H: 40}, RGB(255, 255, 255))
		to.Canvas().ClearDevice(Rect{X: 300, Y: y + 20, W: 1800, H: 30}, RGBA(20, 20, 20, 200))
	}
	b.ResetTimer()
	for b.Loop() {
		c.CompositeCrossFade(from, to, c.Rect(), 0.4)
	}
}

func TestTheRowPassesAllocateNothing(t *testing.T) {
	cc := compositeCases()[3]
	c, from, to := cc.canvases()
	got := testing.AllocsPerRun(20, func() {
		c.CompositeCrossFade(from, to, cc.region, cc.t)
		c.Composite(from, cc.region, Translate(3, -2), 0.7)
		c.DrawLayer(to, cc.region, BlendNormal)
		from.CrossFade(to, cc.region, cc.t)
	})
	if got != 0 {
		t.Errorf("the row passes allocate %v times a run, want none", got)
	}
}
