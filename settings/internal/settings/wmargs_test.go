package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

func grammarOf(t *testing.T, actions []wmconfig.Action, name string) argGrammar {
	t.Helper()
	i := slices.IndexFunc(actions, func(a wmconfig.Action) bool { return a.Name == name })
	if i < 0 {
		t.Fatalf("lys-wm has no action %s", name)
	}
	g, err := parseArgGrammar(actions[i].Args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return g
}

func TestLysWMsArgumentsMatchTheirFormAndWriteBackAlike(t *testing.T) {
	actions := lysWMActions(t)
	for _, tc := range []struct {
		action, args string
		alts         []int
	}{
		{"focus", "left", []int{0}},
		{"workspace", "3", []int{0}},
		{"workspace", "previous", []int{7}},
		{"workspace", "chat", []int{1}},
		{"workspace", "e+1", []int{2}},
		{"movetoworkspace", "3 follow", []int{0, 0}},
		{"movetoworkspace", "3", []int{0, -1}},
		{"columnwidth", "+10%", []int{2}},
		{"columnwidth", "preset next", []int{0}},
		{"columnwidth", "-40px", []int{6}},
		{"scratchpad", "toggle term", []int{0}},
		{"casttarget", "window", []int{0}},
		{"casttarget", "window 5", []int{0}},
		{"global", "org.example.App:start", []int{0}},
		{"exec", "grim -g \"$(slurp)\" -", []int{0}},
		{"swapworkspaces", "left DP-1", []int{0, 0}},
		{"resizeactive", "-20 0", []int{0, 0}},
		{"zoom", "in 1.5", []int{0, 0}},
		{"fullscreen", "", []int{-1, -1}},
		{"overview", "toggle app", []int{0, 2}},
	} {
		g := grammarOf(t, actions, tc.action)
		v, ok := g.match(tc.args)
		if !ok {
			t.Errorf("%s %q does not match its form", tc.action, tc.args)
			continue
		}
		alts := make([]int, len(v))
		for i, s := range v {
			alts[i] = s.alt
		}
		if !slices.Equal(alts, tc.alts) {
			t.Errorf("%s %q takes choices %v, want %v", tc.action, tc.args, alts, tc.alts)
		}
		if got := g.format(v); got != strings.Join(strings.Fields(tc.args), " ") {
			t.Errorf("%s %q writes back as %q", tc.action, tc.args, got)
		}
	}
	for _, tc := range []struct{ action, args string }{
		{"focus", "sideways"},
		{"focus", ""},
		{"workspace", "3 follow"},
		{"columnwidth", "wide"},
		{"resizeactive", "a b"},
		{"kill", "now"},
		{"zoom", "in fast"},
	} {
		if v, ok := grammarOf(t, actions, tc.action).match(tc.args); ok {
			t.Errorf("%s %q matches its form as %+v", tc.action, tc.args, v)
		}
	}
}

func TestAnArgumentGrammarTheEditorCannotBuildIsRefused(t *testing.T) {
	for _, text := range []string{
		"<damage|opaque> [toggle], heap <file>",
		"<left|right",
		"<file>",
		"<command> <n>",
		"[default|on]",
	} {
		if g, err := parseArgGrammar(text); !errors.Is(err, errArgGrammar) {
			t.Errorf("%q parsed as %+v, %v", text, g, err)
		}
	}
}

func TestEveryChoiceOfLysWMsArgumentsIsNamed(t *testing.T) {
	tr := locale.T()
	var missing []string
	for _, a := range lysWMActions(t) {
		g, err := parseArgGrammar(a.Args)
		if err != nil {
			if a.Name != "debug" {
				t.Errorf("%s: %v", a.Name, err)
			}
			continue
		}
		for _, s := range g {
			for _, alt := range s.alts {
				keys := []string{argAltKey(alt)}
				for _, w := range alt {
					for _, h := range w.holes() {
						keys = append(keys, string(h))
					}
				}
				for _, key := range keys {
					if id := "enum-wm-arg-" + key; !tr.Has(id) && !slices.Contains(missing, id) {
						missing = append(missing, id)
					}
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("argument choices without a label:\n%s", strings.Join(missing, "\n"))
	}
}

func wmActionEditorOver(t *testing.T, action string) (*wmActionEditor, *string) {
	t.Helper()
	actions := lysWMActions(t)
	k := testKit(t, "")
	stored := action
	s := slot{get: func() any { return stored }, set: func(v any) error { stored, _ = v.(string); return nil }, unset: func() { stored = "" }}
	return newWMActionEditor(k, s, actions), &stored
}

func TestAnActionsArgumentsArePickedFromItsForm(t *testing.T) {
	e, stored := wmActionEditorOver(t, "move left")
	if e.picked.Text() != "Move window left" || e.outside.Visible() {
		t.Fatalf("the action reads %q, outside %v", e.picked.Text(), e.outside.Visible())
	}
	seg := treetest.First[*choiceControl](t, e.args)
	if got := seg.pick.Items(); !slices.Equal(got, []string{"Left", "Right", "Up", "Down"}) || seg.pick.segments == nil {
		t.Fatalf("the directions read %q", got)
	}
	pickChoice(seg.pick, 3)
	if *stored != "move down" {
		t.Fatalf("picking Down stored %q", *stored)
	}

	e, stored = wmActionEditorOver(t, "workspace 3")
	n := treetest.First[*number](t, e.args)
	if n.Value() != 3 {
		t.Fatalf("the workspace number reads %v", n.Value())
	}
	n.OnValueChanged(5)
	if *stored != "workspace 5" {
		t.Fatalf("a new number stored %q", *stored)
	}
	alts := treetest.First[*choiceControl](t, e.args)
	pickChoice(alts.pick, slices.Index(shownValues(alts), "previous"))
	if *stored != "workspace previous" {
		t.Fatalf("picking previous stored %q", *stored)
	}
	if len(treetest.All[*number](e.args)) != 0 {
		t.Fatal("a choice without a number still shows one")
	}

	e, stored = wmActionEditorOver(t, "movetoworkspace 2")
	toggle := treetest.First[*toggle](t, e.args)
	if toggle.On() {
		t.Fatal("follow shows on without the word")
	}
	toggle.OnChanged(true)
	if *stored != "movetoworkspace 2 follow" {
		t.Fatalf("following stored %q", *stored)
	}

	e, stored = wmActionEditorOver(t, "exec foot")
	ce := treetest.First[*choiceEditor](t, e.args)
	if ce.combo.Action() == nil || ce.entry.Text() != "foot" {
		t.Fatalf("the command is %q, app button %v", ce.entry.Text(), ce.combo.Action() != nil)
	}
	ce.pick("firefox --new-window")
	if *stored != "exec firefox --new-window" {
		t.Fatalf("a picked command stored %q", *stored)
	}
}

func TestArgumentsOutsideTheirFormStayTextAndMarked(t *testing.T) {
	e, stored := wmActionEditorOver(t, "focus sideways")
	if !e.outside.Visible() {
		t.Fatal("arguments outside the form are not marked")
	}
	entry := treetest.First[*text](t, e.args)
	if entry.Text() != "sideways" {
		t.Fatalf("the arguments read %q", entry.Text())
	}
	entry.OnActivate("left")
	if *stored != "focus left" || e.outside.Visible() || len(treetest.All[*choiceControl](e.args)) != 1 {
		t.Fatalf("typing a form's arguments stored %q, outside %v", *stored, e.outside.Visible())
	}

	e, _ = wmActionEditorOver(t, "debug damage on")
	if e.outside.Visible() || treetest.First[*text](t, e.args).Text() != "damage on" {
		t.Fatal("an action whose form the editor does not build is not plain text")
	}
	e, _ = wmActionEditorOver(t, "kill")
	if e.args.Visible() || e.outside.Visible() {
		t.Fatal("an action without arguments shows a form")
	}
}

func TestPickingAnActionStartsItsForm(t *testing.T) {
	e, stored := wmActionEditorOver(t, "kill")
	choices := e.choices()
	if len(choices) != len(e.names.actions) || choices[0].label == "" {
		t.Fatalf("the picker offers %d of %d actions", len(choices), len(e.names.actions))
	}
	if i := slices.IndexFunc(choices, func(c choice) bool { return c.value == "kill" }); choices[i].label != "Close window" || choices[i].group != "kill" {
		t.Fatalf("kill is offered as %+v", choices[i])
	}
	e.choose("focus")
	if *stored != "focus left" || len(treetest.All[*choiceControl](e.args)) != 1 {
		t.Fatalf("picking focus stored %q", *stored)
	}
	e.choose("workspace")
	if *stored != "workspace 1" {
		t.Fatalf("picking workspace stored %q", *stored)
	}
	e.choose("fullscreen")
	if *stored != "fullscreen" {
		t.Fatalf("picking fullscreen stored %q", *stored)
	}
	if got := len(treetest.All[*widget.Box](e.args)); got == 0 {
		t.Fatal("fullscreen's optional choices are not offered")
	}
}
