package dbusx

import (
	"context"
	"fmt"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// Get reads the property prop of iface on obj as a T, converting the
// variant the way dbus.Store does.
func Get[T any](ctx context.Context, obj dbus.BusObject, iface, prop string) (T, error) {
	var v T
	if err := obj.CallWithContext(ctx, PropertiesInterface+".Get", 0, iface, prop).Store(&v); err != nil {
		return v, fmt.Errorf("read %s.%s of %s at %s: %w", iface, prop, obj.Destination(), obj.Path(), err)
	}
	return v, nil
}

// GetAll reads every property of iface on obj.
func GetAll(ctx context.Context, obj dbus.BusObject, iface string) (map[string]dbus.Variant, error) {
	var props map[string]dbus.Variant
	if err := obj.CallWithContext(ctx, PropertiesInterface+".GetAll", 0, iface).Store(&props); err != nil {
		return nil, fmt.Errorf("read the properties of %s of %s at %s: %w", iface, obj.Destination(), obj.Path(), err)
	}
	return props, nil
}

// ChangedProperty is the new value of the property name of iface that
// sig announces, as a T converted the way dbus.Store does, when sig is a
// PropertiesChanged from the object at path carrying it; false for any
// other signal. A PropertiesChanged that does not decode, or whose value
// is not a T, is an error.
func ChangedProperty[T any](sig *dbus.Signal, path dbus.ObjectPath, iface, name string) (T, bool, error) {
	var v T
	if sig.Path != path || !PropertiesChanged.named(sig.Name) {
		return v, false, nil
	}
	changed, err := PropertiesChanged.Decode(sig)
	if err != nil {
		return v, false, fmt.Errorf("%s from %s at %s: %w", sig.Name, sig.Sender, path, err)
	}
	if changed.Interface != iface {
		return v, false, nil
	}
	value, ok := changed.Changed[name]
	if !ok {
		return v, false, nil
	}
	if err := dbus.Store([]any{value}, &v); err != nil {
		return v, false, fmt.Errorf("%s from %s at %s: %s.%s of signature %s: %w", sig.Name, sig.Sender, path, iface, name, value.Signature(), err)
	}
	return v, true, nil
}
