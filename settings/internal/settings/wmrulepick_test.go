package settings

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

func TestAPickedWindowBecomesARuleMatchingExactlyIt(t *testing.T) {
	for _, c := range []struct {
		window wmipc.Window
		want   map[string]any
	}{
		{wmipc.Window{AppID: "org.mozilla.firefox", Title: "Mozilla Firefox"}, map[string]any{appIDKey: `^org\.mozilla\.firefox$`}},
		{wmipc.Window{AppID: "c++ (dev)"}, map[string]any{appIDKey: `^c\+\+ \(dev\)$`}},
		{wmipc.Window{Title: "Picture-in-Picture"}, map[string]any{titleKey: `^Picture-in-Picture$`}},
	} {
		got, err := ruleFromWindow(c.window)
		if err != nil || !maps.Equal(got, c.want) {
			t.Errorf("%+v: %v, %v; want %v", c.window, got, err, c.want)
		}
	}
	if id, exact := exactAppID(exactPattern("org.mozilla.firefox")); !exact || id != "org.mozilla.firefox" {
		t.Errorf("the picked app ID reads back as %q, exact %t", id, exact)
	}
	if _, err := ruleFromWindow(wmipc.Window{ID: 4}); !errors.Is(err, errUnnamedWindow) {
		t.Errorf("a window with neither an app ID nor a title: %v, want %v", err, errUnnamedWindow)
	}
}

func TestPickingAWindowAddsItsRule(t *testing.T) {
	k, w, fake, files := wmKit(t, "rules:\n  - {app_id: foot}\n")
	fake.running = map[string]string{"pickwindow": "7", "windows": `[{"id":3,"app_id":"foot"},{"id":7,"app_id":"org.mozilla.firefox","title":"Mozilla Firefox"}]`}
	c := fieldControl(k, w, wmObjects(rulesKey, appIDKey)).(*cardList)
	treetest.Button(t, c, locale.T().Get("wm-rules-pick")).OnClick()
	if len(c.local) != 2 || c.local[1][appIDKey] != `^org\.mozilla\.firefox$` {
		t.Fatalf("the picked window added %v", c.local)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, `org\.mozilla\.firefox`) || !strings.Contains(got, "app_id: foot") {
		t.Fatalf("the picked rule wrote %q", got)
	}
	if c.picking.Visible() {
		t.Errorf("the pick line still says %q", c.picking.Text())
	}
}

func TestAPickThatFindsNoWindowAddsNoRuleAndSaysWhy(t *testing.T) {
	for _, c := range []struct {
		name    string
		running map[string]string
		want    string
	}{
		{"no lys-wm", nil, locale.T().Get("wm-rules-pick-not-running")},
		{"a click on no window", map[string]string{"pickwindow": "null"}, locale.T().Get("wm-rules-pick-none")},
		{"a window that closed", map[string]string{"pickwindow": "7", "windows": `[{"id":3,"app_id":"foot"}]`}, locale.T().Get("wm-rules-pick-gone")},
		{"a window with no name", map[string]string{"pickwindow": "7", "windows": `[{"id":7}]`}, locale.T().Get("wm-rules-pick-unnamed")},
	} {
		k, w, fake, _ := wmKit(t, "rules:\n  - {app_id: foot}\n")
		fake.running = c.running
		l := fieldControl(k, w, wmObjects(rulesKey, appIDKey)).(*cardList)
		treetest.Button(t, l, locale.T().Get("wm-rules-pick")).OnClick()
		if len(l.local) != 1 {
			t.Errorf("%s added a rule: %v", c.name, l.local)
		}
		if !l.picking.Visible() || l.picking.Text() != c.want {
			t.Errorf("%s says %q, want %q", c.name, l.picking.Text(), c.want)
		}
	}
	reason := fmt.Errorf("wmstate: ask lys-wm for pickwindow: %w", errors.New("lys-wm: compositor: the window pick was cancelled"))
	if got := rulePickRefusal(reason); !strings.Contains(got, "the window pick was cancelled") {
		t.Errorf("a cancelled pick says %q, want lys-wm's reason", got)
	}
	if got := rulePickRefusal(wmstate.ErrNoWindow); got != locale.T().Get("wm-rules-pick-none") {
		t.Errorf("no window says %q", got)
	}
}

func TestOnlyTheTopRulesListPicksWindows(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	if c := fieldControl(k, w, wmObjects(bindsKey, chordKey)).(*cardList); c.spec.pick != nil || c.picking != nil {
		t.Error("the binds list picks windows")
	}
}
