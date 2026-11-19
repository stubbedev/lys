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

func barPlacement(module config.BarModule) func(k *kit) widget.Widget {
	return func(k *kit) widget.Widget {
		t := locale.T()
		row := gapBox(widget.Row)
		row.AddClass("settings-bar-placement")
		status := k.label("", "settings-bar-placement-status", css.Caption)
		var add *widget.Button
		show := func() {
			in := layoutsHold(k.store.svc.Config().Bar.Layout, module)
			key := "settings-bar-placement-absent"
			if in {
				key = "settings-bar-placement-present"
			}
			status.SetText(t.Get(key))
			add.SetVisible(!in)
		}
		add = k.button(k.label(t.Get("settings-bar-placement-add")), func() {
			if err := k.store.set(barLayoutPath, config.Encode(withModuleAdded(k.store.svc.Config().Bar.Layout, module))); err != nil {
				status.SetText(err.Error())
				return
			}
			show()
		}, css.Small, css.SuggestedAction)
		row.AppendAligned(status, true, widget.AlignCenter)
		row.AppendAligned(add, false, widget.AlignCenter)
		show()
		return row
	}
}
