package lockview

import (
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
)

// MaxListedUsers caps the greeter's user list; beyond it the username
// entry carries the load.
const MaxListedUsers = 8

// PowerIconPx is the side of the greeter's footer glyphs.
const PowerIconPx = 20

const (
	sleepIcon    = "ld-moon-symbolic"
	restartIcon  = "ld-rotate-ccw-symbolic"
	shutdownIcon = "ld-power-symbolic"
)

// PowerIcons are the theme icons the power row draws.
func PowerIcons() []string { return []string{sleepIcon, restartIcon, shutdownIcon} }

// ListsUsers reports whether the greeter lists a machine's login users:
// when g asks for the list and there are some, but no more than
// MaxListedUsers.
func ListsUsers(g config.LoginGreeterConfig, accounts int) bool {
	return g.ShowUserList && accounts > 0 && accounts <= MaxListedUsers
}

// UserRow is the row the greeter's user buttons sit in.
func UserRow() *widget.Box {
	row := widget.NewBox(widget.Row, widget.Current().Spacing, 0)
	row.AddClass("lock-users")
	row.SetHAlign(widget.AlignCenter)
	return row
}

// UserButton is one account of the greeter's user list.
type UserButton struct {
	Tile   *AccountTile
	Button *widget.Button
}

// NewUserButton is the button of the account called displayName.
func NewUserButton(font render.Font, displayName string) *UserButton {
	tile := NewAccountTile(font, displayName)
	button := widget.NewButton(tile.Root, 0, 0)
	button.AddClass("lock-user-button", css.Ghost)
	return &UserButton{Tile: tile, Button: button}
}

// SetSelected marks the account as the one being logged in or not.
func (u *UserButton) SetSelected(on bool) {
	if on {
		u.Button.AddClass("selected")
		return
	}
	u.Button.RemoveClass("selected")
}

// GreeterFooter is the row at the bottom of the greeter: lead, then the
// power buttons when g shows them, which run act. power is that row, nil
// when g hides it.
func GreeterFooter(font render.Font, g config.LoginGreeterConfig, act func(logind.PowerAction), lead ...widget.Widget) (footer, power *widget.Box) {
	footer = widget.NewBox(widget.Row, 4*widget.Current().Spacing, 0)
	footer.AddClass("lock-footer")
	for _, w := range lead {
		footer.Append(w, false)
	}
	if g.ShowPowerButtons {
		power = powerRow(font, act)
		footer.Append(power, false)
	}
	return footer, power
}

func powerRow(font render.Font, act func(logind.PowerAction)) *widget.Box {
	th := widget.Current()
	row := widget.NewBox(widget.Row, 4*th.Spacing, 0)
	row.AddClass("lock-power")
	for _, b := range []struct {
		icon   string
		name   string
		action logind.PowerAction
	}{
		{sleepIcon, i18n.Greeter().Get("greeter-sleep"), logind.Suspend},
		{restartIcon, i18n.Greeter().Get("greeter-restart"), logind.Reboot},
		{shutdownIcon, i18n.Greeter().Get("greeter-shutdown"), logind.PowerOff},
	} {
		btn := widget.NewButton(widget.NewThemeIcon(b.icon, PowerIconPx), 0, 0)
		btn.AddClass("lock-power-button", css.Ghost, css.Icon, css.Large)
		btn.SetAccessibleName(b.name)
		btn.OnClick = func() { act(b.action) }
		name := widget.NewLabel(font, widgets.TextPx(styling.Caption, 1), b.name, 0)
		name.AddClass("lock-power-name")
		name.SetAlignment(render.AlignCenter)
		col := widget.NewBox(widget.Column, th.Spacing/2, 0)
		col.AppendAligned(btn, false, widget.AlignCenter).Append(name, false)
		row.Append(col, false)
	}
	return row
}
