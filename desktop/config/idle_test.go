package config

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logind"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
)

func TestIdleTimeoutParsesDurationsAndNever(t *testing.T) {
	for raw, want := range map[string]time.Duration{"90s": 90 * time.Second, "5m": 5 * time.Minute, "1h30m": 90 * time.Minute} {
		var to IdleTimeout
		if err := to.UnmarshalConfig(raw); err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got, ok := to.Timeout().After(); !ok || got != want {
			t.Errorf("%q = %s %v, want %s", raw, got, ok, want)
		}
	}
	var never IdleTimeout
	if err := never.UnmarshalConfig("never"); err != nil {
		t.Fatal(err)
	}
	if _, ok := never.Timeout().After(); ok || never.MarshalConfig() != "never" {
		t.Errorf("never = %v", never.MarshalConfig())
	}
	for raw, want := range map[IdleTimeout]string{mustIdleTimeout("4m"): "4m", mustIdleTimeout("1h"): "1h", mustIdleTimeout("1h30m"): "1h30m", mustIdleTimeout("90s"): "1m30s"} {
		if got := raw.MarshalConfig(); got != want {
			t.Errorf("marshal = %v, want %s", got, want)
		}
	}
}

func TestIdleTimeoutRefusesWhatItCannotCount(t *testing.T) {
	for _, raw := range []any{"0s", "500ms", "-5m", "5", "soon", 300, ""} {
		var to IdleTimeout
		err := to.UnmarshalConfig(raw)
		if err == nil {
			t.Errorf("%v accepted", raw)
		}
		if s, ok := raw.(string); ok && (s == "0s" || s == "500ms" || s == "-5m") && !errors.Is(err, idlepolicy.ErrTimeoutRange) {
			t.Errorf("%q: %v, want ErrTimeoutRange", s, err)
		}
	}
	var d IdleDimPercent
	if err := d.UnmarshalConfig(int64(0)); !errors.Is(err, idlepolicy.ErrDimPercentRange) {
		t.Errorf("dim-percent 0: %v", err)
	}
}

func TestIdleDefaultsArmPerPowerSource(t *testing.T) {
	p := DefaultsIdle().Policy()
	ac, battery := p.Arms(idlepolicy.AC), p.Arms(idlepolicy.Battery)
	if len(ac) != 3 || len(battery) != 5 {
		t.Fatalf("defaults arm %v on AC and %v on battery", ac, battery)
	}
	if ac[0].Stage == idlepolicy.KeyboardBacklight {
		t.Errorf("on AC the keyboard backlight goes off by default: %+v", ac[0])
	}
	if battery[0].Stage != idlepolicy.KeyboardBacklight || battery[0].After != time.Minute {
		t.Errorf("battery keyboard backlight = %+v, want off after 1m", battery[0])
	}
	if battery[4].Stage != idlepolicy.Suspend || battery[4].After != 15*time.Minute {
		t.Errorf("battery suspend = %+v", battery[4])
	}
	if !p.LockBeforeSleep || p.DimPercent.Percent() != 30 {
		t.Errorf("defaults = %+v", p)
	}
}

func TestTheSuspendActionNamesTheLogindSleepItRuns(t *testing.T) {
	if got := DefaultsIdle().Policy().Sleep; got != logind.AutoSleep {
		t.Errorf("default suspend-action sleeps through %s, want auto", got)
	}
	variants := enumVariants(typeOf[SleepAction]())
	want := make([]string, 0, len(logind.SleepActions()))
	for _, a := range logind.SleepActions() {
		want = append(want, a.String())
	}
	if !slices.Equal(variants, want) {
		t.Fatalf("suspend-action variants %v, want logind's sleeps %v", variants, want)
	}
	for _, raw := range variants {
		cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "idle:\n  suspend-action: "+raw+"\n"))
		if err != nil {
			t.Fatalf("suspend-action %s: %v", raw, err)
		}
		if got := cfg.Idle.Policy().Sleep; got.String() != raw {
			t.Errorf("suspend-action %s sleeps through %s", raw, got)
		}
	}
}

func TestAttentionIsOffByDefaultAndEachVariantReachesThePolicy(t *testing.T) {
	if got := DefaultsIdle().Policy().Attention; got != idlepolicy.AttentionOff {
		t.Errorf("default attention %v, want off: the camera stays unused until chosen", got)
	}
	want := map[string]idlepolicy.Attention{
		"off":                idlepolicy.AttentionOff,
		"postpone-dim":       idlepolicy.AttentionPostpone,
		"sooner-when-absent": idlepolicy.AttentionSooner,
	}
	variants := enumVariants(typeOf[IdleAttention]())
	if len(variants) != len(want) {
		t.Fatalf("attention variants %v, want %d", variants, len(want))
	}
	for _, raw := range variants {
		cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "idle:\n  attention: "+raw+"\n"))
		if err != nil {
			t.Fatalf("attention %s: %v", raw, err)
		}
		if got, ok := want[raw]; !ok || cfg.Idle.Policy().Attention != got {
			t.Errorf("attention %s runs %v, want %v", raw, cfg.Idle.Policy().Attention, got)
		}
	}
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "idle:\n  attention: always\n"))
	var d Diagnostic
	if !errors.As(err, &d) || d.Field("Field") != "idle.attention" {
		t.Fatalf("attention always: error %v, want the field's diagnostic", err)
	}
	if cfg.Idle.Attention != IdleAttentionOff {
		t.Errorf("an unknown attention left %q, want off kept", cfg.Idle.Attention)
	}
}

func TestAnUnknownSuspendActionIsTheConfigsError(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "idle:\n  suspend-action: ignore\n  dim-percent: 40\n"))
	var d Diagnostic
	const want = "unknown variant `ignore`, expected one of `auto`, `suspend`, `hibernate`, `hybrid-sleep`, `suspend-then-hibernate`"
	if !errors.As(err, &d) || d.Field("Field") != "idle.suspend-action" || d.Field("Error") != want {
		t.Fatalf("suspend-action ignore: error %v, want %q", err, want)
	}
	if cfg.Idle.SuspendAction != DefaultsIdle().SuspendAction || cfg.Idle.DimPercent.MarshalConfig() != int64(40) {
		t.Errorf("suspend-action ignore: action %q dim %v, want the default kept and the valid sibling applied", cfg.Idle.SuspendAction, cfg.Idle.DimPercent.MarshalConfig())
	}
}

// The idle Sleep choice lists logind's sleeps in logind's order, the
// order lys-wm's lid_action lists them after "ignore", so both pickers
// read alike; each names the logind sleep it is.
func TestSleepActionsAreLogindsSleepsInItsOrder(t *testing.T) {
	var want []string
	for _, s := range logind.SleepActions() {
		want = append(want, s.String())
	}
	got := enumVariants(reflect.TypeFor[SleepAction]())
	if !slices.Equal(got, want) {
		t.Errorf("sleep actions %v, want logind's %v", got, want)
	}
	for _, name := range got {
		if a := SleepAction(name).PowerAction(); a.String() != name {
			t.Errorf("sleep action %q names logind's %s", name, a)
		}
	}
}
