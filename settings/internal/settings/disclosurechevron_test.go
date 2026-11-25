package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

// pointerIcon is a theme whose chevron is a wedge filling the top half
// of its square, so which way it points shows in where it paints.
func pointerIcon(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "hicolor", "scalable", "actions")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	index := "[Icon Theme]\nName=Hicolor\nDirectories=scalable/actions\n\n[scalable/actions]\nSize=16\nMinSize=8\nMaxSize=512\nType=Scalable\n"
	if err := os.WriteFile(filepath.Join(base, "hicolor", "index.theme"), []byte(index), 0o600); err != nil {
		t.Fatal(err)
	}
	const wedge = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M0 0h16v8H0z" fill="#ffffff"/></svg>`
	if err := os.WriteFile(filepath.Join(dir, "ld-chevron-down-symbolic.svg"), []byte(wedge), 0o600); err != nil {
		t.Fatal(err)
	}
	before := widget.IconSearchPaths()
	widget.SetIconSearchPaths([]string{base})
	widget.RefreshIcons()
	t.Cleanup(func() {
		widget.SetIconSearchPaths(before)
		widget.RefreshIcons()
	})
}

// The stylesheet turned an open disclosure's chevron with transform,
// which an icon does not take, so it pointed down open and closed
// alike. Open, it is turned over; closed, it is not.
func TestAnOpenDisclosuresChevronIsTurnedOver(t *testing.T) {
	defer anim.SetInstant(true)()
	pointerIcon(t)
	k := testKit(t, "")
	d := k.headerDisclosure(k.label("Title"), false, func(bool) {})
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(d.button, false)
	apptheme.New(k.store.svc.Config()).Attach(root)
	topHeavy := func() bool {
		root.Arrange(render.Rect{W: 300, H: 60})
		root.Arrange(render.Rect{W: 300, H: 60})
		b := d.chevron.Bounds()
		data := make([]byte, render.Stride(300)*60)
		root.Paint(render.New(data, render.Stride(300), 300, 60))
		var top, bottom int
		for y := b.Y; y < b.Y+b.H; y++ {
			for x := b.X; x < b.X+b.W; x++ {
				if data[y*render.Stride(300)+x*4+3] == 0 {
					continue
				}
				if y < b.Y+b.H/2 {
					top++
				} else {
					bottom++
				}
			}
		}
		if top+bottom == 0 {
			t.Fatal("the chevron paints nothing")
		}
		return top > bottom
	}
	if !topHeavy() {
		t.Error("closed, the chevron is turned over")
	}
	d.setOpen(true)
	if topHeavy() {
		t.Error("open, the chevron is not turned over")
	}
}

// A collapsible section's chevron sat in the header's corner, past the
// column its card's controls and row chevrons end on. It ends on that
// column, and its title starts where a plain section's does.
func TestACollapsibleSectionsChevronEndsOnItsCardsControlColumn(t *testing.T) {
	k := testKit(t, "")
	sw := widget.NewSwitch(false)
	section, _, fold := collapsibleSection(k, "settings-section-durations", widget.NewItemGroup(k.row("Switch", "").withControl(sw)))
	plain := newSection(k, "settings-section-durations", widget.NewItemGroup(k.row("Other", "")))
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(section, false)
	root.Append(plain, false)
	fold.setOpen(true)
	styledLayOut(k, root, 900)
	chevron, control := fold.chevron.Bounds(), sw.Bounds()
	if got, want := chevron.X+chevron.W, control.X+control.W; got != want {
		t.Errorf("the chevron ends at %d, the card's control column at %d", got, want)
	}
	titles := treetest.WithClass(root, "settings-section-title")
	if a, b := titles[0].(*widget.Label).Bounds().X, titles[1].(*widget.Label).Bounds().X; a != b {
		t.Errorf("the collapsible section's title starts at %d, a plain one's at %d", a, b)
	}
}
