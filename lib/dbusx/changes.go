package dbusx

import (
	"context"
	"fmt"
	"sync"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// settleTimeout bounds the read a connection sends itself before it
// announces a change.
const settleTimeout = 5 * time.Second

// PropertyChanges announces changes to the properties exported on one
// connection the way a single-threaded service such as systemd-logind
// orders them: a reply carrying a value from before a change never
// reaches a peer after the PropertiesChanged announcing it.
//
// godbus runs each read on a goroutine of its own and sends its reply
// once the getter has returned, outside any lock the getter took, so a
// lock alone cannot keep an older reply from overtaking a newer signal.
// PropertyChanges has the connection answer reads one at a time in
// arrival order, each reply sent before the next read runs, and
// announces a change only once a read the connection sends itself after
// applying it has been answered: every read that saw the older value
// arrived before that one and has had its reply sent. Replies and
// signals leave one connection in the order they are sent, and the bus
// delivers them in that order.
type PropertyChanges struct {
	conn *dbus.Conn
	mu   sync.Mutex
}

// OrderPropertyChanges has conn answer property reads (Get, GetAll) one
// at a time in arrival order and returns what announces changes to the
// properties exported on it. Call it before anything is exported on
// conn, so that no read is answered out of order; a getter must then
// not wait on another read of conn.
func OrderPropertyChanges(conn *dbus.Conn) *PropertyChanges {
	conn.DispatchInOrder(PropertiesInterface, "Get", "GetAll")
	return &PropertyChanges{conn: conn}
}

// Announce applies change, which returns the properties of iface at path
// it changed with their new values, and sends their PropertiesChanged
// after the reply to every read of the older values. Announcements run
// one at a time, so their signals go out in the order their changes
// were applied. Announce must not run in a getter of conn.
func (c *PropertyChanges) Announce(path dbus.ObjectPath, iface string, change func() map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := change()
	if err := c.settle(path, iface); err != nil {
		return err
	}
	variants := make(map[string]dbus.Variant, len(changed))
	for name, v := range changed {
		variants[name] = dbus.MakeVariant(v)
	}
	return PropertiesChanged.Emit(c.conn, path, ChangedProperties{Interface: iface, Changed: variants, Invalidated: []string{}})
}

// settle reads iface at path from the connection itself, over the bus,
// returning once the reads that arrived before have been answered.
func (c *PropertyChanges) settle(path dbus.ObjectPath, iface string) error {
	ctx, cancel := context.WithTimeout(context.Background(), settleTimeout)
	defer cancel()
	if err := c.conn.Object(c.conn.Names()[0], path).CallWithContext(ctx, PropertiesInterface+".GetAll", 0, iface).Err; err != nil {
		return fmt.Errorf("read %s at %s back before announcing its change: %w", iface, path, err)
	}
	return nil
}
