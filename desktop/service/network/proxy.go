package network

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"strings"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// ProxyMethod is NetworkManager's proxy.method.
type ProxyMethod int32

// Proxy methods NetworkManager knows.
const (
	ProxyNone ProxyMethod = 0
	ProxyAuto ProxyMethod = 1
)

// ProxyMethods are the methods in the order the page offers them.
var ProxyMethods = []ProxyMethod{ProxyNone, ProxyAuto}

const (
	proxySection = "proxy"
	proxyMethod  = "method"
	proxyPACURL  = "pac-url"
	proxyScript  = "pac-script"
)

// The defined refusals of a PAC URL.
var (
	ErrPACURLInvalid = errors.New("a PAC URL is an http://, https:// or file:// address")
	ErrPACURLHost    = errors.New("an http:// or https:// PAC URL names a host")
	ErrPACURLPath    = errors.New("a file:// PAC URL names an absolute path")
)

// Proxy is a connection's proxy as NetworkManager keeps it: none, or
// automatic from a PAC URL, or from the network's own discovery (WPAD)
// when the URL is empty. Only NoProxy and AutoProxy make one, so a
// Proxy always holds what NetworkManager accepts.
type Proxy struct {
	method ProxyMethod
	pac    string
}

// NoProxy is a connection reached directly.
func NoProxy() Proxy { return Proxy{method: ProxyNone} }

// AutoProxy is automatic configuration from pac, or by discovery when
// pac is empty; surrounding spaces are not part of the URL.
func AutoProxy(pac string) (Proxy, error) {
	pac = strings.TrimSpace(pac)
	if pac == "" {
		return Proxy{method: ProxyAuto}, nil
	}
	if err := checkPACURL(pac); err != nil {
		return Proxy{}, fmt.Errorf("the PAC URL %q: %w", pac, err)
	}
	return Proxy{method: ProxyAuto, pac: pac}, nil
}

func checkPACURL(pac string) error {
	u, err := url.Parse(pac)
	if err != nil || strings.ContainsAny(pac, " \t\r\n") {
		return ErrPACURLInvalid
	}
	switch u.Scheme {
	case "http", "https":
		if u.Hostname() == "" {
			return ErrPACURLHost
		}
	case "file":
		if u.Host != "" || !strings.HasPrefix(u.Path, "/") {
			return ErrPACURLPath
		}
	default:
		return ErrPACURLInvalid
	}
	return nil
}

// Method is how the connection finds its proxy.
func (p Proxy) Method() ProxyMethod { return p.method }

// PACURL is where automatic configuration reads its script, "" for
// discovery or no proxy.
func (p Proxy) PACURL() string { return p.pac }

func proxyOf(dict ConnectionDict) (Proxy, error) {
	section, ok := dict[proxySection]
	if !ok {
		return NoProxy(), nil
	}
	v, ok := section[proxyMethod]
	if !ok {
		return NoProxy(), nil
	}
	method, ok := v.Value().(int32)
	if !ok {
		return Proxy{}, fmt.Errorf("proxy.method is a %s, not an int32", v.Signature())
	}
	switch ProxyMethod(method) {
	case ProxyNone:
		return NoProxy(), nil
	case ProxyAuto:
		return Proxy{method: ProxyAuto, pac: variantString(section[proxyPACURL])}, nil
	}
	return Proxy{}, fmt.Errorf("proxy.method %d is not one NetworkManager defines", method)
}

func withProxy(dict ConnectionDict, p Proxy) ConnectionDict {
	out := maps.Clone(dict)
	section := map[string]dbus.Variant{}
	if p.method == ProxyAuto {
		section = maps.Clone(dict[proxySection])
		if section == nil {
			section = map[string]dbus.Variant{}
		}
		delete(section, proxyPACURL)
		if p.pac != "" {
			delete(section, proxyScript)
			section[proxyPACURL] = dbus.MakeVariant(p.pac)
		}
	}
	section[proxyMethod] = dbus.MakeVariant(int32(p.method))
	out[proxySection] = section
	return out
}

var legacyIPKeys = []string{"addresses", "routes"}

func withoutLegacyIP(dict ConnectionDict) ConnectionDict {
	out := maps.Clone(dict)
	for _, family := range []string{"ipv4", "ipv6"} {
		section, ok := dict[family]
		if !ok {
			continue
		}
		section = maps.Clone(section)
		for _, key := range legacyIPKeys {
			delete(section, key)
		}
		out[family] = section
	}
	return out
}

// PrimaryProfile is the saved profile of the connection carrying the
// traffic; false when nothing does.
func (s *Settings) PrimaryProfile(ctx context.Context) (Profile, bool, error) {
	var active dbus.ObjectPath
	if err := s.nm.prop(ctx, nmPath, nmIface, "PrimaryConnection", &active); err != nil {
		return Profile{}, false, fmt.Errorf("read the primary connection: %w", err)
	}
	if active == rootPath {
		return Profile{}, false, nil
	}
	var path dbus.ObjectPath
	if err := s.nm.prop(ctx, active, activeIface, "Connection", &path); err != nil {
		return Profile{}, false, fmt.Errorf("read the profile of the primary connection %s: %w", active, err)
	}
	p, err := s.read(ctx, path)
	if err != nil {
		return Profile{}, false, fmt.Errorf("read the primary profile %s: %w", path, err)
	}
	return p, true, nil
}

// Proxy reads the proxy of the saved profile at path.
func (s *Settings) Proxy(ctx context.Context, path dbus.ObjectPath) (Proxy, error) {
	dict, err := s.Get(ctx, path)
	if err != nil {
		return Proxy{}, fmt.Errorf("read the proxy of %s: %w", path, err)
	}
	p, err := proxyOf(dict)
	if err != nil {
		return Proxy{}, fmt.Errorf("read the proxy of %s: %w", path, err)
	}
	return p, nil
}

// SetProxy makes p the proxy of the saved profile at path and leaves
// the rest of it as it was. The profile is rewritten without its
// secrets, which NetworkManager then keeps as they were, and without
// the deprecated ipv4/ipv6 addresses and routes: address-data and
// route-data carry the same, and the old struct-typed forms do not
// survive a read and a write over the bus.
func (s *Settings) SetProxy(ctx context.Context, path dbus.ObjectPath, p Proxy) error {
	dict, err := s.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("set the proxy of %s: %w", path, err)
	}
	if err := s.Update(ctx, path, withProxy(withoutLegacyIP(dict), p)); err != nil {
		return fmt.Errorf("set the proxy of %s: %w", path, err)
	}
	return nil
}
