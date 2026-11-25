package settings

import (
	"testing"
)

// A percentage field without a page's own editor (Charged Notification)
// read its value as a bare "0". Every percentage field reads in percent.
func TestAPercentageFieldReadsInPercent(t *testing.T) {
	k := testKit(t, "modules:\n  battery:\n    notify-charged-at: 80\n")
	c, ok := fieldControl(k, k.store, field("modules.battery.notify-charged-at")).(*slider)
	if !ok {
		t.Fatal("a percentage is no slider")
	}
	if got, want := c.ValueLabel().Text(), unitPercent.format(0)(80); got != want {
		t.Errorf("80 reads %q, want %q", got, want)
	}
}
