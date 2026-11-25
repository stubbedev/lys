package settings

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// relatedLinks are every link of p's Related section, its groups'
// one after another.
func relatedLinks(p pageSpec) []pageLink {
	var links []pageLink
	for _, g := range p.relatedGroups() {
		links = append(links, g.links...)
	}
	return links
}

// relatedOpens are the pages the Related section of groups opens, one
// click on each of its links in turn, with k's layout.
func relatedOpens(k *kit, groups []linkGroup) []string {
	saved := k.openPage
	defer func() { k.openPage = saved }()
	var opened []string
	k.openPage = func(id string) { opened = append(opened, id) }
	section, ok := relatedSection(k, groups)
	if !ok {
		return nil
	}
	for _, l := range treetest.All[*pageLinkRow](section) {
		l.ClickAt(widget.Point{})
	}
	return opened
}

// unregisteredLinks names each link of the pages, in their Related
// sections or their own sections, to an id no layout holds, with the
// page it is on.
func unregisteredLinks(pages []pageSpec) error {
	var errs []error
	for _, p := range pages {
		links := relatedLinks(p)
		for _, s := range p.sections {
			links = append(links, s.links...)
		}
		for _, l := range links {
			if !registeredPage(l.page) {
				errs = append(errs, fmt.Errorf("the %s page links to %q, which is no page", p.id, l.page))
			}
		}
	}
	return errors.Join(errs...)
}

func everyPage() []pageSpec {
	var pages []pageSpec
	for _, s := range layout(config.Defaults(), bothParts) {
		pages = append(pages, s.pages...)
	}
	return pages
}

func TestEveryLinkNamesARegisteredPage(t *testing.T) {
	if err := unregisteredLinks(everyPage()); err != nil {
		t.Error(err)
	}
	stray := pageSpec{id: "stray", related: linksTo(pageLink{"no-such-page", "settings-link-surface-animation"})}
	err := unregisteredLinks([]pageSpec{stray})
	if err == nil || !strings.Contains(err.Error(), "stray") || !strings.Contains(err.Error(), `"no-such-page"`) {
		t.Errorf("a link to no page reads %v, want the page and the id named", err)
	}
	if !registeredPage(string(config.ModuleTreeman)) {
		t.Error("the optional treeman page is not registered")
	}
}

func TestALinkSkipsAPageTheInstalledPartsLeaveOutAndRefusesNoPage(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(config.Defaults(), parts{shell: true})
	if _, ok := k.pageLinkRow(pageLink{"wm-keyboard", "settings-region-link-keyboard-detail"}); ok {
		t.Error("without lys-wm a link to its keyboard page still shows")
	}
	k.nav = layout(config.Defaults(), bothParts)
	if row, ok := k.pageLinkRow(pageLink{"wm-keyboard", "settings-region-link-keyboard-detail"}); !ok || row.page != "wm-keyboard" {
		t.Errorf("with lys-wm the link to its keyboard page is %v, %v", row, ok)
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), `"no-such-page"`) {
			t.Errorf("a link to no page panics with %v, want the id named", r)
		}
	}()
	k.pageLinkRow(pageLink{"no-such-page", "settings-link-surface-animation"})
}

func TestOnlyThePagesAnimatingASurfaceLinkToMotionEachOnce(t *testing.T) {
	surfaces := animationSurfaces()
	owners := map[string]string{}
	for _, p := range everyPage() {
		motion := 0
		for _, l := range relatedLinks(p) {
			if l.page == motionPageID {
				motion++
			}
		}
		switch {
		case p.animates == "" && motion > 0:
			t.Errorf("the %s page animates no surface but links to Motion", p.id)
		case p.animates != "" && motion != 1:
			t.Errorf("the %s page animates %s but links to Motion %d times", p.id, p.animates, motion)
		}
		if p.animates == "" {
			continue
		}
		if !slices.Contains(surfaces, p.animates) {
			t.Errorf("the %s page animates %s, which is none of %v", p.id, p.animates, surfaces)
		}
		if other, ok := owners[p.animates]; ok {
			t.Errorf("%s is animated by both the %s and the %s page", p.animates, other, p.id)
		}
		owners[p.animates] = p.id
	}
	for _, want := range []string{barPageID, "notifications", "osd", "launcher", "wallpaper", "lock", "privacy"} {
		if !slices.Contains(slices.Collect(maps.Values(owners)), want) {
			t.Errorf("the %s page animates no surface", want)
		}
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "animations.nowhere") {
			t.Errorf("a page animating no surface the config has panics with %v", r)
		}
	}()
	pageSpec{id: "stray", animates: "animations.nowhere"}.relatedGroups()
}

func TestATitledLinkGroupFollowsTheUntitledOneInsideOneSection(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(config.Defaults(), bothParts)
	groups := []linkGroup{
		{title: "settings-lock-section-related", links: []pageLink{{"idle", "settings-lock-link-idle-detail"}}},
	}
	p := pageSpec{id: "stray", related: groups, animates: "animations.lock"}
	section, ok := relatedSection(k, p.relatedGroups())
	if !ok {
		t.Fatal("no Related section")
	}
	tr := locale.T()
	want := []string{tr.Get("settings-section-related"), tr.Get("settings-nav-motion"), tr.Get("settings-link-surface-animation"), tr.Get("settings-lock-section-related"), tr.Get("settings-nav-idle"), tr.Get("settings-lock-link-idle-detail")}
	if got := slices.DeleteFunc(treetest.Labels(section), func(s string) bool { return s == "" }); !slices.Equal(got, want) {
		t.Errorf("the section reads %q, want %q", got, want)
	}
	k.nav = slices.DeleteFunc(k.nav, func(s navSection) bool {
		return slices.ContainsFunc(s.pages, func(p pageSpec) bool { return p.id == "idle" })
	})
	section, _ = relatedSection(k, p.relatedGroups())
	if heads := treetest.WithClass(section, "settings-link-group"); len(heads) != 0 {
		t.Errorf("a group none of whose pages is listed keeps its heading %q", treetest.Labels(heads[0]))
	}
}

// oneRelatedSectionLast holds every page to one section of links to
// other pages, titled Related and last, linking to no page twice. A
// section of the page's spec may link where its setting lives instead
// (a module's Thresholds to the shared warning levels).
func oneRelatedSectionLast() pageCheck {
	tr := locale.T()
	related := tr.Get("settings-section-related")
	return pageCheck{visit: func(t *testing.T, w *window) {
		spec, _ := pageByID(w.sections, w.pageID)
		linkingSpec := map[string]bool{}
		for _, s := range spec.sections {
			if len(s.links) > 0 {
				linkingSpec[tr.Get(s.title)] = true
			}
		}
		sections := treetest.WithClass(w.page, "settings-section")
		var linking []int
		for i, s := range sections {
			title := sectionTitle(s)
			if title == related || linksOnly(s) && !linkingSpec[title] {
				linking = append(linking, i)
			}
		}
		if len(linking) > 1 {
			t.Errorf("%s: %d sections of links to other pages, want one", w.pageID, len(linking))
		}
		if len(linking) == 1 && linking[0] != len(sections)-1 {
			t.Errorf("%s: the Related section is section %d of %d, not the last", w.pageID, linking[0]+1, len(sections))
		}
		seen := map[string]bool{}
		for _, r := range treetest.All[*pageLinkRow](w.page) {
			if seen[r.page] {
				t.Errorf("%s: links to %s twice", w.pageID, r.page)
			}
			seen[r.page] = true
		}
	}}
}

func sectionTitle(section widget.Widget) string {
	titles := treetest.WithClass(section, "settings-section-title")
	if len(titles) == 0 {
		return ""
	}
	if l, ok := titles[0].(*widget.Label); ok {
		return l.Text()
	}
	return ""
}

// linksOnly says whether every row of section links to another page.
func linksOnly(section widget.Widget) bool {
	rows := treetest.All[rowWidget](section)
	return len(rows) > 0 && !slices.ContainsFunc(rows, func(r rowWidget) bool {
		_, link := r.(*pageLinkRow)
		return !link
	})
}
