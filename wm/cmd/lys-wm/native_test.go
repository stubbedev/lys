package main

import (
	"errors"
	"fmt"
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

func TestAPrimaryWithoutVulkanNamesTheCardAndTheWayToGLES(t *testing.T) {
	missing := fmt.Errorf("drm: card0: %w /dev/dri/card0: vulkan: no Vulkan device draws", drmbackend.ErrNoVulkan)
	err := withRendererHint(missing)
	if !errors.Is(err, drmbackend.ErrNoVulkan) {
		t.Fatalf("%v no longer wraps ErrNoVulkan", err)
	}
	for _, want := range []string{"/dev/dri/card0", "--renderer gles", "LYS_WM_RENDERER=gles"} {
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
		"suspend":                logind.Suspend,
		"hibernate":              logind.Hibernate,
		"hybrid-sleep":           logind.HybridSleep,
		"suspend-then-hibernate": logind.SuspendThenHibernate,
	} {
		r := &sleepRecorder{}
		n := &native{logind: r, lidAction: session.LidActions()[name]}
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
	n := &native{logind: &sleepRecorder{}, lidAction: session.LidActions()["ignore"]}
	if policy := n.lidPolicy(); policy.Sleep != nil || policy.OnBattery != nil {
		t.Fatalf("ignore has a lid policy %+v", policy)
	}
}
