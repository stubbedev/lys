package lockview

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/logind"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestTheGreeterListsUsersOnlyWhenAskedAndFew(t *testing.T) {
	on := config.LoginGreeterConfig{ShowUserList: true}
	for _, c := range []struct {
		g        config.LoginGreeterConfig
		accounts int
		want     bool
	}{
		{on, 1, true},
		{on, MaxListedUsers, true},
		{on, 0, false},
		{on, MaxListedUsers + 1, false},
		{config.LoginGreeterConfig{}, 2, false},
	} {
		if got := ListsUsers(c.g, c.accounts); got != c.want {
			t.Errorf("show-user-list %v with %d accounts lists them %v, want %v", c.g.ShowUserList, c.accounts, got, c.want)
		}
	}
}

func TestAUserButtonMarksItsSelection(t *testing.T) {
	u := NewUserButton(treetest.Font(t), "Alex Doe")
	if slices.Contains(u.Button.Classes(), "selected") || u.Tile.Initials() != "AD" {
		t.Fatalf("a new user button has classes %v and initials %q", u.Button.Classes(), u.Tile.Initials())
	}
	u.SetSelected(true)
	if !slices.Contains(u.Button.Classes(), "selected") {
		t.Error("a selected account is not marked")
	}
	u.SetSelected(false)
	if slices.Contains(u.Button.Classes(), "selected") {
		t.Error("an unselected account stays marked")
	}
}

func TestTheGreeterFooterHoldsThePowerButtonsTheConfigShows(t *testing.T) {
	font := treetest.Font(t)
	var acted []logind.PowerAction
	footer, power := GreeterFooter(font, config.LoginGreeterConfig{ShowPowerButtons: true}, func(a logind.PowerAction) { acted = append(acted, a) })
	if power == nil || !treetest.Contains(footer, power) {
		t.Fatal("show-power-buttons builds no power row in the footer")
	}
	g := i18n.Greeter()
	names := []string{g.Get("greeter-sleep"), g.Get("greeter-restart"), g.Get("greeter-shutdown")}
	if got := treetest.Labels(power); !slices.Equal(got, names) {
		t.Errorf("the power row reads %v, want %v", got, names)
	}
	for _, name := range names {
		b := treetest.Button(t, power, name)
		if b.TooltipText() != name {
			t.Errorf("the %s button is tipped %q", name, b.TooltipText())
		}
		b.OnClick()
	}
	if want := []logind.PowerAction{logind.Suspend, logind.Reboot, logind.PowerOff}; !slices.Equal(acted, want) {
		t.Errorf("the power buttons ran %v, want %v", acted, want)
	}

	footer, power = GreeterFooter(font, config.LoginGreeterConfig{}, func(logind.PowerAction) { t.Error("a hidden power row acted") })
	if power != nil || len(treetest.WithClass(footer, "lock-power-button")) != 0 {
		t.Error("show-power-buttons off still builds power buttons")
	}
}
