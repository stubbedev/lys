package bar

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/apptest"
	"github.com/stubbedev/lys/shell/internal/notifytest"
	"github.com/stubbedev/lys/shell/service/sysinfo"
	"github.com/stubbedev/lys/shell/shell/powermenu"
)

type countedNetwork struct {
	fakeNetworkSource
	reads atomic.Int64
	stall atomic.Pointer[chan struct{}]
}

func (f *countedNetwork) Read(ctx context.Context) (network.Snapshot, error) {
	f.reads.Add(1)
	if ch := f.stall.Load(); ch != nil {
		<-*ch
	}
	return f.fakeNetworkSource.Read(ctx)
}

func newCountedNetwork() *countedNetwork {
	f := &countedNetwork{}
	f.snap = network.Snapshot{HasWifi: true, WifiEnabled: true, WifiSSID: "lan", Primary: network.ConnectionWifi}
	f.ticks = make(chan struct{}, 2)
	return f
}

type countedProfiles struct {
	fakePPSource
	reads atomic.Int64
}

func (f *countedProfiles) Read(ctx context.Context) (powerprofiles.Snapshot, error) {
	f.reads.Add(1)
	return f.fakePPSource.Read(ctx)
}

func newCountedProfiles() *countedProfiles {
	f := &countedProfiles{}
	f.snap = powerprofiles.Snapshot{Available: true, Active: powerprofiles.ProfileBalanced}
	f.ticks = make(chan struct{}, 2)
	return f
}

type countedBattery struct {
	*fakeBattery
	reads atomic.Int64
}

func (f *countedBattery) Read(ctx context.Context) (upower.Device, error) {
	f.reads.Add(1)
	return f.fakeBattery.Read(ctx)
}

type countedMedia struct {
	*fakeMediaSource
	positions atomic.Int64
	at        atomic.Int64
}

func (f *countedMedia) Position(context.Context, string) (time.Duration, error) {
	f.positions.Add(1)
	return time.Duration(f.at.Load()), nil
}

type dashboardRig struct {
	t        *testing.T
	a        *app.Application
	reg      *dropdownRegistry
	anchor   *widget.Box
	layer    *app.LayerWindow
	network  *countedNetwork
	profiles *countedProfiles
	battery  *countedBattery
	media    *countedMedia
}

const dashboardRigOutput = "RIG-1"

const (
	dashboardFirstOpenMallocs = 20000
	dashboardReopenMallocs    = 3000
	dashboardPollAllocs       = 30
	dashboardPollReads        = 6
)

func newDashboardRig(t *testing.T) *dashboardRig {
	t.Helper()
	cfg := config.Defaults()
	ctx := styledContext(t, cfg)
	withoutCompose(t)
	a := apptest.Run(t)
	apptest.On(a, func() bool { a.SetSurfaceMotion(false); return true })
	restore := widget.SetAnimationsInstant(true)
	t.Cleanup(restore)
	ctx.App = a
	warmDashboard(t)
	rig := &dashboardRig{
		t: t, a: a,
		network:  newCountedNetwork(),
		profiles: newCountedProfiles(),
		battery:  &countedBattery{fakeBattery: newFakeBattery(upower.Device{Percentage: 64, IsPresent: true, State: upower.StateDischarging, TimeToEmpty: time.Hour})},
		media:    &countedMedia{fakeMediaSource: newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying, CanGoNext: true})},
	}
	ctx.Network, ctx.PowerProfiles, ctx.Battery, ctx.Media = rig.network, rig.profiles, rig.battery, rig.media
	ctx.Notifications = notifytest.NewService(t)
	ctx.Notifications.SetDND(false)
	ctx.Pulse = &fakePulseSource{dev: pulse.Device{Description: "Speakers", Volume: pct(40)}, ticks: make(chan struct{}, 2)}
	full := computeStyle(cfg, styling.Default())
	apptest.On(a, func() error {
		root := widget.NewBox(widget.Row, 0, 0)
		ctx.Theme.Attach(root)
		rig.anchor = widget.NewBox(widget.Row, 0, 0)
		rig.anchor.SetSizeRequest(40, 30)
		root.Append(rig.anchor, false)
		layer, err := a.NewLayer(app.LayerConfig{
			Root: root, Anchor: app.AnchorTop | app.AnchorLeft | app.AnchorRight,
			Height: 30, Namespace: "dashboard-rig", Layer: app.LayerTop,
		})
		if err != nil {
			return err
		}
		rig.reg = newDropdownRegistry(a, cfg, ctx.Font, &full, ctx)
		rig.reg.builders["dashboard"] = noRadiosDashboard
		rig.reg.attachHost(dashboardRigOutput, layer, widget.Size{W: 1600, H: 900})
		rig.layer = layer
		return nil
	})
	apptest.Until(t, a, "the bar is laid out", func() bool { return rig.anchor.Bounds() != render.Rect{} })
	t.Cleanup(func() { apptest.On(a, func() bool { rig.reg.release(); return true }) })
	return rig
}

func (r *dashboardRig) content() widget.Widget {
	r.reg.mu.Lock()
	defer r.reg.mu.Unlock()
	return r.reg.instances["dashboard"]
}

func (r *dashboardRig) reads() int64 {
	return r.network.reads.Load() + r.profiles.reads.Load() + r.battery.reads.Load()
}

func (r *dashboardRig) open() {
	r.t.Helper()
	if err := apptest.On(r.a, func() error { return r.reg.open(dashboardRigOutput, "dashboard", r.anchor, openedByAnchor) }); err != nil {
		r.t.Fatalf("open the dashboard: %v", err)
	}
}

func (r *dashboardRig) measureOpen() (uint64, time.Duration) {
	r.t.Helper()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	r.open()
	apptest.Until(r.t, r.a, "the dashboard is arranged", func() bool {
		content := r.content()
		if content == nil {
			return false
		}
		b, ok := findByClass(content, "quick-actions").(widget.Boundser)
		return ok && b.Bounds().W > 0 && widget.TreeAssetsOwed(content) == 0
	})
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs, elapsed
}

func (r *dashboardRig) close() {
	r.t.Helper()
	apptest.On(r.a, func() bool { r.reg.closeAll(); return true })
	apptest.Dispatched(r.t, r.a)
}

func warmDashboard(t *testing.T) {
	t.Helper()
	widget.ThemeIconExists("ld-layout-dashboard-symbolic")
	i18n.T("dropdown-dashboard-title")
	if _, err := app.Font("sans-serif", 12); err != nil {
		t.Fatalf("the system fonts: %v", err)
	}
}

func withoutCompose(t *testing.T) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "XCompose")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write an empty compose file: %v", err)
	}
	t.Setenv("XCOMPOSEFILE", empty)
}

func (r *dashboardRig) awaitReads(n int64) {
	r.t.Helper()
	apptest.Until(r.t, r.a, "the services are read", func() bool {
		return r.network.reads.Load() >= n && r.profiles.reads.Load() >= n && r.battery.reads.Load() >= n
	})
	apptest.Dispatched(r.t, r.a)
}

func (r *dashboardRig) assertReads(what string, n int64) {
	r.t.Helper()
	if nw, pp, bat := r.network.reads.Load(), r.profiles.reads.Load(), r.battery.reads.Load(); nw != n || pp != n || bat != n {
		r.t.Errorf("%s: network %d, power profiles %d, battery %d reads, want %d each", what, nw, pp, bat, n)
	}
}

func TestDashboardOpenCounts(t *testing.T) {
	rig := newDashboardRig(t)
	firstMallocs, firstTime := rig.measureOpen()
	rig.awaitReads(1)
	rig.assertReads("the first open", 1)
	rig.close()
	if rig.reads() != 3 {
		t.Errorf("closing read the services: %d reads, want the open's 3", rig.reads())
	}
	reopenMallocs, reopenTime := rig.measureOpen()
	rig.awaitReads(2)
	rig.assertReads("a reopen", 2)
	rig.close()
	t.Logf("first open: %d mallocs, %v; reopen: %d mallocs, %v", firstMallocs, firstTime, reopenMallocs, reopenTime)
	if raceEnabled {
		return
	}
	if firstMallocs > dashboardFirstOpenMallocs {
		t.Errorf("the first open made %d allocations, want at most %d", firstMallocs, dashboardFirstOpenMallocs)
	}
	if reopenMallocs > dashboardReopenMallocs {
		t.Errorf("a reopen made %d allocations, want at most %d", reopenMallocs, dashboardReopenMallocs)
	}
}

func TestDashboardOpenDoesNotWaitForAService(t *testing.T) {
	rig := newDashboardRig(t)
	release := make(chan struct{})
	rig.network.stall.Store(&release)
	freed := time.AfterFunc(5*time.Second, func() { close(release) })
	t.Cleanup(func() {
		if freed.Stop() {
			close(release)
		}
	})
	rig.measureOpen()
	select {
	case <-release:
		t.Fatal("the dashboard opened only after the stalled read gave up")
	default:
	}
	if got := rig.network.reads.Load(); got != 1 {
		t.Errorf("network reads while stalled = %d, want the 1 waiting", got)
	}
}

func TestDashboardFollowsOnlyWhileOpen(t *testing.T) {
	rig := newDashboardRig(t)
	rig.open()
	rig.awaitReads(1)
	apptest.Until(t, rig.a, "an open dashboard polls", func() bool { return rig.media.positions.Load() > 0 })
	rig.network.ticks <- struct{}{}
	rig.awaitReads(2)
	rig.close()

	reads, positions := rig.reads(), rig.media.positions.Load()
	rig.network.ticks <- struct{}{}
	rig.battery.update(func(d *upower.Device) { d.Percentage = 11 })
	rig.battery.ticks <- struct{}{}
	time.Sleep(dashboardPoll*2 + dashboardPoll/2)
	apptest.Dispatched(t, rig.a)
	if got := rig.reads() - reads; got != 0 {
		t.Errorf("a closed dashboard read its services %d times", got)
	}
	if got := rig.media.positions.Load() - positions; got != 0 {
		t.Errorf("a closed dashboard polled %d times", got)
	}

	rig.open()
	rig.awaitReads(3)
	percent := findByClass(rig.content(), "battery-percent").(*widget.Label)
	apptest.Until(t, rig.a, "the reopened dashboard shows the change it missed", func() bool { return percent.Text() == "11%" })
}

func TestDashboardMediaProgressFollowsPlayback(t *testing.T) {
	rig := newDashboardRig(t)
	rig.media.players[0].Length = 100 * time.Second
	rig.media.at.Store(int64(10 * time.Second))
	rig.open()
	rig.awaitReads(1)
	content := rig.content()
	seek := findByClass(content, "media-seek-slider").(interface{ Value() float64 })
	elapsed := findByClass(content, "media-time").(*widget.Label)
	progress := func() (float64, string) { return seek.Value(), elapsed.Text() }
	apptest.Until(t, rig.a, "the first position shows", func() bool { v, text := progress(); return v == 10 && text == "0:10" })
	rig.media.at.Store(int64(11 * time.Second))
	apptest.Until(t, rig.a, "the next position shows", func() bool { v, text := progress(); return v == 11 && text == "0:11" })
	rig.media.at.Store(int64(12 * time.Second))
	apptest.Until(t, rig.a, "playback keeps moving the progress", func() bool { v, text := progress(); return v == 12 && text == "0:12" })
	rig.close()
	rig.media.at.Store(int64(20 * time.Second))
	rig.open()
	apptest.Until(t, rig.a, "a reopened dashboard shows where playback is", func() bool { v, text := progress(); return v == 20 && text == "0:20" })
	rig.media.at.Store(int64(21 * time.Second))
	apptest.Until(t, rig.a, "a reopened dashboard follows playback", func() bool { v, text := progress(); return v == 21 && text == "0:21" })
}

func TestDashboardPollCounts(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector's own allocations and reads swamp the count")
	}
	fixedProc(t)
	ctx := styledContext(t, config.Defaults())
	v := openView(t, ctx, noRadiosDashboard).(*dashboardView)
	v.poll()
	v.diskIn = 0
	allocs := testing.AllocsPerRun(dashboardDiskPolls, v.poll)
	v.diskIn = 0
	reads := procReads(t, func() {
		for range dashboardDiskPolls {
			v.poll()
		}
	}) / dashboardDiskPolls
	t.Logf("one poll: %.0f allocs, %d read syscalls", allocs, reads)
	if allocs > dashboardPollAllocs {
		t.Errorf("a poll makes %.0f allocations, want at most %d", allocs, dashboardPollAllocs)
	}
	if reads > dashboardPollReads {
		t.Errorf("a poll makes %d read syscalls, want at most %d", reads, dashboardPollReads)
	}
}

func fixedProc(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "net"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"net/dev": "Inter-|   Receive\n face |bytes\n  eth0: 100 1 0 0 0 0 0 0 200 1 0 0 0 0 0 0\n",
		"stat":    "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 50 0 50 400 0 0 0 0 0 0\ncpu1 50 0 50 400 0 0 0 0 0 0\nintr 0\n",
		"meminfo": "MemTotal: 1000 kB\nMemFree: 500 kB\nMemAvailable: 600 kB\nBuffers: 0 kB\nCached: 0 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n",
		"mounts":  "/dev/root / ext4 rw 0 0\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	proc, sys := sysinfo.ProcRoot, sysinfo.SysRoot
	sysinfo.ProcRoot, sysinfo.SysRoot = dir, t.TempDir()
	t.Cleanup(func() { sysinfo.ProcRoot, sysinfo.SysRoot = proc, sys })
}

func procReads(t *testing.T, fn func()) int64 {
	t.Helper()
	read := func() int64 {
		data, err := os.ReadFile("/proc/self/io")
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "syscr: "); ok {
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				return n
			}
		}
		t.Fatalf("no syscr in /proc/self/io: %q", data)
		return 0
	}
	before := read()
	fn()
	return read() - before
}

func TestTheDashboardHasSessionButtonsOnlyWithAPowerMenu(t *testing.T) {
	ctx := styledContext(t, config.Defaults())
	ctx.PowerMenu = nil
	if n := countClass(openView(t, ctx, noRadiosDashboard), "dashboard-user-session"); n != 0 {
		t.Errorf("without a power menu the dashboard has %d session rows, whose buttons would request nothing; want none", n)
	}
	if n := countClass(openView(t, withPowerMenu(ctx), noRadiosDashboard), "dashboard-user-session"); n != 1 {
		t.Errorf("with a power menu the dashboard has %d session rows, want 1", n)
	}
}

func withPowerMenu(ctx ModuleContext) ModuleContext {
	ctx.PowerMenu = powermenu.New(powermenu.Deps{Config: func() *config.Config { return ctx.Config }, Font: ctx.Font})
	return ctx
}
