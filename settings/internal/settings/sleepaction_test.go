package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/logind/logindtest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func sleepSelect(t *testing.T, yaml string, can map[string]string) (*kit, *sleepActionSelect, *loop) {
	t.Helper()
	bus := dbustest.Start(t)
	logindtest.Serve(t, bus, logindtest.Config{Can: can})
	k, l := systemKit(t, bus)
	if yaml != "" {
		k.store = testStore(t, yaml)
	}
	c, ok := fieldControl(k, k.store, pageRow(t, "idle", "idle.suspend-action")).(*sleepActionSelect)
	if !ok {
		t.Fatal("the sleep action is not the sleep action list")
	}
	l.until("logind's answers", func() bool { return c.can != nil || c.unknown != nil })
	return k, c, l
}

func TestASleepModeTheSystemCannotDoIsOfferedAsUnavailable(t *testing.T) {
	k, c, _ := sleepSelect(t, "", map[string]string{"CanHibernate": "na", "CanHybridSleep": "no"})
	tr := locale.T()
	hibernate := slices.Index(c.variants, "hibernate")
	if got := c.dropdown.Items()[hibernate]; got != tr.Get("settings-sleep-unavailable", i18n.Str("action", c.labels[hibernate])) {
		t.Fatalf("hibernate reads %q", got)
	}
	if got := c.dropdown.Items()[slices.Index(c.variants, "suspend")]; got != c.labels[slices.Index(c.variants, "suspend")] {
		t.Fatalf("suspend reads %q", got)
	}
	if c.problem.Visible() || c.detail.Text() != tr.Get("settings-sleep-detail-suspend") {
		t.Fatalf("the default suspend shows problem %v, detail %q", c.problem.Visible(), c.detail.Text())
	}
	c.dropdown.SetSelected(hibernate)
	if v := k.store.value("idle.suspend-action"); v != "suspend" {
		t.Fatalf("an unavailable hibernate was stored: %v", v)
	}
	if want := tr.Get("settings-sleep-reason-unsupported", i18n.Str("action", c.labels[hibernate])); !c.problem.Visible() || c.problem.Text() != want {
		t.Fatalf("refusing hibernate says %q", c.problem.Text())
	}
	c.dropdown.SetSelected(slices.Index(c.variants, "suspend-then-hibernate"))
	if v := k.store.value("idle.suspend-action"); v != "suspend-then-hibernate" || c.problem.Visible() {
		t.Fatalf("an available mode stored %v, problem shown %v", v, c.problem.Visible())
	}
}

func TestAStoredSleepModeTheSystemRefusesSaysWhy(t *testing.T) {
	_, c, _ := sleepSelect(t, "idle:\n  suspend-action: hybrid-sleep\n", map[string]string{"CanHybridSleep": "challenge"})
	at := slices.Index(c.variants, "hybrid-sleep")
	if c.dropdown.Selected() != at {
		t.Fatalf("selected %d, want the stored hybrid sleep", c.dropdown.Selected())
	}
	if want := locale.T().Get("settings-sleep-reason-challenge", i18n.Str("action", c.labels[at])); c.problem.Text() != want {
		t.Fatalf("a stored mode needing a password says %q", c.problem.Text())
	}
}

func TestWithoutLogindEverySleepModeStaysSelectableAndTheReasonShows(t *testing.T) {
	_, c, _ := sleepSelect(t, "", map[string]string{"CanSuspend": "perhaps"})
	if c.unknown == nil || !strings.Contains(c.problem.Text(), `answered "perhaps"`) {
		t.Fatalf("an answer logind does not give shows %q", c.problem.Text())
	}
	for i, item := range c.dropdown.Items() {
		if item != c.labels[i] {
			t.Errorf("%s reads %q without logind's answer", c.variants[i], item)
		}
	}
}
