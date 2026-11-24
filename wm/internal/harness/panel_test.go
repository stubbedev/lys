package harness

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/udmabuf"
	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/wm/internal/anim"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/ipc"
	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/render"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/testenv"
)

type panelStarter func(t *testing.T, panel render.Transform, configure func(*compositor.Config)) *Harness

func mounted(open func(*loop.Loop) (*headless.Backend, error), panel render.Transform, inherit bool) func(*loop.Loop) (*headless.Backend, error) {
	return func(l *loop.Loop) (*headless.Backend, error) {
		b, err := open(l)
		if err != nil {
			return nil, err
		}
		b.MountPanels(panel)
		if inherit {
			b.InheritFrames()
		}
		return b, nil
	}
}

func startMounted(t *testing.T, panel render.Transform, configure func(*compositor.Config)) *Harness {
	t.Helper()
	return start(t, setup{open: mounted(softBackend, panel, false), configure: configure, traced: true}, small)
}

func startMountedInherited(t *testing.T, panel render.Transform, configure func(*compositor.Config)) *Harness {
	t.Helper()
	return start(t, setup{open: mounted(softBackend, panel, true), configure: configure, traced: true}, small)
}

func startMountedInheritedVulkan(t *testing.T, panel render.Transform, configure func(*compositor.Config)) *Harness {
	t.Helper()
	vulkanTurn.hold(t)
	testenv.Vulkan(t)
	return start(t, setup{open: mounted(vulkanBackend, panel, true), configure: configure, traced: true}, small)
}

func shownOutput(t *testing.T, h *Harness) ipc.Output {
	t.Helper()
	outputs := query[[]ipc.Output](h, wmipc.KindOutputs)
	if len(outputs) != 1 {
		t.Fatalf("outputs %+v, want one", outputs)
	}
	return outputs[0]
}

func TestAnOutputTakesItsPanelsTransform(t *testing.T) {
	t.Parallel()
	h := startMounted(t, render.Rotate270, func(*compositor.Config) {})
	o := shownOutput(t, h)
	if o.Transform != "270" || o.Rect != (wmipc.Rect{Width: small.Size.Y, Height: small.Size.X}) {
		t.Fatalf("an output on a panel mounted right side up shows %+v, want transform 270 at %vx%v", o, small.Size.Y, small.Size.X)
	}
}

func TestAnOutputRuleTransformOverridesThePanels(t *testing.T) {
	t.Parallel()
	for rule, shown := range map[render.Transform]string{render.Normal: "", render.Rotate90: "90"} {
		t.Run(rule.String(), func(t *testing.T) {
			t.Parallel()
			h := startMounted(t, render.Rotate270, transformed(rule))
			want := wmipc.Rect{Width: rule.Size(small.Size).X, Height: rule.Size(small.Size).Y}
			if o := shownOutput(t, h); o.Transform != shown || o.Rect != want {
				t.Fatalf("a rule's %v on a panel mounted right side up shows %+v, want %v", rule, o, want)
			}
		})
	}
}

func TestAnOutputWithoutAPanelTransformStaysUpright(t *testing.T) {
	t.Parallel()
	h := StartWith(t, func(*compositor.Config) {}, small)
	if o := shownOutput(t, h); o.Transform != "" || o.Rect != (wmipc.Rect{Width: small.Size.X, Height: small.Size.Y}) {
		t.Fatalf("an output with no panel orientation shows %+v", o)
	}
}

func TestTheInheritedFrameFadesOnTheTurnedPanelUnturned(t *testing.T) {
	t.Run("imported on the GPU", func(t *testing.T) { testTheInheritedFrameFadesUnturned(t, startMountedInheritedVulkan) })
	t.Run("copied by the CPU", func(t *testing.T) { testTheInheritedFrameFadesUnturned(t, startMountedInherited) })
}

func testTheInheritedFrameFadesUnturned(t *testing.T, start panelStarter) {
	probe, err := udmabuf.New(4096)
	if err != nil {
		t.Skipf("the headless backend makes the inherited frame a udmabuf: %v", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	d := 10 * time.Second
	off, on := false, true
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.FadeIn: {Enabled: &on, Shape: anim.Linear, Duration: &d}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, panel := range []render.Transform{render.Rotate90, render.Rotate270} {
		t.Run(panel.String(), func(t *testing.T) {
			r := &reports{}
			h := start(t, panel, func(cfg *compositor.Config) {
				cfg.Animations = table
				cfg.Report = r.report
			})
			h.Freeze()
			c := testclient.Connect(t, h.Socket())
			w := c.Window("greeter", quadrants)
			c.DispatchUntil("the window to map", w.Mapped)
			h.Eventually("the frame that replaces the inherited one", func(b *headless.Backend, _ *compositor.Compositor) bool {
				return b.Outputs()[0].Takeover() != nil
			})
			var left, right [3]byte
			h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
				first := b.Outputs()[0].Takeover()
				left, right = pixel(first, 1, 1), pixel(first, small.Size.X-2, small.Size.Y-2)
				return nil
			})
			wantLeft := [3]byte{headless.InheritedColor.R, headless.InheritedColor.G, headless.InheritedColor.B}
			wantRight := [3]byte{headless.InheritedRightColor.R, headless.InheritedRightColor.G, headless.InheritedRightColor.B}
			if !near(left, wantLeft) || !near(right, wantRight) {
				t.Fatalf("the first frame over the inherited one on a %v panel shows %v at its top-left and %v at its bottom-right, not the inherited frame's %v and %v where it had them (reported %q)", panel, left, right, wantLeft, wantRight, r.take())
			}
		})
	}
}
