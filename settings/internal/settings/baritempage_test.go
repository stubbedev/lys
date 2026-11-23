package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// templateTail is the sections barItemPage closes every page with, in
// its order.
var templateTail = []string{"settings-section-icons", "settings-section-thresholds", "settings-section-bar-item"}

func barItemPagesOf(sections []navSection, m config.BarModule) []pageSpec {
	var out []pageSpec
	for _, s := range sections {
		for _, p := range s.pages {
			if p.barItem != nil && p.barItem(m) {
				out = append(out, p)
			}
		}
	}
	return out
}

// sectionPreviews counts the previews a page's own sections draw.
func sectionPreviews(k *kit, spec pageSpec) int {
	n := 0
	for _, s := range spec.sections {
		if s.preview != nil {
			n += len(treetest.All[*livePreview](s.preview(k)))
		}
	}
	return n
}

func checkTemplateOrder(t *testing.T, spec pageSpec) {
	t.Helper()
	var at []int
	for i, s := range spec.sections {
		if slices.Contains(templateTail, s.title) {
			at = append(at, i)
		}
	}
	last := -1
	for j, i := range at {
		if i != len(spec.sections)-len(at)+j {
			t.Errorf("the %s page's %s is not after its own sections", spec.id, spec.sections[i].title)
		}
		order := slices.Index(templateTail, spec.sections[i].title)
		if order <= last {
			t.Errorf("the %s page's %s is out of the template's order or shown twice", spec.id, spec.sections[i].title)
		}
		last = order
	}
}

func TestEveryBarModuleHasOnePageBuiltFromTheTemplate(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	for _, m := range config.BuiltinModules {
		pages := barItemPagesOf(k.nav, m)
		if len(pages) != 1 {
			t.Errorf("%s has %d bar item pages, want one", m, len(pages))
			continue
		}
		spec := pages[0]
		if _, ok := spec.lead(k).(*barItemHead); !ok {
			t.Errorf("the %s page does not open with its placement and preview", spec.id)
		}
		checkTemplateOrder(t, spec)
		p := buildPage(k, spec)
		if n := len(treetest.All[*barItemHead](p)); n != 1 {
			t.Errorf("the %s page shows %d heads, want one", spec.id, n)
		}
		if n := len(treetest.All[*placementRow](p)); n != 1 {
			t.Errorf("the %s page shows %d placement headers, want one", spec.id, n)
		}
		if n := len(treetest.All[*livePreview](p)) - sectionPreviews(k, spec); n != 1 {
			t.Errorf("the %s page shows %d previews of its bar item, want one", spec.id, n)
		}
		p.close()
	}
}

func TestEachCustomModuleCardOpensWithItsOwnPlacementAndPreview(t *testing.T) {
	k := testKit(t, gpuModule+"    - id: fan\n      command: echo 1\n")
	k.nav = layout(k.store.svc.Config(), bothParts)
	pages := barItemPagesOf(k.nav, config.CustomModule("gpu"))
	if len(pages) != 1 || pages[0].id != customModulesID {
		t.Fatalf("custom modules are set on %d pages, want the custom modules page", len(pages))
	}
	if pages[0].lead != nil {
		t.Error("the custom modules page opens with a placement of its own besides its cards'")
	}
	e := customEditor(t, k)
	for i, cd := range e.cards.cards {
		cd.disclose.setOpen(true)
		var heads []*barItemHead
		for _, ctl := range cd.controls {
			if h, ok := ctl.(*barItemHead); ok {
				heads = append(heads, h)
			}
		}
		if len(heads) != 1 {
			t.Errorf("card %d opens with %d heads, want one", i, len(heads))
			continue
		}
		if n := len(treetest.All[*livePreview](heads[0])); n != 1 {
			t.Errorf("card %d previews its module %d times", i, n)
		}
		if !heads[0].placement.Visible() {
			t.Errorf("card %d, which has an id, hides its placement", i)
		}
	}
}

func TestABarItemShowsOneStateIconAsARowAndSeveralAsAGrid(t *testing.T) {
	k := testKit(t, "")
	one, ok := iconPerState("modules.clock.icon-name").row()
	if _, isRow := fieldControl(k, k.store, one).(*iconEditor); !ok || !isRow {
		t.Error("a module's one state icon is not an icon row")
	}
	two, ok := iconPerState("modules.nightlight.icon-off", "modules.nightlight.icon-on").row()
	if _, isGrid := fieldControl(k, k.store, two).(*cellGrid); !ok || !isGrid {
		t.Error("a module's two state icons are not a grid")
	}
	if _, ok := (stateIcons{}).row(); ok {
		t.Error("a module with no state icon has an icon row")
	}
	k.nav = layout(k.store.svc.Config(), bothParts)
	for _, s := range k.nav {
		for _, spec := range s.pages {
			if spec.barItem == nil {
				continue
			}
			p := buildPage(k, spec)
			var stacked, grids int
			for _, r := range p.rows {
				switch r.ctl.(type) {
				case *iconEditor:
					stacked++
				case *cellGrid:
					grids++
				}
			}
			if stacked > 1 {
				t.Errorf("the %s page stacks %d icon rows, want its state icons in one grid", spec.id, stacked)
			}
			if grids > 0 && stacked > 0 {
				t.Errorf("the %s page shows its state icons in a grid and %d more in rows", spec.id, stacked)
			}
			p.close()
		}
	}
}

func TestACustomModuleCardEditsTheKeysABuiltInBarItemDoes(t *testing.T) {
	k := testKit(t, gpuModule)
	e := customEditor(t, k)
	cd := e.cards.cards[0]
	cd.disclose.setOpen(true)
	cardControl := func(key string) control {
		t.Helper()
		i := slices.IndexFunc(customCards.fields, func(f cardField) bool { return f.key == key })
		if i < 0 {
			t.Fatalf("the custom module card has no %s", key)
		}
		return cd.controls[i]
	}
	builtIn := func(key string) control {
		t.Helper()
		return fieldControl(k, k.store, barItemRow(t, config.ModuleBattery, key))
	}
	written := func(key string) any {
		t.Helper()
		doc, err := config.ParseYAML(runtimeFile(t, k))
		if err != nil {
			t.Fatalf("runtime.yaml: %v", err)
		}
		modules, _ := doc["modules"].(map[string]any)
		items, _ := modules["custom"].([]any)
		if len(items) != 1 {
			t.Fatalf("runtime.yaml holds %d custom modules", len(items))
		}
		item, _ := items[0].(map[string]any)
		return item[key]
	}

	cardStrip, ok := cardControl("icon-color").(*colorStrip)
	if !ok {
		t.Fatal("the card's colours are not the bar item's colour strip")
	}
	strip := builtIn("settings-bar-item-colors").(*colorStrip)
	labelColor := func(keys barItemKeys) int {
		parts := barItemSectionParts(keys, nil)
		at := slices.IndexFunc(parts, func(p barItemPart) bool { return p.label == "settings-bar-item-colors" })
		return slices.Index(parts[at].keys, "label-color")
	}
	if err := cardStrip.editors[labelColor(customItemKeys(customForm))].slot.set("#123456"); err != nil {
		t.Fatal(err)
	}
	if err := strip.editors[labelColor(moduleKeys("modules.battery"))].slot.set("#123456"); err != nil {
		t.Fatal(err)
	}
	if got := written("label-color"); got != "#123456" {
		t.Errorf("the card's label colour wrote %v to label-color", got)
	}
	if v, _ := runtimeValue(t, k, "modules.battery.label-color"); v != "#123456" {
		t.Errorf("the battery's label colour wrote %v to label-color", v)
	}

	cardMouse, ok := cardControl("left-click").(*mouseActions)
	if !ok {
		t.Fatal("the card's clicks are not the bar item's mouse actions")
	}
	mouse := builtIn("settings-bar-item-mouse").(*mouseActions)
	right := slices.Index(mouseButtons, "right-click")
	for _, m := range []*mouseActions{cardMouse, mouse} {
		if err := m.slots[right].set("notify-send right"); err != nil {
			t.Fatal(err)
		}
	}
	if got := written("right-click"); got != "notify-send right" {
		t.Errorf("the card's right button wrote %v to right-click", got)
	}
	if v, _ := runtimeValue(t, k, "modules.battery.right-click"); v != "notify-send right" {
		t.Errorf("the battery's right button wrote %v to right-click", v)
	}

	cardToggle, ok := cardControl("label-show").(*toggle)
	if !ok {
		t.Fatal("the card's label switch is not the bar item's toggle")
	}
	cardToggle.Toggle()
	toggled := fieldControl(k, k.store, field("modules.battery.label-show")).(*toggle)
	toggled.Toggle()
	if got := written("label-show"); got != false {
		t.Errorf("the card's label switch wrote %v to label-show", got)
	}
	if v, _ := runtimeValue(t, k, "modules.battery.label-show"); v != false {
		t.Errorf("the battery's label switch wrote %v to label-show", v)
	}
}

func TestTheCustomModuleCardHoldsEveryBarItemPartItsDefinitionHas(t *testing.T) {
	var keys []string
	for _, f := range customCards.fields {
		keys = append(keys, f.key)
	}
	parts := barItemSectionParts(customItemKeys(customForm), nil)
	for _, p := range parts {
		if !slices.Contains(keys, p.keys[0]) {
			t.Errorf("the custom module card lacks the bar item's %v", p.keys)
		}
	}
	for _, key := range []string{"icon-show", "label-show", "label-max-length", "border-show", "left-click", "icon-color"} {
		if !slices.ContainsFunc(parts, func(p barItemPart) bool { return p.keys[0] == key }) {
			t.Errorf("the custom modules' bar item parts lack %s", key)
		}
	}
	if slices.ContainsFunc(parts, func(p barItemPart) bool {
		return slices.Contains(p.keys, "color") || slices.Contains(p.keys, "tooltip")
	}) {
		t.Error("the custom modules' bar item parts edit a key their definition does not have")
	}
}

func TestThresholdsComeFromTheSchemaInlineOrAsALink(t *testing.T) {
	audio := thresholdsOf(t, "audio")
	if !slices.ContainsFunc(audio.rows, func(r rowSpec) bool { return r.path == "modules.audio.thresholds" }) || len(audio.links) != 0 {
		t.Errorf("audio's own thresholds are %v with links %v, want them inline alone", audio.rows, audio.links)
	}
	cpu := thresholdsOf(t, "cpu")
	if len(cpu.rows) != 0 || !slices.Equal(cpu.links, []pageLink{thresholdsLink}) {
		t.Errorf("the CPU's thresholds are %v with links %v, want a link to the warning levels", cpu.rows, cpu.links)
	}
	if _, ok := thresholdsSection(config.ModuleClock, nil); ok {
		t.Error("the clock, which reads no thresholds, has a Thresholds section")
	}
	defer func() {
		if recover() == nil {
			t.Error("a page leaving out the thresholds its schema has built")
		}
	}()
	thresholdsSection(config.ModuleAudio, nil)
}

func thresholdsOf(t *testing.T, page string) sectionSpec {
	t.Helper()
	spec, ok := pageByID(layout(config.Defaults(), bothParts), page)
	if !ok {
		t.Fatalf("no %s page", page)
	}
	i := slices.IndexFunc(spec.sections, func(s sectionSpec) bool { return s.title == "settings-section-thresholds" })
	if i < 0 {
		t.Fatalf("the %s page has no Thresholds section", page)
	}
	return spec.sections[i]
}
