package settings

import (
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/xkb"

	"github.com/stubbedev/lys/settings/internal/keys"
	"github.com/stubbedev/lys/settings/internal/locale"
)

// bindCategory is what a bind is for, the group the shortcuts list files it
// under.
type bindCategory int

const (
	categoryWindows bindCategory = iota
	categoryWorkspaces
	categoryLayout
	categoryApps
	categoryScreenshots
	categoryMedia
	categorySystem
	categoryCustom
)

var bindCategoryNames = [...]string{"windows", "workspaces", "layout", "apps", "screenshots", "media", "system", "custom"}

func (c bindCategory) label() string {
	return locale.T().Get("wm-binds-category-" + bindCategoryNames[c])
}

var actionCategories = map[string]bindCategory{
	"focus": categoryWindows, "move": categoryWindows, "kill": categoryWindows, "focuswindow": categoryWindows,
	"minimize": categoryWindows, "restore": categoryWindows, "closewindow": categoryWindows, "swap": categoryWindows,
	"togglefloating": categoryWindows, "place": categoryWindows, "focusfloating": categoryWindows, "fullscreen": categoryWindows,
	"setprop": categoryWindows, "swallow": categoryWindows, "pin": categoryWindows, "maximize": categoryWindows,
	"resizeactive": categoryWindows, "movewindow": categoryWindows, "resizewindow": categoryWindows, "focuslast": categoryWindows,
	"focusurgent": categoryWindows, "cycle": categoryWindows, "switcher": categoryWindows, "showdesktop": categoryWindows,

	"workspace": categoryWorkspaces, "movetoworkspace": categoryWorkspaces, "focusoutput": categoryWorkspaces,
	"movetooutput": categoryWorkspaces, "moveworkspacetooutput": categoryWorkspaces, "swapworkspaces": categoryWorkspaces,
	"scratchpad": categoryWorkspaces, "movetoscratchpad": categoryWorkspaces, "overview": categoryWorkspaces,

	"makegroup": categoryLayout, "setlayout": categoryLayout, "columnwidth": categoryLayout, "consume": categoryLayout,
	"expel": categoryLayout, "center": categoryLayout, "columndisplay": categoryLayout, "changegroup": categoryLayout,
	"focustab": categoryLayout, "focusparent": categoryLayout, "focuschild": categoryLayout, "moveout": categoryLayout,
	"equalize": categoryLayout, "locktab": categoryLayout,

	"exec": categoryApps, "global": categoryApps,

	"casttarget": categoryScreenshots,

	"dpms": categorySystem, "reload": categorySystem, "dismissconfignotice": categorySystem, "exit": categorySystem,
	"nextlayout": categorySystem, "stickykeys": categorySystem, "slowkeys": categorySystem, "submap": categorySystem,
	"zoom": categorySystem, "debug": categorySystem,
}

var (
	mediaKeyPrefixes   = []string{"xf86audio", "xf86monbrightness", "xf86kbdbrightness"}
	mediaPrograms      = []string{"wpctl", "pactl", "pamixer", "amixer", "playerctl", "brightnessctl", "light"}
	screenshotKeys     = []string{"print"}
	screenshotPrograms = []string{"grim", "slurp", "grimblast", "hyprshot", "flameshot", "satty", "swappy", "wayshot"}
)

const screenshotWord = "screenshot"

// category is what the bind is for: its action's, an app's bind refined
// by its key or program to screenshots or media; an action lys-wm does not
// know is custom.
func (n bindNames) category(item map[string]any) bindCategory {
	text, _ := item[actionKey].(string)
	a := splitAction(text)
	if !n.known(a.name) {
		return categoryCustom
	}
	c, ok := actionCategories[a.name]
	if !ok {
		log.Panicf("settings: lys-wm's action %s has no category", a.name)
	}
	if c != categoryApps {
		return c
	}
	chord, _ := item[chordKey].(string)
	key := ""
	if c, err := keys.WM.Parse(chord); err == nil {
		if k, ok := c.Trigger.(keys.Key); ok {
			key = strings.ToLower(xkb.KeysymGetName(k.Sym))
		}
	}
	program := ""
	if a.name == execAction && len(a.args) > 0 {
		program = a.args[0][strings.LastIndexByte(a.args[0], '/')+1:]
	}
	switch {
	case slices.ContainsFunc(mediaKeyPrefixes, func(p string) bool { return strings.HasPrefix(key, p) }), slices.Contains(mediaPrograms, program):
		return categoryMedia
	case slices.Contains(screenshotKeys, key), slices.Contains(screenshotPrograms, program),
		strings.Contains(strings.ToLower(a.argText()), screenshotWord):
		return categoryScreenshots
	}
	return categoryApps
}

func (n bindNames) group(item map[string]any) cardGroupKey {
	c := n.category(item)
	return cardGroupKey{rank: int(c), label: c.label()}
}

// searchText is what a bind is found by: what it does, its category, its
// action as written and its chord, as typed and as it reads.
func (n bindNames) searchText(item map[string]any) string {
	chord, _ := item[chordKey].(string)
	return strings.Join([]string{n.bindLabel(item, 0), n.category(item).label(), itemText(item), keys.WM.Text(chord)}, " ")
}
