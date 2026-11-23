package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestANewCardIsADraftUntilLysWMTakesIt(t *testing.T) {
	k, w, fake, files := wmKit(t, "")
	row := newFieldRow(k, w, wmShortcutsRow())
	k.onWMChange = row.refresh
	c := row.ctl.(*shortcutTable).main
	fake.reject = "binds[1].chord: is empty"
	buttonsOfRole(c, roleAdd)[0].OnClick()
	if len(c.local) != 2 || !failedWith(c.draft, locale.T().Get("settings-list-draft"), "binds[1].chord: is empty") {
		t.Fatalf("%d cards, draft %v %q over %q", len(c.local), c.draft.Visible(), c.draft.Text(), c.draft.detail.Text())
	}
	noFile(t, files.runtime)
	if w.class == "error" || fake.reloads != 0 {
		t.Fatalf("a draft was tried as a change: %q, %d reloads", w.notice, fake.reloads)
	}
	row.refresh()
	if len(c.local) != 2 {
		t.Fatal("a refresh dropped the draft")
	}
	fake.reject = ""
	if err := c.fieldSlot(1, "chord").set("SUPER + W"); err != nil {
		t.Fatal(err)
	}
	if c.draft.Visible() || !strings.Contains(fileText(t, files.runtime), "SUPER + W") {
		t.Fatalf("an accepted card is not written: %q", fileText(t, files.runtime))
	}
}

func TestResettingARowDropsItsDraft(t *testing.T) {
	k, w, fake, _ := wmKit(t, "")
	row := newFieldRow(k, w, wmShortcutsRow())
	k.onWMChange = row.refresh
	c := row.ctl.(*shortcutTable).main
	fake.reject = "binds[1].chord: is empty"
	buttonsOfRole(c, roleAdd)[0].OnClick()
	if !c.draft.Visible() {
		t.Fatal("no draft")
	}
	row.reset.OnClick()
	if c.draft.Visible() || len(c.local) != 1 {
		t.Fatalf("after the reset: draft %v, %d cards, the stored list has 1", c.draft.Visible(), len(c.local))
	}
}

// The row's reset drops a draft in a mode's binds as well as in the
// main binds: a refused reset then leaves no blank card behind.
func TestResettingTheShortcutsDropsAModesDraft(t *testing.T) {
	k, w, fake, _ := wmKit(t, modesConfig)
	row := newFieldRow(k, w, wmShortcutsRow())
	k.onWMChange = row.refresh
	tb := row.ctl.(*shortcutTable)
	mode := tb.group[slices.IndexFunc(tb.group, func(g *modeGroup) bool { return g.name == "apps" })].list
	fake.reject = "submaps.apps[1].chord: is empty"
	buttonsOfRole(mode, roleAdd)[0].OnClick()
	if !mode.draft.Visible() || len(mode.local) != 2 {
		t.Fatalf("draft %v, %d cards", mode.draft.Visible(), len(mode.local))
	}
	row.reset.OnClick()
	if mode.draft.Visible() || len(mode.local) != 1 {
		t.Fatalf("after the reset: the mode's draft %v, %d cards, the stored list has 1", mode.draft.Visible(), len(mode.local))
	}
}
