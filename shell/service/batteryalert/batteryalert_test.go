package batteryalert

import (
	"testing"

	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/powersupply"
)

func discharging(pct float64, w powersupply.WarningLevel) Reading {
	return Reading{Warning: w, Percentage: pct, State: powersupply.StateDischarging}
}

func charging(pct float64) Reading {
	return Reading{Warning: powersupply.WarningNone, Percentage: pct, State: powersupply.StateCharging}
}

func TestNextAlertsOncePerTransition(t *testing.T) {
	type step struct {
		cur  Reading
		want Level
	}
	for _, tc := range []struct {
		name      string
		chargedAt uint8
		steps     []step
	}{
		{"low once, critical once, staying silent", 0, []step{
			{discharging(30, powersupply.WarningNone), 0},
			{discharging(10, powersupply.WarningLow), LevelLow},
			{discharging(9, powersupply.WarningLow), 0},
			{discharging(5, powersupply.WarningCritical), LevelCritical},
			{discharging(4, powersupply.WarningCritical), 0},
			{discharging(2, powersupply.WarningAction), 0},
		}},
		{"plugging in and unplugging re-arms", 0, []step{
			{discharging(9, powersupply.WarningLow), LevelLow},
			{charging(9), 0},
			{discharging(9, powersupply.WarningLow), LevelLow},
			{discharging(4, powersupply.WarningCritical), LevelCritical},
			{charging(4), 0},
			{discharging(4, powersupply.WarningCritical), LevelCritical},
		}},
		{"straight to critical is one critical alert", 0, []step{
			{discharging(3, powersupply.WarningCritical), LevelCritical},
		}},
		{"charged once on crossing the limit", 80, []step{
			{discharging(60, powersupply.WarningNone), 0},
			{charging(60), 0},
			{charging(79), 0},
			{charging(80), LevelCharged},
			{charging(85), 0},
			{discharging(84, powersupply.WarningNone), 0},
			{charging(84), 0},
		}},
		{"plugging in above the limit is no crossing", 80, []step{
			{discharging(90, powersupply.WarningNone), 0},
			{charging(90), 0},
		}},
		{"charged off at zero", 0, []step{
			{charging(50), 0},
			{charging(100), 0},
		}},
		{"a first reading already charged says nothing", 80, []step{
			{Reading{Warning: powersupply.WarningNone, Percentage: 100, State: powersupply.StateFullyCharged}, 0},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prev Reading
			for i, s := range tc.steps {
				got, ok := Next(prev, s.cur, tc.chargedAt)
				if want := s.want != 0; ok != want || got != s.want {
					t.Errorf("step %d (%+v): Next = %v, %v; want %v", i, s.cur, got, ok, s.want)
				}
				prev = s.cur
			}
		})
	}
}

func TestSaverSwitchesAtItsLevelAndRestoresOnAC(t *testing.T) {
	type step struct {
		cur     Reading
		active  string
		profile string
	}
	for _, tc := range []struct {
		name  string
		at    SaverAt
		steps []step
	}{
		{"low switches and AC restores", SaverLow, []step{
			{discharging(30, powersupply.WarningNone), powerprofiles.ProfilePerformance, ""},
			{discharging(10, powersupply.WarningLow), powerprofiles.ProfilePerformance, powerprofiles.ProfilePowerSaver},
			{discharging(5, powersupply.WarningCritical), powerprofiles.ProfilePowerSaver, ""},
			{charging(5), powerprofiles.ProfilePowerSaver, powerprofiles.ProfilePerformance},
			{charging(20), powerprofiles.ProfilePerformance, ""},
			{discharging(10, powersupply.WarningLow), powerprofiles.ProfileBalanced, powerprofiles.ProfilePowerSaver},
		}},
		{"critical waits past low", SaverCritical, []step{
			{discharging(10, powersupply.WarningLow), powerprofiles.ProfileBalanced, ""},
			{discharging(4, powersupply.WarningCritical), powerprofiles.ProfileBalanced, powerprofiles.ProfilePowerSaver},
			{charging(4), powerprofiles.ProfilePowerSaver, powerprofiles.ProfileBalanced},
		}},
		{"never leaves the profile alone", SaverNever, []step{
			{discharging(4, powersupply.WarningCritical), powerprofiles.ProfileBalanced, ""},
			{charging(4), powerprofiles.ProfileBalanced, ""},
		}},
		{"already saving has nothing to restore", SaverLow, []step{
			{discharging(10, powersupply.WarningLow), powerprofiles.ProfilePowerSaver, ""},
			{charging(10), powerprofiles.ProfilePowerSaver, ""},
		}},
		{"on AC at a low level does not switch", SaverLow, []step{
			{Reading{Warning: powersupply.WarningLow, Percentage: 9, State: powersupply.StateCharging}, powerprofiles.ProfileBalanced, ""},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s Saver
			for i, st := range tc.steps {
				next, profile, change := s.Step(tc.at, st.cur, st.active)
				if change != (st.profile != "") || profile != st.profile {
					t.Errorf("step %d: Step = %q, %v; want %q", i, profile, change, st.profile)
				}
				s = next
			}
		})
	}
}

func TestLowPowerIsOfferedOncePerUnplugAtTwentyPercent(t *testing.T) {
	type step struct {
		cur    Reading
		active string
		offer  bool
	}
	for _, tc := range []struct {
		name  string
		steps []step
	}{
		{"once at twenty, again after the next unplug", []step{
			{discharging(25, powersupply.WarningNone), powerprofiles.ProfileBalanced, false},
			{discharging(20, powersupply.WarningNone), powerprofiles.ProfileBalanced, true},
			{discharging(15, powersupply.WarningNone), powerprofiles.ProfileBalanced, false},
			{discharging(9, powersupply.WarningLow), powerprofiles.ProfileBalanced, false},
			{charging(9), powerprofiles.ProfileBalanced, false},
			{discharging(9, powersupply.WarningLow), powerprofiles.ProfileBalanced, true},
		}},
		{"not while power-saver is on, then once it is turned off", []step{
			{discharging(18, powersupply.WarningNone), powerprofiles.ProfilePowerSaver, false},
			{discharging(17, powersupply.WarningNone), powerprofiles.ProfilePerformance, true},
		}},
		{"never on AC or in an unknown state", []step{
			{charging(10), powerprofiles.ProfileBalanced, false},
			{Reading{Percentage: 10, State: powersupply.StateFullyCharged}, powerprofiles.ProfileBalanced, false},
			{Reading{Percentage: 15, State: powersupply.StateUnknown}, powerprofiles.ProfileBalanced, false},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var o Offer
			for i, st := range tc.steps {
				next, offer := o.Step(st.cur, func() (string, bool) { return st.active, true })
				if offer != st.offer {
					t.Errorf("step %d: offer %v, want %v", i, offer, st.offer)
				}
				o = next
			}
		})
	}
	var o Offer
	if _, offer := o.Step(discharging(10, powersupply.WarningLow), func() (string, bool) { return "", false }); offer {
		t.Error("offered Low Power without a power profile to switch")
	}
	asked := false
	if _, offer := o.Step(discharging(30, powersupply.WarningNone), func() (string, bool) { asked = true; return powerprofiles.ProfileBalanced, true }); offer || asked {
		t.Errorf("above twenty percent: offer %v, profile read %v; want neither", offer, asked)
	}
}
