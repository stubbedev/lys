package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/hibernate"
	"github.com/stubbedev/lys/lib/logind/logindtest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func sleepSelect(t *testing.T, yaml string, can map[string]string) (*kit, *sleepAction, *loop) {
	t.Helper()
	return sleepSelectBlocked(t, yaml, can, nil)
}

func sleepSelectBlocked(t *testing.T, yaml string, can map[string]string, blockers func() ([]hibernate.Blocker, error)) (*kit, *sleepAction, *loop) {
	t.Helper()
	bus := dbustest.Start(t)
	logindtest.Serve(t, bus, logindtest.Config{Can: can})
	k, l := systemKit(t, bus)
	if blockers != nil {
		k.hibernation = blockers
	}
	if yaml != "" {
		k.store = testStore(t, yaml)
	}
	c, ok := fieldControl(k, k.store, pageRow(t, "idle", "idle.suspend-action")).(*sleepAction)
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
	if got := c.pick.pick.Items()[hibernate]; got != tr.Get("settings-sleep-unavailable", i18n.Str("action", c.labels[hibernate])) {
		t.Fatalf("hibernate reads %q", got)
	}
	if got := c.pick.pick.Items()[slices.Index(c.variants, "suspend")]; got != c.labels[slices.Index(c.variants, "suspend")] {
		t.Fatalf("suspend reads %q", got)
	}
	if c.problem.Visible() || c.does.Text() != tr.Get("settings-sleep-detail-auto-hibernates") {
		t.Fatalf("the default auto shows problem %v, detail %q", c.problem.Visible(), c.does.Text())
	}
	pickChoice(c.pick.pick, hibernate)
	if v := k.store.value("idle.suspend-action"); v != "auto" {
		t.Fatalf("an unavailable hibernate was stored: %v", v)
	}
	if want := tr.Get("settings-sleep-reason-unsupported", i18n.Str("action", c.labels[hibernate])); !c.problem.Visible() || c.problem.Text() != want {
		t.Fatalf("refusing hibernate says %q", c.problem.Text())
	}
	pickChoice(c.pick.pick, slices.Index(c.variants, "suspend-then-hibernate"))
	if v := k.store.value("idle.suspend-action"); v != "suspend-then-hibernate" || c.problem.Visible() {
		t.Fatalf("an available mode stored %v, problem shown %v", v, c.problem.Visible())
	}
}

func TestAStoredSleepModeTheSystemRefusesSaysWhy(t *testing.T) {
	_, c, _ := sleepSelect(t, "idle:\n  suspend-action: hybrid-sleep\n", map[string]string{"CanHybridSleep": "challenge"})
	at := slices.Index(c.variants, "hybrid-sleep")
	if c.pick.pick.Selected() != at {
		t.Fatalf("selected %d, want the stored hybrid sleep", c.pick.pick.Selected())
	}
	if want := locale.T().Get("settings-sleep-reason-challenge", i18n.Str("action", c.labels[at])); c.problem.Text() != want {
		t.Fatalf("a stored mode needing a password says %q", c.problem.Text())
	}
}

func TestWithoutLogindEverySleepModeStaysSelectableAndTheReasonShows(t *testing.T) {
	_, c, _ := sleepSelect(t, "", map[string]string{"CanSuspend": "perhaps"})
	if c.unknown == nil || c.problem.Text() != locale.T().Get("settings-sleep-unknown") || !strings.Contains(c.problem.detail.Text(), `answered "perhaps"`) || c.problem.TooltipText() != "" {
		t.Fatalf("an answer logind does not give shows %q, explained %q, tooltip %q", c.problem.Text(), c.problem.detail.Text(), c.problem.TooltipText())
	}
	for i, item := range c.pick.pick.Items() {
		if item != c.labels[i] {
			t.Errorf("%s reads %q without logind's answer", c.variants[i], item)
		}
	}
}

func TestAutoSaysWhyItOnlySuspendsWhereHibernationIsBlocked(t *testing.T) {
	blocked := func() ([]hibernate.Blocker, error) {
		return []hibernate.Blocker{hibernate.NoSwap, hibernate.NoResumeDevice}, nil
	}
	_, c, _ := sleepSelectBlocked(t, "", map[string]string{"CanSuspendThenHibernate": "na"}, blocked)
	tr := locale.T()
	reason := tr.Get("settings-hibernate-blocker-no-swap") + "; " + tr.Get("settings-hibernate-blocker-no-resume-device")
	if want := tr.Get("settings-sleep-detail-auto-suspends", i18n.Str("reason", reason)); c.does.Text() != want {
		t.Fatalf("auto without hibernation says %q, want %q", c.does.Text(), want)
	}
	if c.problem.Visible() {
		t.Errorf("auto, which still suspends, shows the problem %q", c.problem.Text())
	}
	if !c.usable("auto") {
		t.Error("auto is offered as unavailable while suspend works")
	}
}

func TestAutoFallsBackToLogindsRefusalWhenNoBlockerIsKnown(t *testing.T) {
	none := func() ([]hibernate.Blocker, error) { return nil, nil }
	_, c, _ := sleepSelectBlocked(t, "", map[string]string{"CanSuspendThenHibernate": "no"}, none)
	tr := locale.T()
	deep := slices.Index(c.variants, "suspend-then-hibernate")
	refusal := tr.Get("settings-sleep-reason-denied", i18n.Str("action", c.labels[deep]))
	if want := tr.Get("settings-sleep-detail-auto-suspends", i18n.Str("reason", refusal)); c.does.Text() != want {
		t.Fatalf("auto with hibernation denied says %q, want %q", c.does.Text(), want)
	}
}

func TestAutoReportsAProbeItCouldNotRun(t *testing.T) {
	failing := func() ([]hibernate.Blocker, error) {
		return nil, errors.New("hibernate: read /proc/swaps: permission denied")
	}
	_, c, _ := sleepSelectBlocked(t, "", map[string]string{"CanSuspendThenHibernate": "na"}, failing)
	tr := locale.T()
	want := tr.Get("settings-sleep-detail-auto-suspends", i18n.Str("reason", tr.Get("settings-hibernate-blockers-unknown")))
	if c.does.Text() != want || strings.Contains(c.does.Text(), "permission denied") || !strings.Contains(c.does.detail.Text(), "permission denied") {
		t.Fatalf("a failed probe reads %q over %q, want %q over the probe's error", c.does.Text(), c.does.detail.Text(), want)
	}
	_, c, _ = sleepSelectBlocked(t, "", nil, failing)
	if want := tr.Get("settings-sleep-detail-auto-hibernates"); c.does.Text() != want || c.does.detail.Text() != "" {
		t.Fatalf("auto where hibernation works says %q, want %q: the probe matters only when it is blocked", c.does.Text(), want)
	}
}

// The lines under the sleep action's choice set the width of its
// control, so the choice started far left of the page's other choices
// and squeezed the row's description to a few words. The lines are the
// row's detail, under its text, and the choice sits in the control
// column the page's other choices share.
func TestTheSleepActionsChoiceSharesThePagesControlColumn(t *testing.T) {
	win := laidOutPage(t, "", "idle", windowWidth)
	sleep := treetest.All[*sleepAction](win.page)
	if len(sleep) != 1 {
		t.Fatalf("%d sleep actions on the idle page", len(sleep))
	}
	if treetest.Contains(sleep[0].pick, sleep[0].does) || !treetest.Contains(win.page, sleep[0].does) {
		t.Fatal("the sleep action's lines are not its row's detail")
	}
	var others []*choiceControl
	for _, c := range treetest.All[*choiceControl](win.page) {
		if c != sleep[0].pick && shownIn(win.page, c) {
			others = append(others, c)
		}
	}
	if len(others) == 0 {
		t.Fatal("the idle page shows no other choice")
	}
	b := sleep[0].Bounds()
	for _, o := range others {
		if ob := o.Bounds(); ob.X != b.X || ob.X+ob.W != b.X+b.W {
			t.Errorf("a choice spans %d..%d, the sleep action %d..%d", ob.X, ob.X+ob.W, b.X, b.X+b.W)
		}
	}
}
