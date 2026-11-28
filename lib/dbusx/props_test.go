package dbusx_test

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	propsIface = "org.example.Props"
	propsPath  = dbus.ObjectPath("/org/example/Props")
)

func propsObject(t *testing.T) dbus.BusObject {
	t.Helper()
	bus := dbustest.Start(t)
	server := bus.Conn(t)
	unexport, err := dbusx.Export(server, propsPath, dbusx.Interface{Name: propsIface, Properties: dbusx.Getters{
		"Name":  func() any { return "lys" },
		"Count": func() any { return uint32(3) },
		"Pair":  func() any { return propsPair{"a", 2} },
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unexport)
	return bus.Conn(t).Object(server.Names()[0], propsPath)
}

type propsPair struct {
	A string
	B uint32
}

func TestGetReadsAPropertyAsItsType(t *testing.T) {
	obj := propsObject(t)
	if pair, err := dbusx.Get[propsPair](t.Context(), obj, propsIface, "Pair"); err != nil || pair != (propsPair{"a", 2}) {
		t.Fatalf("Pair = %+v, %v", pair, err)
	}
	name, err := dbusx.Get[string](t.Context(), obj, propsIface, "Name")
	if err != nil || name != "lys" {
		t.Fatalf("Name = %q, %v", name, err)
	}
	count, err := dbusx.Get[uint32](t.Context(), obj, propsIface, "Count")
	if err != nil || count != 3 {
		t.Fatalf("Count = %d, %v", count, err)
	}
	all, err := dbusx.GetAll(t.Context(), obj, propsIface)
	if err != nil || len(all) != 3 || all["Name"].Value() != "lys" {
		t.Fatalf("GetAll = %v, %v", all, err)
	}
}

func TestGetNamesThePropertyItCouldNotRead(t *testing.T) {
	obj := propsObject(t)
	for _, tc := range []struct {
		name string
		get  func() error
		want string
	}{
		{"wrong type", func() error { _, err := dbusx.Get[bool](t.Context(), obj, propsIface, "Name"); return err }, propsIface + ".Name"},
		{"a number as a string", func() error { _, err := dbusx.Get[string](t.Context(), obj, propsIface, "Count"); return err }, propsIface + ".Count"},
		{"a number as a wider number", func() error { _, err := dbusx.Get[uint64](t.Context(), obj, propsIface, "Count"); return err }, propsIface + ".Count"},
		{"unknown property", func() error { _, err := dbusx.Get[string](t.Context(), obj, propsIface, "Missing"); return err }, propsIface + ".Missing"},
		{"unknown interface", func() error { _, err := dbusx.GetAll(t.Context(), obj, "org.example.Missing"); return err }, "org.example.Missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.get()
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), string(propsPath)) {
				t.Fatalf("err = %v, want one naming %s at %s", err, tc.want, propsPath)
			}
		})
	}
}

func changedSignal(path dbus.ObjectPath, body ...any) *dbus.Signal {
	return &dbus.Signal{Sender: ":1.1", Path: path, Name: dbusx.PropertiesChanged.Name(), Body: body}
}

func TestChangedPropertyIsTheAnnouncedValue(t *testing.T) {
	changed := map[string]dbus.Variant{"Name": dbus.MakeVariant("new")}
	v, ok, err := dbusx.ChangedProperty[string](changedSignal(propsPath, propsIface, changed, []string{}), propsPath, propsIface, "Name")
	if err != nil || !ok || v != "new" {
		t.Fatalf("ChangedProperty = %v %v %v", v, ok, err)
	}
	for name, sig := range map[string]*dbus.Signal{
		"another property":  changedSignal(propsPath, propsIface, map[string]dbus.Variant{"Count": dbus.MakeVariant(uint32(1))}, []string{}),
		"another interface": changedSignal(propsPath, "org.example.Other", changed, []string{}),
		"another object":    changedSignal("/org/example/Other", propsIface, changed, []string{}),
		"another signal":    {Path: propsPath, Name: propsIface + ".Changed", Body: []any{propsIface, changed, []string{}}},
	} {
		if v, ok, err := dbusx.ChangedProperty[string](sig, propsPath, propsIface, "Name"); ok || err != nil {
			t.Errorf("%s: ChangedProperty = %v %v %v, want nothing", name, v, ok, err)
		}
	}
	if _, ok, err := dbusx.ChangedProperty[string](changedSignal(propsPath, propsIface, "not a map", []string{}), propsPath, propsIface, "Name"); ok || err == nil {
		t.Fatalf("a malformed PropertiesChanged gave %v, %v", ok, err)
	}
	if _, ok, err := dbusx.ChangedProperty[uint32](changedSignal(propsPath, propsIface, changed, []string{}), propsPath, propsIface, "Name"); ok || err == nil || !strings.Contains(err.Error(), propsIface+".Name") {
		t.Fatalf("a string read as uint32 gave %v, %v", ok, err)
	}
	numeric := map[string]dbus.Variant{"Name": dbus.MakeVariant(uint32(2))}
	if s, ok, err := dbusx.ChangedProperty[string](changedSignal(propsPath, propsIface, numeric, []string{}), propsPath, propsIface, "Name"); ok || err == nil {
		t.Fatalf("a uint32 read as a string gave %q, %v, %v", s, ok, err)
	}
	if v, ok, err := dbusx.ChangedProperty[dbus.Variant](changedSignal(propsPath, propsIface, numeric, []string{}), propsPath, propsIface, "Name"); !ok || err != nil || v.Value() != uint32(2) {
		t.Fatalf("a variant read gave %v, %v, %v", v, ok, err)
	}
	type pair struct {
		A string
		B uint32
	}
	structured := map[string]dbus.Variant{"Pair": dbus.MakeVariantWithSignature([]any{"a", uint32(2)}, dbus.ParseSignatureMust("(su)"))}
	if p, ok, err := dbusx.ChangedProperty[pair](changedSignal(propsPath, propsIface, structured, []string{}), propsPath, propsIface, "Pair"); !ok || err != nil || p != (pair{"a", 2}) {
		t.Fatalf("a struct property gave %+v, %v, %v", p, ok, err)
	}
}
