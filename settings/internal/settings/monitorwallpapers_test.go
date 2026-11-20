package settings

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
)

func wallpaperFile(t *testing.T, name string, write func(*os.File) error) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := write(f); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func tileEdge(t *testing.T, p *monitorPicker, i int) render.Color {
	t.Helper()
	const w, h = 400, 200
	p.Arrange(render.Rect{W: w, H: h})
	stride := render.Stride(w)
	buf := make([]byte, stride*h)
	p.Paint(render.New(buf, stride, w, h))
	tile := p.screen(p.tiles[i].rect)
	at := (tile.Y+tile.H/2)*stride + (tile.X+1)*4
	return render.RGBA(buf[at+2], buf[at+1], buf[at], buf[at+3])
}

func TestAWallpaperThatCannotBeDecodedSaysSoOnItsTile(t *testing.T) {
	broken := wallpaperFile(t, "broken.png", func(f *os.File) error {
		_, err := f.WriteString("not a picture")
		return err
	})
	good := wallpaperFile(t, "good.png", func(f *os.File) error {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for i := range img.Pix {
			img.Pix[i] = 0xff
		}
		img.Set(0, 0, color.RGBA{A: 0xff})
		return png.Encode(f, img)
	})
	k := testKit(t, "wallpaper:\n  wallpaper: "+good+"\n  monitors:\n    - {name: DP-1, wallpaper: "+broken+"}\n")
	fakeHeads(t, []displays.Head{head("DP-1", 0), head("HDMI-A-1", 1920)}, nil)
	c := fieldControl(k, k.store, field("wallpaper.monitors", monitorWallpaperRow)).(*monitorWallpapers)
	failed := c.picker.thumbs["DP-1"]
	if failed.img != nil || !strings.Contains(failed.failure, broken) {
		t.Fatalf("the broken wallpaper's tile holds %+v, want a failure naming %s", failed, broken)
	}
	_, decodeErr := imagedecode.Open(broken)
	if decodeErr == nil || !strings.Contains(failed.failure, decodeErr.Error()) {
		t.Errorf("the failure %q does not carry the decoder's error %v", failed.failure, decodeErr)
	}
	if name := widget.Describe(c.picker.handles[0]).Name; !strings.Contains(name, failed.failure) {
		t.Errorf("the broken tile is named %q, without its failure", name)
	}
	c.picker.choose("HDMI-A-1")
	if got, want := tileEdge(t, c.picker, 0), widget.Current().DestructiveColor(); got != want {
		t.Errorf("the broken tile's edge is %v, want the destructive %v", got, want)
	}
	shown := c.picker.thumbs["HDMI-A-1"]
	if shown.img == nil || shown.failure != "" {
		t.Errorf("the readable wallpaper's tile holds %+v, want its picture", shown)
	}
	if name := widget.Describe(c.picker.handles[1]).Name; name != c.picker.tiles[1].label+" (HDMI-A-1)" {
		t.Errorf("the readable tile is named %q", name)
	}
	if got := tileEdge(t, c.picker, 1); got == widget.Current().DestructiveColor() {
		t.Error("the readable tile is drawn as failed")
	}
}

type idleDisplays struct{ closed bool }

func (*idleDisplays) Test([]displays.Setting) error  { return nil }
func (*idleDisplays) Apply([]displays.Setting) error { return nil }
func (m *idleDisplays) Close()                       { m.closed = true }

func fakeHeads(t *testing.T, heads []displays.Head, err error) *idleDisplays {
	t.Helper()
	mgr := &idleDisplays{}
	before := connectDisplays
	connectDisplays = func(onHeads func([]displays.Head)) (displayManager, error) {
		if err != nil {
			return nil, err
		}
		onHeads(heads)
		return mgr, nil
	}
	t.Cleanup(func() { connectDisplays = before })
	return mgr
}

func head(name string, x int) displays.Head {
	return displays.Head{Name: name, Enabled: true, X: x, Scale: 1, Current: 0, Modes: []displays.Mode{{Width: 1920, Height: 1080}}}
}

func pickerNames(p *monitorPicker) []string {
	var out []string
	for _, tile := range p.tiles {
		out = append(out, tile.name)
	}
	return out
}

func TestMonitorWallpaperEntriesChangeOnlyTheirMonitor(t *testing.T) {
	stored := []any{map[string]any{"name": "DP-1", "wallpaper": "/a.png", "fit-mode": "fill"}}
	added := withMonitorField(stored, "HDMI-A-1", "wallpaper", "/b.png")
	if len(added) != 2 || added[1].(map[string]any)["wallpaper"] != "/b.png" || added[1].(map[string]any)["fit-mode"] != "fill" {
		t.Errorf("adding a monitor: %v", added)
	}
	changed := withMonitorField(added, "DP-1", "fit-mode", "fit")
	if m, _ := monitorEntry(changed, "DP-1"); m["fit-mode"] != "fit" || m["wallpaper"] != "/a.png" {
		t.Errorf("changing DP-1's fit: %v", m)
	}
	if m, _ := monitorEntry(stored, "DP-1"); m["fit-mode"] != "fill" {
		t.Error("a change edited the stored entries in place")
	}
	if left := withoutMonitor(changed, "DP-1"); len(left) != 1 || left[0].(map[string]any)["name"] != "HDMI-A-1" {
		t.Errorf("removing DP-1: %v", left)
	}
}

func TestThePickerShowsTheConnectedDisplaysAndGivesOneItsOwnWallpaper(t *testing.T) {
	k := testKit(t, "wallpaper:\n  wallpaper: /walls/default.png\n  monitors:\n    - {name: DP-9, wallpaper: /walls/old.png}\n")
	mgr := fakeHeads(t, []displays.Head{head("DP-1", 0), head("HDMI-A-1", 1920)}, nil)
	c := fieldControl(k, k.store, field("wallpaper.monitors", monitorWallpaperRow)).(*monitorWallpapers)
	if got := pickerNames(c.picker); !slices.Equal(got, []string{"DP-1", "HDMI-A-1"}) {
		t.Fatalf("the picker shows %v", got)
	}
	c.picker.handles[1].KeyAction(widget.KeyEnter, 0)
	if c.picker.current() != "HDMI-A-1" {
		t.Fatalf("Enter on the second display selected %q", c.picker.selected)
	}
	treetest.First[*widget.Switch](t, c.detail).SetOn(true)
	if m, ok := monitorEntry(k.store.value("wallpaper.monitors"), "HDMI-A-1"); !ok || m["wallpaper"] != "/walls/default.png" {
		t.Fatalf("its own wallpaper starts from the default: %v %v", m, ok)
	}
	treetest.First[*widget.Switch](t, c.detail).SetOn(false)
	if _, ok := monitorEntry(k.store.value("wallpaper.monitors"), "HDMI-A-1"); ok {
		t.Fatal("switching off kept its entry")
	}
	if _, ok := monitorEntry(k.store.value("wallpaper.monitors"), "DP-9"); !ok {
		t.Fatal("a disconnected display's entry was lost")
	}
	treetest.First[*widget.Button](t, c.away).OnClick()
	if _, ok := monitorEntry(k.store.value("wallpaper.monitors"), "DP-9"); ok {
		t.Error("removing the disconnected display kept it")
	}
	c.close()
	if !mgr.closed {
		t.Error("closing the page left the display connection open")
	}
}

func TestWithoutTheDisplayConnectionThePickerShowsTheOutputNames(t *testing.T) {
	k := testKit(t, "")
	fakeHeads(t, nil, errors.New("no output manager"))
	c := fieldControl(k, k.store, field("wallpaper.monitors", monitorWallpaperRow)).(*monitorWallpapers)
	if got := pickerNames(c.picker); !slices.Equal(got, []string{"DP-1", "HDMI-A-1"}) {
		t.Errorf("the fallback shows %v", got)
	}
}
