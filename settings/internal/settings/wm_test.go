package settings

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const wmSchema = `{"type":"object","properties":{
"gaps":{"type":"object","properties":{"inner":{"type":"integer","minimum":0,"default":8},"outer":{"oneOf":[{"type":"integer"},{"type":"array"}]}}},
"tearing":{"type":"boolean","default":false},
"animations":{"type":"object","properties":{"enabled":{"type":"boolean","default":true},"reduced_motion":{"type":"boolean","default":false},
"curves":{"type":"object","x-lys-wm-curves":[{"name":"ease","bezier":[0.25,0.1,0.25,1]},{"name":"linear","bezier":[0,0,1,1]},{"name":"spring","spring":{"stiffness":170,"damping":26,"mass":1}}],"additionalProperties":{"type":"object","properties":{"bezier":{"type":"array","items":{"type":"number"}},"spring":{"type":"object","properties":{"damping":{"type":"number"},"mass":{"type":"number"},"stiffness":{"type":"number"}},"additionalProperties":false}},"additionalProperties":false}},
"events":{"type":"object","x-lys-wm-events":[{"name":"global"},{"name":"windows","parent":"global"},{"name":"windows_in","parent":"windows","styles":["popin","slide","bud"]},{"name":"workspaces","parent":"global","styles":["slide","slidevert","fade"]}],"additionalProperties":{"type":"object","properties":{"curve":{"type":"string"},"duration":{"type":"integer","minimum":1},"enabled":{"type":"boolean"},"style":{"type":"string"}},"additionalProperties":false}}}},
"workspaces":{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer","minimum":1},"match":{"type":"string","enum":["any","fullscreen","single_tiled"]},"name":{"type":"string"},"output":{"type":"string"},"default":{"type":"boolean","default":false},"layout":{"type":"string","enum":["scroll","tree"]},"on_created_empty":{"type":"string","format":"lys-wm-command"},"gaps":{"type":"object","properties":{"inner":{"type":"integer"},"outer":{"oneOf":[{"type":"integer"},{"type":"array","items":{"type":"integer"}}]}},"additionalProperties":false}},"additionalProperties":false}},
"scratchpads":{"type":"object","additionalProperties":{"type":"object","properties":{"exec":{"type":"string","format":"lys-wm-command"},"hide_on_workspace_switch":{"type":"boolean","default":true},"size":{"type":"string"}},"additionalProperties":false}},
"virtual_outputs":{"type":"array","items":{"type":"object","properties":{"mode":{"type":"string"},"name":{"type":"string"}},"additionalProperties":false}},
"hdr":{"enum":[false,"fullscreen",true]},
"binds":{"type":"array","items":{"type":"object","properties":{"action":{"type":"string","format":"lys-wm-action","x-lys-wm-actions":[{"name":"kill","summary":"Close"},{"name":"exec","args":"<command>","summary":"Run"},{"name":"workspace","args":"<n>","summary":"Show"}]},"chord":{"type":"string","format":"lys-wm-chord"},"flags":{"type":"array","items":{"type":"string","enum":["locked","repeating"]}}},"additionalProperties":false}},
"submaps":{"type":"object","additionalProperties":{"type":"array","items":{"type":"object","properties":{"action":{"type":"string","format":"lys-wm-action","x-lys-wm-actions":[{"name":"kill","summary":"Close"}]},"chord":{"type":"string","format":"lys-wm-chord"}},"additionalProperties":false}}},
"rules":{"type":"array","items":{"type":"object","properties":{"animation":{"oneOf":[{"type":"boolean","enum":[false]},{"type":"object","properties":{"in":{"type":"string"},"out":{"type":"string"}},"additionalProperties":false}]},"app_id":{"type":"string"},"blur_look":{"type":"object","properties":{"grain":{"type":"number"}}},"enabled":{"type":"boolean","default":true},"float":{"type":"boolean"},"title":{"type":"string"}}}},
"borders":{"type":"object","properties":{"active":{"default":"accent","oneOf":[{"type":"string","format":"lys-wm-color"},{"type":"object","properties":{"angle":{"type":"number"},"colors":{"type":"array","minItems":1,"maxItems":3,"items":{"type":"string"}}}}]}}},
"decoration":{"type":"object","properties":{"active_opacity":{"type":"number","minimum":0,"maximum":1,"default":1},"shadow":{"type":"object","properties":{"color":{"type":"string","format":"lys-wm-color","default":"#00000060"}}}}},
"input":{"type":"object","properties":{"touchpad":{"type":"object","properties":{"tap":{"type":"boolean"},"click_method":{"type":"string","enum":["none","clickfinger"]},"accel_profile":{"type":"string","enum":["adaptive","flat"],"default":"adaptive"}}},
"pointer":{"type":"object","properties":{"accel_profile":{"type":"string","enum":["adaptive","flat"],"default":"flat"}}},
"keyboard":{"type":"object","properties":{"layout":{"type":"string","format":"lys-wm-xkb-layout"},"variant":{"type":"string","format":"lys-wm-xkb-variant"},"options":{"type":"string","format":"lys-wm-xkb-options"},"model":{"type":"string","format":"lys-wm-xkb-model"},"rules":{"type":"string","format":"lys-wm-xkb-rules"}}}}},
"gpu":{"type":"string","format":"lys-wm-drm-device"},
"env":{"type":"object","additionalProperties":{"type":"string"}},
"palette":{"type":"object","additionalProperties":{"type":"string"},"default":{"red":"#f38ba8","blue":"#89b4fa"}},
"tabs":{"type":"object","properties":{"font":{"type":"string","format":"lys-wm-font-file"},"icon_theme":{"type":"string","format":"lys-wm-icon-theme"},"label":{"type":"string"}}}}}`

const wmBuiltIn = "binds:\n  - {chord: SUPER + Q, action: kill}\n"

type fakeWM struct {
	reject  string
	reloads int
	checks  int
	running map[string]string
}

func (w *fakeWM) run(_ context.Context, argv ...string) ([]byte, error) {
	if argv[0] == wmconfig.Control && argv[1] != "reload" {
		answer, ok := w.running[argv[1]]
		if !ok {
			return nil, &wmconfig.ToolError{Argv: argv, Exit: 3, Stderr: "lys-wmctl: ipc: no lys-wm socket", Err: errors.New("exit status 3")}
		}
		return []byte(answer), nil
	}
	switch argv[0] + " " + argv[1] {
	case "lys-wm --config-schema":
		return []byte(wmSchema), nil
	case "lys-wm --default-config":
		return []byte(wmBuiltIn), nil
	case "lys-wm --check-config":
		w.checks++
		candidate := argv[slices.Index(argv, "--runtime")+1]
		if w.reject != "" {
			return nil, &wmconfig.ToolError{Argv: argv, Exit: 1, Stderr: candidate + ":3: " + w.reject, Err: errors.New("exit status 1")}
		}
		return nil, nil
	case "lys-wmctl reload":
		w.reloads++
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected program %q", argv)
}

type wmFiles struct{ config, runtime string }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wmKitWith(t *testing.T, config, runtime string) (*kit, *wmBackend, *fakeWM, wmFiles) {
	t.Helper()
	k := testKit(t, "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	files := wmFiles{filepath.Join(dir, "lys-wm", "config.yaml"), filepath.Join(dir, "lys-wm", "runtime.yaml")}
	if config != "" {
		writeFile(t, files.config, config)
	}
	if runtime != "" {
		writeFile(t, files.runtime, runtime)
	}
	fake := &fakeWM{}
	w, err := openWM(k, fake.run)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	return k, w, fake, files
}

func wmKit(t *testing.T, config string) (*kit, *wmBackend, *fakeWM, wmFiles) {
	t.Helper()
	return wmKitWith(t, config, "")
}

func fileText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func noFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestWMRowsFollowTheSchemasTypes(t *testing.T) {
	k, w, _, _ := wmKit(t, "gaps: {inner: 3}\ninput: {touchpad: {tap: true}}\n")
	for path, want := range map[string]any{
		"gaps.inner":                  &number{},
		"tearing":                     &toggle{},
		"decoration.active_opacity":   &slider{},
		"input.touchpad.tap":          &choiceControl{},
		"input.touchpad.click_method": &choiceControl{},
		"tabs.font":                   &wmFontEditor{},
		"tabs.icon_theme":             &choiceEditor{},
		"tabs.label":                  &text{},
		"gpu":                         &choiceEditor{},
		"input.keyboard.layout":       &choiceEditor{},
		"decoration.shadow.color":     &swatchPicker{},
	} {
		c := fieldControl(k, w, wmField(path))
		if fmt.Sprintf("%T", c) != fmt.Sprintf("%T", want) {
			t.Errorf("%s: %T, want %T", path, c, want)
		}
	}
	if got := w.value("gaps.inner"); got != int64(3) {
		t.Errorf("a key in the file: %#v", got)
	}
	if got := w.value("tearing"); got != false {
		t.Errorf("a key the file leaves out takes the schema's default: %#v", got)
	}
	if got := w.value("decoration.active_opacity"); got != float64(1) {
		t.Errorf("a float default: %#v", got)
	}
	if got := w.value("tabs.font"); got != nil {
		t.Errorf("no default and no key: %#v", got)
	}
}

func TestEachDeviceClassShowsItsOwnAccelerationDefault(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	for path, want := range map[string]string{"input.pointer.accel_profile": "flat", "input.touchpad.accel_profile": "adaptive"} {
		if c, ok := fieldControl(k, w, wmField(path)).(*choiceControl); !ok || c.choice.unset != "" {
			t.Errorf("%s: %T, want a choice of profiles with no unset entry", path, c)
		}
		if got := w.value(path); got != want {
			t.Errorf("%s on a fresh config: %#v, want %q", path, got, want)
		}
		if !w.isDefault(path) {
			t.Errorf("%s holding lys-wm's default is not shown as the default", path)
		}
	}
	_, w, _, _ = wmKit(t, "input: {pointer: {accel_profile: adaptive}}\n")
	if got := w.value("input.pointer.accel_profile"); got != "adaptive" {
		t.Errorf("a mouse made adaptive in the config shows %#v", got)
	}
	if w.isDefault("input.pointer.accel_profile") {
		t.Error("an adaptive mouse is shown as lys-wm's default")
	}
	if got := w.value("input.touchpad.accel_profile"); got != "adaptive" {
		t.Errorf("the touchpad followed the mouse's profile: %#v", got)
	}
}

func TestAWMRowNamingNoKeyIsAPageBug(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	fieldControl(k, w, wmField("no.such.key"))
}

func TestAToggleWritesAnOverrideLeavingTheConfigAndReloads(t *testing.T) {
	const declared = "# mine\nunknown: 1\ntearing: false # why\n"
	k, w, fake, files := wmKit(t, declared)
	c := fieldControl(k, w, wmField("tearing")).(*toggle)
	c.OnChanged(true)
	if got := fileText(t, files.runtime); got != "tearing: true\n" {
		t.Fatalf("%q", got)
	}
	if got := fileText(t, files.config); got != declared {
		t.Fatalf("the config changed: %q", got)
	}
	if fake.reloads != 1 || fake.checks != 1 {
		t.Fatalf("%d reloads, %d checks", fake.reloads, fake.checks)
	}
	if got := w.value("tearing"); got != true {
		t.Fatalf("%v", got)
	}
	if src := w.source("tearing"); src.label != locale.T().Get("settings-source-override") || !src.reset {
		t.Fatalf("an override of a config key: %+v", src)
	}
	if w.notice.kind != stateNone {
		t.Fatalf("a good change says %q over %v", w.notice.summary, w.notice.err)
	}
}

func TestARefusedChangeShowsWhyAndTheRowGoesBack(t *testing.T) {
	const before = "gaps:\n  inner: 3\n"
	k, w, fake, files := wmKitWith(t, "", before)
	fake.reject = "gaps.inner: nope"
	row := newFieldRow(k, w, wmField("gaps.inner"))
	n := row.ctl.(*number)
	n.OnValueChanged(9)
	if got := fileText(t, files.runtime); got != before {
		t.Fatalf("%q", got)
	}
	if fake.reloads != 0 {
		t.Fatal("reloaded a refused change")
	}
	if got := w.value("gaps.inner"); got != int64(3) {
		t.Fatalf("the row holds a value the file does not: %v", got)
	}
	if !strings.Contains(errText(w.notice.err), "gaps.inner: nope") || !strings.Contains(errText(w.notice.err), files.runtime) || !w.notice.failed() || w.notice.summary != locale.T().Get("wm-status-not-applied") {
		t.Fatalf("%v %q over %v", w.notice.kind, w.notice.summary, w.notice.err)
	}
	fake.reject = ""
	n.OnValueChanged(4)
	if w.notice.kind != stateNone {
		t.Fatalf("a later good change leaves %q over %v", w.notice.summary, w.notice.err)
	}
	if got := fileText(t, files.runtime); got != "gaps:\n  inner: 4\n" {
		t.Fatalf("%q", got)
	}
}

func TestAHomeManagerConfigTakesChangesAsOverrides(t *testing.T) {
	k := testKit(t, "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	store := filepath.Join(t.TempDir(), "abc-home-manager-files", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, []byte("tearing: false\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "lys-wm", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(store, path); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWM{}
	w, err := openWM(k, fake.run)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	if w.notice.kind != stateNone {
		t.Fatalf("a read-only config warns: %q over %v", w.notice.summary, w.notice.err)
	}
	c := fieldControl(k, w, wmField("tearing")).(*toggle)
	c.OnChanged(true)
	if w.notice.kind != stateNone || fake.checks != 1 || fake.reloads != 1 {
		t.Fatalf("%q over %v, %d checks, %d reloads", w.notice.summary, w.notice.err, fake.checks, fake.reloads)
	}
	if got := fileText(t, filepath.Join(dir, "lys-wm", "runtime.yaml")); got != "tearing: true\n" {
		t.Fatalf("%q", got)
	}
	if fileText(t, store) != "tearing: false\n" {
		t.Fatal("touched a read-only config")
	}
	if target, err := os.Readlink(path); err != nil || target != store {
		t.Fatalf("the link moved: %q, %v", target, err)
	}
	if got := w.value("tearing"); got != true {
		t.Fatalf("%v", got)
	}
	w.reset("tearing")
	if got := w.value("tearing"); got != false {
		t.Fatalf("a reset falls back to the declared config: %v", got)
	}
}

func TestAReadOnlyOverrideFileShowsOnThePage(t *testing.T) {
	k := testKit(t, "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	runtime := filepath.Join(dir, "lys-wm", "runtime.yaml")
	writeFile(t, runtime, "tearing: false\n")
	if err := os.Chmod(runtime, 0o444); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWM{}
	w, err := openWM(k, fake.run)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	if !strings.Contains(errText(w.notice.err), runtime) || w.notice.kind != stateUnavailable || w.notice.summary != locale.T().Get("wm-status-read-only") {
		t.Fatalf("%v %q over %v", w.notice.kind, w.notice.summary, w.notice.err)
	}
	c := fieldControl(k, w, wmField("tearing")).(*toggle)
	c.OnChanged(true)
	if !strings.Contains(errText(w.notice.err), "read-only") || fake.checks != 0 || fake.reloads != 0 {
		t.Fatalf("%q over %v, %d checks, %d reloads", w.notice.summary, w.notice.err, fake.checks, fake.reloads)
	}
	if got := w.value("tearing"); got != false {
		t.Fatalf("%v", got)
	}
	if !newWMStatus(k, w).Visible() {
		t.Fatal("the page does not show it")
	}
}

func TestResetPutsAKeyBackToItsDefaultOverTheConfig(t *testing.T) {
	_, w, _, files := wmKitWith(t, "gaps:\n  inner: 3\n", "gaps:\n  inner: 5\ntearing: true\n")
	if got := w.value("gaps.inner"); got != int64(5) {
		t.Fatalf("%v", got)
	}
	if src := w.source("gaps.inner"); !src.reset || src.label != locale.T().Get("settings-source-override") {
		t.Fatalf("%+v", src)
	}
	w.reset("gaps.inner")
	if got := fileText(t, files.runtime); !strings.Contains(got, "inner: 8") || !strings.Contains(got, "tearing: true") {
		t.Fatalf("the default must cover the config's 3: %q", got)
	}
	if got := w.value("gaps.inner"); got != int64(8) {
		t.Fatalf("%v", got)
	}
	if src := w.source("gaps.inner"); src.reset {
		t.Fatalf("a key at its default offers a reset: %+v", src)
	}
	if src := w.source("tearing"); src.label != locale.T().Get("settings-source-custom") || !src.reset {
		t.Fatalf("an override of no config key: %+v", src)
	}
	w.reset("tearing")
	if got := fileText(t, files.runtime); strings.Contains(got, "tearing") {
		t.Fatalf("a key the config leaves out resets by dropping its override: %q", got)
	}
}

func TestResettingAnOverrideWithoutAConfigRestoresTheBuiltIn(t *testing.T) {
	_, w, _, files := wmKitWith(t, "", "binds:\n  - {chord: SUPER + X, action: exit}\n")
	if src := w.source("binds"); !src.reset || !src.badge {
		t.Fatalf("an override: %+v", src)
	}
	w.reset("binds")
	if got := fileText(t, files.runtime); strings.Contains(got, "binds") {
		t.Fatalf("%q", got)
	}
	binds, ok := w.value("binds").([]any)
	if !ok || len(binds) != 1 || !strings.Contains(fmt.Sprint(binds), "SUPER + Q") {
		t.Fatalf("%#v", w.value("binds"))
	}
	if src := w.source("binds"); src.badge || src.reset {
		t.Fatalf("back at the default: %+v", src)
	}
	noFile(t, files.config)
}

func TestAConfigValueOffTheDefaultResetsThroughAnOverride(t *testing.T) {
	_, w, _, files := wmKit(t, "gaps:\n  inner: 3\n")
	if src := w.source("gaps.inner"); !src.reset {
		t.Fatalf("a config value off the default offers no reset: %+v", src)
	}
	w.reset("gaps.inner")
	if got := fileText(t, files.runtime); !strings.Contains(got, "inner: 8") {
		t.Fatalf("%q", got)
	}
	if got := w.value("gaps.inner"); got != int64(8) {
		t.Fatalf("%v", got)
	}
}

func TestAKeyAtItsDefaultOffersNoReset(t *testing.T) {
	_, w, fake, files := wmKit(t, "gaps:\n  inner: 8\n")
	if src := w.source("gaps.inner"); src.reset {
		t.Fatalf("%+v", src)
	}
	if src := w.source("tearing"); src.reset {
		t.Fatalf("an unset key with a default: %+v", src)
	}
	if fake.checks != 0 {
		t.Fatalf("%d checks", fake.checks)
	}
	noFile(t, files.runtime)
}

func TestANullOverrideClearsTheMappingBelowIt(t *testing.T) {
	_, w, _, _ := wmKitWith(t, "gaps:\n  inner: 3\n", "gaps:\n")
	if got := w.value("gaps.inner"); got != int64(8) {
		t.Fatalf("%v", got)
	}
	if src := w.source("gaps.inner"); src.reset || !src.badge {
		t.Fatalf("a null override leaves the key at its default, overridden: %+v", src)
	}
}

func TestWithoutAConfigAChangeWritesOnlyThatKeyAndKeepsTheBuiltInBinds(t *testing.T) {
	k, w, _, files := wmKit(t, "")
	if src := w.source("tearing"); src.badge || src.reset {
		t.Fatalf("no file: %+v", src)
	}
	binds, ok := w.value("binds").([]any)
	if !ok || len(binds) != 1 {
		t.Fatalf("no file reads the built-in binds: %#v", w.value("binds"))
	}
	c := fieldControl(k, w, wmField("tearing")).(*toggle)
	c.OnChanged(true)
	if got := fileText(t, files.runtime); got != "tearing: true\n" {
		t.Fatalf("%q", got)
	}
	noFile(t, files.config)
	if binds, ok := w.value("binds").([]any); !ok || len(binds) != 1 {
		t.Fatalf("an override lost the built-in binds: %#v", w.value("binds"))
	}
}

func TestAKeyAFileLeavesOutIsNotTheBuiltInConfigs(t *testing.T) {
	_, w, _, _ := wmKit(t, "tearing: true\n")
	if got := w.value("binds"); got != nil {
		t.Fatalf("a file without binds has none: %#v", got)
	}
}

func TestAnOptionalToggleWritesABoolAndUnsets(t *testing.T) {
	k, w, _, files := wmKit(t, "binds: []\n")
	c := fieldControl(k, w, wmField("input.touchpad.tap")).(*choiceControl)
	if c.pick.Selected() != 0 {
		t.Fatalf("unset shows %d", c.pick.Selected())
	}
	pickChoice(c.pick, 1)
	if got := fileText(t, files.runtime); !strings.Contains(got, "tap: true") {
		t.Fatalf("%q", got)
	}
	if got := w.value("input.touchpad.tap"); got != true {
		t.Fatalf("%v", got)
	}
	c.refresh()
	if c.pick.Selected() != 1 {
		t.Fatalf("on shows %d", c.pick.Selected())
	}
	pickChoice(c.pick, 2)
	if got := fileText(t, files.runtime); !strings.Contains(got, "tap: false") {
		t.Fatalf("%q", got)
	}
	pickChoice(c.pick, 0)
	if got := fileText(t, files.runtime); strings.Contains(got, "tap") || strings.Contains(got, "touchpad") {
		t.Fatalf("%q", got)
	}
}

func TestAnEnumOfBooleansAndWordsWritesEachAsItsOwnType(t *testing.T) {
	k, w, _, files := wmKit(t, "binds: []\n")
	c := fieldControl(k, w, wmField("hdr")).(*choiceControl)
	if got, want := c.pick.Items(), []string{locale.T().Get("settings-inherited"), "Off", "fullscreen", "On"}; !slices.Equal(got, want) {
		t.Fatalf("choices %q, want %q", got, want)
	}
	for i, want := range []string{"hdr: false\n", "hdr: fullscreen\n", "hdr: true\n"} {
		pickChoice(c.pick, i+1)
		if got := fileText(t, files.runtime); !strings.Contains(got, want) {
			t.Fatalf("choice %d wrote %q, want %q", i+1, got, want)
		}
		c.refresh()
		if c.pick.Selected() != i+1 {
			t.Fatalf("choice %d reads back as %d", i+1, c.pick.Selected())
		}
	}
}

func TestAChoiceSlotRefusesANameItDoesNotOffer(t *testing.T) {
	var stored any = "fullscreen"
	raw := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() { stored = nil }}
	s := wmChoiceSlot(wmconfig.Field{Kind: wmconfig.Enum, Variants: []any{false, "fullscreen", true}}, raw)
	if err := s.set("false"); err == nil {
		t.Fatal("the word false was taken for the boolean")
	}
	if stored != "fullscreen" {
		t.Fatalf("a refused name stored %#v", stored)
	}
	if err := s.set(wmToggleOff); err != nil || stored != false {
		t.Fatalf("off stored %#v: %v", stored, err)
	}
	if got := s.get(); got != wmToggleOff {
		t.Fatalf("false reads as %#v", got)
	}
	stored = nil
	if got := s.get(); got != nil {
		t.Fatalf("unset reads as %#v", got)
	}
}

func TestBindsAreCardsWithAFilter(t *testing.T) {
	const declared = "# keep\nbinds:\n  - {chord: SUPER + Q, action: kill}\n  - {chord: SUPER + F, action: fullscreen, flags: [locked]}\ntearing: true\n"
	k, w, _, files := wmKit(t, declared)
	c := fieldControl(k, w, wmShortcutsRow()).(*shortcutTable).main
	if len(c.cards) != 2 || c.cards[1].title.Text() != "fullscreen" {
		t.Fatalf("%d cards, the second titled %q", len(c.cards), c.cards[1].title.Text())
	}
	c.filter.SetText("full")
	if c.cards[0].root.Visible() || !c.cards[1].root.Visible() {
		t.Fatal("the filter shows the kill bind or hides the fullscreen one")
	}
	c.filter.SetText("locked super")
	if c.cards[0].root.Visible() || !c.cards[1].root.Visible() {
		t.Fatal("every word must match some value, a flag included")
	}
	c.filter.SetText("")
	if !c.cards[0].root.Visible() {
		t.Fatal("an empty filter hides cards")
	}
	if err := c.fieldSlot(0, "action").set("exit"); err != nil {
		t.Fatal(err)
	}
	got := fileText(t, files.runtime)
	if !strings.Contains(got, "exit") || !strings.Contains(got, "fullscreen") || strings.Contains(got, "tearing") {
		t.Fatalf("%q", got)
	}
	if fileText(t, files.config) != declared {
		t.Fatal("the config changed")
	}
}

func TestARulesNestedObjectAndToggleWriteInsideTheItem(t *testing.T) {
	k, w, _, files := wmKit(t, "rules:\n  - {app_id: foot}\n")
	c := fieldControl(k, w, wmObjects("rules", "app_id")).(*cardList)
	if len(c.spec.fields) != 6 {
		t.Fatalf("%d fields, want animation, app_id, blur_look.grain, enabled, float and title", len(c.spec.fields))
	}
	if len(c.cards[0].controls) != 0 || c.cards[0].root.Children()[2].(*widget.Box).Visible() {
		t.Fatal("a collapsed card built or shows its fields")
	}
	disclose := treetest.WithClass(c.cards[0].root, "card-disclose")[0].(*widget.Button)
	disclose.ClickAt(widget.Point{})
	if len(c.cards[0].controls) != 6 || !c.cards[0].root.Children()[2].(*widget.Box).Visible() {
		t.Fatalf("opening the card built %d controls", len(c.cards[0].controls))
	}
	if ctl, ok := c.cards[0].controls[4].(*choiceControl); !ok || ctl.choice.unset == "" {
		t.Fatalf("a rule's float takes %T, want the inherit-or-pick select", c.cards[0].controls[4])
	}
	grain := nestedSlot(c.fieldSlot(0, "blur_look"), []string{"grain"})
	if err := grain.set(0.5); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, "grain: 0.5") || !strings.Contains(got, "app_id: foot") {
		t.Fatalf("%q", got)
	}
	grain.unset()
	if _, ok := c.local[0]["blur_look"]; ok {
		t.Error("dropping the last nested key left an empty object")
	}
	disclose.ClickAt(widget.Point{})
	if c.cards[0].root.Children()[2].(*widget.Box).Visible() || len(c.cards[0].controls) != 6 {
		t.Error("closing the card left its fields shown, or rebuilt them")
	}
}

func TestAnOffOrObjectKeyInheritsTurnsOffOrTakesItsFields(t *testing.T) {
	k, w, _, files := wmKit(t, "rules:\n  - {app_id: foot}\n")
	c := fieldControl(k, w, wmObjects("rules", "app_id")).(*cardList)
	treetest.WithClass(c.cards[0].root, "card-disclose")[0].(*widget.Button).ClickAt(widget.Point{})
	sw, ok := c.cards[0].controls[0].(*wmSwitch)
	if !ok {
		t.Fatalf("a rule's animation takes %T, want the off-or-fields switch", c.cards[0].controls[0])
	}
	if sw.choice.Selected() != switchInherit || sw.body.Visible() || len(sw.fields) != 2 {
		t.Fatalf("unset shows %d, fields shown %v, %d fields", sw.choice.Selected(), sw.body.Visible(), len(sw.fields))
	}
	sw.choice.OnSelect(switchOff)
	if got := fileText(t, files.runtime); !strings.Contains(got, "animation: false") {
		t.Fatalf("off wrote %q", got)
	}
	if sw.choice.Selected() != switchOff || sw.body.Visible() {
		t.Fatalf("off shows %d, fields shown %v", sw.choice.Selected(), sw.body.Visible())
	}
	sw.choice.OnSelect(switchOn)
	if got := fileText(t, files.runtime); !strings.Contains(got, "animation: false") {
		t.Fatalf("on with no field set wrote %q", got)
	}
	if sw.choice.Selected() != switchOn || !sw.body.Visible() {
		t.Fatalf("on shows %d, fields shown %v", sw.choice.Selected(), sw.body.Visible())
	}
	if err := nestedSlot(c.fieldSlot(0, "animation"), []string{"in"}).set("fade"); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, "in: fade") || strings.Contains(got, "animation: false") {
		t.Fatalf("a field set while off wrote %q", got)
	}
	sw.choice.OnSelect(switchInherit)
	if got := fileText(t, files.runtime); strings.Contains(got, "animation") || !strings.Contains(got, "app_id: foot") {
		t.Fatalf("inherit wrote %q", got)
	}
	if sw.choice.Selected() != switchInherit || sw.body.Visible() {
		t.Fatalf("inherit shows %d, fields shown %v", sw.choice.Selected(), sw.body.Visible())
	}
}

func TestABorderIsAColourOrAGradient(t *testing.T) {
	k, w, _, files := wmKit(t, "")
	c := fieldControl(k, w, wmBorder("borders.active")).(*borderEditor)
	if err := c.write([]any{"blue"}, 0); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, "active: blue") {
		t.Fatalf("one colour: %q", got)
	}
	c.refresh()
	if c.angled.Visible() {
		t.Error("one colour shows a gradient's angle")
	}
	if err := c.write([]any{"mauve", "blue"}, 45); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if got := fileText(t, files.runtime); !strings.Contains(got, "angle: 45") || !strings.Contains(got, "mauve") || !c.angled.Visible() {
		t.Fatalf("a gradient: %q", got)
	}
	if colors, angle := c.parts(); len(colors) != 2 || angle != 45 {
		t.Errorf("parts %v %v", colors, angle)
	}
	if err := c.write(nil, 45); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, files.runtime); strings.Contains(got, "active") {
		t.Fatalf("no colour kept %q", got)
	}
}

func TestABordersAddButtonOffersAColorUnderItsAmericanID(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	c := fieldControl(k, w, wmBorder("borders.active")).(*borderEditor)
	en, err := i18n.Select(locale.Assets(), []i18n.LangID{i18n.Fallback})
	if err != nil {
		t.Fatal(err)
	}
	if !en.Has("wm-border-add-color") {
		t.Fatal("en-US has no wm-border-add-color")
	}
	if en.Has("wm-border-add-colour") {
		t.Error("en-US still carries the British wm-border-add-colour")
	}
	if got, want := treetest.Labels(c.colors.end.add), locale.T().Get("wm-border-add-color"); !slices.Contains(got, want) {
		t.Errorf("the add button reads %q, want %q", got, want)
	}
}

func TestChangesMadeWhileOneIsOnItsWayLandAsOneLatest(t *testing.T) {
	k, w, fake, files := wmKitWith(t, "", "gaps:\n  inner: 1\n")
	var queued []func()
	k.background = func(fn func()) { queued = append(queued, fn) }
	k.invoke = func(fn func()) { fn() }
	if err := w.slot("gaps.inner").set(int64(2)); err != nil {
		t.Fatal(err)
	}
	for v := int64(3); v <= 9; v++ {
		if err := w.slot("gaps.inner").set(v); err != nil {
			t.Fatal(err)
		}
		if got := w.value("gaps.inner"); got != v {
			t.Fatalf("the row shows %v while %d is on its way", got, v)
		}
	}
	if len(queued) != 1 {
		t.Fatalf("%d writers at once", len(queued))
	}
	for len(queued) > 0 {
		fn := queued[0]
		queued = queued[1:]
		fn()
	}
	if got := fileText(t, files.runtime); got != "gaps:\n  inner: 9\n" {
		t.Fatalf("%q", got)
	}
	if fake.checks > 2 {
		t.Fatalf("%d checks for one drag", fake.checks)
	}
}

func TestAChangeBesideTheAppIsRead(t *testing.T) {
	k, w, _, files := wmKit(t, "gaps:\n  inner: 1\n")
	writeFile(t, files.config, "gaps:\n  inner: 7\n")
	if _, err := k.wmBackend(); err != nil {
		t.Fatal(err)
	}
	if got := w.value("gaps.inner"); got != int64(7) {
		t.Fatalf("%v", got)
	}
	writeFile(t, files.runtime, "gaps:\n  inner: 2\n")
	if _, err := k.wmBackend(); err != nil {
		t.Fatal(err)
	}
	if got := w.value("gaps.inner"); got != int64(2) {
		t.Fatalf("an override beside the app: %v", got)
	}
}

func TestResetAllDropsTheWMOverridesToo(t *testing.T) {
	k, w, fake, files := wmKitWith(t, "gaps:\n  inner: 3\n", "gaps:\n  inner: 5\ntearing: true\n")
	if err := k.store.set("osd.enabled", false); err != nil {
		t.Fatal(err)
	}
	cfg := resetAllDialog(k)
	cfg.OnResponse(responseCancel)
	if fake.checks != 0 || w.value("gaps.inner") != int64(5) {
		t.Fatal("cancelling reset the lys-wm overrides")
	}
	cfg.OnResponse(responseConfirm)
	if got := fileText(t, files.runtime); strings.TrimSpace(got) != "" {
		t.Fatalf("%q", got)
	}
	if got := w.value("gaps.inner"); got != int64(3) {
		t.Fatalf("%v", got)
	}
	if fake.reloads != 1 {
		t.Fatalf("%d reloads", fake.reloads)
	}
}

func TestResetAllReachesTheWMOverridesBeforeAnyWMPageOpened(t *testing.T) {
	k := testKit(t, "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	bin := filepath.Join(dir, "bin")
	log := filepath.Join(dir, "calls")
	for _, program := range []string{"lys-wm", "lys-wmctl"} {
		writeFile(t, filepath.Join(bin, program), "#!/bin/sh\necho \"$0 $*\" >> "+log+"\n")
		if err := os.Chmod(filepath.Join(bin, program), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	runtime := filepath.Join(dir, "config", "lys-wm", "runtime.yaml")
	writeFile(t, runtime, "tearing: true\n")
	k.resetAll()
	if got := fileText(t, runtime); strings.TrimSpace(got) != "" {
		t.Fatalf("%q", got)
	}
	calls := fileText(t, log)
	if !strings.Contains(calls, "lys-wm --check-config --runtime") || !strings.Contains(calls, "lys-wmctl reload") {
		t.Fatalf("%q", calls)
	}
}

func TestResetAllWithoutLysWMTouchesNoWMFile(t *testing.T) {
	k := testKit(t, "")
	k.parts = parts{shell: true}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	runtime := filepath.Join(dir, "lys-wm", "runtime.yaml")
	writeFile(t, runtime, "tearing: true\n")
	k.resetAll()
	if got := fileText(t, runtime); got != "tearing: true\n" {
		t.Fatalf("%q", got)
	}
}

func TestASchemaThatCannotBeReadFailsThePageNotTheApp(t *testing.T) {
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := openWM(k, func(context.Context, ...string) ([]byte, error) { return nil, errors.New("lys-wm is too old") }); err == nil || !strings.Contains(err.Error(), "too old") {
		t.Fatalf("%v", err)
	}
}

func wmPageSpecs() []pageSpec {
	var out []pageSpec
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			if p.wm {
				out = append(out, p)
			}
		}
	}
	return out
}

func allWMRows() []rowSpec {
	var rows []rowSpec
	for _, p := range wmPageSpecs() {
		for _, section := range p.sections {
			rows = append(rows, section.rows...)
		}
	}
	for _, section := range loginItemsPage(config.Defaults()).sections {
		rows = append(rows, section.rows...)
	}
	return rows
}

func TestEveryWMRowAndPageHasItsMessagesInEveryLanguage(t *testing.T) {
	for _, lang := range []string{"en-US", "fr"} {
		l, err := i18n.Select(locale.Assets(), []i18n.LangID{i18n.MustLangID(lang)})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range allWMRows() {
			if !l.Has(r.labelKey()) {
				t.Errorf("%s: no %s", lang, r.labelKey())
			}
		}
		for _, p := range wmPageSpecs() {
			for _, key := range []string{p.section, p.navKey, p.header} {
				if !l.Has(key) {
					t.Errorf("%s: no %s", lang, key)
				}
			}
			for _, section := range p.sections {
				if !l.Has(section.title) {
					t.Errorf("%s: no %s", lang, section.title)
				}
			}
		}
		for _, key := range []string{"wm-status-not-running", "enum-wm-toggle-on", "enum-wm-toggle-off"} {
			if !l.Has(key) {
				t.Errorf("%s: no %s", lang, key)
			}
		}
	}
}

func TestEveryPageIDIsUnique(t *testing.T) {
	seen := []string{}
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			if slices.Contains(seen, p.id) {
				t.Errorf("%s twice", p.id)
			}
			seen = append(seen, p.id)
		}
	}
}

func TestAWMPageBuildsItsRowsAndStatus(t *testing.T) {
	k, _, _, _ := wmKit(t, "")
	spec := wmPage("wm-test", "ld-monitor-symbolic", sectionSpec{title: "wm-section-gaps", rows: []rowSpec{wmField("gaps.inner"), wmSides("gaps.outer")}})
	page := buildPage(k, spec)
	if len(page.rows) != 2 || page.status == nil {
		t.Fatalf("%d rows, status %v", len(page.rows), page.status)
	}
	if page.status.Visible() {
		t.Fatal("a status line with nothing to say")
	}
	page.refresh()
}

func TestAWMPageWhoseSchemaFailsSaysSoAndStillBuilds(t *testing.T) {
	k := testKit(t, "")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	page := buildPage(k, wmPage("wm-test", "ld-monitor-symbolic"))
	if page.Scroll == nil || len(page.rows) != 0 {
		t.Fatalf("%+v", page)
	}
}

func TestTheKeyboardLayoutsAreToggledInOrderAndTheVariantsFollowThem(t *testing.T) {
	k, w, _, files := wmKit(t, "input:\n  keyboard:\n    layout: us\n")
	layout := fieldControl(k, w, wmField("input.keyboard.layout")).(*choiceEditor)
	p := openChoices(t, k, layout)
	p.pick(t, "us,dk")
	if got := fileText(t, files.runtime); got != "input:\n  keyboard:\n    layout: us,dk\n" {
		t.Fatalf("%q", got)
	}
	variant := fieldControl(k, w, wmField("input.keyboard.variant")).(*choiceEditor)
	p = openChoices(t, k, variant)
	if !slices.Equal(p.all, []string{"intl", ",nodeadkeys"}) {
		t.Fatalf("offered %q", p.all)
	}
	p.pick(t, ",nodeadkeys")
	if got := w.value("input.keyboard.variant"); got != ",nodeadkeys" {
		t.Fatalf("%v", got)
	}
	if variant.unknown.Visible() {
		t.Fatal("a listed variant is unknown")
	}
	if err := w.slot("input.keyboard.variant").set("nodeadkeys"); err != nil {
		t.Fatal(err)
	}
	variant.refresh()
	if !variant.unknown.Visible() {
		t.Fatal("a variant of another layout is not marked")
	}
	options := fieldControl(k, w, wmField("input.keyboard.options")).(*choiceEditor)
	p = openChoices(t, k, options)
	p.pick(t, "caps:escape")
	if got := w.value("input.keyboard.options"); got != "caps:escape" {
		t.Fatalf("%v", got)
	}
}

// A lys-wm colour is the swatch picker over lys-wm's palette names, the
// built-in ones and the desktop accent among them, each led by its
// colour; the colour button sets a hex, shown as custom; a name the
// palette lacks is listed as it is.
func TestAWMColorTakesAHexOrAPaletteName(t *testing.T) {
	k, w, _, _ := wmKit(t, "palette:\n  accent: '#ff0000'\n")
	c := fieldControl(k, w, wmField("decoration.shadow.color")).(*swatchPicker)
	c.swatch.OnColorSet(render.RGBA(0, 255, 0, 255))
	if got := w.value("decoration.shadow.color"); got != "#00ff00" {
		t.Fatalf("%v", got)
	}
	c.refresh()
	if c.ids[c.drop.Selected()] != colorCustom || !c.swatch.Visible() {
		t.Fatalf("a hex shows row %q, the colour button %v", c.ids[c.drop.Selected()], c.swatch.Visible())
	}
	if want := []string{colorCustom, "accent", "blue", "red"}; !slices.Equal(slices.DeleteFunc(slices.Clone(c.ids), func(id string) bool { return id == "" }), want) {
		t.Fatalf("offered %q, want custom and the palette's names", c.ids)
	}
	if got := listedSwatches(t, c); len(got) != 4 || got[0] != render.RGBA(0, 255, 0, 255) || got[1] != render.RGBA(255, 0, 0, 255) {
		t.Fatalf("the rows lead with swatches %x, want the custom green, then the accent's red", got)
	}
	c.drop.SetSelected(slices.Index(c.ids, "accent"))
	if got := w.value("decoration.shadow.color"); got != "accent" || c.swatch.Visible() {
		t.Fatalf("picking accent wrote %v, the colour button %v", got, c.swatch.Visible())
	}
	if err := w.slot("decoration.shadow.color").set("teal"); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if c.ids[c.drop.Selected()] != "teal" {
		t.Fatalf("a name the palette lacks shows row %q", c.ids[c.drop.Selected()])
	}
}

func TestAnUnlistedDRMDeviceIsKeptAndMarked(t *testing.T) {
	k, w, _, _ := wmKit(t, "gpu: card7\n")
	c := fieldControl(k, w, wmField("gpu")).(*choiceEditor)
	if c.entry.Text() != "card7" || !c.unknown.Visible() {
		t.Fatalf("%q %v", c.entry.Text(), c.unknown.Visible())
	}
	p := openChoices(t, k, c)
	p.pick(t, "card0")
	if got := w.value("gpu"); got != "card0" || c.unknown.Visible() {
		t.Fatalf("%v %v", got, c.unknown.Visible())
	}
}

func TestALongCardTitleShrinksInsteadOfWideningTheList(t *testing.T) {
	long := "exec lys-shell launcher -show clipboard && wtype -M ctrl v -m ctrl && notify-send \"Pasted selection\" " + strings.Repeat("and more ", 20)
	k, w, _, _ := wmKit(t, "binds:\n  - {chord: SUPER + V, action: '"+strings.ReplaceAll(long, "'", "''")+"'}\n")
	c := fieldControl(k, w, wmShortcutsRow()).(*shortcutTable).main
	k, w, _, _ = wmKit(t, "binds:\n  - {chord: SUPER + V, action: kill}\n")
	short := fieldControl(k, w, wmShortcutsRow()).(*shortcutTable).main
	if got, base := c.Measure(widget.Row, widget.Unknown).Min, short.Measure(widget.Row, widget.Unknown).Min; got > base {
		t.Fatalf("a long title makes the list at least %dpx wide, a short one %dpx", got, base)
	}
}

var unlistedWMKeys = map[string]string{
	"outputs":         "edited by the Displays arrangement, which writes the output rules itself",
	"primary_output":  "set by the Displays page's Primary display switch on the selected display",
	"virtual_outputs": "added, sized and removed on the Displays arrangement, which writes the list itself",
}

func TestEveryWMConfigKeyHasARow(t *testing.T) {
	k := testKit(t, "")
	w, err := k.wmBackend()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]bool{}
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			for _, sec := range p.sections {
				for _, r := range sec.flatRows() {
					if p.wm || r.wm {
						for _, path := range append([]string{r.path}, r.also...) {
							rows[path] = true
						}
					}
					if r.wmTwin != "" {
						rows[r.wmTwin] = true
					}
				}
			}
		}
	}
	leaves := map[string]bool{}
	for _, l := range w.schema.Leaves() {
		path := strings.Join(l, ".")
		leaves[path] = true
		_, unlisted := unlistedWMKeys[path]
		switch covered := rowCovers(rows, path); {
		case !covered && !unlisted:
			t.Errorf("lys-wm's %s has no row on any page", path)
		case covered && unlisted:
			t.Errorf("lys-wm's %s has a row but is listed as having none", path)
		}
	}
	if len(leaves) < 150 {
		t.Fatalf("only %d lys-wm leaves; the walk missed the schema", len(leaves))
	}
	for key := range unlistedWMKeys {
		if !leaves[key] {
			t.Errorf("%s is listed as having no row but is no lys-wm key", key)
		}
	}
}

// Every lys-wm choice a page offers reads by name, a list's choices
// too: the title bar's buttons read as their raw close, minimize and
// maximize while a list's element took its type from the list marker.
func TestEveryLysWMChoiceOnAPageIsNamed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := testKit(t, "").wmBackend()
	if err != nil {
		t.Fatal(err)
	}
	tr := locale.T()
	var missing []string
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			for _, sec := range p.sections {
				for _, r := range sec.flatRows() {
					if !r.wm && !p.wm {
						continue
					}
					meta, ok := w.field(r.path)
					if elem, listed := w.field(r.path + "." + wmconfig.Each); listed && meta.Kind != config.FieldEnum {
						meta, ok = elem, true
					}
					if !ok || meta.Kind != config.FieldEnum || meta.Type == wmToggleType {
						continue
					}
					for _, v := range meta.Variants {
						if key := config.EnumLabelKey(meta.Type, v); !tr.Has(key) && !slices.Contains(missing, key) {
							missing = append(missing, key)
						}
					}
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("lys-wm choices without a label:\n%s", strings.Join(missing, "\n"))
	}
}

func TestTheTitleBarSideIsPickedFromPictures(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := testKit(t, "")
	w, err := k.wmBackend()
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := w.field("titlebars.side")
	if !ok {
		t.Fatal("lys-wm has no titlebars.side")
	}
	var stored any = "right"
	r := wmField("titlebars.side", pictureChoice(titlebarSidePictures))
	c := r.editor(k, slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() {}}, meta).(*choiceCards)
	if len(c.cards) != 2 || c.cards[0].label != locale.T().Get("enum-wm-side-left") || c.current != "right" {
		t.Fatalf("%d cards, first %q, current %q", len(c.cards), c.cards[0].label, c.current)
	}
	c.cards[0].PressAt(widget.Point{})
	if stored != "left" {
		t.Errorf("the left card stored %v", stored)
	}
}

func TestOuterGapsWriteOneNumberOrFour(t *testing.T) {
	k, w, _, files := wmKit(t, "")
	c := fieldControl(k, w, wmSides("gaps.outer")).(*sidesEditor)
	if !c.all.Empty() || !c.same.On() || widget.IsEnabled(c.same) {
		t.Fatal("an inherited gap shows a value or offers to split")
	}
	c.all.KeyAction(widget.KeyUp, 0)
	if got := fileText(t, files.runtime); !strings.Contains(got, "outer: 1") {
		t.Fatalf("setting an inherited gap: %q", got)
	}
	c.refresh()
	c.same.OnChanged(false)
	c.refresh()
	if got := fileText(t, files.runtime); !strings.Contains(got, "- 1") || !c.diagram.Visible() {
		t.Fatalf("splitting equal sides wrote %q, diagram %v", got, c.diagram.Visible())
	}
	c.spins[0].KeyAction(widget.KeyUp, 0)
	if got := fileText(t, files.runtime); !strings.Contains(got, "- 2") {
		t.Fatalf("one side changed: %q", got)
	}
	c.refresh()
	c.same.OnChanged(true)
	c.refresh()
	if got := fileText(t, files.runtime); !strings.Contains(got, "outer: 2") || c.diagram.Visible() {
		t.Fatalf("joining the sides wrote %q, diagram %v", got, c.diagram.Visible())
	}
	c.all.Clear()
	c.all.OnCleared()
	c.refresh()
	if got := fileText(t, files.runtime); strings.Contains(got, "outer") || !c.all.Empty() {
		t.Fatalf("inheriting kept %q", got)
	}
}

func TestSidesThatTakeNoSingleNumberWriteFourEvenWhenEqual(t *testing.T) {
	k, _, _, _ := wmKit(t, "")
	var stored any
	c := newSidesEditor(k, slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() { stored = nil }}, false)
	if !c.diagram.Visible() {
		t.Fatal("a key with no single number hides its sides")
	}
	c.spins[0].KeyAction(widget.KeyUp, 0)
	got, _ := stored.([]any)
	if want := []any{int64(1), int64(1), int64(1), int64(1)}; !slices.Equal(got, want) {
		t.Fatalf("stored %v, want %v", stored, want)
	}
}

func TestTheSplitIsTheStoredFormAndARefusalReturnsToIt(t *testing.T) {
	k, _, _, _ := wmKit(t, "")
	var stored any = int64(6)
	refuse := false
	c := newSidesEditor(k, slot{get: func() any { return stored }, set: func(v any) error {
		if refuse {
			return errors.New("refused")
		}
		stored = v
		return nil
	}, unset: func() {}}, true)
	refuse = true
	c.same.OnChanged(false)
	c.refresh()
	if !c.same.On() || c.diagram.Visible() {
		t.Fatalf("a refused split shows same %v, diagram %v", c.same.On(), c.diagram.Visible())
	}
	stored = []any{int64(1), int64(2), int64(3), int64(4)}
	c.refresh()
	if c.same.On() || !c.diagram.Visible() || c.spins[2].Value() != 3 {
		t.Errorf("stored sides show same %v, diagram %v, bottom %v", c.same.On(), c.diagram.Visible(), c.spins[2].Value())
	}
}

func TestLysWMSpeedsAndRepeatSlideOverTheirUsualRange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := testKit(t, "")
	w, err := k.wmBackend()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"input.pointer.accel_speed", "input.touchpad.accel_speed", "input.trackpoint.accel_speed"} {
		if _, ok := fieldControl(k, w, wmField(path)).(*slider); !ok {
			t.Errorf("%s is not a slider", path)
		}
	}
	rows := map[string]bool{}
	for _, sec := range wmKeyboardPage().sections {
		for _, r := range sec.rows {
			if r.editor != nil {
				rows[r.path] = true
			}
		}
	}
	if !rows["input.keyboard.repeat_rate"] || !rows["input.keyboard.repeat_delay"] {
		t.Errorf("the repeat rows keep the plain number: %v", rows)
	}
	if _, ok := fieldControl(k, w, wmField("input.pointer.scroll_factor")).(*slider); ok {
		t.Error("a key outside the presentation table became a slider")
	}
}

func TestHotCornersArePickedOnAScreen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := testKit(t, "")
	if _, err := k.wmBackend(); err != nil {
		t.Fatal(err)
	}
	stored := map[string]any{}
	slots := make([]slot, len(hotCornerKeys))
	paths := make([]string, len(hotCornerKeys))
	for i, c := range hotCornerKeys {
		path := "hot_corners." + c
		paths[i] = path
		slots[i] = slot{get: func() any { return stored[path] }, set: func(v any) error { stored[path] = v; return nil }, unset: func() { delete(stored, path) }}
	}
	h := newHotCorners(k, paths, slots)
	if h.selected != 0 || h.set(0) {
		t.Fatalf("the screen opens on corner %d, set %v", h.selected, h.set(0))
	}
	h.screen.handles[3].KeyAction(widget.KeyEnter, 0)
	if h.selected != 3 || h.title.Text() != locale.T().Get(wmKey(paths[3])) {
		t.Fatalf("choosing the bottom right shows %d %q", h.selected, h.title.Text())
	}
	stored[paths[3]] = map[string]any{"action": "overview"}
	stored[paths[1]] = false
	if !h.set(3) || h.set(1) || h.set(2) {
		t.Errorf("set corners read %v %v %v", h.set(3), h.set(1), h.set(2))
	}
}

func TestThePaletteIsAGridOfSwatchesThatPinsOnlyWhatChanges(t *testing.T) {
	k, w, _, files := wmKit(t, "")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	if !slices.Equal(p.shown, []string{"blue", "red"}) {
		t.Fatalf("the grid shows %q, want the built-in colours", p.shown)
	}
	for i, tile := range p.tiles {
		if _, ok := tile.(*colorEditor); !ok {
			t.Fatalf("%s is a %T, not the colour button alone: lys-wm's palette takes no names", p.shown[i], tile)
		}
	}
	if err := p.colorSlot("red").set("#ff0000"); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, "red: '#ff0000'") || strings.Contains(got, "blue") {
		t.Fatalf("changing red wrote %q", got)
	}
	p.name.SetText("mint")
	p.add()
	if !slices.Contains(p.shown, "mint") {
		t.Fatalf("an added colour is missing: %q", p.shown)
	}
	p.name.SetText("blue")
	p.add()
	if !p.problem.Visible() || p.problem.Text() != locale.T().Get("wm-palette-taken", i18n.Str("name", "blue")) {
		t.Errorf("a taken name says %q", p.problem.Text())
	}
	p.reset("red")
	if got := fileText(t, files.runtime); strings.Contains(got, "red") {
		t.Errorf("resetting red kept %q", got)
	}
}
