package launcher

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

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
	"github.com/stubbedev/lys/shell/service/fileindex"
)

func writeRedPNG(t *testing.T, path string) {
	t.Helper()
	const side = 64
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// combiOverFiles opens combi over a folder holding report.png and
// notes/report-draft.txt, with only the window mode beside the files.
func combiOverFiles(t *testing.T) (*harness, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeRedPNG(t, filepath.Join(root, "report.png"))
	if err := os.WriteFile(filepath.Join(root, "notes", "report-draft.txt"), []byte("draft"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t)
	h.s.d.Sheet = apptheme.New(h.cfg).Sheet
	h.cfg.Launcher.Search.FileRoots = []string{root}
	h.cfg.Launcher.Combi.Modes = []string{"window"}
	mode := "combi"
	h.open(launcheripc.SessionOptions{Mode: &mode}, false)
	h.until("the file index", func() bool { return h.s.files.index != nil && h.s.files.index.Snapshot().Ready })
	return h, root
}

func TestCombiShowsFileRowsWithTheirFolderAndThumbnail(t *testing.T) {
	useBundledIcons(t)
	h, root := combiOverFiles(t)
	h.s.cur.views.Entry.SetText("report")
	h.until("the file rows", func() bool { return h.s.model.len() == 2 })
	if first, _ := h.s.model.item(0); first.MatchText != "report.png" {
		t.Errorf("the first row is %q, want the shallower file", first.MatchText)
	}
	second, _ := h.s.model.item(1)
	if !strings.Contains(second.Display, filepath.Join(root, "notes")) {
		t.Errorf("the second row shows %q, without its folder", second.Display)
	}
	data := make([]byte, render.Stride(renderW)*renderH)
	red := func() int {
		n := 0
		for i := 0; i+3 < len(data); i += 4 {
			if data[i+3] == 255 && data[i+2] > 240 && data[i+1] < 16 && data[i] < 16 {
				n++
			}
		}
		return n
	}
	h.until("the picture's thumbnail", func() bool {
		paintFrame(h.s.cur.views.panelFrame, data)
		return red() > 64
	})
	savePanel(t, data, "combi-files")
	h.s.cur.views.Entry.SetText("!files draft")
	h.until("the bang's one file", func() bool { return h.s.model.len() == 1 && strings.HasPrefix(h.s.model.textAt(0), "report-draft.txt") })
	paintFrame(h.s.cur.views.panelFrame, data)
	savePanel(t, data, "combi-files-bang")
}

// The search panel reads the index combi's files search: one index for
// both, opened once, and a file created while it runs is a row in the
// next panel without the panel walking anything.
func TestThePanelAndCombiShareOneLiveFileIndex(t *testing.T) {
	h := newPanelHarness(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old-report.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	opened := 0
	h.s.files.open = func(c fileindex.Config) (*fileindex.Index, error) {
		opened++
		return fileindex.Open(c)
	}
	h.cfg.Launcher.Search.FileRoots = []string{root}
	h.cfg.Launcher.Search.Providers = []config.LauncherSearchProvider{config.SearchFiles}
	p, _ := h.openPanel()
	h.until("the file index", func() bool { return h.s.files.index != nil && h.s.files.index.Snapshot().Ready })
	h.typed(p, "report")
	h.waitTitles(p, "old-report.txt")
	h.key("Escape")
	h.until("the panel to close", func() bool { return h.s.panel == nil })

	if err := os.WriteFile(filepath.Join(root, "new-report.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h.until("the index to see the new file", func() bool {
		for _, f := range h.s.files.index.Snapshot().Files {
			if f.Name == "new-report.txt" {
				return true
			}
		}
		return false
	})
	p, _ = h.openPanel()
	h.typed(p, "report")
	h.until("both files", func() bool {
		got := p.rowTitles()
		slices.Sort(got)
		return slices.Equal(got, []string{"new-report.txt", "old-report.txt"})
	})
	h.key("Escape")
	h.until("the panel to close", func() bool { return h.s.panel == nil })

	mode := "combi"
	h.open(launcheripc.SessionOptions{Mode: &mode}, false)
	h.until("combi", func() bool { return h.s.cur != nil })
	if opened != 1 {
		t.Errorf("the panel and combi opened %d indexes, want one", opened)
	}
}

func TestAPanelWithoutFilesOpensNoIndex(t *testing.T) {
	h := newPanelHarness(t)
	h.s.files.open = func(fileindex.Config) (*fileindex.Index, error) {
		t.Error("a panel that searches no files opened the file index")
		return nil, errors.New("no index here")
	}
	p, _ := h.openPanel()
	h.typed(p, "report")
	if h.s.files.index != nil {
		t.Error("the panel holds a file index")
	}
}
