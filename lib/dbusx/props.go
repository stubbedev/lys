package dbusx

import (
	"context"
	"fmt"
	"reflect"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// Get reads the property prop of iface on obj as a T. A value whose
// D-Bus signature is not T's is an error: it is never converted.
func Get[T any](ctx context.Context, obj dbus.BusObject, iface, prop string) (T, error) {
	var raw dbus.Variant
	if err := obj.CallWithContext(ctx, PropertiesInterface+".Get", 0, iface, prop).Store(&raw); err != nil {
		var zero T
		return zero, fmt.Errorf("read %s.%s of %s at %s: %w", iface, prop, obj.Destination(), obj.Path(), err)
	}
	v, err := variantAs[T](raw)
	if err != nil {
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
// sig announces, as a T, when sig is a PropertiesChanged from the object
// at path carrying it; false for any other signal. A PropertiesChanged
// that does not decode, or whose value's D-Bus signature is not T's, is
// an error.
func ChangedProperty[T any](sig *dbus.Signal, path dbus.ObjectPath, iface, name string) (T, bool, error) {
	var zero T
	if sig.Path != path || !PropertiesChanged.named(sig.Name) {
		return zero, false, nil
	}
	changed, err := PropertiesChanged.Decode(sig)
	if err != nil {
		return zero, false, fmt.Errorf("%s from %s at %s: %w", sig.Name, sig.Sender, path, err)
	}
	if changed.Interface != iface {
		return zero, false, nil
	}
	value, ok := changed.Changed[name]
	if !ok {
		return zero, false, nil
	}
	if wire, ok := sig.Body[1].(map[string]dbus.Variant); ok {
		value = wire[name]
	}
	v, err := variantAs[T](value)
	if err != nil {
		return zero, false, fmt.Errorf("%s from %s at %s: %s.%s: %w", sig.Name, sig.Sender, path, iface, name, err)
	}
	return v, true, nil
}

func variantAs[T any](value dbus.Variant) (T, error) {
	var v T
	want := reflect.TypeFor[T]()
	if want == reflect.TypeFor[dbus.Variant]() {
		return any(value).(T), nil
	}
	if got, sig := value.Signature(), dbus.SignatureOfType(want); got != sig {
		return v, fmt.Errorf("the value is a %T of signature %s, not a %s of signature %s", value.Value(), got, want, sig)
	}
	if err := dbus.Store([]any{value.Value()}, &v); err != nil {
		return v, fmt.Errorf("decode the %s value: %w", value.Signature(), err)
	}
	return v, nil
}
