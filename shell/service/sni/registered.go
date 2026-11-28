package sni

import (
	"context"
	"errors"
	"fmt"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func registrations(ctx context.Context, conn *dbus.Conn) ([]string, error) {
	var regs []string
	if err := conn.Object(WatcherName, WatcherPath).CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, WatcherIface, "RegisteredStatusNotifierItems").Store(&regs); err != nil {
		return nil, fmt.Errorf("read the items registered with %s: %w", WatcherName, err)
	}
	return regs, nil
}

// Registered reads, once, the items registered with the running
// watcher, without their pixmaps. An item that does not answer is left
// out and named in the error beside the items that did.
func Registered(ctx context.Context, conn *dbus.Conn) ([]Item, error) {
	regs, err := registrations(ctx, conn)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(regs))
	var errs []error
	for _, reg := range regs {
		bus, path := ParseAddress(reg)
		it, _, err := readItem(ctx, conn, bus, path, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("read the tray item %s: %w", reg, err))
			continue
		}
		items = append(items, *it)
	}
	return items, errors.Join(errs...)
}
