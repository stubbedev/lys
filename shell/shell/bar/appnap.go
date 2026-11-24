package bar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/desktop/service/powersupply"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/nap"

	"github.com/stubbedev/lys/shell/service/appnap"
	"github.com/stubbedev/lys/shell/service/idleinhibit"
)

const (
	procDir = "/proc"
	sysDir  = "/sys"
)

type appNapDeps struct {
	session, system *dbus.Conn
	audio           *pulse.Service
	inhibit         *idleinhibit.State
	config          func() *config.Config
	reloads         <-chan struct{}
}

func startAppNap(d appNapDeps) func() {
	if detectCompositor() != compositor.LysWM {
		return func() {}
	}
	if d.session == nil || d.system == nil || d.audio == nil {
		log.Print("app nap: hidden apps are not napped without the session and system buses and the sound server")
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := runAppNap(ctx, d); err != nil {
			log.Printf("app nap: %v; hidden apps are not napped", err)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func runAppNap(ctx context.Context, d appNapDeps) error {
	windows, err := lyswm.WatchWindows()
	if err != nil {
		return fmt.Errorf("follow lys-wm's windows: %w", err)
	}
	defer windows.Close()
	audio, stopAudio, err := d.audio.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("follow the sound server: %w", err)
	}
	defer stopAudio()
	inhibits, stopInhibits := d.inhibit.Changes()
	defer stopInhibits()
	battery := powersupply.New(powersupply.Dir)
	supply, stopSupply, err := battery.SubscribeOnBattery(ctx)
	if err != nil {
		return fmt.Errorf("follow the power supply: %w", err)
	}
	defer func() {
		if err := stopSupply(); err != nil {
			log.Printf("app nap: %v", err)
		}
	}()
	src := appnap.Sources{
		Windows:   windows.Windows,
		Streams:   func() []int32 { return streamPIDs(d.audio) },
		Holds:     func(ctx context.Context) ([]appnap.Hold, error) { return holds(ctx, d.session, d.inhibit) },
		OnBattery: battery.OnBattery,
		Levels:    func() (map[string]nap.Level, error) { return napLevels(d.config().Battery.AppNap) },
		Proc:      procDir,
	}
	n := appnap.NewNapper(appnap.NewSystemd(d.session, nap.NewClient(d.system), sysDir))
	ticks := mergeTicks(ctx, windows.Ticks(), audio, inhibits, supply, d.reloads)
	appnap.Run(ctx, n, src, ticks, func(err error) { log.Printf("app nap: %v", err) })
	return nil
}

func mergeTicks(ctx context.Context, sources ...<-chan struct{}) <-chan struct{} {
	out := make(chan struct{}, 1)
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-src:
					if !ok {
						return
					}
					select {
					case out <- struct{}{}:
					default:
					}
				}
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func streamPIDs(audio *pulse.Service) []int32 {
	streams := slices.Concat(audio.PlaybackStreams(), audio.RecordingStreams())
	pids := make([]int32, 0, len(streams))
	for _, s := range streams {
		if s.PID > 0 && !s.Corked {
			pids = append(pids, int32(s.PID)) //nolint:gosec // a pid fits 31 bits
		}
	}
	return pids
}

func holds(ctx context.Context, session *dbus.Conn, inhibit *idleinhibit.State) ([]appnap.Hold, error) {
	held := inhibit.Holds()
	out := make([]appnap.Hold, 0, len(held))
	var errs error
	for _, h := range held {
		var pid uint32
		err := session.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, h.Owner).Store(&pid)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("the process of %s, which holds idle for %q: %w", h.Owner, h.App, err))
			continue
		}
		id, err := intconv.Exact[int32](pid)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("the process of %s: %w", h.Owner, err))
			continue
		}
		out = append(out, appnap.Hold{PID: id, App: h.App})
	}
	return out, errs
}

func napLevels(levels map[string]config.NapLevel) (map[string]nap.Level, error) {
	out := make(map[string]nap.Level, len(levels))
	for app, l := range levels {
		parsed, err := nap.ParseLevel(string(l))
		if err != nil {
			return nil, fmt.Errorf("the nap level of %s: %w", app, err)
		}
		out[app] = parsed
	}
	return out, nil
}
