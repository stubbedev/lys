package harness

import (
	"testing"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/ipc"
	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/render"
)

func startMounted(t *testing.T, panel render.Transform, configure func(*compositor.Config)) *Harness {
	t.Helper()
	open := func(l *loop.Loop) (*headless.Backend, error) {
		b := headless.New(l)
		b.MountPanels(panel)
		return b, nil
	}
	return start(t, setup{open: open, configure: configure, traced: true}, small)
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
