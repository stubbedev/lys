package settings

import (
	"log"
	"slices"
	"sync"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// pageLink is a link to another page: its id, and the message saying
// what the link finds there.
type pageLink struct{ page, detail string }

// linkGroup is a run of a page's related pages inside its one Related
// section. The untitled group leads it; a titled one follows under its
// heading.
type linkGroup struct {
	title string
	links []pageLink
}

// linksTo is the untitled group of links alone, no group without links.
func linksTo(links ...pageLink) []linkGroup {
	if len(links) == 0 {
		return nil
	}
	return []linkGroup{{links: links}}
}

// surfaceAnimationLink is the link every page showing an animated
// surface ends its untitled group with.
var surfaceAnimationLink = pageLink{motionPageID, "settings-link-surface-animation"}

// relatedGroups are what the page's Related section lists: its related
// groups, with the link to the Motion page in the untitled one while the
// page animates a surface. A surface the config does not animate is a
// page bug.
func (p pageSpec) relatedGroups() []linkGroup {
	if p.animates == "" {
		return p.related
	}
	if !slices.Contains(animationSurfaces(), p.animates) {
		log.Panicf("settings: the %s page animates %s, which is no animated surface", p.id, p.animates)
	}
	groups := slices.Clone(p.related)
	if len(groups) == 0 || groups[0].title != "" {
		groups = slices.Insert(groups, 0, linkGroup{})
	}
	groups[0].links = append(slices.Clone(groups[0].links), surfaceAnimationLink)
	return groups
}

// relatedSection is a page's one Related section: a card of rows per
// group, a row per linked page the layout holds, the untitled group's
// card first and each titled one's under its title, a caption over the
// card rather than a row inside it, held closer to its own card than to
// the one above. False while the layout holds none of them.
func relatedSection(k *kit, groups []linkGroup) (widget.Widget, bool) {
	t := locale.T()
	section := gapBox(widget.Column)
	section.AddClass("settings-section")
	section.Append(sectionHead(k, "settings-section-related"), false)
	linked := false
	for _, g := range groups {
		rows := widget.NewItemGroup()
		shown := false
		for _, l := range g.links {
			if row, ok := k.pageLinkRow(l); ok {
				rows.Append(row)
				shown = true
			}
		}
		if !shown {
			continue
		}
		linked = true
		if g.title == "" {
			section.Append(sectionGroup(rows), false)
			continue
		}
		th := widget.Current()
		titled := widget.NewBox(widget.Column, th.Spacing/2, 0)
		titled.SetPadding(render.Insets{Top: th.Spacing})
		titled.Append(k.label(t.Get(g.title), "settings-link-group", css.Caption, css.DimLabel), false)
		titled.Append(sectionGroup(rows), false)
		section.Append(titled, false)
	}
	if !linked {
		return nil, false
	}
	return section, true
}

// pageLinkRow is the row navigating to another page from within a page:
// titled by the page's sidebar entry over what the link finds there, its
// chevron opening it.
type pageLinkRow struct {
	*settingRow
	page string
}

// pageLinkRow is the row opening l's page, false while the layout leaves
// that page out.
func (k *kit) pageLinkRow(l pageLink) (*pageLinkRow, bool) {
	target, ok := k.linkTarget(l.page)
	if !ok {
		return nil, false
	}
	t := locale.T()
	return &pageLinkRow{k.chevronRow(t.Get(target.navKey), t.Get(l.detail), target.navKey, func() { k.openPage(target.id) }), target.id}, true
}

// pageButton is the link-role button reading text that opens the page
// id from within a line of text, false while the layout leaves that
// page out.
func (k *kit) pageButton(text, id string) (*widget.Button, bool) {
	target, ok := k.linkTarget(id)
	if !ok {
		return nil, false
	}
	return k.linkButton(text, func() { k.openPage(target.id) }), true
}

// chevronRow is a row titled title over detail that opens what it names,
// which the tooltipKey message says: the whole row is the link, its
// chevron on the control edge.
func (k *kit) chevronRow(title, detail, tooltipKey string, open func()) *settingRow {
	r := k.row(title, detail)
	r.SetTooltip(locale.T().Get(tooltipKey))
	r.SetOnActivate(open)
	return r.withControl(k.icon("ld-chevron-right-symbolic", "row-chevron"))
}

// linkTarget is the page a link to id opens. A registered page the
// layout leaves out (lys-wm's while it is not installed, lys-shell's
// while it is not) is skipped on purpose; an id no layout holds is a
// page bug.
func (k *kit) linkTarget(id string) (pageSpec, bool) {
	if target, ok := pageByID(k.nav, id); ok {
		return target, true
	}
	if !registeredPage(id) {
		log.Panicf("settings: a link names the page %q, which no layout holds", id)
	}
	return pageSpec{}, false
}

// registeredPage says whether id is a page some install shows: a page
// of the layout with lys-shell and lys-wm both installed, the optional
// pages (optionalpages.go) among them.
func registeredPage(id string) bool {
	registered.once.Do(func() {
		registered.ids = map[string]bool{}
		for _, s := range registeredLayout() {
			for _, p := range s.pages {
				registered.ids[p.id] = true
			}
		}
	})
	return registered.ids[id]
}

// registered are the ids registeredPage reads, gathered on its first
// call.
var registered struct {
	once sync.Once
	ids  map[string]bool
}

// registeredLayout lays out every page some install shows. init sets it:
// the pages it lays out link to pages themselves, which a variable
// initialised to it would make an initialisation cycle of.
var registeredLayout func() []navSection

func init() {
	registeredLayout = func() []navSection { return layout(config.Defaults(), parts{shell: true, wm: true}) }
}
