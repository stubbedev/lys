package lockview

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestTheMediaRowShowsATrackAndHidesWithoutOne(t *testing.T) {
	m := NewMediaRow(treetest.Font(t), func() {}, func() {})
	if m.Root.Visible() {
		t.Fatal("a new media row shows before a track")
	}
	m.Set(Track{Title: "Blue in Green", Artist: "Miles Davis", Playing: true, CanNext: true}, true)
	if !m.Root.Visible() || m.Title.Text() != "Blue in Green" || m.Artist.Text() != "Miles Davis" || !m.Next.Visible() {
		t.Errorf("a playing track shows %v, reading %q by %q, next %v", m.Root.Visible(), m.Title.Text(), m.Artist.Text(), m.Next.Visible())
	}
	if m.Play.AccessibleName() != i18n.T("lock-media-pause") || m.Play.TooltipText() != i18n.T("lock-media-pause") {
		t.Errorf("a playing track's button is named %q, tipped %q", m.Play.AccessibleName(), m.Play.TooltipText())
	}
	if m.Next.TooltipText() != i18n.T("lock-media-next") {
		t.Errorf("the next button is tipped %q", m.Next.TooltipText())
	}
	m.Set(Track{Title: "So What"}, true)
	if m.Artist.Visible() || m.Next.Visible() || m.Play.AccessibleName() != i18n.T("lock-media-play") {
		t.Errorf("a paused track without artist or next shows artist %v, next %v, button %q", m.Artist.Visible(), m.Next.Visible(), m.Play.AccessibleName())
	}
	m.Set(Track{Title: "So What"}, false)
	if m.Root.Visible() {
		t.Error("the media row stays once nothing plays")
	}
}

func TestTheBatteryChipShowsTheChargeAndHidesWithoutABattery(t *testing.T) {
	b := NewBatteryChip(treetest.Font(t), "ld-battery-charging-symbolic")
	if b.Root.Visible() {
		t.Fatal("a new battery chip shows before a battery")
	}
	b.Set(BatteryLevel{Percent: 64}, true)
	if !b.Root.Visible() || b.Label.Text() != i18n.T("lock-battery", i18n.Str("percent", "64")) || b.Charging.Visible() {
		t.Errorf("a discharging battery shows %v reading %q charging %v", b.Root.Visible(), b.Label.Text(), b.Charging.Visible())
	}
	b.Set(BatteryLevel{Percent: 65, Charging: true}, true)
	if !b.Charging.Visible() {
		t.Error("a charging battery shows no charging icon")
	}
	b.Set(BatteryLevel{}, false)
	if b.Root.Visible() {
		t.Error("a machine without a battery shows the chip")
	}
}
