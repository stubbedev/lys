package daemon_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/internal/history"
)

func (f *fixture) display(t *testing.T, connector, status string) {
	t.Helper()
	dir := filepath.Join(f.root, "sys", "class", "drm", connector)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTheChosenLimitIsHeld(t *testing.T) {
	f := newFixture(t)
	f.write(t, f.cfg.LimitPath(), "90")
	result := f.eval(t, 1, 12, 0)
	if got := f.thresholds(t); got != "85/90" {
		t.Errorf("thresholds %s, want 85/90", got)
	}
	if result.Charge.Limit != charge.Limit90 || result.Charge.Reason != charge.ReasonHolding {
		t.Errorf("state %+v", result.Charge)
	}
}

func TestAnUnknownLimitIsAnError(t *testing.T) {
	f := newFixture(t)
	f.write(t, f.cfg.LimitPath(), "87")
	if _, err := daemon.Eval(f.cfg, f.log, time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC), f.profiles); err == nil {
		t.Error("a limit of 87 is no error")
	}
	if got := f.thresholds(t); got != "75/80" {
		t.Errorf("thresholds moved to %s", got)
	}
}

func TestADepartureOpensTheWindowAndAPastOneDoesNot(t *testing.T) {
	f := newFixture(t)
	now := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	f.write(t, f.cfg.DeparturePath(), strconv.FormatInt(now.Add(time.Hour).Unix(), 10))
	result := f.eval(t, 1, 12, 0)
	if got := f.thresholds(t); got != "95/100" || result.Charge.Reason != charge.ReasonEvent {
		t.Errorf("thresholds %s, state %+v", got, result.Charge)
	}
	f.write(t, f.cfg.DeparturePath(), strconv.FormatInt(now.Add(-time.Hour).Unix(), 10))
	f.eval(t, 1, 12, 0)
	if got := f.thresholds(t); got != "75/80" {
		t.Errorf("past departure: thresholds %s", got)
	}
}

func TestABareChargerChargesToFullAndADockHolds(t *testing.T) {
	f := newFixture(t)
	f.display(t, "card0-eDP-1", "connected")
	f.display(t, "card0-DP-1", "disconnected")
	if result := f.eval(t, 1, 12, 0); f.thresholds(t) != "95/100" || result.Charge.Reason != charge.ReasonTravel {
		t.Errorf("bare charger: %s, %+v", f.thresholds(t), result.Charge)
	}
	f.display(t, "card0-DP-1", "connected")
	if result := f.eval(t, 1, 12, 0); f.thresholds(t) != "75/80" || result.Charge.Dock != charge.DockDocked {
		t.Errorf("docked: %s, %+v", f.thresholds(t), result.Charge)
	}
}

func TestAnUnplugThatUndocksIsRecordedFromTheDock(t *testing.T) {
	f := newFixture(t)
	f.display(t, "card0-eDP-1", "connected")
	f.display(t, "card0-DP-1", "connected")
	f.eval(t, 1, 12, 0)
	f.display(t, "card0-DP-1", "disconnected")
	f.eval(t, 1, 12, 1)
	f.unplug(t)
	f.eval(t, 1, 12, 2)
	f.plug(t)
	f.eval(t, 1, 13, 0)
	f.unplug(t)
	f.eval(t, 1, 14, 0)
	entries, err := history.Load(f.cfg.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	want := []history.Entry{{Dow: 1, Minute: 722, Docked: true}, {Dow: 1, Minute: 840}}
	if len(entries) != 2 || entries[0] != want[0] || entries[1] != want[1] {
		t.Errorf("history %+v, want %+v", entries, want)
	}
}
