package wallpaper

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// D-Bus identity.
const (
	ServiceName = "dev.stubbe.lys.Wallpaper1"
	ServicePath = dbus.ObjectPath("/dev/stubbe/lys/Wallpaper")
	Interface   = "dev.stubbe.lys.Wallpaper1"
)

// ColorsExtracted is the signal the service sends from ServicePath each
// time an extraction finished.
var ColorsExtracted = dbusx.NewSignal[struct{}](Interface, "ColorsExtracted")

// Daemon serves a Service as dev.stubbe.lys.Wallpaper1. An
// empty monitor argument targets every monitor.
type Daemon struct {
	svc *Service
}

// Export serves the object and its read-only properties under the
// well-known name and forwards every finished extraction as the
// ColorsExtracted signal. The returned function stops the forwarding,
// releases the name and unexports the object. A name already owned is
// dbusx.ErrNameTaken and leaves nothing exported.
func Export(conn *dbus.Conn, svc *Service) (func(), error) {
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:      ServiceName,
		Path:      ServicePath,
		Interface: Interface,
		Methods:   &Daemon{svc: svc},
		Properties: dbusx.Getters{
			"IsCycling":      func() any { return svc.CyclingConfig() != nil },
			"ThemingMonitor": func() any { return svc.ThemingMonitor() },
		},
		Signals: []dbusx.SignalDecl{ColorsExtracted},
	})
	if err != nil {
		return nil, fmt.Errorf("wallpaper: serve %s: %w", ServiceName, err)
	}
	extracted, unsubscribe := svc.Extracted()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-extracted:
				if err := ColorsExtracted.Emit(conn, ServicePath, struct{}{}); err != nil {
					log.Printf("wallpaper: %v", err)
				}
			}
		}
	}()
	return func() {
		close(done)
		unsubscribe()
		release()
	}, nil
}

// SetWallpaper sets path on a monitor, or all monitors for "".
func (d *Daemon) SetWallpaper(path, monitor string) *dbus.Error {
	if err := d.svc.SetWallpaper(path, monitor); err != nil {
		return dbusx.Failed(err.Error())
	}
	return nil
}

// SetFitMode sets the fit mode on a monitor, or all monitors for "".
func (d *Daemon) SetFitMode(mode, monitor string) *dbus.Error {
	fit, err := ParseFitMode(mode)
	if err != nil {
		return dbusx.InvalidArgs(err.Error())
	}
	d.svc.SetFitMode(fit, monitor)
	return nil
}

// StartCycling starts cycling directory every intervalSecs.
func (d *Daemon) StartCycling(directory string, intervalSecs uint32, mode string) *dbus.Error {
	m, err := ParseCyclingMode(mode)
	if err != nil {
		return dbusx.InvalidArgs(err.Error())
	}
	if err := d.svc.StartCycling(directory, time.Duration(intervalSecs)*time.Second, m); err != nil {
		return dbusx.Failed(err.Error())
	}
	return nil
}

// StopCycling stops cycling.
func (d *Daemon) StopCycling() *dbus.Error {
	d.svc.StopCycling()
	return nil
}

// Next advances every monitor to its next cycle image.
func (d *Daemon) Next() *dbus.Error {
	d.svc.Advance()
	return nil
}

// Previous rewinds every monitor to its previous cycle image.
func (d *Daemon) Previous() *dbus.Error {
	d.svc.Rewind()
	return nil
}

// ExtractColors extracts colors from the theming wallpaper.
func (d *Daemon) ExtractColors() *dbus.Error {
	if err := d.svc.ExtractColors(context.Background()); err != nil {
		return dbusx.Failed(err.Error())
	}
	return nil
}

// WallpaperForMonitor is a monitor's wallpaper path, "" when unknown or
// unset.
func (d *Daemon) WallpaperForMonitor(monitor string) (string, *dbus.Error) {
	path, _ := d.svc.Wallpaper(monitor)
	return path, nil
}

// GetFitMode is a monitor's fit mode; "fill" for an unknown monitor.
func (d *Daemon) GetFitMode(monitor string) (string, *dbus.Error) {
	fit, _ := d.svc.FitModeOf(monitor)
	return fit.String(), nil
}

// GetIsCycling reports whether cycling is active.
func (d *Daemon) GetIsCycling() (bool, *dbus.Error) {
	return d.svc.CyclingConfig() != nil, nil
}

// SetThemingMonitor picks the theming monitor; "" is the default.
func (d *Daemon) SetThemingMonitor(monitor string) *dbus.Error {
	d.svc.SetThemingMonitor(monitor)
	return nil
}

// RegisterMonitor registers a monitor.
func (d *Daemon) RegisterMonitor(monitor string) *dbus.Error {
	d.svc.RegisterMonitor(monitor)
	return nil
}

// UnregisterMonitor unregisters a monitor.
func (d *Daemon) UnregisterMonitor(monitor string) *dbus.Error {
	d.svc.UnregisterMonitor(monitor)
	return nil
}

// ListMonitors lists the registered monitors.
func (d *Daemon) ListMonitors() ([]string, *dbus.Error) {
	return d.svc.MonitorNames(), nil
}

// Client drives a running daemon (the WallpaperProxy the CLI uses).
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// Connect dials the session bus.
func Connect() (*Client, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to D-Bus session bus: %w", err) //nolint:staticcheck // a user-facing CLI message
	}
	return NewClient(conn), nil
}

// NewClient wraps an existing connection.
func NewClient(conn *dbus.Conn) *Client {
	return &Client{conn: conn, obj: conn.Object(ServiceName, ServicePath)}
}

// Close drops the bus connection.
func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) call(ctx context.Context, method string, out any, args ...any) error {
	call := c.obj.CallWithContext(ctx, Interface+"."+method, 0, args...)
	if call.Err != nil || out == nil {
		return call.Err
	}
	return call.Store(out)
}

// SetWallpaper sets path on monitor ("" for all).
func (c *Client) SetWallpaper(ctx context.Context, path, monitor string) error {
	return c.call(ctx, "SetWallpaper", nil, path, monitor)
}

// SetFitMode sets the fit mode on monitor ("" for all).
func (c *Client) SetFitMode(ctx context.Context, mode, monitor string) error {
	return c.call(ctx, "SetFitMode", nil, mode, monitor)
}

// StartCycling starts cycling directory.
func (c *Client) StartCycling(ctx context.Context, directory string, intervalSecs uint32, mode string) error {
	return c.call(ctx, "StartCycling", nil, directory, intervalSecs, mode)
}

// StopCycling stops cycling.
func (c *Client) StopCycling(ctx context.Context) error { return c.call(ctx, "StopCycling", nil) }

// Next advances the cycle.
func (c *Client) Next(ctx context.Context) error { return c.call(ctx, "Next", nil) }

// Previous rewinds the cycle.
func (c *Client) Previous(ctx context.Context) error { return c.call(ctx, "Previous", nil) }

// ExtractColors re-runs color extraction.
func (c *Client) ExtractColors(ctx context.Context) error { return c.call(ctx, "ExtractColors", nil) }

// WallpaperForMonitor reads a monitor's wallpaper path.
func (c *Client) WallpaperForMonitor(ctx context.Context, monitor string) (string, error) {
	var path string
	err := c.call(ctx, "WallpaperForMonitor", &path, monitor)
	return path, err
}

// GetFitMode reads a monitor's fit mode.
func (c *Client) GetFitMode(ctx context.Context, monitor string) (string, error) {
	var mode string
	err := c.call(ctx, "GetFitMode", &mode, monitor)
	return mode, err
}

// GetIsCycling reads whether cycling is active.
func (c *Client) GetIsCycling(ctx context.Context) (bool, error) {
	var on bool
	err := c.call(ctx, "GetIsCycling", &on)
	return on, err
}

// SetThemingMonitor picks the theming monitor ("" for the default).
func (c *Client) SetThemingMonitor(ctx context.Context, monitor string) error {
	return c.call(ctx, "SetThemingMonitor", nil, monitor)
}

// ListMonitors lists the registered monitors.
func (c *Client) ListMonitors(ctx context.Context) ([]string, error) {
	var names []string
	err := c.call(ctx, "ListMonitors", &names)
	return names, err
}
