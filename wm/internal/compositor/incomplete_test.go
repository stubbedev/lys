package compositor

import (
	"image"
	"testing"
	"time"

	"github.com/stubbedev/lys-wm/internal/backend"
	"github.com/stubbedev/lys-wm/internal/backend/headless"
	"github.com/stubbedev/lys-wm/internal/loop"
	"github.com/stubbedev/lys-wm/internal/render"
	"github.com/stubbedev/lys-wm/internal/surface"
	"github.com/stubbedev/lys-wm/internal/testenv"
)

type offThreadRenderer struct {
	render.Renderer
	offThread  int
	incomplete uint64
}

func (r *offThreadRenderer) Counts() render.Counts {
	counts := r.Renderer.Counts()
	counts.Incomplete = r.incomplete
	return counts
}

func (r *offThreadRenderer) BuildOffThread() error {
	r.offThread++
	return nil
}

func (r *offThreadRenderer) PipelinesBuilt() (bool, error) { return true, nil }

type offThreadBackend struct {
	*headless.Backend
	r *offThreadRenderer
}

func (b offThreadBackend) Renderer() render.Renderer { return b.r }

func TestAFrameWithAnIncompletePassIsRedrawnWhole(t *testing.T) {
	var b offThreadBackend
	size := image.Pt(320, 240)
	r := setupWith(t, func(l *loop.Loop, _ *spyRenderer) backend.Backend {
		hb := headless.New(l)
		if _, err := hb.AddOutput(backend.Mode{Size: size, RefreshMHz: 60000}); err != nil {
			t.Fatal(err)
		}
		b = offThreadBackend{Backend: hb, r: &offThreadRenderer{Renderer: hb.Renderer()}}
		return b
	}, func(*Config) {})
	o := b.Outputs()[0]
	await := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(testenv.HangLimit)
		for {
			var ok bool
			r.on(func() { ok = cond() })
			if ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(time.Millisecond)
		}
	}
	r.on(func() {
		if b.r.offThread != 1 {
			t.Errorf("the compositor asked its renderer to build off the thread %d times, want once", b.r.offThread)
		}
	})
	await("the first frames to settle", func() bool { return o.Frames() > 0 && !o.FrameOwed() })
	frameAsked := func(incomplete bool) uint64 {
		t.Helper()
		var frames uint64
		r.on(func() {
			frames = o.Frames()
			if incomplete {
				b.r.incomplete++
			}
			if err := o.ScheduleFrame(); err != nil {
				t.Fatal(err)
			}
		})
		await("the frames to settle", func() bool { return !o.FrameOwed() })
		var n uint64
		r.on(func() { n = o.Frames() - frames })
		return n
	}
	unchanged := frameAsked(false)
	if redrawn := frameAsked(true); redrawn != unchanged+1 {
		t.Errorf("a frame with an incomplete pass was followed by %d frames, one with none by %d: want one redraw more", redrawn, unchanged)
	}
	if d := o.LastDamage(); len(d) != 1 || d[0] != (image.Rectangle{Max: size}) {
		t.Errorf("the redraw repainted %v, want the whole output", d)
	}
	if again := frameAsked(false); again != unchanged {
		t.Errorf("once no pass was incomplete a frame was followed by %d frames, want %d", again, unchanged)
	}
}

func TestInvalidatedBlurCachesAreRedrawnWhole(t *testing.T) {
	area, spec := image.Rect(0, 0, 8, 8), chainSpec{offset: 1, passes: 1}
	fresh := func() *blurCache {
		return &blurCache{area: area, spec: spec, chain: []render.TextureTarget{nil}}
	}
	bc := blurCaches{bySurface: map[*surface.Surface]*blurCache{{}: fresh()}, xray: fresh(), mica: fresh()}
	caches := []*blurCache{bc.xray, bc.mica}
	for _, b := range bc.bySurface {
		caches = append(caches, b)
	}
	for _, b := range caches {
		if blurStale(b, spec, area, nil, colorOut{}) {
			t.Fatal("an unchanged blur is stale before invalidate")
		}
	}
	bc.invalidate()
	for _, b := range caches {
		if !blurStale(b, spec, area, nil, colorOut{}) {
			t.Error("an invalidated blur is not stale")
		}
	}
}
