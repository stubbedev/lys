package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheIdlePageExplainsTheCameraCheckAndOffersItsChoices(t *testing.T) {
	row := pageRow(t, "idle", "idle.attention")
	if row.enabledWhen.path != "idle.enable" {
		t.Errorf("the attention row follows %q, want idle.enable: without the idle policy it does nothing", row.enabledWhen.path)
	}
	if desc := description(row.labelKey()); desc == "" {
		t.Error("the attention row has no explanation to read before turning it on")
	}
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, row).(*enumSelect)
	if !ok {
		t.Fatal("the attention row is not a choice")
	}
	tr := locale.T()
	want := []string{tr.Get("enum-idle-attention-off"), tr.Get("enum-idle-attention-postpone-dim"), tr.Get("enum-idle-attention-sooner-when-absent")}
	if got := c.Items(); !slices.Equal(got, want) {
		t.Fatalf("choices %q, want %q", got, want)
	}
	if got := c.Selection(); got != want[0] {
		t.Errorf("selected %q, want %q: the camera stays unused until chosen", got, want[0])
	}
	if v := k.store.value("idle.attention"); v != "off" {
		t.Errorf("stored %v before any choice, want off", v)
	}
	c.SetSelected(2)
	if v := k.store.value("idle.attention"); v != "sooner-when-absent" {
		t.Errorf("choosing the third option stored %v, want sooner-when-absent", v)
	}
}
