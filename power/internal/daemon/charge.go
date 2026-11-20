package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/polkit"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/power/internal/busauth"
	"github.com/stubbedev/lys/power/internal/chargepolicy"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/dock"
	"github.com/stubbedev/lys/power/internal/history"
	"github.com/stubbedev/lys/power/internal/statefile"
)

// ChargeAction is the polkit action that chooses the charge limit, asks
// for a full charge and passes the next departure.
const ChargeAction polkit.Action = "dev.stubbe.lys.power.charge"

// ChargeInput gathers what the charge policy decides on at now.
func ChargeInput(cfg config.Config, now time.Time, onAC bool) (chargepolicy.Input, error) {
	limit, err := ReadLimit(cfg.LimitPath())
	if err != nil {
		return chargepolicy.Input{}, err
	}
	docked, err := dock.Read(cfg.SysDir)
	if err != nil {
		return chargepolicy.Input{}, err
	}
	requested, err := exists(cfg.FullNowPath())
	if err != nil {
		return chargepolicy.Input{}, fmt.Errorf("checking charge-to-full flag: %w", err)
	}
	entries, err := history.Load(cfg.HistoryPath())
	if err != nil {
		return chargepolicy.Input{}, fmt.Errorf("reading unplug history: %w", err)
	}
	departure, err := ReadDeparture(cfg.DeparturePath())
	if err != nil {
		return chargepolicy.Input{}, err
	}
	return chargepolicy.Input{
		Now: now, Limit: limit, OnAC: onAC, Requested: requested,
		Dock: docked, History: entries, Departure: departure,
	}, nil
}

// ReadLimit is the charge limit stored at path, the default before one
// is chosen.
func ReadLimit(path string) (charge.Limit, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return charge.DefaultLimit, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading the charge limit: %w", err)
	}
	percent, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 8)
	if err != nil {
		return 0, fmt.Errorf("reading the charge limit in %s: %w", path, err)
	}
	limit, err := charge.ParseLimit(percent)
	if err != nil {
		return 0, fmt.Errorf("reading the charge limit in %s: %w", path, err)
	}
	return limit, nil
}

// ReadDeparture is the departure stored at path, zero when none is.
func ReadDeparture(path string) (time.Time, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("reading the next departure: %w", err)
	}
	unix, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || unix <= 0 {
		return time.Time{}, fmt.Errorf("reading the next departure in %s: %q is no time", path, raw)
	}
	return time.Unix(unix, 0), nil
}

// RequestFull asks for a full charge until the next unplug.
func RequestFull(cfg config.Config, now time.Time) error {
	path := cfg.FullNowPath()
	err := os.WriteFile(path, []byte(strconv.FormatInt(now.Unix(), 10)), 0o600)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s is missing, is lys-power.service running?", cfg.RuntimeDir)
	}
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// recordDocked keeps the docked mark for the AC session: set once the
// laptop is seen docked on AC, cleared on every AC edge after the unplug
// it describes was recorded.
func recordDocked(cfg config.Config, onAC, edge bool, state charge.Dock) error {
	path := cfg.DockedPath()
	if edge {
		if _, err := removeIfExists(path); err != nil {
			return fmt.Errorf("clearing the docked mark: %w", err)
		}
	}
	if !onAC || state != charge.DockDocked {
		return nil
	}
	marked, err := exists(path)
	if err != nil || marked {
		return err
	}
	if err := statefile.Write(path, nil); err != nil {
		return fmt.Errorf("marking the AC session docked: %w", err)
	}
	return nil
}

// charges serves dev.stubbe.lys.Charge1: the methods change the files
// the policy reads, and the state of each evaluation is published.
type charges struct {
	cfg       config.Config
	conn      *dbus.Conn
	authorize busauth.Authorizer
	log       *slog.Logger
	wakes     chan struct{}

	mu      sync.Mutex
	server  *charge.Server
	release func()
}

func newCharges(cfg config.Config, conn *dbus.Conn, authorize busauth.Authorizer, log *slog.Logger) *charges {
	return &charges{cfg: cfg, conn: conn, authorize: authorize, log: log, wakes: make(chan struct{}, 1)}
}

// publish serves the state, claiming the name with the first.
func (c *charges) publish(st charge.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.server == nil {
		server, release, err := charge.Serve(c.conn, c, st)
		if err != nil {
			c.log.Error("the charge policy is not on the bus", "err", err)
			return
		}
		c.server, c.release = server, release
		return
	}
	if err := c.server.Publish(st); err != nil {
		c.log.Error("publishing the charge policy", "err", err)
	}
}

func (c *charges) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.release != nil {
		c.release()
	}
}

func (c *charges) allowed(ctx context.Context, caller charge.Caller) error {
	if caller.UID == 0 {
		return nil
	}
	return c.authorize(ctx, caller)
}

func (c *charges) SetLimit(ctx context.Context, caller charge.Caller, limit charge.Limit) error {
	if err := c.allowed(ctx, caller); err != nil {
		return err
	}
	if err := statefile.Write(c.cfg.LimitPath(), []byte(strconv.Itoa(limit.Percent()))); err != nil {
		return fmt.Errorf("storing the charge limit %d%%: %w", limit, err)
	}
	notify(c.wakes)
	return nil
}

func (c *charges) ChargeFull(ctx context.Context, caller charge.Caller) error {
	if err := c.allowed(ctx, caller); err != nil {
		return err
	}
	return RequestFull(c.cfg, time.Now())
}

func (c *charges) SetDeparture(ctx context.Context, caller charge.Caller, at time.Time) error {
	if err := c.allowed(ctx, caller); err != nil {
		return err
	}
	path := c.cfg.DeparturePath()
	if at.IsZero() {
		if _, err := removeIfExists(path); err != nil {
			return fmt.Errorf("clearing the next departure: %w", err)
		}
		return nil
	}
	if err := statefile.Write(path, strconv.AppendInt(nil, at.Unix(), 10)); err != nil {
		return fmt.Errorf("storing the next departure %v: %w", at, err)
	}
	return nil
}

// watchDisplays wakes on every kernel uevent from the DRM subsystem,
// where a display being connected or disconnected is announced.
func watchDisplays() (*fdWake, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return nil, fmt.Errorf("opening a uevent socket: %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: 1}); err != nil {
		return nil, errors.Join(fmt.Errorf("listening for kernel uevents: %w", err), unix.Close(fd))
	}
	w := &fdWake{file: os.NewFile(uintptr(fd), "uevent"), c: make(chan struct{}, 1)}
	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, err := w.file.Read(buf)
			if errors.Is(err, unix.ENOBUFS) {
				notify(w.c)
				continue
			}
			if err != nil {
				return
			}
			if dock.IsDRM(buf[:n]) {
				notify(w.c)
			}
		}
	}()
	return w, nil
}
