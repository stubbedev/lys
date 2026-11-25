package bar

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/shell/service/sysinfo"
)

func TestNetstatLabelPlaceholders(t *testing.T) {
	rate := sysinfo.Rate{
		Interface: "wlan0",
		RxPerSec:  1536,  // 1.5 KiB
		TxPerSec:  10240, // 10.0 KiB
	}
	cfg := config.DefaultsNetstat()
	if got := netstatLabel(cfg.Format, rate); got != "1.5 KiB/s 10.0 KiB/s" {
		t.Errorf("default = %q", got)
	}
	got := netstatLabel("{{ down_kib }}/{{ down_mib }}/{{ down_gib }} {{ interface }}", rate)
	// "{:.0}" rounds 1.5 KiB to 2; MiB keeps one decimal.
	if got != "2/0.0/0.00 wlan0" {
		t.Errorf("kib/mib/gib = %q", got)
	}
	got = netstatLabel("{{ up_kib }} {{ up_mib }} {{ up_gib }}", rate)
	if got != "10 0.0 0.00" {
		t.Errorf("up = %q", got)
	}
	// Zero traffic renders the auto units as plain bytes.
	if got := netstatLabel("{{ down_auto }}", sysinfo.Rate{}); got != "0 B/s" {
		t.Errorf("zero = %q", got)
	}
}

func TestNetstatAutoRatesAreTheShellLocalesByteSizePerSecond(t *testing.T) {
	for _, c := range []struct {
		perSec uint64
		want   string
	}{
		{1023, "1,023 B/s"},
		{1<<20 - 1, "1.0 MiB/s"},
		{123456789, "117.7 MiB/s"},
	} {
		rate := sysinfo.Rate{RxPerSec: c.perSec, TxPerSec: c.perSec}
		got := netstatLabel("{{ down_auto }}|{{ up_auto }}", rate)
		if got != c.want+"|"+c.want {
			t.Errorf("%d B/s = %q, want en-US's %q both ways", c.perSec, got, c.want)
		}
		if formatted := i18n.Shell().ByteSize(c.perSec) + "/s"; got != formatted+"|"+formatted {
			t.Errorf("%d B/s = %q, want the shell formatter's %q", c.perSec, got, formatted)
		}
	}
}

func TestLoadFileAppliesNetstat(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	good := "modules:\n  netstat:\n    interface: eth0\n    format: \"{{ down_mib }} down\"\n    poll-interval-ms: 5000\n"
	if err := osWrite(path, good); err != nil {
		t.Fatal(err)
	}
	c, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if c.Netstat.Interface != "eth0" || c.Netstat.PollIntervalMs != 5000 || c.Netstat.Format != "{{ down_mib }} down" {
		t.Errorf("netstat = %+v", c.Netstat)
	}

	if err := osWrite(path, "modules:\n  netstat:\n    poll-interval-ms: -1\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadFile(path); err == nil {
		t.Error("negative interval: want a load error")
	}
}
