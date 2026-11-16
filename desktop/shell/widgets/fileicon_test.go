package widgets

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func paintOnce(t *testing.T, icon *widget.Icon) {
	t.Helper()
	const px = 32
	icon.Arrange(render.Rect{W: px, H: px})
	data := make([]byte, render.Stride(px)*px)
	icon.Paint(render.New(data, render.Stride(px), px, px))
}

func postedIcon(t *testing.T, path string) (icon *widget.Icon, posted <-chan func(), shown *bool) {
	t.Helper()
	queue := make(chan func(), 4)
	shown = new(bool)
	icon = NewAsyncFileIcon(path, "ld-file-symbolic", 32, func(fn func()) { queue <- fn }, func() { *shown = true })
	paintOnce(t, icon)
	if got := icon.Name(); got != "ld-file-symbolic" {
		t.Errorf("icon shows %q before its load, want the fallback", got)
	}
	return icon, queue, shown
}

func TestAFileThatPaintsNothingKeepsTheFallbackAndThePaintFinishes(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "icon.png")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	garbage := filepath.Join(dir, "garbage.png")
	if err := os.WriteFile(garbage, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"a FIFO": fifo, "undecodable": garbage, "missing": filepath.Join(dir, "gone.png")} {
		icon, posted, shown := postedIcon(t, path)
		for drained := false; !drained; {
			select {
			case fn := <-posted:
				fn()
			case <-time.After(quietWindow):
				drained = true
			}
		}
		if *shown || icon.Name() != "ld-file-symbolic" {
			t.Errorf("%s: shown %v as %q, want the fallback kept", name, *shown, icon.Name())
		}
	}
}

func TestADecodableFileReplacesTheFallback(t *testing.T) {
	path := treetest.GreenSVG(t)
	icon, posted, shown := postedIcon(t, path)
	for !*shown {
		waittest.For(t, posted, "the icon's load to post its next step")()
	}
	if icon.Name() != path {
		t.Errorf("shown as %q, want the file", icon.Name())
	}
}

func TestAFileIconsFirstPaintAtAFractionalScaleIsExactAndDecodesNothing(t *testing.T) {
	for _, sc := range treetest.FractionalScales {
		t.Run(sc.Name, func(t *testing.T) {
			path := treetest.GreenSVG(t)
			center := treetest.FractionalFirstPaint(t, sc.Num, func(post func(func())) *widget.Icon {
				return NewAsyncFileIcon(path, "", 16, post, nil)
			})
			treetest.ExpectGreen(t, sc.Name, center)
		})
	}
}
