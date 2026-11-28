package notifications

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// Server serves org.freedesktop.Notifications on the session bus and
// lys-shell's dev.stubbe.lys.Notifications1 extension beside it.
type Server struct {
	svc     *Service
	release func()
}

// Serve takes the well-known names and exports the objects; Release
// drops them. A name already owned is dbusx.ErrNameTaken and leaves
// nothing exported.
func Serve(conn *dbus.Conn, svc *Service) (*Server, error) {
	svc.SetEmitter(func(signal string, args ...any) {
		if err := conn.Emit(ObjPath, signal, args...); err != nil {
			log.Printf("notifications: emit %s: %v", signal, err)
		}
	})
	server := &Server{svc: svc}
	release, err := dbusx.Serve(conn, dbusx.Service{Name: Interface, Path: ObjPath, Interface: Interface, Methods: server})
	if err != nil {
		return nil, fmt.Errorf("notifications: serve %s: %w", Interface, err)
	}
	releaseExts, err := ServeLysShell(conn, svc)
	if err != nil {
		release()
		return nil, err
	}
	server.release = func() {
		releaseExts()
		release()
	}
	return server, nil
}

// Release drops the names and unexports the objects.
func (s *Server) Release() error {
	s.release()
	return nil
}

// Notify handles the spec's Notify call.
func (s *Server) Notify(appName string, replacesID uint32, appIcon, summary, body string, actions []string, hints map[string]dbus.Variant, expireTimeout int32) (uint32, *dbus.Error) {
	return s.svc.NotifyHints(appName, replacesID, appIcon, summary, body, actions, hints, expireTimeout), nil
}

// CloseNotification handles the spec's CloseNotification call; an
// unknown id is not an error (the spec leaves it to the server).
func (s *Server) CloseNotification(id uint32) *dbus.Error {
	s.svc.Close(id, ClosedCall)
	return nil
}

// GetCapabilities handles the spec's GetCapabilities call.
func (s *Server) GetCapabilities() ([]string, *dbus.Error) {
	return Capabilities, nil
}

// GetServerInformation handles the spec's information call.
func (s *Server) GetServerInformation() (string, string, string, string, *dbus.Error) {
	return ServerName, ServerVendor, ServerVersion, SpecVersion, nil
}

// lys-shell's control extension, which `lys-shell notify` drives.
const (
	LysShellName = "dev.stubbe.lys.Notifications1"
	LysShellPath = dbus.ObjectPath("/dev/stubbe/lys/Notifications")
)

// defaultPopupDuration is how long a popup shows, in milliseconds, until configured.
const defaultPopupDuration = 5000

// lysShellDaemon is the dev.stubbe.lys.Notifications1 object (LysShellDaemon).
type lysShellDaemon struct {
	svc           *Service
	popupDuration atomic.Uint32
}

// DismissAll empties the history.
func (d *lysShellDaemon) DismissAll() *dbus.Error {
	d.svc.DismissAll()
	return nil
}

// Dismiss removes one notification as dismissed by the user.
func (d *lysShellDaemon) Dismiss(id uint32) *dbus.Error {
	d.svc.Close(id, Dismissed)
	return nil
}

// SetDnd sets do-not-disturb.
func (d *lysShellDaemon) SetDnd(enabled bool) *dbus.Error {
	d.svc.SetDND(enabled)
	return nil
}

// SetDndUntil turns do-not-disturb on until a Unix time in seconds.
func (d *lysShellDaemon) SetDndUntil(until int64) *dbus.Error {
	at := time.Unix(until, 0)
	if !at.After(time.Now()) {
		return dbusx.InvalidArgs("do-not-disturb until a time that has passed")
	}
	d.svc.SetDNDUntil(at)
	return nil
}

// ToggleDnd flips do-not-disturb.
func (d *lysShellDaemon) ToggleDnd() *dbus.Error {
	d.svc.ToggleDND()
	return nil
}

// SetPopupDuration stores the popup duration the property reports.
func (d *lysShellDaemon) SetPopupDuration(ms uint32) *dbus.Error {
	d.popupDuration.Store(ms)
	return nil
}

// ListRow is one List row: (id, arrival in Unix milliseconds, app,
// summary, body). The last row's (Stamp, ID) is the next page's cursor.
type ListRow struct {
	ID      uint32
	Stamp   int64
	App     string
	Summary string
	Body    string
}

// List is the history's first page, newest first: up to limit rows,
// fewer at the end of the history.
func (d *lysShellDaemon) List(limit uint32) ([]ListRow, *dbus.Error) {
	return d.page(nil, limit)
}

// ListBefore is the page after the row (stamp, id): up to limit rows
// strictly older, newest first.
func (d *lysShellDaemon) ListBefore(stamp int64, id, limit uint32) ([]ListRow, *dbus.Error) {
	return d.page(&Key{Stamp: stamp, ID: id}, limit)
}

func (d *lysShellDaemon) page(before *Key, limit uint32) ([]ListRow, *dbus.Error) {
	notifs, err := d.svc.Page(before, intconv.Clamp[int](limit))
	if err != nil {
		return nil, dbus.MakeFailedError(err)
	}
	rows := make([]ListRow, 0, len(notifs))
	for _, n := range notifs {
		rows = append(rows, ListRow{n.ID, n.Key().Stamp, n.AppName, n.Summary, n.Body})
	}
	return rows, nil
}

// ServeLysShell exports dev.stubbe.lys.Notifications1 over svc, and
// announces Dnd, DndReason and DndUntil in a PropertiesChanged whenever
// do-not-disturb changes, until the returned release.
func ServeLysShell(conn *dbus.Conn, svc *Service) (func(), error) {
	d := &lysShellDaemon{svc: svc}
	d.popupDuration.Store(defaultPopupDuration)
	focus := func(f Focus) map[string]dbus.Variant {
		return map[string]dbus.Variant{
			"Dnd":       dbus.MakeVariant(f.On),
			"DndReason": dbus.MakeVariant(f.Reason.String()),
			"DndUntil":  dbus.MakeVariant(unixOrZero(f.Until)),
		}
	}
	properties := dbusx.Getters{
		"PopupDuration": func() any { return d.popupDuration.Load() },
		"Count":         func() any { return intconv.Clamp[uint32](svc.Count()) },
		"PopupCount":    func() any { return intconv.Clamp[uint32](len(svc.Popups())) },
	}
	for name := range focus(Focus{}) {
		properties[name] = func() any { return focus(svc.Focus())[name].Value() }
	}
	events, stop := svc.Subscribe()
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       LysShellName,
		Path:       LysShellPath,
		Interface:  LysShellName,
		Methods:    d,
		Properties: properties,
	})
	if err != nil {
		stop()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range events {
			if ev.Kind != EventDnd {
				continue
			}
			changed := dbusx.ChangedProperties{Interface: LysShellName, Changed: focus(svc.Focus()), Invalidated: []string{}}
			if err := dbusx.PropertiesChanged.Emit(conn, LysShellPath, changed); err != nil {
				log.Printf("notifications: announce the do-not-disturb change: %v", err)
			}
		}
	}()
	return func() {
		stop()
		<-done
		release()
	}, nil
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
