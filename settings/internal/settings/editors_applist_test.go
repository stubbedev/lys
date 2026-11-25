package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func notifyAppsOf(t *testing.T, k *kit) *notifyApps {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "notifications", notifyBlockPath)).(*notifyApps)
	if !ok {
		t.Fatal("the notification app rules are not one list of apps")
	}
	return c
}

func storedList(k *kit, path string) []any {
	got, _ := k.store.value(path).([]any)
	return got
}

func TestEachAppANotificationRuleNamesIsOneRowWithACheckPerRule(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  notifications:\n    blocklist: ['*chromium*']\n    sound-mute-apps: [firefox, '*chromium*']\n    dnd:\n      allow-apps: [org.gnome.Calculator]\n")
	c := notifyAppsOf(t, k)
	if !slices.Equal(c.rows, []string{"*chromium*", "firefox", "org.gnome.Calculator"}) {
		t.Fatalf("rows %q, want each pattern once in the order the rules list them", c.rows)
	}
	for pattern, want := range map[string][]bool{
		"*chromium*":           {true, true, false},
		"firefox":              {false, true, false},
		"org.gnome.Calculator": {false, false, true},
	} {
		for i, check := range c.checks[pattern] {
			if check.Checked() != want[i] {
				t.Errorf("%s: %s checked %v, want %v", pattern, appRules[i].path, check.Checked(), want[i])
			}
		}
	}
	if got := treetest.Labels(c.list); !slices.Contains(got, "Firefox") || !slices.Contains(got, "*chromium*") {
		t.Errorf("an installed app reads by its name, a pattern as written: %q", got)
	}
	c.checks["firefox"][2].SetChecked(true)
	if got := storedList(k, notifyFocusPath); !slices.Equal(got, []any{"org.gnome.Calculator", "firefox"}) {
		t.Errorf("letting Firefox through Focus wrote %v", got)
	}
	c.checks["firefox"][1].SetChecked(false)
	c.checks["firefox"][2].SetChecked(false)
	if !slices.Contains(c.rows, "firefox") || slices.Contains(storedList(k, notifyMutePath), any("firefox")) {
		t.Errorf("with every check off Firefox's row is %v and silent %v", c.rows, storedList(k, notifyMutePath))
	}
	treetest.All[*widget.Button](c.list)[0].OnClick()
	if slices.Contains(c.rows, "*chromium*") || len(storedList(k, notifyBlockPath)) != 0 || slices.Contains(storedList(k, notifyMutePath), any("*chromium*")) {
		t.Errorf("removing *chromium* left rows %q, blocked %v, silent %v", c.rows, storedList(k, notifyBlockPath), storedList(k, notifyMutePath))
	}
}

func TestAnAppAddedIsPickedFromTheInstalledOnesOrTypedAsAPattern(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  notifications:\n    blocklist: [firefox]\n")
	c := notifyAppsOf(t, k)
	f := &fakePickers{}
	k.pickers = f
	c.end.add.ClickAt(widget.Point{})
	if f.content == nil || f.anchor != widget.Widget(c.end.add) {
		t.Fatal("the app picker did not open beside the add button")
	}
	flow := treetest.First[*widget.FlowBox](t, f.content)
	offered := shownLabels(flow)
	if slices.Contains(offered, "Firefox") || !slices.Contains(offered, "Calculator") {
		t.Fatalf("the picker offers %q: an app with a rule is offered again, or one without is not", offered)
	}
	flow.OnActivate(slices.Index(offered, "Calculator"))
	if !slices.Equal(c.rows, []string{"firefox", "org.gnome.Calculator"}) {
		t.Fatalf("rows after picking Calculator %q", c.rows)
	}
	for _, rule := range appRules[1:] {
		if len(storedList(k, rule.path)) != 0 {
			t.Errorf("picking an app wrote %s before a check was set", rule.path)
		}
	}
	c.pattern.OnActivate("  org.example.*  ")
	c.pattern.OnActivate("firefox")
	c.pattern.OnActivate("   ")
	if !slices.Equal(c.rows, []string{"firefox", "org.gnome.Calculator", "org.example.*"}) || c.pattern.Text() != "" {
		t.Fatalf("rows after typing patterns %q, the field holds %q", c.rows, c.pattern.Text())
	}
	c.checks["org.example.*"][1].SetChecked(true)
	if got := storedList(k, notifyMutePath); !slices.Equal(got, []any{"org.example.*"}) {
		t.Errorf("silencing the pattern wrote %v", got)
	}
}

func TestNoAppRuleSaysEveryAppIsLetThrough(t *testing.T) {
	k := testKit(t, "")
	c := notifyAppsOf(t, k)
	if !c.end.empty.Visible() || len(c.rows) != 0 {
		t.Errorf("with no rule the note shows %v over rows %q", c.end.empty.Visible(), c.rows)
	}
	if row := notifyAppsRow(); !slices.Equal(row.also, []string{notifyMutePath, notifyFocusPath}) {
		t.Errorf("the row is found by %s and %q, want the silent and Focus lists too", row.path, row.also)
	}
	for _, sec := range soundPage(k.store.svc.Config()).sections {
		for _, r := range sec.rows {
			if r.path == notifyMutePath {
				t.Error("the silent apps still have a row on the Sound page")
			}
		}
	}
}
