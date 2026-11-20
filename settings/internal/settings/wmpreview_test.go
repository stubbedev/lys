package settings

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func lysWMKit(t *testing.T, configYAML string) *kit {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if configYAML != "" {
		writeFile(t, filepath.Join(dir, "lys-wm", "config.yaml"), configYAML)
	}
	k := testKit(t, "")
	if _, err := k.wmBackend(); err != nil {
		t.Fatal(err)
	}
	return k
}

func previewOf(t *testing.T, p *settingsPage) *windowPreview {
	t.Helper()
	previews := treetest.All[*windowPreview](p)
	if len(previews) != 1 {
		t.Fatalf("the page holds %d window previews, want one", len(previews))
	}
	return previews[0]
}

func TestTheWindowPagesPreviewTheDecorationTheirDraftSets(t *testing.T) {
	for _, id := range []string{"wm-windows", "wm-effects"} {
		k := lysWMKit(t, "")
		p := layoutPage(t, k, id)
		pv := previewOf(t, p)
		if pv.bad != nil || !pv.area.Visible() {
			t.Fatalf("%s: lys-wm's defaults preview as %v", id, pv.bad)
		}
		if id == "wm-effects" {
			if pv.look.blur.enabled || !pv.look.shadow.enabled {
				t.Errorf("the default look previews blur %t and shadow %t", pv.look.blur.enabled, pv.look.shadow.enabled)
			}
			setRow(t, p, "decoration.blur.enabled", true)
			setRow(t, p, "decoration.shadow.enabled", false)
			if !pv.look.blur.enabled || pv.look.shadow.enabled {
				t.Errorf("blur on and shadow off preview blur %t and shadow %t", pv.look.blur.enabled, pv.look.shadow.enabled)
			}
			continue
		}
		before := pv.look
		setRow(t, p, "borders.width", int64(6))
		setRow(t, p, "gaps.inner", int64(30))
		setRow(t, p, "decoration.inactive_opacity", 0.5)
		setRow(t, p, "borders.active", map[string]any{"colors": []any{"#ff0000", "#0000ff"}, "angle": int64(45)})
		setRow(t, p, "titlebars.floating", true)
		setRow(t, p, "titlebars.side", "right")
		setRow(t, p, "titlebars.buttons", []any{"close"})
		l := pv.look
		if l.borderWidth != 6 || l.gapInner != 30 || l.opacity[windowUnfocused] != 0.5 {
			t.Errorf("the draft previews border %d, gap %d, inactive opacity %v", l.borderWidth, l.gapInner, l.opacity[windowUnfocused])
		}
		if want := (borderLook{colors: []render.Color{render.RGB(0xff, 0, 0), render.RGB(0, 0, 0xff)}, angle: 45}); !reflect.DeepEqual(l.border[windowFocused], want) {
			t.Errorf("the gradient previews as %+v, want %+v", l.border[windowFocused], want)
		}
		if !l.titlebar.shown || l.titlebar.buttonsLeft || !slices.Equal(l.titlebar.buttons, []string{"close"}) {
			t.Errorf("the title bar previews as %+v", l.titlebar)
		}
		if reflect.DeepEqual(before, l) {
			t.Error("the preview did not follow the draft")
		}
	}
}

func TestTheWindowPreviewDrawsTheBorderAndGapsItReads(t *testing.T) {
	k := lysWMKit(t, "")
	l, bad := readWindowLook(k.wm.value, func(text string) (render.Color, bool) { return wmColorRGB(k, text) })
	if bad != nil {
		t.Fatal(bad)
	}
	l.shadow.enabled = false
	l.borderWidth, l.rounding = 8, 0
	red := render.RGB(0xff, 0, 0)
	l.border[windowFocused] = borderLook{colors: []render.Color{red}}
	const w, h = 400, windowPreviewH
	screen := render.Rect{W: w, H: h}
	frame := windowFrames(l, screen)[windowFocused]
	paint := func(l windowLook) render.Color {
		data := make([]byte, render.Stride(w)*h)
		paintWindows(render.New(data, render.Stride(w), w, h), l, screen)
		at := (frame.Y+1)*render.Stride(w) + (frame.X+frame.W/2)*4
		return render.ColorFromBytes(data[at:])
	}
	if got := paint(l); got != red {
		t.Errorf("the focused window's top border paints %#x, want %#x", got, red)
	}
	l.borderWidth = 0
	if got := paint(l); got == red {
		t.Error("a window with no border still paints its colour")
	}
	wide := l
	wide.gapInner = 80
	for _, look := range []windowLook{l, wide} {
		f := windowFrames(look, screen)
		if got := f[windowUnfocused].X - f[windowFocused].X - f[windowFocused].W; got != previewPixels(look.gapInner) {
			t.Errorf("an inner gap of %d previews %d apart, want %d", look.gapInner, got, previewPixels(look.gapInner))
		}
	}
}

func TestAValueThePreviewCannotDrawSaysWhichInsteadOfDrawing(t *testing.T) {
	k := lysWMKit(t, "borders:\n  active: nosuchcolour\n")
	pv := previewOf(t, layoutPage(t, k, "wm-windows"))
	if pv.bad == nil || pv.bad.path != "borders.active" {
		t.Fatalf("an unknown border colour previews as %v", pv.bad)
	}
	tr := locale.T()
	want := tr.Get("wm-preview-unreadable", i18n.Str("setting", tr.Get("wm-borders-active")), i18n.Str("value", "nosuchcolour"))
	if pv.area.Visible() || !pv.state.Visible() || pv.state.Text() != want {
		t.Errorf("the preview shows the drawing %t and says %q, want %q", pv.area.Visible(), pv.state.Text(), want)
	}
	k = lysWMKit(t, "")
	for _, c := range []struct {
		path  string
		value any
	}{
		{"decoration.active_opacity", 1.5},
		{"titlebars.side", "middle"},
		{"gaps.outer", []any{int64(1), int64(2)}},
		{"decoration.rounding", "round"},
	} {
		_, bad := readWindowLook(func(path string) any {
			if path == c.path {
				return c.value
			}
			return k.wm.value(path)
		}, func(text string) (render.Color, bool) { return wmColorRGB(k, text) })
		if bad == nil || bad.path != c.path {
			t.Errorf("%s at %v reads as %v", c.path, c.value, bad)
		}
	}
}
