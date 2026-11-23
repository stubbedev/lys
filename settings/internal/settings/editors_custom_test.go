package settings

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const gpuModule = "modules:\n  custom:\n    - id: gpu\n      command: echo 72\n"

func customEditor(t *testing.T, k *kit) *customModulesEditor {
	t.Helper()
	e, ok := fieldControl(k, k.store, pageRow(t, "custom", customModulesPath)).(*customModulesEditor)
	if !ok {
		t.Fatal("the custom modules are not edited as cards")
	}
	return e
}

func customItem(t *testing.T, k *kit, i int) map[string]any {
	t.Helper()
	items, _ := k.store.value(customModulesPath).([]any)
	if i >= len(items) {
		t.Fatalf("%d custom modules stored, want more than %d", len(items), i)
	}
	m, _ := items[i].(map[string]any)
	return m
}

func TestCustomModulesAreCardsThatWriteOnlyTheirField(t *testing.T) {
	k := testKit(t, gpuModule)
	before := k.store.value(customModulesPath)
	e := customEditor(t, k)
	if len(e.cards.cards) != 1 || !reflect.DeepEqual(k.store.value(customModulesPath), before) {
		t.Fatalf("%d cards; building them changed the modules", len(e.cards.cards))
	}
	if e.notice.Visible() || e.raw.Visible() || !e.toggle.Visible() {
		t.Errorf("plain modules show notice %v, text %v, toggle %v", e.notice.Visible(), e.raw.Visible(), e.toggle.Visible())
	}
	if err := e.cards.fieldSlot(0, "interval-ms").set(int64(1000)); err != nil {
		t.Fatal(err)
	}
	m := customItem(t, k, 0)
	if m["interval-ms"] != int64(1000) || m["command"] != "echo 72" || m["id"] != "gpu" {
		t.Errorf("stored %v", m)
	}
}

func TestACustomModuleCardShowsTheDefaultsOfWhatItLeavesOut(t *testing.T) {
	unset := slot{get: func() any { return nil }, set: func(any) error { return nil }, unset: func() {}}
	for key, want := range map[string]any{"icon-show": true, "mode": "poll", "interval-ms": int64(5000), "format": "{{ output }}"} {
		if got := customForm.shown(unset, key).get(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s shows %v (%T), want %v", key, got, got, want)
		}
	}
	set := slot{get: func() any { return false }}
	if got := customForm.shown(set, "icon-show").get(); got != false {
		t.Errorf("a stored value showed as %v", got)
	}
}

func TestEveryFieldOfADefaultCustomModuleIsOnItsCard(t *testing.T) {
	def := []any{config.Encode(config.DefaultsCustomModuleDefinition())}
	if beyond := beyondCards(def, customCards.editedKeys()); len(beyond) != 0 {
		t.Errorf("fields with values no card edits: %v", beyond)
	}
	if meta := customForm.meta("on-action"); meta.Type != "Command" {
		t.Errorf("on-action is a %s, not a command", meta.Type)
	}
	if _, ok := customForm.field("command").editor(testKit(t, ""), slot{get: func() any { return nil }}).(*choiceEditor); !ok {
		t.Error("the command is not picked from the command presets")
	}
}

func TestACustomModuleBeyondTheCardsShowsTheTextWithANotice(t *testing.T) {
	k := testKit(t, gpuModule+"      color-map:\n        warning:\n          icon-color: red\n")
	e := customEditor(t, k)
	if !e.notice.Visible() || !e.raw.Visible() || e.toggle.Visible() || !strings.Contains(e.notice.Text(), "color-map") {
		t.Fatalf("notice %v %q, text %v, toggle %v", e.notice.Visible(), e.notice.Text(), e.raw.Visible(), e.toggle.Visible())
	}
	if !strings.Contains(e.raw.area.Text(), "color-map") {
		t.Errorf("the text lacks the field beyond the cards: %q", e.raw.area.Text())
	}
	_ = e.cards.fieldSlot(0, "hide-if-empty").set(true)
	if m := customItem(t, k, 0); m["color-map"] == nil || m["hide-if-empty"] != true {
		t.Errorf("a card edit dropped the field it does not edit: %v", m)
	}
	plain := customEditor(t, testKit(t, gpuModule))
	plain.toggle.OnClick()
	if !plain.raw.Visible() || plain.notice.Visible() {
		t.Errorf("Edit as text shows text %v, notice %v", plain.raw.Visible(), plain.notice.Visible())
	}
}

func TestACustomModuleTheShellRefusesIsHeldWithTheReason(t *testing.T) {
	k := testKit(t, gpuModule)
	e := customEditor(t, k)
	_ = e.cards.fieldSlot(0, "mode").set("sometimes")
	if !failedWith(e.cards.draft, locale.T().Get("settings-list-draft"), "sometimes") {
		t.Fatalf("a refused mode shows draft %v %q over %q", e.cards.draft.Visible(), e.cards.draft.Text(), e.cards.draft.detail.Text())
	}
	if m := customItem(t, k, 0); m["mode"] != "poll" {
		t.Errorf("the refused mode was stored: %v", m["mode"])
	}
	e.cards.local = append(e.cards.local, e.cards.spec.blank())
	e.cards.local = e.cards.local[1:]
	e.cards.discardDraft()
	e.cards.commit()
	if items, _ := k.store.value(customModulesPath).([]any); len(items) != 0 {
		t.Errorf("a module without an id was written: %v", items)
	}
}

func TestRunOnceShowsWhatTheCommandPrints(t *testing.T) {
	k := testKit(t, "")
	t2 := locale.T()
	for command, want := range map[string]string{
		"echo 72": t2.Get("settings-custom-run-output", i18n.Str("output", "72")),
		"true":    t2.Get("settings-custom-run-empty"),
		"":        t2.Get("settings-custom-run-no-command"),
	} {
		value := command
		r := newCommandRun(k, stringSlot(&value, func() {}), "settings-custom-run-empty")
		r.run()
		if got := r.output.Text(); !strings.Contains(got, want) || !r.output.Visible() || r.problem.Visible() {
			t.Errorf("%q shows %q, want %q", command, got, want)
		}
	}
	value := "echo no; exit 3"
	r := newCommandRun(k, stringSlot(&value, func() {}), "settings-custom-run-empty")
	r.run()
	if r.output.Visible() || !failedWith(r.problem, t2.Get("settings-custom-run-failed"), "exit status 3") {
		t.Errorf("a failing command shows output %v, says %q over %q", r.output.Visible(), r.problem.Text(), r.problem.detail.Text())
	}
}

func TestACustomModuleCardAddsItselfToTheBar(t *testing.T) {
	k := testKit(t, "")
	id := "gpu"
	p := newPlacementRow(k, func() (config.BarModule, bool) { return config.CustomModule(id), id != "" })
	if !p.Visible() || !p.add.Visible() {
		t.Fatal("a module outside the bar offers no add")
	}
	p.add.ClickAt(widget.Point{})
	if !layoutsHold(k.store.svc.Config().Bar.Layout, "custom-gpu") || p.add.Visible() {
		t.Error("add did not put custom-gpu in the bar")
	}
	id = ""
	p.refresh()
	if p.Visible() {
		t.Error("a module without an id offers a bar placement")
	}
}
