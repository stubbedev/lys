package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logind"

	drmbackend "github.com/stubbedev/lys/wm/internal/backend/drm"
	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/looptest"
	"github.com/stubbedev/lys/wm/internal/session"
)

func TestPrimaryGPUFollowsTheRule(t *testing.T) {
	igpu := gpuCard{name: "card0", render: true}
	dgpu := gpuCard{name: "card1", render: true, bootVGA: true}
	vkms := gpuCard{name: "card2"}
	cases := []struct {
		name       string
		cards      []gpuCard
		configured string
		want       int
		why        string
	}{
		{"the config wins", []gpuCard{igpu, dgpu}, "card0", 0, "set by gpu in the config"},
		{"then the boot VGA device", []gpuCard{igpu, dgpu}, "", 1, "the boot VGA device"},
		{"a boot VGA device without a render node is skipped", []gpuCard{{name: "card0", bootVGA: true}, igpu}, "", 1, "the first card with a render node"},
		{"then the first render node", []gpuCard{vkms, igpu}, "", 1, "the first card with a render node"},
		{"display-only cards render in software as a last resort", []gpuCard{vkms}, "", 0, "the first card; none has a render node"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, why, err := choosePrimary(tc.cards, tc.configured)
			if err != nil || got != tc.want || !strings.HasPrefix(why, tc.why) {
				t.Fatalf("choosePrimary = %d, %q, %v", got, why, err)
			}
		})
	}
	if _, _, err := choosePrimary([]gpuCard{igpu}, "card7"); err == nil || !strings.Contains(err.Error(), "card7 is not a DRM card on this seat (card0)") {
		t.Fatalf("a configured card that is missing: %v", err)
	}
	if _, _, err := choosePrimary(nil, ""); err == nil {
		t.Fatal("no cards chose a primary")
	}
}

func TestTheLidPolicyRunsOnceTheWakeHoldoffPasses(t *testing.T) {
	l, err := loop.New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	n := &native{l: l}
	var checks []time.Time
	n.onPower(func() {
		checks = append(checks, time.Now())
		l.Stop()
	})
	holdoff := 50 * time.Millisecond
	if err := n.recheckAfterWake(holdoff); err != nil {
		t.Fatal(err)
	}
	woke := time.Now()
	if err := errors.Join(n.PrepareForSleep(true), n.PrepareForSleep(false)); err != nil {
		t.Fatal(err)
	}
	looptest.RunUntil(t, l, "the lid policy after the wake holdoff", func() bool { return len(checks) > 0 })
	if len(checks) != 1 || checks[0].Sub(woke) < holdoff {
		t.Fatalf("checks %v after waking at %v, want one after the %v holdoff", checks, woke, holdoff)
	}

	if err := errors.Join(n.PrepareForSleep(false), n.PrepareForSleep(true)); err != nil {
		t.Fatal(err)
	}
	looptest.RunAwhile(t, l, 4*holdoff, func() bool { return len(checks) > 1 })
	if len(checks) != 1 {
		t.Fatalf("a sleep that began during the holdoff still ran the check: %d checks", len(checks))
	}
}

func TestEachDRMRendererOpensItsOwnStack(t *testing.T) {
	for _, c := range []struct {
		kind rendererKind
		want drmbackend.Renderer
	}{{vulkanRenderer, drmbackend.Vulkan}, {lavapipeRenderer, drmbackend.Lavapipe}} {
		if got := drmRenderer(c.kind); got != c.want {
			t.Errorf("renderer %v opens the drm stack %d, want %d", c.kind, got, c.want)
		}
	}
}

func TestAPrimaryWithoutVulkanNamesTheCardAndTheWayToLavapipe(t *testing.T) {
	missing := fmt.Errorf("drm: card0: %w /dev/dri/card0: vulkan: no Vulkan device draws", drmbackend.ErrNoVulkan)
	err := withRendererHint(missing)
	if !errors.Is(err, drmbackend.ErrNoVulkan) {
		t.Fatalf("%v no longer wraps ErrNoVulkan", err)
	}
	for _, want := range []string{"/dev/dri/card0", "--renderer lavapipe", "LYS_WM_RENDERER=lavapipe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v does not say %q", err, want)
		}
	}
	other := errors.New("drm: card0: atomic modesetting refused")
	if got := withRendererHint(other); got.Error() != other.Error() {
		t.Errorf("a failure with a Vulkan driver present became %v", got)
	}
}

type sleepRecorder struct {
	broker
	slept []logind.PowerAction
}

func (r *sleepRecorder) Sleep(action logind.PowerAction) error {
	r.slept = append(r.slept, action)
	return nil
}

func TestTheLidPolicySleepsThroughTheConfiguredAction(t *testing.T) {
	for name, want := range map[string]logind.PowerAction{
		"auto":                   logind.AutoSleep,
		"suspend":                logind.Suspend,
		"hibernate":              logind.Hibernate,
		"hybrid-sleep":           logind.HybridSleep,
		"suspend-then-hibernate": logind.SuspendThenHibernate,
	} {
		r := &sleepRecorder{}
		n := &native{logind: r, lidAction: lidActionNamed(t, name)}
		policy := n.lidPolicy()
		if policy.Sleep == nil || policy.OnBattery == nil {
			t.Fatalf("lid action %s: no lid policy", name)
		}
		if err := policy.Sleep(); err != nil {
			t.Fatal(err)
		}
		if len(r.slept) != 1 || r.slept[0] != want {
			t.Errorf("lid action %s slept through %v, want %s", name, r.slept, want)
		}
	}
}

func TestTheIgnoreLidActionHasNoLidPolicy(t *testing.T) {
	n := &native{logind: &sleepRecorder{}, lidAction: lidActionNamed(t, "ignore")}
	if policy := n.lidPolicy(); policy.Sleep != nil || policy.OnBattery != nil {
		t.Fatalf("ignore has a lid policy %+v", policy)
	}
}

type lidInhibitRecorder struct {
	broker
	inhibits int
}

func (r *lidInhibitRecorder) InhibitLidSwitch() (*os.File, error) {
	r.inhibits++
	return os.Open(os.DevNull)
}

func TestTheLogindLidActionTakesNoInhibitorAndHasNoLidPolicy(t *testing.T) {
	r := &lidInhibitRecorder{}
	n := &native{logind: r}
	subsystems, err := n.takeLid(lidActionNamed(t, "logind"))
	if err != nil {
		t.Fatal(err)
	}
	if r.inhibits != 0 || n.lid != nil {
		t.Errorf("logind took the lid switch inhibitor %d times", r.inhibits)
	}
	if len(subsystems) != 0 || n.wake != nil {
		t.Errorf("logind watches %v and rechecks after a wake %v, want neither", subsystems, n.wake != nil)
	}
	if policy := n.lidPolicy(); policy.Sleep != nil || policy.OnBattery != nil {
		t.Errorf("logind has a lid policy %+v", policy)
	}
}

func TestTheOtherLidActionsTakeTheInhibitor(t *testing.T) {
	for _, a := range session.LidActions() {
		if a.LeftToLogind() {
			continue
		}
		t.Run(a.String(), func(t *testing.T) {
			l, err := loop.New()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			r := &lidInhibitRecorder{}
			n := &native{l: l, logind: r}
			subsystems, err := n.takeLid(a)
			if err != nil {
				t.Fatal(err)
			}
			defer n.lid.Close()
			if r.inhibits != 1 || n.lid == nil {
				t.Errorf("%s took the lid switch inhibitor %d times, want once", a, r.inhibits)
			}
			_, sleeps := a.Sleep()
			if watches := slices.Equal(subsystems, []session.Subsystem{session.PowerSupply}); watches != sleeps || (n.wake != nil) != sleeps {
				t.Errorf("%s watches %v and rechecks after a wake %v, want both only when it sleeps (%v)", a, subsystems, n.wake != nil, sleeps)
			}
			if policy := n.lidPolicy(); (policy.Sleep != nil) != sleeps {
				t.Errorf("%s has a lid policy %v, want one only when it sleeps (%v)", a, policy.Sleep != nil, sleeps)
			}
		})
	}
}

func TestADeniedLidInhibitorStopsTheStartUnlessTheLidIsLeftToLogind(t *testing.T) {
	n := &native{logind: failingInhibit{}}
	if _, err := n.takeLid(lidActionNamed(t, "ignore")); !errors.Is(err, errInhibitDenied) {
		t.Fatalf("a denied inhibitor: %v, want it reported", err)
	}
	if _, err := n.takeLid(lidActionNamed(t, "logind")); err != nil {
		t.Fatalf("logind asked for the denied inhibitor: %v", err)
	}
}

var errInhibitDenied = errors.New("logind: permission denied")

type failingInhibit struct{ broker }

func (failingInhibit) InhibitLidSwitch() (*os.File, error) { return nil, errInhibitDenied }

func lidActionNamed(t *testing.T, name string) session.LidAction {
	t.Helper()
	actions := session.LidActions()
	i := slices.IndexFunc(actions, func(a session.LidAction) bool { return a.String() == name })
	if i < 0 {
		t.Fatalf("no lid action %q", name)
	}
	return actions[i]
}
