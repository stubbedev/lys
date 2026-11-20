package network

import (
	"errors"
	"reflect"
	"testing"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestAPACURLIsAnHTTPHTTPSOrFileAddress(t *testing.T) {
	for _, c := range []struct {
		in, want string
		err      error
	}{
		{"http://wpad.example/proxy.pac", "http://wpad.example/proxy.pac", nil},
		{"  https://proxy.example:8443/auto.pac ", "https://proxy.example:8443/auto.pac", nil},
		{"file:///etc/proxy.pac", "file:///etc/proxy.pac", nil},
		{"", "", nil},
		{"   ", "", nil},
		{"proxy.example/proxy.pac", "", ErrPACURLInvalid},
		{"ftp://proxy.example/proxy.pac", "", ErrPACURLInvalid},
		{"http://proxy example/proxy.pac", "", ErrPACURLInvalid},
		{"http:///proxy.pac", "", ErrPACURLHost},
		{"https://:8080/proxy.pac", "", ErrPACURLHost},
		{"file://host/etc/proxy.pac", "", ErrPACURLPath},
		{"file:proxy.pac", "", ErrPACURLPath},
	} {
		p, err := AutoProxy(c.in)
		if !errors.Is(err, c.err) {
			t.Errorf("AutoProxy(%q) = %v, want %v", c.in, err, c.err)
			continue
		}
		if err == nil && (p.Method() != ProxyAuto || p.PACURL() != c.want) {
			t.Errorf("AutoProxy(%q) = %v %q, want automatic from %q", c.in, p.Method(), p.PACURL(), c.want)
		}
	}
}

func TestAProfileWithoutAProxySettingIsReachedDirectly(t *testing.T) {
	for _, dict := range []ConnectionDict{
		wifiConnection("u", "Home", []byte("home")),
		{proxySection: {}},
		{proxySection: {proxyMethod: dbus.MakeVariant(int32(0)), proxyPACURL: dbus.MakeVariant("")}},
	} {
		p, err := proxyOf(dict)
		if err != nil || p != NoProxy() {
			t.Errorf("proxyOf(%v) = %+v, %v; want no proxy", dict, p, err)
		}
	}
}

func TestAProxyMethodNetworkManagerDoesNotDefineIsRefused(t *testing.T) {
	for _, v := range []dbus.Variant{dbus.MakeVariant(int32(7)), dbus.MakeVariant("auto")} {
		if p, err := proxyOf(ConnectionDict{proxySection: {proxyMethod: v}}); err == nil {
			t.Errorf("proxy.method %v read as %+v", v, p)
		}
	}
}

func TestNoProxyCarriesOnlyTheMethodAndAutomaticKeepsTheOtherKeys(t *testing.T) {
	dict := wifiConnection("u", "Home", []byte("home"))
	dict[proxySection] = map[string]dbus.Variant{
		proxyMethod:    dbus.MakeVariant(int32(1)),
		proxyPACURL:    dbus.MakeVariant("http://old.example/proxy.pac"),
		proxyScript:    dbus.MakeVariant("function FindProxyForURL(u, h) { return 'DIRECT'; }"),
		"browser-only": dbus.MakeVariant(true),
	}
	none := withProxy(dict, NoProxy())
	if want := (map[string]dbus.Variant{proxyMethod: dbus.MakeVariant(int32(0))}); !reflect.DeepEqual(none[proxySection], want) {
		t.Errorf("no proxy wrote %v, want %v", none[proxySection], want)
	}
	if !reflect.DeepEqual(none["802-11-wireless"], dict["802-11-wireless"]) {
		t.Errorf("setting the proxy changed another setting: %v", none)
	}
	discover, _ := AutoProxy("")
	got := withProxy(dict, discover)[proxySection]
	if _, ok := got[proxyPACURL]; ok || got["browser-only"].Value() != true || got[proxyScript].Value() == nil {
		t.Errorf("discovery wrote %v, want the script and browser-only kept and no URL", got)
	}
	pac, _ := AutoProxy("https://new.example/proxy.pac")
	got = withProxy(dict, pac)[proxySection]
	if got[proxyPACURL].Value() != "https://new.example/proxy.pac" || got[proxyScript].Value() != nil || got[proxyMethod].Value() != int32(1) {
		t.Errorf("a PAC URL wrote %v", got)
	}
	if dict[proxySection][proxyPACURL].Value() != "http://old.example/proxy.pac" {
		t.Error("writing a proxy changed the profile it was read from")
	}
}

func TestTheProxyOfThePrimaryProfileReachesNetworkManager(t *testing.T) {
	f := startFakeNM(t)
	home := wifiConnection("u-home", "Home", []byte("home"))
	home["ipv6"] = map[string]dbus.Variant{
		"method":       dbus.MakeVariant("auto"),
		"addresses":    dbus.MakeVariant([]any{}),
		"routes":       dbus.MakeVariant([]any{}),
		"address-data": dbus.MakeVariant([]map[string]dbus.Variant{}),
	}
	path := f.addProfile(home)
	f.setPrimaryProfile(path)
	svc, err := StartConnections(t.Context(), f.client())
	if err != nil {
		t.Fatal(err)
	}
	prof, ok, err := svc.Settings.PrimaryProfile(t.Context())
	if err != nil || !ok || prof.Path != path || prof.ID != "Home" {
		t.Fatalf("the primary profile is %+v, %v, %v; want Home at %s", prof, ok, err, path)
	}
	pac, err := AutoProxy("http://wpad.home/proxy.pac")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.SetProxy(t.Context(), path, pac); err != nil {
		t.Fatal(err)
	}
	saved, _ := f.profile(path)
	if got := saved[proxySection]; got[proxyMethod].Value() != int32(1) || got[proxyPACURL].Value() != "http://wpad.home/proxy.pac" {
		t.Fatalf("NetworkManager holds the proxy %v", got)
	}
	if string(saved["802-11-wireless"]["ssid"].Value().([]byte)) != "home" || saved["ipv6"]["method"].Value() != "auto" {
		t.Errorf("setting the proxy lost the rest of the profile: %v", saved)
	}
	if _, ok := saved["ipv6"]["address-data"]; !ok {
		t.Errorf("setting the proxy lost ipv6.address-data: %v", saved["ipv6"])
	}
	for _, legacy := range legacyIPKeys {
		if v, ok := saved["ipv6"][legacy]; ok {
			t.Errorf("setting the proxy wrote back the deprecated ipv6.%s %v", legacy, v)
		}
	}
	if got, err := svc.Settings.Proxy(t.Context(), path); err != nil || got != pac {
		t.Errorf("the proxy reads back as %+v, %v; want %+v", got, err, pac)
	}
	if err := svc.Settings.SetProxy(t.Context(), path, NoProxy()); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.Settings.Proxy(t.Context(), path); err != nil || got != NoProxy() {
		t.Errorf("the proxy reads back as %+v, %v; want none", got, err)
	}
}

func TestWithoutAPrimaryConnectionThereIsNoProfileToProxy(t *testing.T) {
	f := startFakeNM(t)
	f.setNoPrimary()
	svc, err := StartConnections(t.Context(), f.client())
	if err != nil {
		t.Fatal(err)
	}
	if prof, ok, err := svc.Settings.PrimaryProfile(t.Context()); ok || err != nil {
		t.Errorf("the primary profile is %+v, %v, %v; want none", prof, ok, err)
	}
	if err := svc.Settings.SetProxy(t.Context(), settingPath(99), NoProxy()); err == nil {
		t.Error("the proxy of a profile NetworkManager does not have was set")
	}
}
