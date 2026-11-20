package settings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

func openRule(t *testing.T, c *cardList, i int) (*ruleMatchEditor, *ruleEffects) {
	t.Helper()
	treetest.WithClass(c.cards[i].root, "card-disclose")[0].(*widget.Button).ClickAt(widget.Point{})
	if len(c.cards[i].controls) != 2 {
		t.Fatalf("an open rule holds %d parts, want what it applies to and its effects", len(c.cards[i].controls))
	}
	return c.cards[i].controls[0].(*ruleMatchEditor), c.cards[i].controls[1].(*ruleEffects)
}

func chipTexts(c cardAside) []string {
	var out []string
	for _, chip := range treetest.All[*widget.Chip](c) {
		out = append(out, chip.Text())
	}
	return out
}

func TestEveryKeyOfALysWMRuleHasItsPartInTheRuleEditor(t *testing.T) {
	k := lysWMKit(t, "")
	for _, kind := range []ruleKind{windowRules, layerRules} {
		keys, ok := k.wm.schema.Keys([]string{kind.path, wmconfig.Each})
		if !ok {
			t.Fatalf("lys-wm's %s holds no rules", kind.path)
		}
		for key := range kind.groups {
			if !slices.Contains(keys, key) {
				t.Errorf("%s: %s has a part in the rule editor but is no key of lys-wm's", kind.path, key)
			}
		}
		parts := splitRuleKeys(k.wm, kind)
		if 1+len(parts.match)+len(parts.more)+len(parts.effects) != len(keys) || len(parts.effects) == 0 || parts.toggle.key != ruleEnabledKey {
			t.Errorf("%s: %d keys split as %+v", kind.path, len(keys), parts)
		}
		for _, key := range parts.effects {
			if label := effectLabel(key); label == wmKey(rulesLabels+"."+key) {
				t.Errorf("%s: the effect %s has no name", kind.path, key)
			}
		}
	}
}

func TestARuleShowsOnlyItsEffectsAndAddsOneFromTheMenu(t *testing.T) {
	k, w, _, files := wmKit(t, "rules:\n  - {app_id: foot}\n")
	c := fieldControl(k, w, wmRules(windowRules)).(*cardList)
	if got := chipTexts(c.cards[0].aside); len(got) != 0 {
		t.Fatalf("a rule with no effect shows the chips %v", got)
	}
	_, effects := openRule(t, c, 0)
	if !effects.none.Visible() || len(effects.shown) != 0 {
		t.Fatalf("a rule with no effect shows %v", effects.shown)
	}
	offered := effects.offered()
	values := make([]string, len(offered))
	for i, o := range offered {
		values[i] = o.value
	}
	if !slices.Equal(values, []string{"float", "animation", "blur_look"}) {
		t.Fatalf("the add menu offers %v, want the effects by group and never what the rule applies to", values)
	}
	if want := locale.T().Get("wm-rules-group-placement"); offered[0].group != want {
		t.Errorf("float is offered under %q, want %q", offered[0].group, want)
	}
	effects.addEffect("float")
	if got := fileText(t, files.runtime); !strings.Contains(got, "float: true") || !strings.Contains(got, "app_id: foot") {
		t.Fatalf("adding float wrote %q", got)
	}
	if !slices.Equal(effects.shown, []string{"float"}) || effects.none.Visible() {
		t.Errorf("after adding float the rule shows %v", effects.shown)
	}
	if got, want := chipTexts(c.cards[0].aside), []string{locale.T().Get("wm-rules-float")}; !slices.Equal(got, want) {
		t.Errorf("the card's chips read %v, want %v", got, want)
	}
	before := fileText(t, files.runtime)
	effects.addEffect("blur_look")
	if got := fileText(t, files.runtime); got != before {
		t.Errorf("adding an effect with no value yet wrote %q", got)
	}
	if !slices.Equal(effects.shown, []string{"blur_look", "float"}) {
		t.Errorf("an effect added without a value is not shown: %v", effects.shown)
	}
	effects.removeEffect("float")
	if got := fileText(t, files.runtime); strings.Contains(got, "float") || !strings.Contains(got, "app_id: foot") {
		t.Errorf("removing float wrote %q", got)
	}
	effects.removeEffect("blur_look")
	if len(effects.shown) != 0 || !effects.none.Visible() {
		t.Errorf("after removing both the rule shows %v", effects.shown)
	}
}

func TestTheRuleEditorKeepsWhatItDoesNotUnderstand(t *testing.T) {
	k, w, _, files := wmKit(t, "rules:\n  - {app_id: '^(foot|kitty)$', future: 3}\n")
	c := fieldControl(k, w, wmRules(windowRules)).(*cardList)
	if got := c.cards[0].title.Text(); got != "^(foot|kitty)$" {
		t.Errorf("a rule over two apps is titled %q, want its pattern", got)
	}
	_, effects := openRule(t, c, 0)
	effects.addEffect("float")
	got := fileText(t, files.runtime)
	for _, want := range []string{"^(foot|kitty)$", "future: 3", "float: true"} {
		if !strings.Contains(got, want) {
			t.Errorf("the edited rule lost %q: %q", want, got)
		}
	}
}

func TestFurtherConditionsShowWhenARuleHasOne(t *testing.T) {
	k := lysWMKit(t, "rules:\n  - {app_id: foot}\n  - {app_id: foot, is_floating: true}\n")
	c := fieldControl(k, k.wm, wmRules(windowRules)).(*cardList)
	plain, _ := openRule(t, c, 0)
	floating, _ := openRule(t, c, 1)
	if plain.more.Visible() {
		t.Error("a rule matching its app alone opens the further conditions")
	}
	if !floating.more.Visible() {
		t.Error("a rule matching floating windows hides that condition")
	}
	plain.toggle.OnClick()
	if !plain.more.Visible() || plain.label.Text() != locale.T().Get("wm-rules-fewer-conditions") {
		t.Errorf("the toggle shows the conditions %t and reads %q", plain.more.Visible(), plain.label.Text())
	}
}

func TestALayerRuleNamesTheShellSurfaceItAppliesTo(t *testing.T) {
	k := lysWMKit(t, "layer_rules:\n  - {namespace: '^lys-shell-bar-', blur: true}\n  - {namespace: '^my-panel$'}\n")
	c := fieldControl(k, k.wm, wmRules(layerRules)).(*cardList)
	if got, want := c.cards[0].title.Text(), locale.T().Get("wm-layer-lys-shell-bar"); got != want {
		t.Errorf("a rule over the bar is titled %q, want %q", got, want)
	}
	if got := c.cards[1].title.Text(); got != "^my-panel$" {
		t.Errorf("a rule over a surface lys-shell does not map is titled %q, want its pattern", got)
	}
	if got, want := chipTexts(c.cards[0].aside), []string{locale.T().Get("wm-rules-blur")}; !slices.Equal(got, want) {
		t.Errorf("the bar's rule shows the chips %v, want %v", got, want)
	}
	choices := layerChoices()
	if len(choices) != len(layers.All) || choices[0].value != layers.Bar.Pattern() {
		t.Fatalf("the surfaces offered are %v", choices)
	}
	for _, ch := range choices {
		if strings.HasPrefix(ch.label, "wm-layer-") {
			t.Errorf("the surface %s has no name", ch.value)
		}
	}
	_, effects := openRule(t, c, 1)
	if slices.ContainsFunc(effects.offered(), func(ch choice) bool { return ch.value == namespaceKey }) {
		t.Error("the add menu offers the namespace a rule applies to")
	}
	effects.addEffect("shadow")
	got, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lys-wm", "runtime.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "shadow: true") || !strings.Contains(string(got), "^my-panel$") || !strings.Contains(string(got), "blur: true") {
		t.Errorf("adding a shadow to the second rule wrote %q", got)
	}
}

func TestARuleCardSwitchesItsRuleOffAndBackOn(t *testing.T) {
	k, w, _, files := wmKit(t, "rules:\n  - {app_id: foot, float: true}\n  - {app_id: mpv, enabled: false}\n")
	c := fieldControl(k, w, wmRules(windowRules)).(*cardList)
	foot, mpv := c.cards[0], c.cards[1]
	if !foot.toggle.On() || foot.title.HasClass(css.DimLabel) {
		t.Fatal("a rule without enabled reads as switched off")
	}
	if mpv.toggle.On() || !mpv.title.HasClass(css.DimLabel) {
		t.Fatal("a rule with enabled: false reads as switched on")
	}
	if got, want := foot.toggle.AccessibleName(), locale.T().Get("wm-rules-enabled"); got != want || strings.HasPrefix(got, "wm-") {
		t.Errorf("the switch is named %q, want %q", got, want)
	}
	foot.toggle.SetOn(false)
	if got := fileText(t, files.runtime); !strings.Contains(got, "enabled: false") || !strings.Contains(got, "float: true") {
		t.Fatalf("switching the rule off wrote %q", got)
	}
	if !foot.title.HasClass(css.DimLabel) {
		t.Error("a rule switched off keeps its title bright")
	}
	foot.toggle.SetOn(true)
	mpv.toggle.SetOn(true)
	if got := fileText(t, files.runtime); strings.Contains(got, "enabled") || !strings.Contains(got, "app_id: mpv") {
		t.Errorf("switching the rules on wrote %q, want enabled left out as lys-wm's default", got)
	}
	if foot.title.HasClass(css.DimLabel) || mpv.title.HasClass(css.DimLabel) {
		t.Error("a rule switched back on keeps its title dimmed")
	}
}

func TestALayerRuleCardSwitchesItsRuleOff(t *testing.T) {
	k := lysWMKit(t, "layer_rules:\n  - {namespace: '^lys-shell-bar-', blur: true}\n")
	c := fieldControl(k, k.wm, wmRules(layerRules)).(*cardList)
	c.cards[0].toggle.SetOn(false)
	got, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lys-wm", "runtime.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "enabled: false") || !strings.Contains(string(got), "blur: true") {
		t.Errorf("switching the bar's rule off wrote %q", got)
	}
}
