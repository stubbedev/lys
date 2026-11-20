package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func clearHistoryControl(t *testing.T, k *kit) *clearHistory {
	t.Helper()
	p := buildPage(k, launcherPage(nil))
	return treetest.First[*clearHistory](t, p)
}

func TestClearingTheLauncherHistoryAsksThenRunsLysShell(t *testing.T) {
	k := testKit(t, "")
	var ran [][]string
	k.shellCommand = func(args ...string) error {
		ran = append(ran, args)
		return nil
	}
	pickers := &fakePickers{}
	k.pickers = pickers
	c := clearHistoryControl(t, k)
	c.button.OnClick()
	if len(pickers.dialogs) != 1 || len(ran) != 0 {
		t.Fatalf("clear opened %d dialogs and ran %q before an answer", len(pickers.dialogs), ran)
	}
	ask := pickers.dialogs[0]
	if ask.Title != locale.T().Get(clearHistoryKey+"-title") || !ask.Modal || ask.CancelResponse != responseCancel {
		t.Fatalf("dialog %q modal %v cancel %q", ask.Title, ask.Modal, ask.CancelResponse)
	}
	ask.OnResponse(responseCancel)
	if len(ran) != 0 || c.outcome.Visible() {
		t.Fatalf("cancelling ran %q", ran)
	}
	ask.OnResponse(responseConfirm)
	if len(ran) != 1 || !slices.Equal(ran[0], []string{"launcher", "-clear-history"}) {
		t.Fatalf("confirming ran %q, want lys-shell launcher -clear-history", ran)
	}
	if !c.outcome.Visible() || c.outcome.Text() != locale.T().Get(clearHistoryKey+"-done") {
		t.Errorf("a cleared history shows %q (visible %v)", c.outcome.Text(), c.outcome.Visible())
	}
}

func TestAFailedClearSaysWhy(t *testing.T) {
	k := testKit(t, "")
	k.shellCommand = func(...string) error { return errors.New("lys-shell is not installed") }
	pickers := &fakePickers{}
	k.pickers = pickers
	c := clearHistoryControl(t, k)
	c.button.OnClick()
	pickers.dialogs[0].OnResponse(responseConfirm)
	if !c.outcome.Visible() || !strings.Contains(c.outcome.Text(), "lys-shell is not installed") || c.outcome.Text() == locale.T().Get(clearHistoryKey+"-done") {
		t.Errorf("a failed clear shows %q (visible %v)", c.outcome.Text(), c.outcome.Visible())
	}
	if !c.button.Enabled() {
		t.Error("after a failed clear the button stays disabled")
	}
}

func TestWithoutAWindowToAskInNothingIsCleared(t *testing.T) {
	k := testKit(t, "")
	var ran [][]string
	k.shellCommand = func(args ...string) error {
		ran = append(ran, args)
		return nil
	}
	k.pickers = nil
	c := clearHistoryControl(t, k)
	c.button.OnClick()
	if len(ran) != 0 || !c.outcome.Visible() || !strings.Contains(c.outcome.Text(), "no window to ask in") {
		t.Errorf("without a window clear ran %q and shows %q", ran, c.outcome.Text())
	}
}
