package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// The Users page's avatar circle was filled in the muted color, about
// 1.04:1 against its card, so only its initials showed. It stands off
// the card at least as a field's border does.
func TestTheUsersAvatarStandsOffItsCard(t *testing.T) {
	win := laidOutPage(t, "", "users", sweepW)
	avatars := treetest.All[*widget.Avatar](win.page)
	if len(avatars) == 0 {
		t.Fatal("the Users page shows no avatar")
	}
	card := backgroundBehind(avatars[0])
	if got, want := contrastOf(widget.StyleBackground(avatars[0]), card), contrastOf(widget.Current().Border, card); got < want {
		t.Errorf("the avatar stands %.2f:1 off its card, a field's border %.2f:1", got, want)
	}
}
