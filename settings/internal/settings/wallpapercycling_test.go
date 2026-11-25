package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// cyclingOptionsRow is the wallpaper page's Cycling Options row.
func cyclingOptionsRow(t *testing.T, p *settingsPage) *fieldRow {
	t.Helper()
	i := slices.IndexFunc(p.rows, func(r *fieldRow) bool { return r.path == "wallpaper.cycling-directory" && r.action })
	if i < 0 {
		t.Fatal("the wallpaper page has no Cycling Options row")
	}
	return p.rows[i]
}

// Cycling Options shows, its options in it, only while a cycling
// directory is set: without one it would stand as a title over nothing.
func TestCyclingOptionsShowOnlyWhileADirectoryIsSet(t *testing.T) {
	k := testKit(t, "")
	p := layoutPage(t, k, "wallpaper")
	row := cyclingOptionsRow(t, p)
	g := row.ctl.(*cyclingOptions)
	if row.Visible() || g.showsNested() {
		t.Fatalf("with no directory the options row shows %v, its options %v", row.Visible(), g.showsNested())
	}
	setRow(t, p, "wallpaper.cycling-directory", "/walls")
	if !row.Visible() || !g.showsNested() || len(g.rows) != 3 {
		t.Fatalf("with a directory the options row shows %v, its %d options %v", row.Visible(), len(g.rows), g.showsNested())
	}
	if row.badge.Visible() || row.reset.Enabled() {
		t.Error("the directory's value badges the options row or arms its reset")
	}
	var paths []string
	for _, r := range g.rows {
		paths = append(paths, r.path)
	}
	if want := []string{"wallpaper.cycling-mode", "wallpaper.cycling-interval-mins", "wallpaper.cycling-same-image"}; !slices.Equal(paths, want) {
		t.Errorf("the options row holds %v, want %v", paths, want)
	}
	setRow(t, p, "wallpaper.cycling-directory", "")
	if row.Visible() {
		t.Error("clearing the directory left the options row showing")
	}
}

// The wallpaper's path sits where every path row puts its path, in the
// card's control column beside its text, not stacked under it: its field
// ends at the cycling directory's right edge and starts beside its
// row's title, with the picker grid under the row's text.
func TestTheWallpapersPathSitsInTheControlColumnLikeCyclingsPath(t *testing.T) {
	win := laidOutPage(t, "", "wallpaper", windowWidth)
	files := treetest.All[*fileEditor](win.page)
	if len(files) != 2 {
		t.Fatalf("the wallpaper page holds %d path fields, want the image's and cycling's", len(files))
	}
	image, cycling := files[0].Bounds(), files[1].Bounds()
	if image.X+image.W != cycling.X+cycling.W || image.X != cycling.X {
		t.Errorf("the image's path spans %d..%d, cycling's %d..%d", image.X, image.X+image.W, cycling.X, cycling.X+cycling.W)
	}
	g := treetest.First[*wallpaperGrid](t, win.page)
	if below := g.below.Bounds(); below.Y < image.Y+image.H || below.X >= image.X {
		t.Errorf("the picker grid sits at %v beside the path at %v, want it under the row's text", below, image)
	}
}
