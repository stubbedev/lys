package gpu

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys-lib/fourcc"
	"github.com/stubbedev/lys-lib/render"
	"github.com/stubbedev/lys-lib/third_party/vk"
)

func awaitPrepared(t *testing.T, d *Device) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		ok, err := d.Prepared()
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the pipelines were not built within a minute")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestAfterPrepareNoPassBuildsAPipeline(t *testing.T) {
	d := openDevice(t)
	if err := d.Prepare(fourcc.ARGB8888, fourcc.XRGB8888); err != nil {
		t.Fatal(err)
	}
	awaitPrepared(t, d)
	tg := newTarget(t, d, 32, 32)
	src := newTarget(t, d, 8, 8)
	runPass(t, d, src, []image.Rectangle{{Max: src.Size()}}, func(p *Pass) { p.Clear(color.RGBA{A: 0xff}) })
	tex, err := src.Texture()
	if err != nil {
		t.Fatal(err)
	}
	stops, err := d.NewTexture(fourcc.ARGB8888, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer stops.Close()
	before := d.PipelineBuilds()
	ct := ColorTransform{Matrix: [9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1}, Scale: 1}
	for _, colored := range []bool{false, true} {
		runPass(t, d, tg, []image.Rectangle{{Max: tg.Size()}}, func(p *Pass) {
			if colored {
				p.SetColor(&ct)
			}
			r := image.Rect(0, 0, 16, 16)
			p.FillRect(r, color.RGBA{A: 0xff}, Mask{})
			p.DrawTexture(tex, FRect{W: 8, H: 8}, r, TransformNormal, 1, Mask{})
			p.FillGradient(r, Gradient{Stops: stops, To: [2]float32{16, 0}, Alpha: 1}, Mask{})
			p.DrawGlass(tex, FRect{W: 8, H: 8}, r, 1, Mask{}, Glass{Saturation: 1, Brightness: 1, Contrast: 1})
			if !colored {
				p.Kawase(tex, r, 1, false)
				p.FillGoo(r, color.RGBA{A: 0xff}, Goo{A: RoundRect{Rect: r}, B: RoundRect{Rect: r}})
			}
			if p.Incomplete() {
				t.Error("a prepared pass skipped a draw")
			}
		})
	}
	if n := d.PipelineBuilds() - before; n != 0 {
		t.Errorf("%d pipelines built while passes recorded after Prepare", n)
	}
}

func TestPipelineBuildsCountsTheFirstDrawOfAPipelineOnly(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 8, 8)
	fill := func() {
		runPass(t, d, tg, []image.Rectangle{{Max: tg.Size()}}, func(p *Pass) {
			p.FillRect(image.Rect(0, 0, 8, 8), color.RGBA{R: 0xff, A: 0xff}, Mask{})
		})
	}
	before := d.PipelineBuilds()
	fill()
	first := d.PipelineBuilds()
	if first == before {
		t.Fatal("the first fill into a new target built no pipeline")
	}
	fill()
	if n := d.PipelineBuilds() - first; n != 0 {
		t.Errorf("a second identical fill built %d pipelines", n)
	}
	if err := d.Prepare(fourcc.ABGR8888); err != nil {
		t.Fatal(err)
	}
	awaitPrepared(t, d)
	if d.PipelineBuilds() == first {
		t.Error("the pipelines Prepare built on its worker were not counted")
	}
}

func TestBeforeBuildRunsBeforeEachPipelineADrawBuilds(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 8, 8)
	var hooked, builtFirst uint64
	d.BeforeBuild(func() {
		if d.PipelineBuilds() != builtFirst+hooked {
			t.Error("a pipeline was built before its hook ran")
		}
		hooked++
	})
	builtFirst = d.PipelineBuilds()
	fill := func() {
		runPass(t, d, tg, []image.Rectangle{{Max: tg.Size()}}, func(p *Pass) {
			p.FillRect(image.Rect(0, 0, 8, 8), color.RGBA{R: 0xff, A: 0xff}, Mask{})
		})
	}
	fill()
	if built := d.PipelineBuilds() - builtFirst; hooked == 0 || hooked != built {
		t.Fatalf("the hook ran %d times for %d pipelines built", hooked, built)
	}
	fill()
	if built := d.PipelineBuilds() - builtFirst; hooked != built {
		t.Errorf("the hook ran %d times for %d pipelines built", hooked, built)
	}
	if err := d.Prepare(fourcc.ABGR8888); err != nil {
		t.Fatal(err)
	}
	ran := hooked
	awaitPrepared(t, d)
	if hooked != ran {
		t.Errorf("the hook ran %d times for pipelines built on the worker", hooked-ran)
	}
}

func TestPrepareForBuildsWhatAnOffscreenTargetsDrawsTakeOnce(t *testing.T) {
	d := openDevice(t)
	tg, err := d.NewOffscreenFloat(16, 16)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tg.Close)
	src := newTarget(t, d, 8, 8)
	if err := d.PrepareFor(tg, src); err != nil {
		t.Fatal(err)
	}
	awaitPrepared(t, d)
	prepared := d.PipelineBuilds()
	if err := d.PrepareFor(tg); err != nil {
		t.Fatal(err)
	}
	if ok, err := d.Prepared(); err != nil || !ok {
		t.Fatalf("a second PrepareFor of a prepared format left work: %v, %v", ok, err)
	}
	runPass(t, d, src, []image.Rectangle{{Max: src.Size()}}, func(p *Pass) { p.Clear(color.RGBA{A: 0xff}) })
	tex, err := src.Texture()
	if err != nil {
		t.Fatal(err)
	}
	runPass(t, d, tg, []image.Rectangle{{Max: tg.Size()}}, func(p *Pass) {
		r := image.Rect(0, 0, 8, 8)
		p.FillRect(r, color.RGBA{R: 0xff, A: 0xff}, Mask{})
		p.DrawTexture(tex, FRect{W: 8, H: 8}, r, TransformNormal, 1, Mask{})
		p.Kawase(tex, r, 1, false)
		if p.Incomplete() {
			t.Error("a pass into a target PrepareFor prepared skipped a draw")
		}
	})
	if n := d.PipelineBuilds() - prepared; n != 0 {
		t.Errorf("%d pipelines built after PrepareFor finished: by a second PrepareFor of the format or by draws into it", n)
	}
	closed := newTarget(t, d, 4, 4)
	closed.Close()
	if err := d.PrepareFor(closed); err == nil {
		t.Error("PrepareFor of a closed target succeeded")
	}
}

func TestAPipelineTheWorkerCannotBuildFailsEveryDrawThatNeedsIt(t *testing.T) {
	d := openDevice(t)
	if err := d.Prepare(); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("refused")
	key := pipelineKey{owner: t, which: 1}
	builds := 0
	build := func() (vk.Pipeline, error) {
		builds++
		return 0, refused
	}
	publish := func(vk.Pipeline) { t.Error("a refused pipeline was published") }
	if p, err := d.lazyPipeline(key, 0, build, publish); p != 0 || err != nil {
		t.Fatalf("the first ask: %v, %v, want it queued", p, err)
	}
	deadline := time.Now().Add(time.Minute)
	var err error
	for err == nil {
		if time.Now().After(deadline) {
			t.Fatal("the worker never reported the refusal")
		}
		_, err = d.Prepared()
		time.Sleep(time.Millisecond)
	}
	if !errors.Is(err, refused) {
		t.Fatalf("Prepared reported %v, want the refusal", err)
	}
	for range 2 {
		if _, err := d.lazyPipeline(key, 0, build, publish); !errors.Is(err, refused) {
			t.Fatalf("a later ask: %v, want the refusal", err)
		}
	}
	awaitPrepared(t, d)
	if builds != 1 {
		t.Errorf("the refused pipeline was built %d times, want once", builds)
	}
}

func TestAPassBeforeItsPipelineExistsSkipsTheDrawAndSaysSo(t *testing.T) {
	d := openDevice(t)
	if err := d.Prepare(); err != nil {
		t.Fatal(err)
	}
	tg := newTarget(t, d, 8, 8)
	red := color.RGBA{R: 0xff, A: 0xff}
	draw := func() bool {
		p, err := d.Begin(tg, []image.Rectangle{{Max: tg.Size()}})
		if err != nil {
			t.Fatal(err)
		}
		p.Clear(color.RGBA{A: 0xff})
		p.FillRect(image.Rect(0, 0, 8, 8), red, Mask{})
		incomplete := p.Incomplete()
		if err := p.End(); err != nil {
			t.Fatal(err)
		}
		return incomplete
	}
	builds := d.PipelineBuilds()
	if !draw() && d.PipelineBuilds() == builds {
		t.Log("the worker beat the first draw")
	}
	awaitPrepared(t, d)
	if draw() {
		t.Fatal("a pass after the worker finished skipped a draw")
	}
	px, err := tg.ReadPixels()
	if err != nil {
		t.Fatal(err)
	}
	if got := px.RGBAAt(4, 4); got != red {
		t.Errorf("drew %v, want %v", got, red)
	}
}

func TestThePipelineCachePersistsAcrossDevices(t *testing.T) {
	d := openDevice(t)
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	path, ok := pipelineCachePath(d.cacheUUID)
	if !ok {
		t.Fatal("no cache path with XDG_CACHE_HOME set")
	}
	d.prep.path = path
	if err := d.Prepare(fourcc.ARGB8888); err != nil {
		t.Fatal(err)
	}
	awaitPrepared(t, d)
	if err := d.savePipelineCache(); err != nil {
		t.Fatal(err)
	}
	first, err := os.Stat(path)
	if err != nil || first.Size() == 0 {
		t.Fatalf("the cache file: %v, %v", first, err)
	}
	if rel, err := filepath.Rel(dir, path); err != nil || filepath.Dir(rel) != "lys-lib" {
		t.Errorf("the cache lands at %s", path)
	}
	leftovers, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 1 {
		t.Errorf("%d files beside the cache: a temporary was left behind", len(leftovers)-1)
	}
}

func TestTargetsPreparedOnAWorkerReachFramesWithoutStoppingThem(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 32, 32)
	rec := render.NewRecorder(32, 32, 1, 1)
	t.Cleanup(rec.Recording().End)
	rec.Clear(rec.Rect(), render.RGB(1, 2, 3))
	if err := d.Render(rec.Recording(), tg); err != nil {
		t.Fatal(err)
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	const count = 16
	made := make(chan *Target, count)
	errs := make(chan error, 1)
	go func() {
		for range count {
			p, err := d.PrepareOffscreen(32, 32)
			if err != nil {
				errs <- err
				return
			}
			made <- p
		}
		close(made)
	}()
	waits := d.BlockingWaits()
	var got []*Target
	deadline := time.Now().Add(time.Minute)
	for open := true; open; {
		select {
		case err := <-errs:
			t.Fatal(err)
		case p, more := <-made:
			if !more {
				open = false
				break
			}
			got = append(got, p)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the worker never finished")
		}
		err := d.Render(rec.Recording(), tg)
		if err != nil && !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
	}
	if n := d.BlockingWaits() - waits; n != 0 {
		t.Errorf("%d blocking waits while the worker allocated", n)
	}
	if len(got) != count {
		t.Fatalf("%d targets of %d", len(got), count)
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.s.view == 0 || p.s.fb == 0 {
			t.Fatal("a prepared target left its view and framebuffer for its first draw")
		}
		if err := d.Render(rec.Recording(), p); err != nil {
			t.Fatal(err)
		}
		px, err := p.ReadPixels()
		if err != nil {
			t.Fatal(err)
		}
		if c := px.RGBAAt(1, 1); c != (color.RGBA{1, 2, 3, 255}) {
			t.Errorf("a prepared target drew %v", c)
		}
		if _, ok := d.targets[p]; !ok {
			t.Error("a prepared target is not registered after a frame")
		}
		p.Close()
	}
	last, err := d.PrepareOffscreen(8, 8)
	if err != nil {
		t.Fatal(err)
	}
	last.Close()
	if _, ok := d.targets[last]; ok {
		t.Error("a closed prepared target stayed registered")
	}
}

func TestCloseWaitsForThePreparesRunningAndRefusesLaterOnes(t *testing.T) {
	d := openDevice(t)
	const workers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, workers*64)
	for range workers {
		wg.Go(func() {
			<-start
			for range 64 {
				_, err := d.PrepareOffscreen(64, 64)
				results <- err
				if err != nil {
					return
				}
			}
		})
	}
	close(start)
	for len(results) < workers {
		runtime.Gosched()
	}
	d.Close()
	if n := len(d.targets); n != 0 {
		t.Errorf("%d targets outlived Close", n)
	}
	wg.Wait()
	close(results)
	refused := 0
	for err := range results {
		switch {
		case errors.Is(err, ErrDeviceClosed):
			refused++
		case err != nil:
			t.Errorf("a prepare during Close: %v", err)
		}
	}
	if refused == 0 {
		t.Log("every prepare finished before Close")
	}
	if _, err := d.PrepareOffscreen(8, 8); !errors.Is(err, ErrDeviceClosed) {
		t.Errorf("a prepare after Close: %v, want ErrDeviceClosed", err)
	}
	if d.prepared.Load() != nil {
		t.Error("prepared targets were left on the stack")
	}
}
