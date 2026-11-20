package bar

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/cavaview"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	analyzer "github.com/stubbedev/lys/shell/cava"
)

// cavaRate is the fixed capture rate the plan is built for.
const cavaRate = 44100

// cavaModule is one bar's spectrum, fed by the analyzer its config shares.
type cavaModule struct {
	paint    *cavaview.Spectrum
	analyzer *cavaAnalyzer
}

// cavaAnalyzer is the plan and its capture, run on the frame timer's goroutine
// at the configured framerate while there is sound or the bars are still
// falling, and parked otherwise: one capture and one FFT however many bars show
// it, frames reaching each bar's loop latest-only. samples and frame are the
// tick's buffers, reused frame to frame.
type cavaAnalyzer struct {
	plan   *analyzer.Plan
	source analyzer.Input
	shared cavaview.Frame
	hub    *pollHub

	frameEvery time.Duration
	every      func(time.Duration, func()) func()

	mu      sync.Mutex
	samples []float64
	frame   []float64
	// stopTick is the running frame timer, nil while parked.
	stopTick func()
	retired  bool
	bars     map[*cavaBarSlot]struct{}
	released func()
}

// cavaBarSlot hands frames to one bar's loop: the newest frame, with one call
// waiting at a time.
type cavaBarSlot struct {
	paint *cavaview.Spectrum
	post  func(func())
	apply func()

	mu      sync.Mutex
	values  []float64
	spare   []float64
	waiting bool
}

func newCavaBarSlot(paint *cavaview.Spectrum, post func(func())) *cavaBarSlot {
	s := &cavaBarSlot{paint: paint, post: post, values: make([]float64, paint.Bars()), spare: make([]float64, paint.Bars())}
	s.apply = func() {
		s.mu.Lock()
		s.values, s.spare = s.spare, s.values
		s.waiting = false
		frame := s.values
		s.paint.SetFrame(frame)
		s.mu.Unlock()
	}
	return s
}

func (s *cavaBarSlot) give(frame []float64) {
	s.mu.Lock()
	copy(s.spare, frame)
	queue := !s.waiting
	s.waiting = true
	s.mu.Unlock()
	if queue {
		s.post(s.apply)
	}
}

func goEvery(every time.Duration, fn func()) func() {
	done := make(chan struct{})
	stop := sync.OnceFunc(func() { close(done) })
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fn()
			}
		}
	}()
	return stop
}

func (a *cavaAnalyzer) join(life context.Context, spawn func(func()), slot *cavaBarSlot) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bars[slot] = struct{}{}
	spawn(func() {
		<-life.Done()
		a.leave(slot)
	})
	a.resumeLocked()
}

func (a *cavaAnalyzer) leave(slot *cavaBarSlot) {
	a.mu.Lock()
	delete(a.bars, slot)
	last := len(a.bars) == 0
	if last {
		a.retired = true
		if a.stopTick != nil {
			a.stopTick()
			a.stopTick = nil
		}
	}
	a.mu.Unlock()
	if last {
		if a.released != nil {
			a.released()
		}
		a.source.Stop()
	}
}

// resume starts the frame timer; sound reaching a parked analyzer lands here,
// and once its last bar left it stays stopped.
func (a *cavaAnalyzer) resume() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resumeLocked()
}

func (a *cavaAnalyzer) resumeLocked() {
	if a.retired || a.stopTick != nil {
		return
	}
	a.stopTick = a.every(a.frameEvery, a.tick)
}

// tick analyzes what arrived and hands on what moved: with no new samples the
// bars still fall off. Once silence leaves nothing moving the analyzer parks, so
// a quiet session costs no frames. The capture keeps itself attached across
// default changes and server kills.
func (a *cavaAnalyzer) tick() {
	a.mu.Lock()
	if a.stopTick == nil {
		a.mu.Unlock()
		return
	}
	a.samples = a.source.Drained(a.samples)
	if a.hub != nil && a.hub.locked.Load() {
		a.mu.Unlock()
		return
	}
	a.frame = a.plan.Execute(a.frame, a.samples)
	moved := a.shared.Advance(a.frame)
	park := !moved && analyzer.Silent(a.samples)
	if moved {
		for slot := range a.bars {
			slot.give(a.frame)
		}
	}
	if park {
		a.stopTick()
		a.stopTick = nil
	}
	a.mu.Unlock()
	if park {
		a.source.NotifySound(a.resume)
	}
}

// cavaInputSupported reports the inputs the shell captures: PipeWire
// and PulseAudio (both through the PulseAudio protocol, pipewire-pulse
// on a PipeWire system), a FIFO and squeezelite's shared memory. With
// any other input the module fails to load with the reason.
func cavaInputSupported(input config.CavaInput) bool {
	switch input {
	case config.CavaInputPipeWire, config.CavaInputPulse, config.CavaInputFifo, config.CavaInputShmem:
		return true
	}
	return false
}

// cavaInput builds the configured capture.
func cavaInput(ctx ModuleContext, cfg config.CavaConfig, channels, capacity int) (analyzer.Input, error) {
	switch cfg.Input {
	case config.CavaInputFifo:
		return analyzer.NewFifoInput(cfg.Source, channels, capacity)
	case config.CavaInputShmem:
		return analyzer.NewShmemInput(cfg.Source, channels, capacity)
	}
	if ctx.Pulse == nil {
		return nil, errCavaNoAudio
	}
	return analyzer.NewSource(ctx.Pulse, cfg.Source, channels, capacity)
}

func newCava(ctx ModuleContext) (Module, error) {
	cfg := ctx.Config.Cava
	if !cavaInputSupported(cfg.Input) {
		return nil, fmt.Errorf("cava: the %q input is not built in (pipe-wire, pulse, fifo and shmem are)", cfg.Input)
	}
	// monstercat and waves only affect libcava's output stage, which the
	// analyzer does not run, so they change nothing.
	bars, channels := cavaview.Bars(cfg)
	plan, err := analyzer.NewPlan(bars/channels, cavaRate, channels, float64(cfg.NoiseReduction), true, int(cfg.LowCutoff), int(cfg.HighCutoff))
	if err != nil {
		return nil, err
	}
	vertical := ctx.Config.Bar.Location.IsVertical()
	paint := cavaview.NewSpectrum(cfg, bars, resolveModuleColor(ctx, cfg.Color), float64(ctx.Config.Bar.Scale), vertical)
	module := &cavaModule{paint: paint}
	if ctx.App == nil {
		module.analyzer = &cavaAnalyzer{plan: plan}
		return module, nil
	}
	an, err := sharedCavaAnalyzer(ctx, cfg, plan, bars, channels)
	if err != nil {
		return nil, err
	}
	module.analyzer = an
	an.join(ctx.Life(), ctx.Go, newCavaBarSlot(paint, ctx.Invoke))
	return module, nil
}

// sharedCavaAnalyzer is the snapshot's one analyzer, its capture starting on
// the first bar that asks.
func sharedCavaAnalyzer(ctx ModuleContext, cfg config.CavaConfig, plan *analyzer.Plan, bars, channels int) (*cavaAnalyzer, error) {
	key := configKey("cava", ctx.Config)
	build := func() (*cavaAnalyzer, error) {
		source, err := cavaInput(ctx, cfg, channels, plan.InputSize())
		if err != nil {
			return nil, err
		}
		if err := source.Start(); err != nil {
			return nil, err
		}
		return newCavaAnalyzer(plan, source, bars, cfg.Style, ctx.Polls, time.Second/time.Duration(cfg.Framerate), goEvery), nil
	}
	if ctx.Polls == nil {
		return build()
	}
	return ctx.Polls.cavaAnalyzer(key, build)
}

func newCavaAnalyzer(plan *analyzer.Plan, source analyzer.Input, bars int, style config.CavaStyle, hub *pollHub, frame time.Duration, every func(time.Duration, func()) func()) *cavaAnalyzer {
	return &cavaAnalyzer{plan: plan, source: source, shared: cavaview.NewFrame(bars, style), hub: hub, frameEvery: frame, every: every, bars: map[*cavaBarSlot]struct{}{}}
}

var errCavaNoAudio = errors.New("cava: no audio server connection")

func (m *cavaModule) Root() widget.Widget { return m.paint }

// resolveModuleColor resolves a module color value against the palette.
func resolveModuleColor(ctx ModuleContext, cv config.ColorValue) render.Color {
	color, ok := styling.ResolveColor(cv, ctx.Style.palette)
	if !ok {
		return render.Color(0)
	}
	return color
}
