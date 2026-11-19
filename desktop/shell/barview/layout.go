package barview

import (
	"fmt"
	"math"
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/styling"
)

// FindLayout resolves the layout for one output's connector name: the
// exact monitor match wins, then "*" as the fallback, and extends
// chains inherit the unset sections of their parent layout. A monitor
// with neither an exact entry nor a "*" entry gets no bar.
func FindLayout(layouts []config.BarLayout, connector string) (config.BarLayout, bool) {
	if layout, ok := findByMonitor(layouts, connector); ok {
		return mergeParent(layout, layouts, map[string]bool{}), true
	}
	if layout, ok := findByMonitor(layouts, AllMonitors); ok {
		return mergeParent(layout, layouts, map[string]bool{}), true
	}
	return config.BarLayout{}, false
}

// AllMonitors is the monitor a layout names to cover every output.
const AllMonitors = "*"

// LayoutFor is the output's layout and whether it shows a bar.
func LayoutFor(cfg *config.Config, connector string) (config.BarLayout, bool) {
	layout, ok := FindLayout(cfg.Bar.Layout, connector)
	return layout, ok && layout.Show
}

func findByMonitor(layouts []config.BarLayout, monitor string) (config.BarLayout, bool) {
	for _, candidate := range layouts {
		if candidate.Monitor == monitor {
			return candidate, true
		}
	}
	return config.BarLayout{}, false
}

func mergeParent(layout config.BarLayout, all []config.BarLayout, visited map[string]bool) config.BarLayout {
	resolved := layout
	extends, ok := resolved.ExtendsName()
	if !ok {
		return resolved
	}
	if visited[extends] {
		return resolved
	}
	visited[extends] = true
	parent, ok := findByMonitor(all, extends)
	if !ok {
		return resolved
	}
	parent = mergeParent(parent, all, visited)
	if len(resolved.Left) == 0 {
		resolved.Left = parent.Left
	}
	if len(resolved.Center) == 0 {
		resolved.Center = parent.Center
	}
	if len(resolved.Right) == 0 {
		resolved.Right = parent.Right
	}
	if resolved.AutoHide == nil {
		resolved.AutoHide = parent.AutoHide
	}
	return resolved
}

// Edge is the lys-ui edge a bar at location runs along.
func Edge(location config.Location) widget.BarEdge {
	switch location {
	case config.LocationTop:
		return widget.BarTop
	case config.LocationBottom:
		return widget.BarBottom
	case config.LocationLeft:
		return widget.BarLeft
	case config.LocationRight:
		return widget.BarRight
	}
	panic(fmt.Sprintf("barview: location %q has no edge", location))
}

// Floating reports whether an inset pulls the bar off its edges: the
// floating class, which the bar carries beside its element, edge and
// connector.
func Floating(cfg *config.Config) bool {
	return !cfg.Bar.InsetEdge.IsZero() || !cfg.Bar.InsetEnds.IsZero()
}

// ModuleAppender adds one module of a bar item to the item's box.
type ModuleAppender func(row *widget.Box, item config.BarItem)

// NewBar assembles one bar on lys-ui's Bar, so the bar CSS styles it:
//
//	bar.<edge>.<connector>[.floating]   (the --bar-* variables inline)
//	├ box.start
//	├ box.center
//	└ box.end
//
// The bar's CSS margins carry the insets and the shadow margin, its
// border, background and padding the bar chrome; each zone spaces its
// items by the module gap. An empty connector adds no class.
func NewBar(cfg *config.Config, layout config.BarLayout, connector string, module ModuleAppender) *widget.Bar {
	bar := widget.NewBar(Edge(cfg.Bar.Location))
	if connector != "" {
		bar.AddClass(connector)
	}
	Fill(bar, cfg, layout, module)
	return bar
}

// Fill gives bar the config-derived parts of NewBar: its floating
// class, the --bar-* variables, and the zones' modules, which replace
// the ones it held. The edge is the window's to move.
func Fill(bar *widget.Bar, cfg *config.Config, layout config.BarLayout, module ModuleAppender) {
	bar.SetClass("floating", Floating(cfg))
	bar.SetInlineStyle(InlineDecls(styling.BarCSS(cfg.Bar, cfg.Styling)))
	for _, z := range []struct {
		box   *widget.Box
		items []config.BarItem
	}{{bar.Start(), layout.Left}, {bar.Center(), layout.Center}, {bar.End(), layout.Right}} {
		z.box.Clear()
		FillZone(z.box, cfg, z.items, module)
	}
}

// FillZone builds every item of one layout section into a bar zone:
// one box.bar-item per layout item, a group's carrying the bar-group
// class and the group name as its id, module adding each module inside.
// The zone spaces its items by the module gap and a group its modules
// by the group module gap: box gaps, not margins.
func FillZone(zone *widget.Box, cfg *config.Config, items []config.BarItem, module ModuleAppender) {
	axis := widget.Row
	if cfg.Bar.Location.IsVertical() {
		axis = widget.Column
	}
	zone.SetSpacing(ModuleGap(cfg))
	for _, item := range items {
		box := widget.NewBox(axis, 0, 0)
		box.AddClass("bar-item")
		modules := []config.BarItem{item}
		if item.IsGroup() {
			box.SetSpacing(GroupGap(cfg))
			box.AddClass("bar-group")
			box.SetID(item.Group.Name)
			modules = item.Group.Modules
		}
		for _, inner := range modules {
			module(box, inner)
		}
		zone.Append(box, false)
	}
}

// ModuleBox is the box a button module sits in: `module`, the module's
// own type class (the layout name and any classes the module adds), and
// the layout entry's instance class, so ancestor selectors like
// `.media-disc button.bar-button ...` reach the button.
func ModuleBox(item config.BarItem, classes ...string) *widget.Box {
	wrap := widget.NewBox(widget.Row, 0, 0)
	wrap.AddClass("module", string(item.Module))
	wrap.AddClass(classes...)
	if item.Class != "" {
		wrap.AddClass(item.Class)
	}
	return wrap
}

// OwnChrome marks the root of a module that is not a bar button (the
// workspace rows, the separator): the module and instance classes and
// the module's title as its accessible name.
func OwnChrome(root widget.Widget, item config.BarItem, cfg *config.Config) {
	if n, ok := root.(interface{ AddClass(...string) }); ok {
		n.AddClass("module", item.Class)
	}
	if n, ok := root.(interface{ SetAccessibleName(string) }); ok {
		n.SetAccessibleName(ModuleTitle(string(item.Module), cfg))
	}
}

// CustomDefinition is the [modules.custom] definition a custom-<id>
// layout name refers to.
func CustomDefinition(name string, cfg *config.Config) (config.CustomModuleDefinition, bool) {
	const prefix = "custom-"
	if !strings.HasPrefix(name, prefix) {
		return config.CustomModuleDefinition{}, false
	}
	return cfg.CustomByID(name[len(prefix):])
}

// ModuleButton is the bar-button view a layout item's module carries;
// a module without the key set (or no config) gets the neutral set.
func ModuleButton(name string, cfg *config.Config) config.ButtonConfig {
	if cfg != nil {
		if b, ok := cfg.ModuleButton(name); ok {
			return b
		}
		if def, ok := CustomDefinition(name, cfg); ok {
			return def.Button()
		}
	}
	return config.ButtonConfig{IconShow: true, LabelShow: true, Tooltip: true}
}

// ModuleTitle is the name a module's button gives assistive technology
// and its tooltip: bar-module-<name> translated, a custom module's id.
func ModuleTitle(name string, cfg *config.Config) string {
	if cfg != nil {
		if def, ok := CustomDefinition(name, cfg); ok {
			return def.Id
		}
	}
	return i18n.T("bar-module-" + name)
}

// ModuleGap spaces the items of a bar zone, in pixels.
func ModuleGap(cfg *config.Config) int {
	return int(math.Round(cfg.Bar.ModuleGap.ResolvePx(styling.RemBase, float64(cfg.Bar.Scale))))
}

// GroupGap spaces the modules of a group, in pixels.
func GroupGap(cfg *config.Config) int {
	return int(math.Round(cfg.Bar.ButtonGroupModuleGap.ResolvePx(styling.RemBase, float64(cfg.Bar.Scale))))
}

// LabelPx is the module label size: pixels are literal; multipliers
// scale the stylesheet base (--bar-btn-label-size: 1.04rem) by the bar
// scale and the configured multiplier.
func LabelPx(cfg *config.Config) float64 {
	size := cfg.Bar.ButtonLabelSize
	if size.Unit == config.SizePixels {
		return float64(size.Value)
	}
	return buttonLabelBaseRem * styling.RemBase * float64(cfg.Bar.Scale) * float64(size.Value)
}

const buttonLabelBaseRem = 1.04

// Font is the bar's label face: the sans font at LabelPx, with the
// fallback chain behind it.
func Font(cfg *config.Config) (render.Font, error) {
	face, err := app.Font(cfg.General.FontSans, LabelPx(cfg))
	if err != nil {
		return nil, fmt.Errorf("bar: font %q: %w", cfg.General.FontSans, err)
	}
	return app.FontFallback(face), nil
}
