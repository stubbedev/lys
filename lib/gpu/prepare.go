package gpu

import (
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/stubbedev/lys-lib/atomicfile"
	"github.com/stubbedev/lys-lib/debug"
	"github.com/stubbedev/lys-lib/fourcc"
	"github.com/stubbedev/lys-lib/third_party/vk"
	"github.com/stubbedev/lys-lib/xdg"
)

// pipelineKey names a pipeline being built: which of owner's, for which
// target format.
type pipelineKey struct {
	owner  any
	target vk.Format
	which  int
}

type pipelineJob struct {
	key     pipelineKey
	build   func() (vk.Pipeline, error)
	publish func(vk.Pipeline)
}

type builtPipeline struct {
	job      pipelineJob
	pipeline vk.Pipeline
	err      error
}

// preparer builds pipelines on a worker once Prepare has been called, so a
// frame never compiles one: the device's goroutine publishes what is done
// at the next Begin, and until then Pass.Incomplete reports what it skipped.
type preparer struct {
	cache  vk.PipelineCache
	path   string
	builds atomic.Uint64
	// async is whether Prepare was called; queued is what is on its way,
	// failed what the worker could not build, and formats the target
	// formats prepared, all the device goroutine's.
	async   bool
	queued  map[pipelineKey]struct{}
	failed  map[pipelineKey]error
	formats map[vk.Format]struct{}
	// beforeBuild is what BeforeBuild set.
	beforeBuild func()

	mu      sync.Mutex
	jobs    []pipelineJob
	built   []builtPipeline
	running bool
	stop    bool
	idle    sync.Cond
}

// lazyPipeline is the pipeline have, or else one build makes: at once on a
// device that never called Prepare, else on the worker, with 0 for now.
func (d *Device) lazyPipeline(key pipelineKey, have vk.Pipeline, build func() (vk.Pipeline, error), publish func(vk.Pipeline)) (vk.Pipeline, error) {
	if have != 0 {
		return have, nil
	}
	pr := &d.prep
	if !pr.async {
		if pr.beforeBuild != nil {
			pr.beforeBuild()
		}
		p, err := build()
		if err != nil {
			return 0, err
		}
		publish(p)
		return p, nil
	}
	if err, ok := pr.failed[key]; ok {
		return 0, err
	}
	if _, ok := pr.queued[key]; ok {
		return 0, nil
	}
	if pr.queued == nil {
		pr.queued = map[pipelineKey]struct{}{}
	}
	pr.queued[key] = struct{}{}
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.jobs = append(pr.jobs, pipelineJob{key: key, build: build, publish: publish})
	if !pr.running {
		pr.running = true
		go d.buildPipelines()
	}
	return 0, nil
}

func (d *Device) buildPipelines() {
	pr := &d.prep
	for {
		pr.mu.Lock()
		if len(pr.jobs) == 0 || pr.stop {
			pr.mu.Unlock()
			if err := d.savePipelineCache(); err != nil {
				debug.Log("gpu", "pipeline cache not saved: %v", err)
			}
			pr.mu.Lock()
			if len(pr.jobs) > 0 && !pr.stop {
				pr.mu.Unlock()
				continue
			}
			pr.running = false
			if pr.idle.L != nil {
				pr.idle.Broadcast()
			}
			pr.mu.Unlock()
			return
		}
		job := pr.jobs[0]
		pr.jobs = pr.jobs[1:]
		pr.mu.Unlock()
		p, err := job.build()
		if err != nil {
			err = fmt.Errorf("gpu: a pipeline built ahead of its first draw: %w", err)
		}
		pr.mu.Lock()
		pr.built = append(pr.built, builtPipeline{job: job, pipeline: p, err: err})
		pr.mu.Unlock()
	}
}

// publishPipelines hands the pipelines the worker finished to their owners,
// and keeps what it failed to build failing: a draw that needs one fails
// with the driver's refusal instead of being skipped for good.
func (d *Device) publishPipelines() error {
	pr := &d.prep
	if !pr.async {
		return nil
	}
	pr.mu.Lock()
	built := pr.built
	pr.built = nil
	pr.mu.Unlock()
	var errs []error
	for _, b := range built {
		delete(pr.queued, b.job.key)
		if b.err != nil {
			if pr.failed == nil {
				pr.failed = map[pipelineKey]error{}
			}
			pr.failed[b.job.key] = b.err
			errs = append(errs, b.err)
			continue
		}
		b.job.publish(b.pipeline)
	}
	return errors.Join(errs...)
}

// Prepare builds, on a worker, the pipelines drawing into targets of formats
// takes, including the ones a device builds only when first drawn, and from
// then on any pipeline a frame finds missing goes to the worker too: nothing
// compiles while a pass records. A pass that meets a pipeline not built yet
// skips that draw and reports Incomplete. Prepared reports when the worker has
// nothing left. Bad formats fail at once; a driver's refusal comes from a later
// Begin.
func (d *Device) Prepare(formats ...fourcc.Format) error {
	vfs := make([]vk.Format, 0, len(formats))
	for _, f := range formats {
		pf, ok := lookupPixelFormat(f)
		if !ok {
			return fmt.Errorf("gpu: Prepare of DRM format %s, no target format", f)
		}
		vfs = append(vfs, pf.vk)
	}
	return d.prepare(vfs)
}

// PrepareFor is Prepare for the formats of targets, offscreen ones too; a
// format already prepared costs nothing.
func (d *Device) PrepareFor(targets ...*Target) error {
	vfs := make([]vk.Format, 0, len(targets))
	for _, t := range targets {
		switch {
		case t == nil || t.closed:
			return errors.New("gpu: PrepareFor of a closed target")
		case t.s.d != d:
			return errors.New("gpu: PrepareFor of another device's target")
		}
		vfs = append(vfs, t.s.format())
	}
	return d.prepare(vfs)
}

func (d *Device) prepare(formats []vk.Format) error {
	cs, err := d.colorInit()
	if err != nil {
		return err
	}
	pr := &d.prep
	pr.async = true
	for _, vf := range formats {
		if _, ok := pr.formats[vf]; ok {
			continue
		}
		ct, err := d.compTargetOf(vf)
		if err != nil {
			return err
		}
		if _, err := d.glassPipeline(ct); err != nil {
			return err
		}
		for _, kind := range []opaqueKind{opaqueTexture, opaqueFill} {
			if _, _, err := d.opaquePipeline(ct, kind); err != nil {
				return err
			}
		}
		if _, err := d.colorGradPipeline(ct); err != nil {
			return err
		}
		if _, err := d.colorGlassPipeline(ct); err != nil {
			return err
		}
		if _, _, err := d.colorPlainPipeline(cs, ct); err != nil {
			return err
		}
		if err := d.prepareCuts(ct); err != nil {
			return err
		}
		for _, kind := range d.yuvKinds() {
			if err := prepareYUV(d, kind, ct, vf); err != nil {
				return err
			}
		}
		if pr.formats == nil {
			pr.formats = map[vk.Format]struct{}{}
		}
		pr.formats[vf] = struct{}{}
	}
	return nil
}

// BeforeBuild has f run before each pipeline a draw builds at its first use on
// a device that never called Prepare, on the goroutine drawing: f may lower
// that thread's priority for the driver's compile.
func (d *Device) BeforeBuild(f func()) { d.prep.beforeBuild = f }

// PipelineBuilds counts the pipelines this device has built, on its own
// goroutine or on the worker Prepare starts.
func (d *Device) PipelineBuilds() uint64 { return d.prep.builds.Load() }

// Prepared reports whether the worker has built everything asked of it; the
// pipelines reach passes from the next Begin.
func (d *Device) Prepared() (bool, error) {
	if err := d.publishPipelines(); err != nil {
		return false, err
	}
	pr := &d.prep
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return len(pr.jobs) == 0 && !pr.running && len(pr.built) == 0, nil
}

func (d *Device) stopPipelines() {
	pr := &d.prep
	pr.mu.Lock()
	pr.stop = true
	if pr.idle.L == nil {
		pr.idle.L = &pr.mu
	}
	for pr.running {
		pr.idle.Wait()
	}
	pr.mu.Unlock()
	if err := d.publishPipelines(); err != nil {
		debug.Log("gpu", "closing: %v", err)
	}
}

func pipelineCachePath(uuid [16]uint8) (string, bool) {
	dir, ok := xdg.OS.CacheDir("lys-lib")
	if !ok {
		return "", false
	}
	return filepath.Join(dir, "vulkan-pipelines-"+hex.EncodeToString(uuid[:])+".bin"), true
}

// openPipelineCache starts the cache from what an earlier run saved for this
// driver and device; its UUID is in the file's name, and the driver checks the
// data's own header.
func (d *Device) openPipelineCache(uuid [16]uint8) error {
	pr := &d.prep
	var data []byte
	if path, ok := pipelineCachePath(uuid); ok {
		pr.path = path
		var err error
		if data, err = os.ReadFile(path); err != nil && !errors.Is(err, os.ErrNotExist) { //nolint:gosec // the path is the user's own cache directory
			debug.Log("gpu", "pipeline cache %s not read: %v", path, err)
			data = nil
		}
	}
	ci := vk.PipelineCacheCreateInfo{SType: vk.StructureTypePipelineCacheCreateInfo}
	if len(data) > 0 {
		ci.InitialDataSize = uintptr(len(data))
		ci.PInitialData = (*uintptr)(unsafe.Pointer(unsafe.SliceData(data)))
	}
	if r := d.cmds.CreatePipelineCache(d.device, &ci, nil, &pr.cache); r != vk.Success {
		return fmt.Errorf("gpu: vkCreatePipelineCache from %d bytes: %w", len(data), resultError(r))
	}
	return nil
}

func (d *Device) savePipelineCache() error {
	pr := &d.prep
	if pr.cache == 0 || pr.path == "" {
		return nil
	}
	var n uintptr
	if r := d.cmds.GetPipelineCacheData(d.device, pr.cache, &n, nil); r != vk.Success {
		return fmt.Errorf("gpu: vkGetPipelineCacheData: %w", resultError(r))
	}
	if n == 0 {
		return nil
	}
	data := make([]byte, n)
	if r := d.cmds.GetPipelineCacheData(d.device, pr.cache, &n, (*uintptr)(unsafe.Pointer(unsafe.SliceData(data)))); r != vk.Success && r != vk.Incomplete {
		return fmt.Errorf("gpu: vkGetPipelineCacheData of %d bytes: %w", n, resultError(r))
	}
	if err := os.MkdirAll(filepath.Dir(pr.path), 0o700); err != nil {
		return fmt.Errorf("gpu: the pipeline cache's directory: %w", err)
	}
	if err := atomicfile.WriteFile(pr.path, data[:n], 0o600); err != nil {
		return fmt.Errorf("gpu: the pipeline cache %s: %w", pr.path, err)
	}
	return nil
}

// yuvKind is one way of sampling YUV textures, whose pipelines depend on the
// target format.
type yuvKind interface {
	pipeline(d *Device, target vk.Format) (vk.Pipeline, error)
	coloredPipeline(d *Device, ct *compTarget, target vk.Format) (vk.Pipeline, vk.PipelineLayout, error)
}

func (d *Device) yuvKinds() []yuvKind {
	kinds := make([]yuvKind, 0, len(d.comp.ycbcr)+len(d.comp.yuv444)+len(d.comp.planes))
	for _, k := range d.comp.ycbcr {
		kinds = append(kinds, k)
	}
	for _, k := range d.comp.yuv444 {
		kinds = append(kinds, k)
	}
	for _, k := range d.comp.planes {
		kinds = append(kinds, k)
	}
	return kinds
}

func prepareYUV(d *Device, kind yuvKind, ct *compTarget, target vk.Format) error {
	if _, err := kind.pipeline(d, target); err != nil {
		return err
	}
	_, _, err := kind.coloredPipeline(d, ct, target)
	return err
}

// prepareKind queues a new YUV kind's pipelines for every prepared format,
// so its first draw into any of them finds them built.
func (d *Device) prepareKind(kind yuvKind) error {
	pr := &d.prep
	if !pr.async {
		return nil
	}
	formats := slices.Sorted(maps.Keys(pr.formats))
	for _, vf := range formats {
		ct, err := d.compTargetOf(vf)
		if err != nil {
			return err
		}
		if err := prepareYUV(d, kind, ct, vf); err != nil {
			return err
		}
	}
	return nil
}

// prepareCuts queues the alpha cut variants into ct's target.
func (d *Device) prepareCuts(ct *compTarget) error {
	cs, err := d.cutInit()
	if err != nil {
		return err
	}
	colorLayout, err := d.cutColorLayout(cs)
	if err != nil {
		return err
	}
	for frag := range cutFrags {
		layout := cs.layout
		if frag == cutFragTextureColor || frag == cutFragGlassColor {
			layout = colorLayout
		}
		if _, err := d.cutPipeline(ct, frag, layout); err != nil {
			return err
		}
	}
	return nil
}
