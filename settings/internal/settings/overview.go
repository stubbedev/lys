package settings

import (
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const overviewID = "overview"

const overviewColumns = 3

func overviewPage() pageSpec {
	return pageSpec{
		id: overviewID, navKey: "settings-nav-overview", icon: "ld-layout-grid-symbolic",
		header: "settings-page-overview", lead: overviewTiles,
	}
}

func overviewTiles(k *kit) widget.Widget {
	t := locale.T()
	th := widget.Current()
	grid := widget.NewFlowBox(th.Spacing, th.Spacing)
	grid.AddClass("settings-overview")
	grid.SetMaxChildrenPerLine(overviewColumns)
	grid.SetJustify(widget.JustifyFill)
	grid.SetSelectionMode(widget.SelectionNone)
	for _, s := range k.nav {
		if s.key == "settings-nav-home" || len(s.pages) == 0 {
			continue
		}
		first := s.pages[0].id
		face := widget.NewBox(widget.Column, th.Spacing/2, th.Padding)
		face.AppendAligned(k.icon(s.pages[0].icon, "settings-overview-icon"), false, widget.AlignStart)
		face.Append(k.label(t.Get(s.key), "settings-overview-title", css.Heading), false)
		face.Append(wrapped(k.label(t.Attr(s.key, "description"), "settings-overview-description", css.Caption, css.DimLabel)), false)
		tile := k.button(face, func() { k.openPage(first) }, css.Card, "settings-overview-tile")
		tile.SetAccessibleName(t.Get(s.key))
		grid.Append(tile)
	}
	return grid
}
