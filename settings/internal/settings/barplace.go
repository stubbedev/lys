package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const barLayoutPath = "bar.layout"

func layoutsHold(layouts []config.BarLayout, m config.BarModule) bool {
	holds := func(items []config.BarItem) bool {
		return slices.ContainsFunc(items, func(it config.BarItem) bool {
			return it.Module == m || it.Group != nil && slices.ContainsFunc(it.Group.Modules, func(g config.BarItem) bool { return g.Module == m })
		})
	}
	return slices.ContainsFunc(layouts, func(l config.BarLayout) bool { return holds(l.Left) || holds(l.Center) || holds(l.Right) })
}

func withModuleAdded(layouts []config.BarLayout, m config.BarModule) []config.BarLayout {
	out := slices.Clone(layouts)
	for i := range out {
		out[i].Left, out[i].Center, out[i].Right = cloneItems(out[i].Left), cloneItems(out[i].Center), cloneItems(out[i].Right)
	}
	at := slices.IndexFunc(out, func(l config.BarLayout) bool { return l.Monitor == "*" })
	if at < 0 && len(out) > 0 {
		at = 0
	}
	if at < 0 {
		return append(out, config.BarLayout{Monitor: "*", Show: true, Right: []config.BarItem{{Module: m}}})
	}
	out[at].Right = append(out[at].Right, config.BarItem{Module: m})
	return out
}

type placementRow struct {
	*widget.Box
	k       *kit
	module  barModuleOf
	status  *widget.Label
	add     *widget.Button
	problem *inlineState
}

func newPlacementRow(k *kit, module barModuleOf) *placementRow {
	p := &placementRow{Box: gapBox(widget.Column), k: k, module: module}
	p.AddClass("settings-bar-placement")
	p.status = k.label("", "settings-bar-placement-status", css.CaptionHeading, css.DimLabel)
	p.add = k.listAddButton("settings-bar-placement-add", p.place)
	p.problem = k.inlineState()
	line := gapBox(widget.Row)
	line.AppendAligned(p.status, true, widget.AlignCenter)
	line.AppendAligned(p.add, false, widget.AlignCenter)
	p.Append(line, false)
	p.Append(p.problem, false)
	p.refresh()
	return p
}

func (p *placementRow) place() {
	m, ok := p.module()
	if !ok {
		return
	}
	err := p.k.store.set(barLayoutPath, config.Encode(withModuleAdded(p.k.store.svc.Config().Bar.Layout, m)))
	p.problem.Result(locale.T().Get("settings-bar-placement-failed"), err)
	if err == nil {
		p.refresh()
	}
}

func (p *placementRow) refresh() {
	m, ok := p.module()
	p.SetVisible(ok)
	if !ok {
		return
	}
	in := layoutsHold(p.k.store.svc.Config().Bar.Layout, m)
	key := "settings-bar-placement-absent"
	if in {
		key = "settings-bar-placement-present"
	}
	p.status.SetText(locale.T().Get(key))
	p.add.SetVisible(!in)
}

// barItemHead is what a bar item opens with: whether its module is in
// the bar, with Add to Bar, over its preview, and a note above both set
// apart as a section is. A page's head follows the config, and a custom
// module card's follows its card.
type barItemHead struct {
	*widget.Box
	placement *placementRow
	preview   previewWidget
}

func newBarItemHead(k *kit, module barModuleOf, preview previewWidget) *barItemHead {
	h := &barItemHead{Box: sectionStack(), placement: newPlacementRow(k, module), preview: preview}
	h.AddClass("settings-bar-item-head")
	shown := gapBox(widget.Column)
	shown.Append(h.placement, false)
	shown.Append(preview, false)
	h.Append(shown, false)
	return h
}

// noted puts note above the head.
func (h *barItemHead) noted(note widget.Widget) *barItemHead {
	h.InsertAt(0, note, false)
	return h
}

func (h *barItemHead) configChanged() {
	h.placement.refresh()
	h.preview.configChanged()
}

func (h *barItemHead) refresh() { h.configChanged() }
