package settings

import (
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

// barItemParts are what a bar item page holds of its own, around what
// barItemPage gives every one.
type barItemParts struct {
	// note, when set, says what the item is, ahead of everything else.
	note func(k *kit) widget.Widget
	// preview, when set, stands in for the module's button preview:
	// the visualizer Cava draws, or a preview stepping through the
	// module's states.
	preview func(k *kit) previewWidget
	// sections are the page's own, after the preview.
	sections []sectionSpec
	// icons are the module's state icons, leading the Icons section.
	icons stateIcons
	// iconRows follow them there: the icon lists and maps, and the
	// choices of which icon shows.
	iconRows []rowSpec
	// thresholds edit the module's own thresholds. A module whose
	// section holds a thresholds key has them; one the shared warning
	// levels name links to those instead.
	thresholds []rowSpec
}

// previewWidget is a preview the page redraws as the config changes.
type previewWidget interface {
	widget.Widget
	configWatcher
}

// barItemPage is the page of a built-in bar module, in the order every
// bar item page shares: whether the module is in the bar, its preview,
// the page's own sections, the Icons, the Thresholds, the Bar Item
// section and the related pages. page names its sidebar entry.
func barItemPage(module config.BarModule, page pageSpec, parts barItemParts) pageSpec {
	preview := parts.preview
	if preview == nil {
		preview = func(k *kit) previewWidget { return barItemPreview(k, only(module)) }
	}
	page.lead = func(k *kit) widget.Widget {
		head := newBarItemHead(k, only(module), preview(k))
		if parts.note == nil {
			return head
		}
		return head.noted(parts.note(k))
	}
	page.sections = slices.Clone(parts.sections)
	if s, ok := iconsSection(parts.icons, parts.iconRows); ok {
		page.sections = append(page.sections, s)
	}
	if s, ok := thresholdsSection(module, parts.thresholds); ok {
		page.sections = append(page.sections, s)
	}
	if s := barItemSection(module); len(s.rows) > 0 {
		page.sections = append(page.sections, s)
	}
	page.related = linksTo(relatedTo(module)...)
	return asBarItemPage(page, func(m config.BarModule) bool { return m == module })
}

// asBarItemPage marks page as the bar item page of the modules sets
// holds for; its lead and footer act on its rows.
func asBarItemPage(page pageSpec, sets func(config.BarModule) bool) pageSpec {
	page.barItem = sets
	page.extrasOnRows = true
	return page
}

// only names m alone.
func only(m config.BarModule) barModuleOf {
	return func() (config.BarModule, bool) { return m, true }
}

// barModuleOf is the bar module an item names, while it names one.
type barModuleOf func() (config.BarModule, bool)

// stateIcons are the icons a bar item shows, one per state: columns
// headed by the states, in rows when it shows them for several kinds of
// thing (the network's wifi, wired and VPN).
type stateIcons struct {
	states []string
	rows   []gridRow
}

// iconPerState is an icon per path, each headed by its field's label.
func iconPerState(paths ...string) stateIcons {
	states := make([]string, len(paths))
	for i, p := range paths {
		states[i] = field(p).labelKey()
	}
	return stateIcons{states: states, rows: []gridRow{{cells: stateIconCells(paths...)}}}
}

// stateIconCells are an icon picker per path; an empty path leaves its cell
// empty.
func stateIconCells(paths ...string) []rowSpec { return gridCells(paths, iconRow) }

// row is the icons' one row: the lone icon's picker, or a grid of them
// under their states.
func (s stateIcons) row() (rowSpec, bool) {
	var cells []rowSpec
	for _, r := range s.rows {
		for _, c := range r.cells {
			if c.path != "" {
				cells = append(cells, c)
			}
		}
	}
	switch len(cells) {
	case 0:
		return rowSpec{}, false
	case 1:
		return cells[0], true
	}
	return cellGridRow("settings-state-icons", s.states, s.rows...), true
}

func iconsSection(icons stateIcons, more []rowSpec) (sectionSpec, bool) {
	var rows []rowSpec
	if r, ok := icons.row(); ok {
		rows = append(rows, r)
	}
	rows = append(rows, more...)
	return sectionSpec{title: "settings-section-icons", rows: rows}, len(rows) > 0
}

// thresholdsSection holds the module's own thresholds, or links to the
// shared warning levels it reads; which, the config's schema says.
func thresholdsSection(module config.BarModule, own []rowSpec) (sectionSpec, bool) {
	path := moduleSection(module) + ".thresholds"
	if hasKey(path) != slices.ContainsFunc(own, func(r rowSpec) bool { return r.path == path }) {
		log.Panicf("settings: the %s page edits thresholds %v, but the schema's %s exists: %v", module, own, path, hasKey(path))
	}
	s := sectionSpec{title: "settings-section-thresholds", rows: own}
	if hasKey(sharedThresholds(module)) {
		s.links = []pageLink{thresholdsLink}
	}
	return s, len(s.rows) > 0 || len(s.links) > 0
}

// sharedThresholds is the path of the warning levels a module shares
// with the dashboard.
func sharedThresholds(m config.BarModule) string { return "thresholds." + string(m) }

// relatedTo are the pages a bar module's page points to for what its
// bar item only shows.
func relatedTo(module config.BarModule) []pageLink {
	links := slices.Clone(moduleLinks[module])
	if hasDropdown(module) {
		links = append(links, dropdownSizesLink)
	}
	return links
}
