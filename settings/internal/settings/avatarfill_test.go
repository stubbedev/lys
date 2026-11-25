package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// The Users page's avatar circle was filled in the muted color, about
// 1.04:1 against its card, and then in the border color, still about
// 1.2:1, so only its initials showed. Its edge stands off the card at
// least as WCAG asks a control's boundary to.
func TestTheUsersAvatarStandsOffItsCard(t *testing.T) {
	win := laidOutPage(t, "", "users", sweepW)
	avatars := treetest.All[*widget.Avatar](win.page)
	if len(avatars) == 0 {
		t.Fatal("the Users page shows no avatar")
	}
	a := avatars[0].Bounds()
	area := render.Rect{W: a.X + a.W + 4, H: a.Y + a.H + 4}
	stride := render.Stride(area.W)
	data := make([]byte, stride*area.H)
	win.page.Paint(render.New(data, stride, area.W, area.H))
	at := func(x, y int) render.Color {
		i := y*stride + 4*x
		return render.RGB(data[i+2], data[i+1], data[i])
	}
	card, fill := at(a.X-2, a.Y+a.H/2), widget.StyleBackground(avatars[0])
	if got := contrastOf(fill, card); got < minUIContrast {
		t.Errorf("the avatar stands %.2f:1 off its card, want %d:1", got, minUIContrast)
	}
	ink := 1.0
	for y := a.Y + a.H/4; y < a.Y+3*a.H/4; y++ {
		for x := a.X + a.W/4; x < a.X+3*a.W/4; x++ {
			ink = max(ink, contrastOf(at(x, y), fill))
		}
	}
	if ink < minUIContrast {
		t.Errorf("the avatar's initials stand %.2f:1 off its fill, want %d:1", ink, minUIContrast)
	}
}
