package vulkan_test

import (
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lgpu "github.com/stubbedev/lys-lib/gpu"

	"github.com/stubbedev/lys-wm/internal/gpu"
	"github.com/stubbedev/lys-wm/internal/render"
	"github.com/stubbedev/lys-wm/internal/render/rendertest"
	"github.com/stubbedev/lys-wm/internal/render/soft"
	"github.com/stubbedev/lys-wm/internal/render/vulkan"
	"github.com/stubbedev/lys-wm/internal/testenv"
)

var _ render.Renderer = (*vulkan.Renderer)(nil)

func target(t testing.TB, r *vulkan.Renderer, size image.Point) render.ReadTarget {
	t.Helper()
	tg, err := r.NewTarget(size)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tg.Destroy(); err != nil {
			t.Error(err)
		}
	})
	return tg
}

func TestScreenShadersMatchTheSoftwareRenderer(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.ScreenShaders(t, r, target(t, r, rendertest.Size), rendertest.ExampleShaders(t)...)
}

func TestParityWithTheSoftwareRenderer(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.Parity(t, r, target(t, r, rendertest.Size))
}

func TestGPUWaitsCountTheDevicesWaitsAndNothingElse(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.GPUWaitsCountOnlyWaits(t, r, target(t, r, rendertest.Size))
}

func TestOpaqueOpsDrawWithoutBlendingAndTranslucentOnesWithIt(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.OpaqueRunsWithoutBlending(t, r, target(t, r, image.Pt(64, 64)), func() uint64 { return vulkan.OpaqueRuns(r) })
}

func TestEveryTextureFormatDrawsItsColour(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.EveryFormat(t, r, target(t, r, image.Pt(1, 1)))
}

func TestReadKeepsTheStridePadding(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.ReadKeepsTheStridePadding(t, r, target(t, r, image.Pt(5, 3)))
}

func TestReadRowsWritesOnlyThoseRows(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.ReadRowsWritesOnlyThoseRows(t, r, target(t, r, image.Pt(5, 3)))
}

func TestMisuseFailsPrecisely(t *testing.T) {
	r := testenv.Vulkan(t)
	other := testenv.Vulkan(t)
	if _, err := r.Begin(soft.NewBuffer(4, 4), nil); err == nil || !strings.Contains(err.Error(), "cannot render into *soft.Buffer") {
		t.Fatalf("a soft target: %v", err)
	}
	if _, err := r.NewTexture(render.ARGB8888, image.Pt(4, 4), 8, make([]byte, 64)); err == nil {
		t.Fatal("a stride shorter than a row was accepted")
	}
	tg := target(t, r, image.Pt(8, 8))
	if _, err := r.Begin(target(t, other, image.Pt(8, 8)), nil); err == nil || !strings.Contains(err.Error(), "another renderer") {
		t.Fatalf("another renderer's target: %v", err)
	}
	p, err := r.Begin(tg, render.Region{image.Rect(0, 0, 8, 8)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Begin(tg, nil); err == nil {
		t.Fatal("a second Begin was accepted")
	}
	p.DrawTexture(rendertest.Load(t, other).ARGB, render.FRect{W: 7, H: 5}, image.Rect(0, 0, 7, 5), render.Normal, 1, render.Mask{})
	if err := p.End(); err == nil || !strings.Contains(err.Error(), "another renderer") {
		t.Fatalf("a foreign texture: %v", err)
	}
	for name, draw := range map[string]func(render.Pass){
		"not premultiplied": func(p render.Pass) { p.FillRect(image.Rect(0, 0, 1, 1), render.Color{R: 0xff, A: 0x10}, render.Mask{}) },
		"source outside texture": func(p render.Pass) {
			p.DrawTexture(rendertest.Load(t, r).ARGB, render.FRect{W: 8, H: 5}, image.Rect(0, 0, 8, 5), render.Normal, 1, render.Mask{})
		},
		"alpha above one": func(p render.Pass) {
			p.DrawTexture(rendertest.Load(t, r).ARGB, render.FRect{W: 7, H: 5}, image.Rect(0, 0, 7, 5), render.Normal, 2, render.Mask{})
		},
	} {
		p, err := r.Begin(tg, nil)
		if err != nil {
			t.Fatal(err)
		}
		draw(p)
		if err := p.End(); err == nil {
			t.Errorf("%s: End succeeded", name)
		}
	}
}

func TestUploadsAcrossBands(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.UploadsAcrossBands(t, r, target(t, r, rendertest.UploadTargetSize()))
}

func TestSurvivesAFaultingUpload(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.SurvivesAFaultingUpload(t, r, target(t, r, image.Pt(1, 1)))
}

func TestStreamedUpdatesStayInOrder(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.StreamedUpdatesStayInOrder(t, r, target(t, r, image.Pt(1, 1)))
}

func TestOverlappingDamageBlendsOnce(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.OverlappingDamageBlendsOnce(t, r, target(t, r, rendertest.DamageSize))
}

func TestScaledDrawsKeepFlatColor(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.ScaledDrawsKeepFlatColor(t, r, target(t, r, rendertest.ScaledSize))
}

func TestMaskEdgesMatchTheSoftwareRendererAcrossA4KOutput(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.MaskEdgesAcrossA4KOutput(t, r, target(t, r, rendertest.FourKStrip))
}

func TestAnAsyncReadSeesTheFrameItWasStartedAfter(t *testing.T) {
	rendertest.AsyncReadSeesItsFrame(t, testenv.Vulkan(t))
}

func TestASteadyPassDoesNotAllocate(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector allocates on its own")
	}
	r := testenv.Vulkan(t)
	tex := rendertest.Load(t, r)
	tg := target(t, r, rendertest.Size)
	damage := render.Region{image.Rect(0, 0, 32, 32)}
	whole := render.FRect{W: float64(rendertest.TexSize.X), H: float64(rendertest.TexSize.Y)}
	var failed error
	draw := func() {
		p, err := r.Begin(tg, damage)
		if err != nil {
			failed = err
			return
		}
		p.FillRect(image.Rect(0, 0, 32, 32), render.Color{A: 0xff}, render.Mask{})
		p.DrawTexture(tex.ARGB, whole, image.Rectangle{Max: rendertest.TexSize}, render.Normal, 1, render.Mask{})
		p.DrawTexture(tex.XRGB, whole, image.Rectangle{Max: rendertest.TexSize}.Add(image.Pt(8, 8)), render.Normal, 1, render.Mask{})
		if err := p.End(); err != nil && failed == nil {
			failed = err
		}
	}
	draw()
	n := testing.AllocsPerRun(50, draw)
	if failed != nil {
		t.Fatal(failed)
	}
	if n != 0 {
		t.Fatalf("a steady pass allocated %v times", n)
	}
}

func TestChromaSitingReachesTheDevice(t *testing.T) {
	for _, c := range []struct {
		in   gpu.Siting
		want lgpu.Siting
	}{{gpu.SitingUnspecified, lgpu.SitingDefault}, {gpu.SitedAt0, lgpu.SitedAt0}, {gpu.SitedAtHalf, lgpu.SitedAtHalf}, {gpu.SitedAt1, lgpu.SitingDefault}} {
		got := vulkan.YCbCrOf(gpu.YUVEncoding{Chroma: gpu.ChromaSiting{Horizontal: c.in, Vertical: c.in}})
		if got.Chroma.X != c.want || got.Chroma.Y != c.want {
			t.Errorf("siting %v: %+v, want %v on both axes", c.in, got.Chroma, c.want)
		}
	}
}

func hardware(t testing.TB) (*vulkan.Renderer, string) {
	t.Helper()
	nodes, err := filepath.Glob("/dev/dri/renderD*")
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		r, err := vulkan.OpenRenderNode(node)
		if err != nil {
			continue
		}
		t.Cleanup(func() {
			if err := r.Close(); err != nil {
				t.Error(err)
			}
		})
		t.Logf("Vulkan device: %s on %s", r.Device().Name(), node)
		return r, node
	}
	t.Skip("no Vulkan driver drives a render node here; unset VK_DRIVER_FILES on a GPU machine to run this")
	return nil, ""
}

func TestParityOnHardware(t *testing.T) {
	r, _ := hardware(t)
	rendertest.Parity(t, r, target(t, r, rendertest.Size))
}

func TestDMABUFTexturesOnHardware(t *testing.T) {
	r, node := hardware(t)
	rendertest.DMABUFTextures(t, r, node)
}

func TestExternalYUVTexturesOnHardware(t *testing.T) {
	r, node := hardware(t)
	rendertest.ExternalYUV(t, r, node)
}

func TestYUVThroughAColorTransformOnHardware(t *testing.T) {
	r, node := hardware(t)
	rendertest.YUVThroughAColorTransform(t, r, node)
}

func TestYUVHintsSelectTheConversionOnHardware(t *testing.T) {
	r, node := hardware(t)
	rendertest.YUVHintsSelectTheConversion(t, r, node)
}

func TestFrameFencesReportWhenTheGPUFinishedOnHardware(t *testing.T) {
	r, _ := hardware(t)
	rendertest.FrameFencesReportTheGPUFinishing(t, r)
}

func TestARenderNodeNamesTheMainDeviceOnHardware(t *testing.T) {
	r, node := hardware(t)
	got, err := r.RenderNode()
	if err != nil {
		t.Fatal(err)
	}
	if got != node {
		t.Fatalf("a renderer opened on %s names %s", node, got)
	}
}

func TestColorParityWithTheSoftwareRenderer(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.ColorParity(t, r, target(t, r, rendertest.Size))
}

func TestTargetsCountAndAFrameOfPassesIsOneSubmit(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.TargetsAndSubmitsCount(t, r, target(t, r, rendertest.Size), 1)
}

func TestAKawaseLevelOverwritesItsTargetWithoutAClear(t *testing.T) {
	rendertest.KawaseOverwrites(t, testenv.Vulkan(t))
}

func TestALostDeviceIsReplacedAndEveryObjectRemade(t *testing.T) {
	rendertest.SurvivesALostDevice(t, testenv.Vulkan(t))
}

func TestAlphaCuts(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.AlphaCuts(t, r, target(t, r, rendertest.CutTargetSize()))
}

func TestAlphaCutOverDMABUFOnHardware(t *testing.T) {
	r, node := hardware(t)
	rendertest.AlphaCutOverDMABUF(t, r, node, target(t, r, rendertest.CutTargetSize()))
}

func TestEveryCompileLeavesRealtimeFirst(t *testing.T) {
	r := testenv.Vulkan(t)
	rendertest.CompilesLeaveRealtimeOncePerDrawState(t, r, target(t, r, image.Pt(4, 4)), target(t, r, image.Pt(8, 8)))
}

func paintUntilComplete(t *testing.T, r *vulkan.Renderer, paint func() error) (compiles uint64) {
	t.Helper()
	deadline := time.Now().Add(testenv.HangLimit)
	for {
		before := r.Counts().Incomplete
		n, err := rendertest.CompilesOnAPinnedThread(paint)
		if err != nil {
			t.Fatal(err)
		}
		compiles += n
		if r.Counts().Incomplete == before {
			return compiles
		}
		if time.Now().After(deadline) {
			t.Fatalf("frames still skipped draws after %v", testenv.HangLimit)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestOffThreadFramesNeverCompileOnTheirThreadAndWarmOnesBuildNothing(t *testing.T) {
	r := testenv.Vulkan(t)
	early := target(t, r, image.Pt(4, 4))
	if err := r.BuildOffThread(); err != nil {
		t.Fatal(err)
	}
	late := target(t, r, image.Pt(8, 8))
	e := rendertest.EveryEffect(t, r)
	for _, tt := range []render.Target{early, late} {
		if n := paintUntilComplete(t, r, rendertest.PaintEffects(r, tt, e)); n != 0 {
			t.Errorf("frames into a %v target left realtime %d times", tt.Size(), n)
		}
	}
	warm := r.Counts()
	n, err := rendertest.CompilesOnAPinnedThread(rendertest.PaintEffects(r, early, e))
	if err != nil {
		t.Fatal(err)
	}
	got := r.Counts()
	if n != 0 || got.Programs != warm.Programs || got.Incomplete != warm.Incomplete || got.ColdDraws != warm.ColdDraws {
		t.Errorf("a warm frame left realtime %d times, built %d pipelines, skipped draws in %d passes and drew %d cold draws", n, got.Programs-warm.Programs, got.Incomplete-warm.Incomplete, got.ColdDraws-warm.ColdDraws)
	}
}

func TestAFrameThatSkippedADrawDrawsItOnceItsPipelineIsBuilt(t *testing.T) {
	r := testenv.Vulkan(t)
	if err := r.BuildOffThread(); err != nil {
		t.Fatal(err)
	}
	red := render.Color{R: 0xff, A: 0xff}
	check := func(when string) {
		t.Helper()
		tt := target(t, r, image.Pt(2, 2))
		if n := paintUntilComplete(t, r, func() error {
			p, err := r.Begin(tt, render.Region{{Max: tt.Size()}})
			if err != nil {
				return err
			}
			p.Clear(render.Color{})
			p.FillRect(image.Rect(0, 0, 2, 2), red, render.Mask{})
			return p.End()
		}); n != 0 {
			t.Errorf("%s: left realtime %d times", when, n)
		}
		pix := make([]byte, 2*2*4)
		if err := tt.Read(pix, 2*4); err != nil {
			t.Fatal(err)
		}
		if got := (render.Color{R: pix[2], G: pix[1], B: pix[0], A: pix[3]}); got != red {
			t.Errorf("%s: the completed frame drew %v, want %v", when, got, red)
		}
	}
	check("on the first device")
	r.LoseDevice()
	if err := r.Recover(); err != nil {
		t.Fatal(err)
	}
	check("after a device loss")
}

func TestOnceItsPipelinesAreBuiltTheFirstFrameIntoANewTargetIsComplete(t *testing.T) {
	r := testenv.Vulkan(t)
	if err := r.BuildOffThread(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(testenv.HangLimit)
	for {
		built, err := r.PipelinesBuilt()
		if err != nil {
			t.Fatal(err)
		}
		if built {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the pipelines were not built within %v", testenv.HangLimit)
		}
		time.Sleep(time.Millisecond)
	}
	before := r.Counts()
	if before.Programs == 0 {
		t.Error("the pipelines built off the thread were not counted")
	}
	e := rendertest.EveryEffect(t, r)
	n, err := rendertest.CompilesOnAPinnedThread(rendertest.PaintEffects(r, target(t, r, image.Pt(4, 4)), e))
	if err != nil {
		t.Fatal(err)
	}
	got := r.Counts()
	if n != 0 || got.Incomplete != before.Incomplete || got.Programs != before.Programs {
		t.Errorf("the first frame into a new target left realtime %d times, skipped draws in %d passes and built %d pipelines", n, got.Incomplete-before.Incomplete, got.Programs-before.Programs)
	}
}
