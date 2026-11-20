package settings

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func terminalAppsFixture(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	app := func(id, keys string) {
		put(t, filepath.Join(data, "applications", id), "[Desktop Entry]\nType=Application\nExec=true\n"+keys)
	}
	app("foot.desktop", "Name=Foot\nIcon=foot\nCategories=System;TerminalEmulator;\n")
	app("org.wezfurlong.wezterm.desktop", "Name=WezTerm\nIcon=wezterm\nCategories=System;TerminalEmulator;Utility;\n")
	app("firefox.desktop", "Name=Firefox\nIcon=firefox\nCategories=Network;WebBrowser;\n")
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "none"))
	t.Setenv("XDG_CURRENT_DESKTOP", "lys-wm")
}

func swallowOf(t *testing.T, p *settingsPage) *swallowTerminals {
	t.Helper()
	c, ok := rowOf(t, p, swallowTerminalsPath).ctl.(*swallowTerminals)
	if !ok {
		t.Fatalf("the %s row holds %T, want the terminal list", swallowTerminalsPath, rowOf(t, p, swallowTerminalsPath).ctl)
	}
	return c
}

func pickTerminal(t *testing.T, k *kit, c *swallowTerminals, name string) []string {
	t.Helper()
	f := &fakePickers{}
	k.pickers = f
	c.strip.add.ClickAt(widget.Point{})
	if f.content == nil {
		t.Fatal("the terminal picker did not open")
	}
	flow := treetest.First[*widget.FlowBox](t, f.content)
	offered := shownLabels(flow)
	at := slices.Index(offered, name)
	if at < 0 {
		t.Fatalf("%s is not offered among %q", name, offered)
	}
	flow.OnActivate(at)
	return offered
}

func TestTerminalsArePickedAndWrittenAsTheirExactAppIDs(t *testing.T) {
	terminalAppsFixture(t)
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-windows")
	c := swallowOf(t, p)
	except := rowOf(t, p, swallowExceptPath)
	if !c.strip.Visible() || c.written.Visible() || !c.strip.none.Visible() {
		t.Fatal("with no terminals the row is not an empty list")
	}
	if widget.IsEnabled(except) {
		t.Error("the titles that never swallow are editable while nothing swallows")
	}
	if offered := pickTerminal(t, k, c, "Foot"); !slices.Equal(offered, []string{"Foot", "WezTerm"}) {
		t.Fatalf("the picker offers %q, want the installed terminals only", offered)
	}
	acceptedByLysWM(t, k)
	if got := k.wm.value(swallowTerminalsPath); got != "^(foot)$" {
		t.Fatalf("picking Foot wrote %v", got)
	}
	if offered := pickTerminal(t, k, c, "WezTerm"); slices.Contains(offered, "Foot") {
		t.Errorf("the picker offers Foot, which is listed already: %q", offered)
	}
	acceptedByLysWM(t, k)
	if got := k.wm.value(swallowTerminalsPath); got != `^(foot|org\.wezfurlong\.wezterm)$` {
		t.Fatalf("adding WezTerm wrote %v", got)
	}
	if got := texts(c.strip.chips); !slices.Equal(got, []string{"Foot", "WezTerm"}) {
		t.Fatalf("the list reads %q", got)
	}
	if !widget.IsEnabled(except) {
		t.Error("the titles that never swallow stay disabled while terminals swallow")
	}
	c.strip.drop(0)
	c.strip.drop(0)
	acceptedByLysWM(t, k)
	if got := k.wm.value(swallowTerminalsPath); got != nil {
		t.Fatalf("removing every terminal left %v", got)
	}
}

func TestAHandWrittenPatternIsKeptUntilReplacedByAList(t *testing.T) {
	terminalAppsFixture(t)
	k := lysWMKit(t, "swallow:\n  terminals: ^(foot|kitty).*$\n  except_title: wev\n")
	p := layoutPage(t, k, "wm-windows")
	c := swallowOf(t, p)
	if c.strip.Visible() || !c.written.Visible() || c.pattern.Text() != "^(foot|kitty).*$" {
		t.Fatalf("a hand-written pattern shows list %v, written %v reading %q", c.strip.Visible(), c.written.Visible(), c.pattern.Text())
	}
	c.refresh()
	if got := k.wm.value(swallowTerminalsPath); got != "^(foot|kitty).*$" || len(k.wm.overrides) != 0 {
		t.Fatalf("showing the pattern rewrote it: %v, overrides %v", got, k.wm.overrides)
	}
	treetest.Button(t, c.written, locale.T().Get("wm-swallow-terminals-replace")).ClickAt(widget.Point{})
	acceptedByLysWM(t, k)
	if got := k.wm.value(swallowTerminalsPath); got != nil {
		t.Fatalf("replacing the pattern left %v", got)
	}
	if !c.strip.Visible() || c.written.Visible() || c.strip.chips.Len() != 0 {
		t.Fatal("after replacing, the row is not an empty list")
	}
	pickTerminal(t, k, c, "Foot")
	acceptedByLysWM(t, k)
	if got := k.wm.value(swallowTerminalsPath); got != "^(foot)$" {
		t.Fatalf("picking Foot after replacing wrote %v", got)
	}
}

func TestOnlyAnExactMatchOfAppIDsReadsAsAList(t *testing.T) {
	for pattern, want := range map[string][]string{
		"^(foot)$": {"foot"},
		"^foot$":   {"foot"},
		`^(foot|kitty|Alacritty|org\.wezfurlong\.wezterm)$`: {"foot", "kitty", "Alacritty", "org.wezfurlong.wezterm"},
	} {
		got, ok := terminalIDs(pattern)
		if !ok || !slices.Equal(got, want) {
			t.Errorf("%q reads as %q %v, want %q", pattern, got, ok, want)
		}
		if ok && len(got) > 1 && terminalsPattern(got) != pattern {
			t.Errorf("%q writes back as %q", pattern, terminalsPattern(got))
		}
	}
	for _, pattern := range []string{
		"foot", "^foot", "foot$", "^(foot|kitty).*$", "^(foot|)$", "^(foot|foot)$", "^(foo.t)$", `^(foot\)$`,
		"^(foot)|(kitty)$", "^(a(b))$", `^(foot\`, "^()$", "^$",
	} {
		if got, ok := terminalIDs(pattern); ok {
			t.Errorf("%q reads as the list %q", pattern, got)
		}
	}
	if got := terminalsPattern([]string{"org.wezfurlong.wezterm", "a+b"}); got != `^(org\.wezfurlong\.wezterm|a\+b)$` {
		t.Errorf("the pattern for two apps is %q", got)
	}
}
