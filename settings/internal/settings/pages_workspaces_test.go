package settings

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/workspaces/workspacestest"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func runCompositor(t *testing.T, env string) {
	t.Helper()
	for _, k := range []string{compositor.LysWMSocketEnv, compositor.HyprlandSocketEnv, compositor.NiriSocketEnv, compositor.MangoSocketEnv, compositor.SwaySocketEnv} {
		t.Setenv(k, "")
	}
	if env != "" {
		t.Setenv(env, "/run/test-socket")
	}
}

func workspaceKeysShown(t *testing.T) []string {
	t.Helper()
	var keys []string
	for _, s := range workspacesPage(config.Defaults()).sections {
		for _, r := range s.rows {
			keys = append(keys, strings.TrimPrefix(r.path, workspacesModule+"."))
		}
	}
	return keys
}

func TestTheWorkspacesPageShowsOnlyWhatTheRunningCompositorReads(t *testing.T) {
	for _, tc := range []struct {
		env        string
		shown, not []string
	}{
		{compositor.NiriSocketEnv, []string{"label-strategy", "hide-trailing-empty", "monitor-specific", "display-mode"}, []string{"show-special", "min-tag-count", "numbering"}},
		{compositor.HyprlandSocketEnv, []string{"show-special", "numbering", "active-on-other-monitor-color", "monitor-specific"}, []string{"label-strategy", "hide-empty"}},
		{compositor.MangoSocketEnv, []string{"hide-empty", "min-tag-count", "display-mode"}, []string{"monitor-specific", "workspace-ignore", "show-special"}},
		{compositor.LysWMSocketEnv, []string{"monitor-specific", "workspace-ignore"}, []string{"label-strategy", "hide-trailing-empty", "min-tag-count"}},
	} {
		runCompositor(t, tc.env)
		keys := workspaceKeysShown(t)
		for _, k := range tc.shown {
			if !slices.Contains(keys, k) {
				t.Errorf("%s: %s is not shown", tc.env, k)
			}
		}
		for _, k := range tc.not {
			if slices.Contains(keys, k) {
				t.Errorf("%s: %s is shown though the compositor does not read it", tc.env, k)
			}
		}
	}
}

func TestWithoutACompositorTheWorkspacesPageShowsEveryKey(t *testing.T) {
	runCompositor(t, "")
	keys := workspaceKeysShown(t)
	for _, k := range []string{"label-strategy", "show-special", "min-tag-count", "hide-empty", "monitor-specific"} {
		if !slices.Contains(keys, k) {
			t.Errorf("%s is not shown without a compositor", k)
		}
	}
}

func TestTheWorkspacesPageNamesTheCompositorItFollows(t *testing.T) {
	k := testKit(t, "")
	runCompositor(t, compositor.SwaySocketEnv)
	if got := strings.Join(treetest.Labels(workspacesCompositorNote(k, sessionCompositor())), " "); !strings.Contains(got, "sway") {
		t.Errorf("under sway the note reads %q", got)
	}
	runCompositor(t, "")
	if got := strings.Join(treetest.Labels(workspacesCompositorNote(k, sessionCompositor())), " "); strings.Contains(got, "sway") || got == "" {
		t.Errorf("without a compositor the note reads %q", got)
	}
}

func workspaceDivider(t *testing.T, k *kit) *choiceControl {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "workspaces", workspacesModule+".divider")).(*choiceControl)
	if !ok {
		t.Fatal("the workspace divider is not picked from a list")
	}
	return c
}

func TestTheWorkspaceDividerIsPickedFromCommonSeparators(t *testing.T) {
	k := testKit(t, "")
	c := workspaceDivider(t, k)
	if len(c.pick.Items()) != 6 || shownValues(c)[c.pick.Selected()] != " " {
		t.Fatalf("offers %q with %q selected", c.pick.Items(), shownValues(c)[c.pick.Selected()])
	}
	pickChoice(c.pick, slices.Index(shownValues(c), " · "))
	if v := k.store.value(workspacesModule + ".divider"); v != " · " {
		t.Fatalf("stored %q", v)
	}
	pickChoice(c.pick, slices.Index(shownValues(c), ""))
	if v := k.store.value(workspacesModule + ".divider"); v != "" {
		t.Fatalf("none stored %q", v)
	}
}

func TestAHandEditedWorkspaceDividerIsKeptUntilAnotherIsPicked(t *testing.T) {
	k := testKit(t, "modules:\n  workspaces:\n    divider: \" » \"\n")
	c := workspaceDivider(t, k)
	if len(c.pick.Items()) != 7 || shownValues(c)[c.pick.Selected()] != " » " {
		t.Fatalf("offers %q with %q selected", c.pick.Items(), shownValues(c)[c.pick.Selected()])
	}
	if v := k.store.value(workspacesModule + ".divider"); v != " » " {
		t.Fatalf("a hand-edited divider was rewritten to %q", v)
	}
	pickChoice(c.pick, slices.Index(shownValues(c), " | "))
	c.refresh()
	if len(c.pick.Items()) != 6 || k.store.value(workspacesModule+".divider") != " | " {
		t.Fatalf("after picking | it offers %q, stored %q", c.pick.Items(), k.store.value(workspacesModule+".divider"))
	}
}

const workspaceIgnorePath = workspacesModule + ".workspace-ignore"

func workspaceIgnoreChips(t *testing.T, k *kit, on compositor.Kind) *nameChips {
	t.Helper()
	return fieldControl(k, k.store, field(workspaceIgnorePath, workspaceIgnoreRow(on))).(*nameChips)
}

func TestUnderLysWMEachWorkspaceItKeepsIsAChipThatHidesIt(t *testing.T) {
	k, _, fake, _ := wmKit(t, "")
	fake.running = map[string]string{"workspaces": `[{"id":2,"output":"DP-1"},{"id":1,"output":"DP-1"},{"id":5,"output":"HDMI-A-1"}]`}
	if err := k.store.set(workspaceIgnorePath, []any{"scratch*"}); err != nil {
		t.Fatal(err)
	}
	c := workspaceIgnoreChips(t, k, compositor.LysWM)
	if want := []string{"1", "2", "5", "scratch*"}; !slices.Equal(c.labels, want) || c.unread.Visible() {
		t.Fatalf("chips %q (unread %v), want %q", c.labels, c.unread.Visible(), want)
	}
	if !slices.Equal(c.chips.Checked(), []int{3}) {
		t.Fatal("the chips do not show what is hidden")
	}
	pressChip(c.chips, 1)
	if got := k.store.value(workspaceIgnorePath); !reflect.DeepEqual(got, []any{"scratch*", "2"}) {
		t.Fatalf("hiding workspace 2 stored %v", got)
	}
}

func TestWithoutARunningLysWMTheWorkspaceChipsSayWhy(t *testing.T) {
	k, _, _, _ := wmKit(t, "")
	c := workspaceIgnoreChips(t, k, compositor.LysWM)
	wantClosedFailure(t, "the workspace chips without lys-wm", c.unread.stateParts, c.unread.Text(), locale.T().Get("settings-names-unread"), "no lys-wm is running")
	if len(c.labels) != 0 {
		t.Fatalf("chips %q", c.labels)
	}
}

func TestUnderHyprlandNiriAndSwayEachWorkspaceIsAChipThatHidesIt(t *testing.T) {
	for _, tc := range []struct {
		on     compositor.Kind
		fake   func(testing.TB, string)
		reply  string
		labels []string
		hidden string
	}{
		{compositor.Hyprland, workspacestest.Hyprland, `[{"id":2,"name":"2"},{"id":-1337,"name":"web"},{"id":1,"name":"1"}]`, []string{"-1337", "1", "2", "scratch*"}, "-1337"},
		{compositor.Niri, workspacestest.Niri, `[{"id":7,"idx":2,"name":null},{"id":3,"idx":1,"name":"chat"}]`, []string{"2", "chat", "scratch*"}, "chat"},
		{compositor.Sway, workspacestest.Sway, `[{"id":5,"num":10,"name":"10"},{"id":4,"num":-1,"name":"web"},{"id":3,"num":1,"name":"1"}]`, []string{"1", "10", "web", "scratch*"}, "10"},
	} {
		t.Run(tc.on.String(), func(t *testing.T) {
			tc.fake(t, tc.reply)
			k := testKit(t, "")
			if err := k.store.set(workspaceIgnorePath, []any{"scratch*"}); err != nil {
				t.Fatal(err)
			}
			c := workspaceIgnoreChips(t, k, tc.on)
			if !slices.Equal(c.labels, tc.labels) || c.unread.Visible() {
				t.Fatalf("chips %q (unread %v %q), want %q", c.labels, c.unread.Visible(), c.unread.Text(), tc.labels)
			}
			at := slices.Index(c.labels, tc.hidden)
			if !slices.Equal(c.chips.Checked(), []int{len(c.labels) - 1}) {
				t.Fatal("the chips do not show what is hidden")
			}
			pressChip(c.chips, at)
			if got := k.store.value(workspaceIgnorePath); !reflect.DeepEqual(got, []any{"scratch*", tc.hidden}) {
				t.Fatalf("hiding %s stored %v", tc.hidden, got)
			}
		})
	}
}

func TestWithoutTheCompositorsSocketTheWorkspaceChipsSayWhy(t *testing.T) {
	t.Setenv(compositor.SwaySocketEnv, filepath.Join(t.TempDir(), "gone.sock"))
	k := testKit(t, "")
	c := workspaceIgnoreChips(t, k, compositor.Sway)
	wantClosedFailure(t, "the workspace chips without sway's socket", c.unread.stateParts, c.unread.Text(), locale.T().Get("settings-names-unread"), "sway")
	if len(c.labels) != 0 {
		t.Fatalf("chips %q", c.labels)
	}
}

func TestATypedWorkspacePatternIsHiddenOnce(t *testing.T) {
	k := testKit(t, "")
	c := workspaceIgnoreChips(t, k, compositor.Unknown)
	tr := locale.T()
	if len(c.labels) != 0 || c.unread.Visible() {
		t.Fatalf("without a compositor the chips offer %q, unread %v", c.labels, c.unread.Visible())
	}
	c.entry.SetText("  ")
	c.add()
	if got := k.store.value(workspaceIgnorePath); c.problem.Text() != tr.Get("settings-workspace-ignore-empty") || !reflect.DeepEqual(got, []any{}) {
		t.Fatalf("an empty pattern shows %q, stored %v", c.problem.Text(), got)
	}
	c.entry.SetText(" 1? ")
	c.add()
	if got := k.store.value(workspaceIgnorePath); !reflect.DeepEqual(got, []any{"1?"}) || c.problem.Visible() || !slices.Equal(c.labels, []string{"1?"}) {
		t.Fatalf("stored %v, problem %v, chips %q", got, c.problem.Visible(), c.labels)
	}
	c.entry.SetText("1?")
	c.add()
	if c.problem.Text() != tr.Get("settings-workspace-ignore-listed") {
		t.Fatalf("a repeat shows %q", c.problem.Text())
	}
}

const windowPreviewsPath = workspacesModule + ".window-previews"

func TestEveryCompositorShowsTheWorkspaceWindowPreviewsRow(t *testing.T) {
	for _, env := range []string{compositor.LysWMSocketEnv, compositor.HyprlandSocketEnv, compositor.NiriSocketEnv, compositor.MangoSocketEnv, compositor.SwaySocketEnv, ""} {
		runCompositor(t, env)
		if !slices.Contains(workspaceKeysShown(t), "window-previews") {
			t.Errorf("%q: window-previews is not shown", env)
		}
	}
}

func TestTheWorkspaceWindowPreviewsRowWritesTheKey(t *testing.T) {
	k := testKit(t, "")
	spec := pageRow(t, "workspaces", windowPreviewsPath)
	if label := locale.T().Get(spec.labelKey()); label == "" || label == spec.labelKey() {
		t.Errorf("the row reads %q", label)
	}
	c, ok := fieldControl(k, k.store, spec).(*toggle)
	if !ok {
		t.Fatalf("the row is a %T, want a switch", fieldControl(k, k.store, spec))
	}
	if c.On() {
		t.Fatal("window previews are on by default")
	}
	c.Toggle()
	if v := k.store.value(windowPreviewsPath); v != true {
		t.Fatalf("turning previews on stored %v, want true", v)
	}
	c.Toggle()
	if v := k.store.value(windowPreviewsPath); v != false {
		t.Fatalf("turning previews off stored %v, want false", v)
	}
	k2 := testKit(t, "modules:\n  workspaces:\n    window-previews: true\n")
	if c := fieldControl(k2, k2.store, spec).(*toggle); !c.On() {
		t.Error("with window-previews: true the switch is off")
	}
}
