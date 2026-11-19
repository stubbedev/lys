package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func TestEveryLysWMActionHasACategory(t *testing.T) {
	names := bindNames{actions: lysWMActions(t)}
	for _, a := range names.actions {
		if c := names.category(map[string]any{"action": a.Name}); c == categoryCustom {
			t.Errorf("lys-wm's %s files under custom", a.Name)
		}
	}
}

func TestABindFilesUnderWhatItIsFor(t *testing.T) {
	names := bindNames{actions: lysWMActions(t)}
	for _, tc := range []struct {
		chord, action string
		want          bindCategory
	}{
		{"SUPER + Q", "kill", categoryWindows},
		{"SUPER + 3", "workspace 3", categoryWorkspaces},
		{"SUPER + T", "makegroup tab", categoryLayout},
		{"SUPER + Return", "exec firefox", categoryApps},
		{"SUPER + Space", "global dev.stubbe.lys.shell:launcher", categoryApps},
		{"SUPER + S", "exec grim -g region", categoryScreenshots},
		{"Print", "exec my-capture", categoryScreenshots},
		{"SUPER + P", "global dev.stubbe.lys.shell:screenshot", categoryScreenshots},
		{"XF86AudioRaiseVolume", "exec my-volume up", categoryMedia},
		{"SUPER + M", "exec /usr/bin/playerctl play-pause", categoryMedia},
		{"SUPER + SHIFT + R", "reload", categorySystem},
		{"SUPER + X", "frobnicate", categoryCustom},
		{"SUPER + Y", "", categoryCustom},
	} {
		if got := names.category(map[string]any{"chord": tc.chord, "action": tc.action}); got != tc.want {
			t.Errorf("%s → %q files under %s, want %s", tc.chord, tc.action, bindCategoryNames[got], bindCategoryNames[tc.want])
		}
	}
}

func groupTitles(c *cardList) []string {
	var out []string
	for _, w := range c.list.Children() {
		for _, g := range c.groups {
			if widget.Widget(g.root) == w && g.root.Visible() {
				out = append(out, g.title.Text())
			}
		}
	}
	return out
}

func TestBindsAreGroupedByWhatTheyAreForAndFoundByWhatTheyDo(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: SUPER + Return, action: exec foot}\n  - {chord: SUPER + X, action: frobnicate}\n  - {chord: SUPER + 1, action: workspace 1}\n  - {chord: SUPER + Q, action: kill}\n")
	if got := groupTitles(c); !slices.Equal(got, []string{"Windows", "Workspaces", "Apps", "Custom"}) {
		t.Fatalf("the groups read %q", got)
	}
	shown := func() []int {
		var out []int
		for i, cd := range c.cards {
			if cd.root.Visible() {
				out = append(out, i)
			}
		}
		return out
	}
	for query, want := range map[string][]int{
		"close":   {3},
		"workspa": {2},
		"super+q": {3},
		"apps":    {0},
		"frob":    {1},
		"nothing": nil,
	} {
		c.filter.SetText(query)
		if got := shown(); !slices.Equal(got, want) {
			t.Errorf("%q shows binds %v, want %v", query, got, want)
		}
	}
	c.filter.SetText("close")
	if got := groupTitles(c); !slices.Equal(got, []string{"Windows"}) {
		t.Fatalf("filtered, the groups read %q; want only the one with a bind shown", got)
	}
	c.filter.SetText("")
	openCard(c, 3)
	if err := c.fieldSlot(3, "action").set("workspace 2"); err != nil {
		t.Fatal(err)
	}
	if got := groupTitles(c); !slices.Equal(got, []string{"Workspaces", "Apps", "Custom"}) {
		t.Fatalf("after the action changed the groups read %q", got)
	}
	ws := c.groups[int(categoryWorkspaces)].list.Children()
	if len(ws) != 2 || ws[0] != widget.Widget(c.cards[2].root) || ws[1] != widget.Widget(c.cards[3].root) {
		t.Fatal("the moved bind is not among the workspace binds in list order")
	}
	if len(c.cards[3].controls) == 0 || !c.cards[3].root.Children()[2].(*widget.Box).Visible() {
		t.Fatal("the moved bind closed")
	}
}
