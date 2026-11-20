package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
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

func workspaceDivider(t *testing.T, k *kit) *valueSelect {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "workspaces", workspacesModule+".divider")).(*valueSelect)
	if !ok {
		t.Fatal("the workspace divider is not picked from a list")
	}
	return c
}

func TestTheWorkspaceDividerIsPickedFromCommonSeparators(t *testing.T) {
	k := testKit(t, "")
	c := workspaceDivider(t, k)
	if len(c.Items()) != 6 || c.shown[c.Selected()] != " " {
		t.Fatalf("offers %q with %q selected", c.Items(), c.shown[c.Selected()])
	}
	c.OnSelect(slices.Index(c.shown, " · "))
	if v := k.store.value(workspacesModule + ".divider"); v != " · " {
		t.Fatalf("stored %q", v)
	}
	c.OnSelect(slices.Index(c.shown, ""))
	if v := k.store.value(workspacesModule + ".divider"); v != "" {
		t.Fatalf("none stored %q", v)
	}
}

func TestAHandEditedWorkspaceDividerIsKeptUntilAnotherIsPicked(t *testing.T) {
	k := testKit(t, "modules:\n  workspaces:\n    divider: \" » \"\n")
	c := workspaceDivider(t, k)
	if len(c.Items()) != 7 || c.shown[c.Selected()] != " » " {
		t.Fatalf("offers %q with %q selected", c.Items(), c.shown[c.Selected()])
	}
	if v := k.store.value(workspacesModule + ".divider"); v != " » " {
		t.Fatalf("a hand-edited divider was rewritten to %q", v)
	}
	c.OnSelect(slices.Index(c.shown, " | "))
	c.refresh()
	if len(c.Items()) != 6 || k.store.value(workspacesModule+".divider") != " | " {
		t.Fatalf("after picking | it offers %q, stored %q", c.Items(), k.store.value(workspacesModule+".divider"))
	}
}
