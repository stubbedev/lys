package settings

import (
	"strings"
	"testing"
)

func TestANewCardIsADraftUntilLysWMTakesIt(t *testing.T) {
	k, w, fake, files := wmKit(t, "")
	row := newFieldRow(k, w, wmShortcutsRow())
	k.onWMChange = row.refresh
	c := row.ctl.(*shortcutTable).main
	fake.reject = "binds[1].chord: is empty"
	buttonsOfRole(c, roleAdd)[0].OnClick()
	if len(c.local) != 2 || !c.draft.Visible() || !strings.Contains(c.draft.Text(), "binds[1].chord: is empty") {
		t.Fatalf("%d cards, draft %v %q", len(c.local), c.draft.Visible(), c.draft.Text())
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
