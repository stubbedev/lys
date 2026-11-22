package bar

import (
	"fmt"
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/shell/powermenu"
)

// avatarPx is the user avatar's logical box in the session row.
const avatarPx = 32

// sessionPowerAction is the power-menu action a dashboard session
// action runs.
func sessionPowerAction(action config.SessionAction) powermenu.Action {
	switch action {
	case config.SessionActionLock:
		return powermenu.Lock
	case config.SessionActionLogOut:
		return powermenu.LogOut
	case config.SessionActionSuspend:
		return powermenu.Suspend
	case config.SessionActionReboot:
		return powermenu.Reboot
	case config.SessionActionPowerOff:
		return powermenu.ShutDown
	}
	panic(fmt.Sprintf("dashboard: session action %q has no power-menu action", action))
}

// currentAccount is the user the dashboard shows (a seam for tests).
var currentAccount = useraccount.Current

// setAvatar paints the account's face (the AccountsService icon, else
// ~/.face), looked up afresh, onto the tile as its background image,
// cover-fit inside the rounded box, reporting whether it found one.
func setAvatar(tile *widget.Box, acct useraccount.Account) bool {
	path := useraccount.FindAvatar(acct.Name, acct.Home)
	if path == "" {
		return false
	}
	tile.SetInlineStylePriority("background-image: url('"+path+"');", apptheme.ComponentPriority)
	return true
}

// userSessionSection is the dashboard's user-session card: the card over the session row — the avatar
// (the account's avatar, else the user glyph) and the username in .user-info, one
// icon button per configured action in .session-actions pinned to the
// row's end, each the power menu's action: its glyph and label, and
// a click requests it of the power menu, which asks before ending the
// session. The face is read when the dropdown opens; the glyph shows
// only without one, never over it. The stylesheet inks
// everything through the classes (user-avatar image, user-name,
// button.icon.session-btn image), so nothing carries a tint here.
func userSessionSection(ctx ModuleContext) widget.Widget {
	font, _ := dropdownFont(ctx)
	cfg := ctx.Config.Dashboard
	card := widget.NewBox(widget.Column, 0, 0)
	card.AddClass(css.Card, "dashboard-card")
	acct, err := currentAccount()
	if err != nil {
		log.Printf("dashboard: the session row names no user: %v", err)
	}
	row := widget.NewItem(font, acct.RealName, "")
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("dashboard-user-session")
	tile := widget.NewBox(widget.Row, 0, 0)
	tile.AddClass("user-avatar")
	widgets.Pin(tile, avatarPx, avatarPx)
	if err != nil || !setAvatar(tile, acct) {
		tile.Append(widget.NewThemeIcon("ld-user-symbolic", avatarPx), false)
	}
	row.SetMedia(tile, true)
	for _, action := range cfg.UserSession.Actions {
		power := sessionPowerAction(action)
		row.AddAction(ghostIconButton(power.Icon(), itemIconPx, power.Label(), func() { ctx.PowerMenu.Request(power) }, "session-btn"))
	}
	card.Append(row, false)
	return card
}
