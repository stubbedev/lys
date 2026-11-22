package sleepreport_test

import (
	"context"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/sleep"

	"github.com/stubbedev/lys/shell/service/sleepreport"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

var slept = time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)

func night(day int, depth sleep.Depth, drain sleep.Drain) sleep.Report {
	from := slept.Add(time.Duration(day) * 24 * time.Hour)
	return sleep.Report{Slept: from, Woke: from.Add(8 * time.Hour), Depth: depth, DeepShare: 0.4, Drain: drain}
}

type sent struct{ summary, body string }

type notifier struct{ sent chan sent }

func (n *notifier) Send(_ context.Context, _, summary, body, _ string) (uint32, error) {
	n.sent <- sent{summary, body}
	return 1, nil
}

type rig struct {
	bus     *dbustest.Bus
	server  *sleep.Server
	release func()
	n       *notifier
}

func (r *rig) serve(t *testing.T, reports []sleep.Report) {
	t.Helper()
	s, release, err := sleep.Serve(r.bus.Conn(t), reports)
	if err != nil {
		t.Fatal(err)
	}
	r.server, r.release = s, release
	t.Cleanup(func() { r.release() })
}

func (r *rig) publish(t *testing.T, reports []sleep.Report) {
	t.Helper()
	if err := r.server.Publish(reports); err != nil {
		t.Fatal(err)
	}
}

func start(t *testing.T, initial []sleep.Report) (*rig, *sleepreport.Monitor) {
	t.Helper()
	r := &rig{bus: dbustest.Start(t), n: &notifier{sent: make(chan sent, 8)}}
	r.serve(t, initial)
	m := sleepreport.Follow(r.bus.Conn(t), func(err error) { t.Error(err) })
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	waittest.Until(t, func() bool { _, running := m.Value(); return running }, "the first reports")
	announce := sleepreport.Announcing(m)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		announce(ctx, r.n)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return r, m
}

func noSend(t *testing.T, n *notifier) {
	t.Helper()
	select {
	case s := <-n.sent:
		t.Errorf("told of %+v", s)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAShallowSleepThatDrainedTheBatteryIsToldOfOnce(t *testing.T) {
	old := night(0, sleep.DepthShallow, sleep.Drew(2000))
	r, m := start(t, []sleep.Report{old})
	bad := night(1, sleep.DepthShallow, sleep.Drew(1400))
	bad.Blockers, bad.WokenBy = []string{"USB2PLL_OFF_STS"}, []string{"PNP0C0D:00", "rtc0"}
	r.publish(t, []sleep.Report{old, bad})
	got := waittest.For(t, r.n.sent, "the notification")
	if got.summary != i18n.T("battery-sleep-drained") {
		t.Errorf("summary %q", got.summary)
	}
	want := i18n.T("battery-sleep-drained-body", i18n.Str("watts", "1.4"), i18n.Str("hours", "8.0"), i18n.Str("percent", "40")) + "\n" +
		i18n.T("battery-sleep-blocked", i18n.Str("devices", "USB2PLL_OFF_STS")) + "\n" +
		i18n.T("battery-sleep-woken", i18n.Str("devices", "PNP0C0D:00, rtc0"))
	if got.body != want {
		t.Errorf("body %q, want %q", got.body, want)
	}
	r.publish(t, []sleep.Report{old, bad})
	r.release()
	waittest.Until(t, func() bool { _, running := m.Value(); return running == false }, "lys-power gone")
	r.serve(t, []sleep.Report{old, bad})
	waittest.Until(t, func() bool { v, running := m.Value(); return running && len(v) == 2 }, "lys-power back")
	noSend(t, r.n)
}

func TestAGoodOrUnjudgedSleepRaisesNothing(t *testing.T) {
	r, _ := start(t, nil)
	deep := night(0, sleep.DepthDeep, sleep.Drew(90))
	onAC := night(1, sleep.DepthShallow, sleep.Drain{})
	unmeasured := night(2, sleep.DepthUnmeasured, sleep.Drew(3000))
	r.publish(t, []sleep.Report{deep})
	r.publish(t, []sleep.Report{deep, onAC})
	r.publish(t, []sleep.Report{deep, onAC, unmeasured})
	noSend(t, r.n)
}

func TestTheFirstReportsAreNotNews(t *testing.T) {
	var seen sleepreport.Seen
	bad := night(0, sleep.DepthShallow, sleep.Drew(1400))
	seen, _, tell := seen.Step([]sleep.Report{bad})
	if tell {
		t.Fatal("a sleep from before the shell started was told of")
	}
	if _, _, tell := seen.Step([]sleep.Report{bad}); tell {
		t.Error("the same sleep was told of again")
	}
	later := night(1, sleep.DepthShallow, sleep.Drew(1400))
	if _, r, tell := seen.Step([]sleep.Report{bad, later}); !tell || !r.Slept.Equal(later.Slept) {
		t.Errorf("a new sleep that drained is told: %v %+v", tell, r)
	}
	var empty sleepreport.Seen
	empty, _, _ = empty.Step(nil)
	if _, _, tell := empty.Step([]sleep.Report{later}); !tell {
		t.Error("the first sleep after an empty list was not told of")
	}
}
