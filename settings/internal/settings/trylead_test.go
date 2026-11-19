package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func pageLead(t *testing.T, k *kit, id string) *tryButton {
	t.Helper()
	spec, ok := pageByID(layout(k.store.svc.Config(), bothParts), id)
	if !ok || spec.lead == nil {
		t.Fatalf("page %s has no lead", id)
	}
	tries := treetest.All[*tryButton](spec.lead(k))
	if len(tries) != 1 {
		t.Fatalf("page %s leads with %d try buttons", id, len(tries))
	}
	return tries[0]
}

func TestTheNotificationsPageSendsATestNotification(t *testing.T) {
	k := testKit(t, "")
	var sent []string
	k.notify = func(summary, body string) error {
		sent = append(sent, summary, body)
		return nil
	}
	b := pageLead(t, k, "notifications")
	b.button.OnClick()
	tr := locale.T()
	if want := []string{tr.Get("settings-test-notification-summary"), tr.Get("settings-test-notification-body")}; !slices.Equal(sent, want) {
		t.Fatalf("the button sent %q, want %q", sent, want)
	}
	if b.outcome.Visible() {
		t.Errorf("a sent notification shows %q", b.outcome.Text())
	}
}

func TestATestNotificationNoDaemonTakesSaysWhy(t *testing.T) {
	k := testKit(t, "")
	b := pageLead(t, k, "notifications")
	b.button.OnClick()
	if !b.outcome.Visible() || !strings.Contains(b.outcome.Text(), "no notification daemon in the tests") {
		t.Errorf("a refused notification shows %q (visible %v)", b.outcome.Text(), b.outcome.Visible())
	}
}

func TestTheOSDPageShowsAVolumeToast(t *testing.T) {
	k := testKit(t, "")
	var ran [][]string
	k.shellCommand = func(args ...string) error {
		ran = append(ran, args)
		return nil
	}
	pageLead(t, k, "osd").button.OnClick()
	want := []string{"toast", locale.T().Get("settings-osd-preview-label"), "--icon", "ld-volume-2-symbolic", "--percentage", "60"}
	if len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("Show OSD ran %q, want lys-shell %q", ran, want)
	}
}

func TestAToastPresetCardShowsItsPresetAndItsCommand(t *testing.T) {
	k := testKit(t, "osd:\n  presets:\n    - id: brb\n      label: Back soon\n")
	var ran [][]string
	k.shellCommand = func(args ...string) error {
		ran = append(ran, args)
		return errors.New("lys-shell is not running")
	}
	c := fieldControl(k, k.store, field("osd.presets", toastPresetList))
	tries := treetest.All[*presetTry](c)
	if len(tries) != 1 {
		t.Fatalf("one preset has %d try controls", len(tries))
	}
	p := tries[0]
	if got := p.command.Text(); got != "lys-shell toast --preset brb" {
		t.Errorf("the preset's command reads %q", got)
	}
	b := treetest.All[*tryButton](p)[0]
	b.button.OnClick()
	if len(ran) != 1 || !slices.Equal(ran[0], []string{"toast", "--preset", "brb"}) {
		t.Errorf("trying the preset ran %q", ran)
	}
	if !b.outcome.Visible() || !strings.Contains(b.outcome.Text(), "lys-shell is not running") {
		t.Errorf("a failed try shows %q (visible %v)", b.outcome.Text(), b.outcome.Visible())
	}
}

func TestTheTryLeadsLeaveWithLysShell(t *testing.T) {
	for _, id := range []string{"notifications", "osd"} {
		spec, _ := pageByID(layout(nil, bothParts), id)
		if _, shown := spec.on(parts{wm: true}); shown {
			t.Errorf("%s shows without lys-shell", id)
		}
		if got, shown := spec.on(parts{shell: true}); !shown || got.lead == nil {
			t.Errorf("%s lost its try button with lys-shell", id)
		}
	}
}
