package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func trayItemsWith(t *testing.T, yaml string, l trayListing) (*kit, *trayItemsList) {
	t.Helper()
	was := runningTrayItems
	runningTrayItems = func() trayListing { return l }
	t.Cleanup(func() { runningTrayItems = was })
	k := testKit(t, yaml)
	c, ok := fieldControl(k, k.store, pageRow(t, "systray", trayBlacklistPath)).(*trayItemsList)
	if !ok {
		t.Fatal("the systray page does not open on the running items")
	}
	return k, c
}

var discordAndSteam = trayListing{items: []trayItem{
	{id: "steam", title: "Steam", icon: "steam"},
	{id: "com.discord.Discord", title: "Discord", icon: "discord"},
}}

func TestTheRunningTrayItemsAreListedAndASwitchHidesOne(t *testing.T) {
	k, c := trayItemsWith(t, "", discordAndSteam)
	labels := treetest.Labels(c)
	if d, s := slices.Index(labels, "Discord"), slices.Index(labels, "Steam"); d < 0 || s < d {
		t.Fatalf("the rows read %q, want Discord then Steam", labels)
	}
	switches := treetest.All[*widget.Switch](c)
	if len(switches) != 2 || !switches[0].On() || !switches[1].On() {
		t.Fatalf("%d switches, want two on", len(switches))
	}
	switches[0].Toggle()
	if got := k.store.value(trayBlacklistPath); !slices.Equal(got.([]any), []any{"com.discord.Discord"}) {
		t.Fatalf("hiding Discord stored %v, want its exact id", got)
	}
	switches = treetest.All[*widget.Switch](c)
	switches[0].Toggle()
	if got, _ := k.store.value(trayBlacklistPath).([]any); len(got) != 0 {
		t.Errorf("showing Discord again left %v", got)
	}
}

func TestATrayItemHiddenByAPatternSaysSoAndKeepsItsSwitch(t *testing.T) {
	k, c := trayItemsWith(t, "modules:\n  systray:\n    blacklist: ['*discord*']\n", discordAndSteam)
	sw := treetest.All[*widget.Switch](c)[0]
	if sw.On() || sw.Enabled() {
		t.Fatalf("Discord under *discord*: switch on %v, enabled %v; want off and fixed", sw.On(), sw.Enabled())
	}
	if !slices.ContainsFunc(treetest.Labels(c), func(l string) bool { return strings.Contains(l, "*discord*") }) {
		t.Errorf("the row does not name the pattern: %q", treetest.Labels(c))
	}
	if steam := treetest.All[*widget.Switch](c)[1]; !steam.On() || !steam.Enabled() {
		t.Error("Steam, which no pattern names, is not shown")
	}
	if got := k.store.value(trayBlacklistPath).([]any); !slices.Equal(got, []any{"*discord*"}) {
		t.Errorf("building the rows rewrote the blacklist: %v", got)
	}
}

func TestATrayItemsIconIsAnOverrideForItsExactID(t *testing.T) {
	k, c := trayItemsWith(t, "modules:\n  systray:\n    overrides:\n      - name: '*'\n        color: red\n", discordAndSteam)
	s := c.overrideIcon("steam")
	if err := s.set("ld-gamepad-symbolic"); err != nil {
		t.Fatal(err)
	}
	got := k.store.value(trayOverridesPath).([]any)
	if len(got) != 2 || got[0].(map[string]any)["name"] != "steam" || got[0].(map[string]any)["icon"] != "ld-gamepad-symbolic" {
		t.Fatalf("picking Steam's icon stored %v, want its own override ahead of the pattern", got)
	}
	if s.get() != "ld-gamepad-symbolic" || c.overrideIcon("com.discord.Discord").get() != "" {
		t.Error("the icons read back wrong")
	}
	s.unset()
	got = k.store.value(trayOverridesPath).([]any)
	if len(got) != 1 || got[0].(map[string]any)["name"] != "*" {
		t.Errorf("clearing Steam's icon left %v, want the pattern alone", got)
	}
}

func TestATrayThatCannotBeReadSaysWhy(t *testing.T) {
	_, c := trayItemsWith(t, "", trayListing{err: errors.New("read the items registered with org.kde.StatusNotifierWatcher: no owner")})
	labels := treetest.Labels(c)
	if !slices.Contains(labels, locale.T().Get("settings-tray-items-none")) {
		t.Errorf("no items: rows read %q", labels)
	}
	if !slices.ContainsFunc(labels, func(l string) bool { return strings.Contains(l, "no owner") }) {
		t.Errorf("the failure is not shown: %q", labels)
	}
}
