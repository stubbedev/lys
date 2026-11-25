package barview

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func sampleButton(t *testing.T, bar *widget.Bar, module string) *Button {
	t.Helper()
	for _, w := range treetest.WithClass(bar, module) {
		if b := treetest.All[*Button](w); len(b) == 1 {
			return b[0]
		}
	}
	t.Fatalf("the preview has no %s button", module)
	return nil
}

func TestThePreviewDrawsEveryItemOfTheLayout(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar.Layout = []config.BarLayout{{
		Monitor: AllMonitors, Show: true,
		Left:   []config.BarItem{{Module: config.ModuleWorkspaces}, {Module: config.ModuleCava}},
		Center: []config.BarItem{{Module: config.ModuleClock, Class: "mine"}},
		Right: []config.BarItem{
			{Module: config.ModuleBattery},
			{Module: config.ModuleSeparator},
			{Group: &config.BarGroup{Name: "status", Modules: []config.BarItem{{Module: config.ModuleNetwork}, {Module: config.ModuleAudio}}}},
		},
	}}
	bar, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri)
	if !ok {
		t.Fatal("the default layout shows no bar")
	}
	if got := []int{len(bar.Start().Children()), len(bar.Center().Children()), len(bar.End().Children())}; got[0] != 2 || got[1] != 1 || got[2] != 3 {
		t.Fatalf("zones hold %v items, want 2, 1 and 3", got)
	}
	clock := sampleButton(t, bar, "clock")
	want := ClockLabel(cfg.Clock, SampleTime, ClockNames(i18n.Shell()), func() string { return "" })
	if clock.Label().Text() != want || want == "" {
		t.Errorf("the clock shows %q, want the sample time in its format, %q", clock.Label().Text(), want)
	}
	if len(treetest.WithClass(bar, "mine")) != 1 {
		t.Error("the clock's instance class is missing")
	}
	if b := sampleButton(t, bar, "battery"); b.Label().Text() != "45%" || b.Icon() == nil || b.Icon().Name() != cfg.Battery.LevelIcon(45) {
		t.Errorf("the battery shows %q beside %v, want its sample level beside that level's icon", b.Label().Text(), b.Icon())
	}
	if len(treetest.All[*widget.BarSeparator](bar)) != 1 {
		t.Error("the separator is not a rule")
	}
	group := treetest.WithClass(bar, "bar-group")
	if len(group) != 1 || len(treetest.All[*Button](group[0])) != 2 {
		t.Errorf("the status group holds %d groups, want one with two buttons", len(group))
	}
	if b := sampleButton(t, bar, "cava"); b.Label().Text() != ModuleTitle("cava", cfg) {
		t.Errorf("a module without an icon of its own shows %q, want its title", b.Label().Text())
	}
	strips := treetest.All[*widget.SegmentIndicator](bar)
	if len(strips) != 1 || !strips[0].HasClass("workspaces") || !strips[0].HasClass("module") || len(treetest.All[*Button](strips[0])) != 0 {
		t.Fatalf("the workspaces module previews as %d strips, want one workspace strip in the module's chrome", len(strips))
	}
}

func TestThePreviewFollowsTheConfig(t *testing.T) {
	cfg := config.Defaults()
	before, _ := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri)
	if b := sampleButton(t, before, "battery"); b.HasClass("icon-only") || !b.Label().Visible() || before.Edge() != widget.BarTop {
		t.Fatalf("the default battery is icon-only %v with its label shown %v on edge %v", b.HasClass("icon-only"), b.Label().Visible(), before.Edge())
	}
	cfg.Battery.LabelShow = false
	cfg.Bar.ButtonVariant = config.ButtonBlockPrefix
	cfg.Bar.Location = config.LocationLeft
	after, _ := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri)
	b := sampleButton(t, after, "battery")
	if !b.HasClass("icon-only") || b.Label().Visible() {
		t.Errorf("label-show off: icon-only %v, label shown %v", b.HasClass("icon-only"), b.Label().Visible())
	}
	if !b.HasClass(config.ButtonBlockPrefix.CSSClass()) || after.Edge() != widget.BarLeft {
		t.Errorf("the block-prefix variant on the left: classes %v, edge %v", b.Classes(), after.Edge())
	}
}

func TestThereIsNoPreviewWhereTheConfigShowsNoBar(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar.Layout = []config.BarLayout{{Monitor: "DP-1", Show: true, Center: []config.BarItem{{Module: config.ModuleClock}}}}
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri); ok {
		t.Error("a layout for DP-1 alone previews a bar on every monitor")
	}
	cfg.Bar.Layout = []config.BarLayout{{Monitor: AllMonitors, Show: false, Center: []config.BarItem{{Module: config.ModuleClock}}}}
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri); ok {
		t.Error("a hidden layout previews a bar")
	}
	cfg.Bar.Layout = nil
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri); ok {
		t.Error("no layout previews a bar")
	}
}

func stripLabels(s *WorkspaceStrip) []string {
	var out []string
	for _, w := range treetest.WithClass(s.Segments(), "workspace-label") {
		out = append(out, w.(*widget.Label).Text())
	}
	return out
}

func TestTheWorkspaceSampleFollowsTheConfig(t *testing.T) {
	cfg := config.Defaults()
	font := treetest.Font(t)
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); !slices.Equal(got, []string{"web", "2"}) {
		t.Errorf("the default sample shows %v, want web and 2 with the empty last workspace hidden", got)
	}
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.LysWM)); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Errorf("lys-wm, which reads neither hide-trailing-empty nor label-strategy, shows %v, want 1, 2 and 3", got)
	}
	if s := SampleWorkspaces(cfg, font, compositor.Niri); !s.Segments().HasClass("workspaces") || s.Segments().Active() != 0 {
		t.Errorf("the sample strip has classes %v and active segment %d, want the workspaces row with the first active", s.Segments().Classes(), s.Segments().Active())
	}
	cfg.Workspaces.HideTrailingEmpty = false
	cfg.Workspaces.LabelStrategy = config.LabelIndex
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Errorf("hide-trailing-empty off under index shows %v, want 1, 2 and 3", got)
	}
	cfg.Workspaces.AppIconsShow = true
	s := SampleWorkspaces(cfg, font, compositor.Niri)
	if icons := treetest.WithClass(s.Segments(), "workspace-icon"); len(icons) != 3 || len(treetest.WithClass(s.Segments(), "workspace-icon-empty")) != 1 {
		t.Errorf("app-icons-show previews %d icons, want the first workspace's deduped icon, the second's and the empty one", len(icons))
	}
	cfg.Workspaces.WorkspaceMap = config.WorkspaceMap{"web": {Label: new("www")}}
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); len(got) == 0 || got[0] != "www" {
		t.Errorf("a workspace-map label previews %v, want www first", got)
	}
	tags := SampleWorkspaces(cfg, font, compositor.Mango)
	if got := stripLabels(tags); !slices.Equal(got, []string{"1", "2", "3"}) || len(treetest.WithClass(tags.Segments(), "tag-1")) != 1 {
		t.Errorf("mango previews tags %v, want its three numbered tags", got)
	}
}

func TestTheHyprlandSampleNumbersItsWorkspacesAsHyprlandDoes(t *testing.T) {
	cfg := config.Defaults()
	font := treetest.Font(t)
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Hyprland)); !slices.Equal(got, []string{"4", "5", "6"}) {
		t.Errorf("absolute numbering previews %v, want the ids 4, 5 and 6", got)
	}
	cfg.Workspaces.Numbering = config.NumberingRelative
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Hyprland)); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Errorf("relative numbering previews %v, want 1, 2 and 3 on the monitor", got)
	}
	cfg.Workspaces.LabelUseName = true
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Hyprland)); !slices.Equal(got, []string{"web", "2", "3"}) {
		t.Errorf("label-use-name previews %v, want the named workspace by its name", got)
	}
	cfg.Workspaces.WorkspaceMap = config.WorkspaceMap{"5": {Label: new("mail")}}
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Hyprland)); !slices.Equal(got, []string{"web", "mail", "3"}) {
		t.Errorf("a workspace-map label previews %v, want it over the number", got)
	}
	cfg.Workspaces.WorkspaceMap = nil
	cfg.Workspaces.LabelUseName = false
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); !slices.Equal(got, []string{"web", "2"}) {
		t.Errorf("Hyprland's numbering changes niri's sample to %v, want web and 2", got)
	}
}

func TestTheWorkspaceSampleLeavesOutWhatTheConfigHides(t *testing.T) {
	cfg := config.Defaults()
	font := treetest.Font(t)
	cfg.Workspaces.LabelStrategy = config.LabelNameOnly
	s := SampleWorkspaces(cfg, font, compositor.Niri)
	if got := stripLabels(s); !slices.Equal(got, []string{"web"}) || s.Segments().Count() != 2 {
		t.Errorf("name-only previews %v on %d segments, want the named workspace's label and a bare second segment", got, s.Segments().Count())
	}
	cfg.Workspaces.DisplayMode = config.DisplayModeNone
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); len(got) != 0 {
		t.Errorf("display-mode none previews labels %v", got)
	}
	cfg.Workspaces.WorkspaceIgnore = []string{"*"}
	if s := SampleWorkspaces(cfg, font, compositor.Niri); s.Segments().Count() != 0 || s.Segments().Active() != -1 {
		t.Errorf("ignoring every workspace previews %d segments, active %d", s.Segments().Count(), s.Segments().Active())
	}
}

func TestEveryFormattedModuleDeclaresItsPlaceholders(t *testing.T) {
	for m := range moduleFormats {
		if len(config.Placeholders(formatPath(m))) == 0 {
			t.Errorf("%s declares no placeholders to preview", formatPath(m))
		}
	}
}

func TestASampleLabelFollowsItsModulesFormat(t *testing.T) {
	cfg := config.Defaults()
	for _, tc := range []struct {
		module config.BarModule
		format *string
		set    string
		want   string
	}{
		{config.ModuleBattery, &cfg.Battery.Format, "Bat {{ percent }}", "Bat 45"},
		{config.ModuleCPU, &cfg.CPU.Format, "{{ percent }}% @ {{ freq_ghz }}", "45% @ 3.2"},
		{config.ModuleKeyboardInput, &cfg.KeyboardInput.Format, "{{ alias | lower }}", "en"},
		{config.ModuleMail, &cfg.Mail.Format, "Unread: {{ count }}", "Unread: 3"},
		{config.ModuleWeather, &cfg.Weather.Format, "{{ condition }}, {{ temp }}{{ temp_unit }}", "Partly cloudy, 22°C"},
	} {
		before, _ := sampleLabel(cfg, tc.module, true)
		*tc.format = tc.set
		if got, failure := sampleLabel(cfg, tc.module, true); got != tc.want || got == before || failure != "" {
			t.Errorf("%s format %q previews %q (was %q), want %q", tc.module, tc.set, got, before, tc.want)
		}
	}
}

func TestTheWorldClockAndCustomModulesPreviewTheirFormats(t *testing.T) {
	cfg := config.Defaults()
	cfg.WorldClock.Format = "TYO {{ tz('Asia/Tokyo', '%H:%M') }}"
	cfg.Custom = []config.CustomModuleDefinition{config.DefaultsCustomModuleDefinition()}
	cfg.Custom[0].Id, cfg.Custom[0].Format = "temp", "{{ output }}°C"
	layout := config.BarLayout{Monitor: AllMonitors, Show: true, Center: []config.BarItem{{Module: config.ModuleWorldClock}, {Module: config.CustomModule("temp")}}}
	bar := SampleBar(cfg, treetest.Font(t), layout, compositor.LysWM)
	clock := sampleButton(t, bar, "world-clock")
	if got := clock.Label().Text(); got != "TYO 18:41" || clock.HasClass(FailedClass) {
		t.Errorf("the world clock previews %q (failed %v), want the sample time in Tokyo", got, clock.HasClass(FailedClass))
	}
	if got := sampleButton(t, bar, "custom-temp").Label().Text(); got != "72°C" {
		t.Errorf("the custom module previews %q, want its format over the sample output", got)
	}
}

func TestAWorldClockFormatThatCannotRenderPreviewsAsTheFailedModule(t *testing.T) {
	cfg := config.Defaults()
	cfg.WorldClock.Format = "{{ tz('Mars/Olympus', '%H') }}"
	layout := config.BarLayout{Monitor: AllMonitors, Show: true, Center: []config.BarItem{{Module: config.ModuleWorldClock}}}
	clock := sampleButton(t, SampleBar(cfg, treetest.Font(t), layout, compositor.LysWM), "world-clock")
	if !clock.HasClass(FailedClass) || !strings.Contains(clock.TooltipText(), i18n.T("bar-world-clock-invalid-zone", i18n.Str("zone", "Mars/Olympus"))) {
		t.Errorf("an unknown zone previews failed %v with tooltip %q", clock.HasClass(FailedClass), clock.TooltipText())
	}
}

func TestASampleLabelShowsWhatTheModuleShowsForAFormatItCannotRender(t *testing.T) {
	if got := sampleFormatLabel(config.ModuleBattery, "{{ percent"); got != "" {
		t.Errorf("an unterminated template previews %q, want the empty label the module shows", got)
	}
	if got := sampleFormatLabel(config.ModuleBattery, "[{{ pending }}]"); got != "[]" {
		t.Errorf("an undeclared variable previews %q, want nothing in its place", got)
	}
	if got := sampleFormatLabel(config.ModuleMail, "{{ count | upper }}"); got != "{{ count | upper }}" {
		t.Errorf("mail, which replaces rather than renders, previews %q, want the format as written", got)
	}
}

func TestAStateShowsItsModulesRealStateInPlaceOfTheSamples(t *testing.T) {
	cfg := config.Defaults()
	cfg.KeybindMode.Format = "[{{ mode }}]"
	cfg.KeyboardInput.Format = "{{ alias }}"
	font := treetest.Font(t)
	layout := config.BarLayout{Monitor: AllMonitors, Show: true, Center: []config.BarItem{{Module: config.ModuleKeybindMode}, {Module: config.ModuleKeyboardInput}}}
	pressed := 0
	bar := SampleBar(cfg, font, layout, compositor.LysWM,
		KeybindModeState("move"),
		KeyboardInputState("German", map[string]string{"German": "DE"}).OnPress(func() { pressed++ }))
	if got := sampleButton(t, bar, "keybind-mode").Label().Text(); got != "[move]" {
		t.Errorf("keybind-mode in move shows %q, want [move]", got)
	}
	keyboard := sampleButton(t, bar, "keyboard-input")
	if got := keyboard.Label().Text(); got != "DE" {
		t.Errorf("keyboard-input on German aliased DE shows %q, want DE", got)
	}
	keyboard.ClickAt(widget.Point{})
	if pressed != 1 {
		t.Errorf("a press on the keyboard-input chip ran its state's press %d times, want once", pressed)
	}
	bar = SampleBar(cfg, font, layout, compositor.LysWM, KeybindModeState(""), KeyboardInputState("French", map[string]string{"German": "DE"}))
	if got := sampleButton(t, bar, "keybind-mode").Label().Text(); got != "["+i18n.T("bar-keybind-mode-default")+"]" {
		t.Errorf("keybind-mode in the default submap shows %q, want the default word", got)
	}
	if got := sampleButton(t, bar, "keyboard-input").Label().Text(); got != "French" {
		t.Errorf("keyboard-input on a layout without an alias shows %q, want the layout's own name", got)
	}
}

// Keybind Mode's format previews the mode the bar shows outside every
// submap, so its format sample and its bar preview read the same word.
func TestKeybindModesFormatSampleIsTheDefaultSubmapsWord(t *testing.T) {
	en, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID("en-US")})
	if err != nil {
		t.Fatal(err)
	}
	want := en.Get("bar-keybind-mode-default")
	if got := config.PlaceholderSamples(formatPath(config.ModuleKeybindMode))["mode"]; got != want {
		t.Errorf("keybind-mode's format sample is %q, want the en-US default submap's %q", got, want)
	}
	if got := KeybindModeVars("resize")["mode"]; got != "resize" {
		t.Errorf("keybind-mode in the resize submap names %q, want resize", got)
	}
}

func TestAModuleWithoutAStateKeepsItsSamples(t *testing.T) {
	cfg := config.Defaults()
	cfg.KeybindMode.Format = "[{{ mode }}]"
	layout := config.BarLayout{Monitor: AllMonitors, Show: true, Center: []config.BarItem{{Module: config.ModuleKeybindMode}}}
	bar := SampleBar(cfg, treetest.Font(t), layout, compositor.LysWM, KeyboardInputState("German", nil))
	if got, want := sampleButton(t, bar, "keybind-mode").Label().Text(), sampleFormatLabel(config.ModuleKeybindMode, "[{{ mode }}]"); got != want || got == "[move]" {
		t.Errorf("keybind-mode with only a keyboard state shows %q, want its sample %q", got, want)
	}
}
