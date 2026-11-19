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
	k      *kit
	module func() (config.BarModule, bool)
	status *widget.Label
	add    *widget.Button
}

func newPlacementRow(k *kit, module func() (config.BarModule, bool)) *placementRow {
	p := &placementRow{Box: gapBox(widget.Row), k: k, module: module}
	p.AddClass("settings-bar-placement")
	p.status = k.label("", "settings-bar-placement-status", css.Caption)
	p.add = k.button(k.label(locale.T().Get("settings-bar-placement-add")), p.place, css.Small, css.SuggestedAction)
	p.AppendAligned(p.status, true, widget.AlignCenter)
	p.AppendAligned(p.add, false, widget.AlignCenter)
	p.refresh()
	return p
}

func (p *placementRow) place() {
	m, ok := p.module()
	if !ok {
		return
	}
	if err := p.k.store.set(barLayoutPath, config.Encode(withModuleAdded(p.k.store.svc.Config().Bar.Layout, m))); err != nil {
		p.status.SetText(err.Error())
		return
	}
	p.refresh()
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

func barPlacement(module config.BarModule) func(k *kit) widget.Widget {
	return func(k *kit) widget.Widget {
		return newPlacementRow(k, func() (config.BarModule, bool) { return module, true })
	}
}
