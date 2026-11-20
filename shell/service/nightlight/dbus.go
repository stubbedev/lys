package nightlight

import (
	"log"
	"sync"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// ServiceName, ServicePath and EnabledProperty are dev.stubbe.lys.Nightlight1,
// the shell's night light as other processes see and switch it.
const (
	ServiceName     = "dev.stubbe.lys.Nightlight1"
	ServicePath     = dbus.ObjectPath("/dev/stubbe/lys/Nightlight")
	EnabledProperty = "Enabled"
	SetEnabled      = ServiceName + ".SetEnabled"
)

// Daemon is the dev.stubbe.lys.Nightlight1 object: Enabled says whether
// the filter is on, announced in a PropertiesChanged on every change, and
// SetEnabled switches it as the bar's toggle does.
type Daemon struct {
	mu       sync.Mutex
	on       bool
	conn     *dbus.Conn
	switchTo func(on bool)
}

// NewDaemon is the object over switchTo, which turns the filter on or
// off on the shell's loop.
func NewDaemon(switchTo func(on bool)) *Daemon { return &Daemon{switchTo: switchTo} }

// SetEnabled turns the night light on or off.
func (d *Daemon) SetEnabled(on bool) *dbus.Error {
	d.switchTo(on)
	return nil
}

// Show records whether the filter is on and announces a change.
func (d *Daemon) Show(on bool) {
	d.mu.Lock()
	changed := d.on != on
	d.on = on
	conn := d.conn
	d.mu.Unlock()
	if !changed || conn == nil {
		return
	}
	announce := dbusx.ChangedProperties{Interface: ServiceName, Changed: map[string]dbus.Variant{EnabledProperty: dbus.MakeVariant(on)}, Invalidated: []string{}}
	if err := dbusx.PropertiesChanged.Emit(conn, ServicePath, announce); err != nil {
		log.Printf("nightlight: announce the switch: %v", err)
	}
}

func (d *Daemon) enabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.on
}

// Export serves the daemon on conn under ServiceName.
func (d *Daemon) Export(conn *dbus.Conn) (func(), error) {
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       ServiceName,
		Path:       ServicePath,
		Interface:  ServiceName,
		Methods:    d,
		Properties: dbusx.Getters{EnabledProperty: func() any { return d.enabled() }},
	})
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.conn = conn
	d.mu.Unlock()
	return func() {
		d.mu.Lock()
		d.conn = nil
		d.mu.Unlock()
		release()
	}, nil
}
