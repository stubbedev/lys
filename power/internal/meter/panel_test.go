package meter_test

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/meter"
)

const (
	psr2Sleeping = "Sink support: PSR = yes [0x03], Panel Replay = no, Panel Replay Selective Update = no\n" +
		"PSR mode: PSR2\n" +
		"Source PSR/PanelReplay ctl: enabled [0xc4040e30]\n" +
		"Source PSR/PanelReplay status: DEEP_SLEEP [0x80000000]\n" +
		"Busy frontbuffer bits: 0x00000000\n" +
		"Performance counter: 0\n" +
		"Frame:\tPSR2 SU blocks:\n0\t0\n1\t0\n" +
		"PSR2 selective fetch: enabled\n"
	psr2Capturing = "Sink support: PSR = yes [0x03], Panel Replay = no\n" +
		"PSR mode: PSR2 enabled\n" +
		"Source PSR ctl: enabled [0xc4040e30]\n" +
		"Source PSR status: CAPTURE [0x10000000]\n"
	psrDisabled = "Sink support: PSR = yes [0x03], Panel Replay = no\n" +
		"PSR mode: disabled\n" +
		"PSR sink not reliable: no\n"
	psrUnsupported = "Sink support: PSR = no, Panel Replay = no, Panel Replay Selective Update = no\n"
)

func TestParsePSRStatus(t *testing.T) {
	for _, tc := range []struct {
		name       string
		raw        string
		mode       energy.SelfRefresh
		refreshing bool
	}{
		{"PSR2 in deep sleep", psr2Sleeping, energy.SelfRefreshPSR2, true},
		{"PSR2 capturing, on an older kernel", psr2Capturing, energy.SelfRefreshPSR2, false},
		{"PSR1 entered", "Sink support: PSR = yes [0x01]\nPSR mode: PSR1 enabled\nSource PSR status: SRDENT [0x40040006]\n", energy.SelfRefreshPSR1, true},
		{"PSR1 idle", "Sink support: PSR = yes [0x01]\nPSR mode: PSR1\nSource PSR/PanelReplay status: IDLE [0x0]\n", energy.SelfRefreshPSR1, false},
		{"PSR1 does not read PSR2's states", "Sink support: PSR = yes [0x01]\nPSR mode: PSR1\nSource PSR/PanelReplay status: DEEP_SLEEP [0x0]\n", energy.SelfRefreshPSR1, false},
		{"Panel Replay asleep", "Sink support: PSR = no, Panel Replay = yes\nPSR mode: Panel Replay Enabled\nSource PSR/PanelReplay status: SLEEP [0x0]\n", energy.SelfRefreshPanelReplay, true},
		{"Panel Replay selective update standing by", "Sink support: PSR = no, Panel Replay = yes, Panel Replay Selective Update = yes\nPSR mode: Panel Replay Selective Update Enabled\nSource PSR/PanelReplay status: SU_STANDBY [0x0]\n", energy.SelfRefreshPanelReplaySU, true},
		{"disabled", psrDisabled, energy.SelfRefreshOff, false},
		{"unsupported", psrUnsupported, energy.SelfRefreshUnsupported, false},
	} {
		mode, refreshing, err := meter.ParsePSRStatus([]byte(tc.raw))
		if err != nil || mode != tc.mode || refreshing != tc.refreshing {
			t.Errorf("%s: %s refreshing %v, %v; want %s refreshing %v", tc.name, mode, refreshing, err, tc.mode, tc.refreshing)
		}
	}
	for name, bad := range map[string]string{
		"an unknown mode":            "Sink support: PSR = yes\nPSR mode: PSR3\nSource PSR status: SLEEP [0x0]\n",
		"an enabled mode, no status": "Sink support: PSR = yes\nPSR mode: PSR2\n",
		"a supported sink, no mode":  "Sink support: PSR = yes [0x03]\n",
		"nothing":                    "",
	} {
		if mode, _, err := meter.ParsePSRStatus([]byte(bad)); err == nil {
			t.Errorf("%s parsed as %s", name, mode)
		}
	}
}

func TestPanelsAreReadOncePerDeviceFromTheDebugfs(t *testing.T) {
	mc := newMachine(t)
	mc.dri.write("0000:00:02.0/i915_edp_psr_status", psr2Sleeping)
	mc.dri.write("0000:00:02.0/i915_edp_psr_debug", "0x0\n")
	mc.dri.link("0", "0000:00:02.0")
	mc.dri.write("128/clients", "")
	mc.dri.write("0000:03:00.0/xe_edp_psr_status", psrDisabled)
	mc.dri.write("version", "1\n")
	s, gaps := mc.sample(t)
	want := []energy.PanelState{
		{Device: "0000:00:02.0", Mode: energy.SelfRefreshPSR2, Refreshing: true},
		{Device: "0000:03:00.0", Mode: energy.SelfRefreshOff},
	}
	if !reflect.DeepEqual(s.Panels, want) || gaps.PanelsUnreadable {
		t.Errorf("panels = %+v, gaps %+v; want %+v", s.Panels, gaps, want)
	}
}

func TestWithoutADebugfsThereAreNoPanels(t *testing.T) {
	mc := newMachine(t)
	s, gaps := mc.sample(t)
	if s.Panels != nil || gaps.PanelsUnreadable {
		t.Errorf("panels = %+v, gaps %+v; want none and no gap", s.Panels, gaps)
	}
}

func TestAMalformedPanelStatusIsAnError(t *testing.T) {
	mc := newMachine(t)
	mc.dri.write("0/xe_edp_psr_status", "Sink support: PSR = yes\nPSR mode: PSR9\n")
	if _, _, err := mc.m.Sample(time.Now); err == nil {
		t.Error("a status with an unknown mode sampled")
	}
}

func TestAnUnreadableDebugfsIsAGapNotAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory without read permission")
	}
	mc := newMachine(t)
	mc.dri.write("0/i915_edp_psr_status", psr2Sleeping)
	if err := os.Chmod(mc.dri.root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(mc.dri.root, 0o755); err != nil {
			t.Error(err)
		}
	})
	s, gaps := mc.sample(t)
	if s.Panels != nil || !gaps.PanelsUnreadable {
		t.Errorf("panels = %+v, gaps %+v; want none and the gap named", s.Panels, gaps)
	}
}
