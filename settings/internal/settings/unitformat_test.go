package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
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

// The scale chips read "100 %" beside every other percentage's "5%". In
// every language a scale percentage reads as the percent unit writes the
// same number.
func TestAScalePercentReadsAsThePercentUnit(t *testing.T) {
	available, err := locale.Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range available {
		l, err := i18n.Select(locale.Assets(), []i18n.LangID{lang})
		if err != nil {
			t.Fatal(err)
		}
		scale := l.Get("settings-scale-percent", i18n.Int("n", 125))
		unit := l.Attr(string(unitPercent), "value", i18n.Str("n", "125"))
		if scale != unit {
			t.Errorf("%s: a scale reads %q, a percentage %q", lang, scale, unit)
		}
	}
}
