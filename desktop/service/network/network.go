// Package network reads NetworkManager state over the system D-Bus:
// overall state, wifi (enabled flag, active AP ssid and strength), wired
// (activated device), with StateChanged/PropertiesChanged ticks.
package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	// BusName is NetworkManager's well-known name on the system bus.
	BusName     = "org.freedesktop.NetworkManager"
	nmPath      = "/org/freedesktop/NetworkManager"
	nmIface     = "org.freedesktop.NetworkManager"
	properties  = "org.freedesktop.DBus.Properties"
	deviceIface = "org.freedesktop.NetworkManager.Device"
	apIface     = "org.freedesktop.NetworkManager.AccessPoint"
	// wiredIface carries the link speed, the IP config objects the
	// addresses.
	wiredIface     = "org.freedesktop.NetworkManager.Device.Wired"
	ip4ConfigIface = "org.freedesktop.NetworkManager.IP4Config"
	ip6ConfigIface = "org.freedesktop.NetworkManager.IP6Config"
)

// Device types (NM_DEVICE_TYPE_*).
const (
	deviceEthernet = 1
	deviceWifi     = 2
)

// Status is a device's connectivity (NetworkStatus), ordered so the
// better of two is the greater.
type Status int

// Statuses.
const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
)

// statusOf maps a device state to a status: activated is
// connected, prepare through secondaries is connecting, and every
// other state (unmanaged, unavailable, disconnected, deactivating,
// failed) is disconnected.
func statusOf(s DeviceState) Status {
	switch s {
	case DeviceActivated:
		return StatusConnected
	case DevicePrepare, DeviceConfig, DeviceNeedAuth, DeviceIPConfig, DeviceIPCheck, DeviceSecondaries:
		return StatusConnecting
	}
	return StatusDisconnected
}

// ConnectionType is NM's PrimaryConnectionType.
type ConnectionType int

// Connection types; ConnectionOther is any type without its own.
const (
	ConnectionNone ConnectionType = iota
	ConnectionWifi
	ConnectionWired
	ConnectionVPN
	ConnectionWireGuard
	ConnectionBluetooth
	ConnectionOther
)

// connectionTypeOf parses a PrimaryConnectionType; "" is no primary
// connection.
func connectionTypeOf(nm string) ConnectionType {
	switch nm {
	case "":
		return ConnectionNone
	case "802-11-wireless":
		return ConnectionWifi
	case "802-3-ethernet":
		return ConnectionWired
	case "vpn":
		return ConnectionVPN
	case "wireguard":
		return ConnectionWireGuard
	case "bluetooth":
		return ConnectionBluetooth
	}
	return ConnectionOther
}

// Snapshot is one network state read.
type Snapshot struct {
	// WifiEnabled is NM's WirelessEnabled.
	WifiEnabled bool
	// Primary is the type of NM's primary connection.
	Primary ConnectionType
	// HasWifi and HasWired report a wireless and an ethernet device;
	// Wifi and Wired are their connectivity, the best device's when
	// there are several.
	HasWifi, HasWired bool
	Wifi, Wired       Status
	// The active access point's SSID, strength, and frequency (MHz),
	// and the device's first IPv4 address, when wifi is connected.
	WifiSSID      string
	WifiStrength  uint8
	WifiFrequency uint32
	WifiIP4       string
	// The link speed in Mb/s and first IPv4 address, when wired is
	// connected.
	WiredSpeed uint32
	WiredIP4   string
}

// Source is the module's seam.
type Source interface {
	Read(ctx context.Context) (Snapshot, error)
	// Subscribe ticks on NM state changes. The channel closes when ctx
	// completes; stop releases the match and reports what releasing it
	// met.
	Subscribe(ctx context.Context) (<-chan struct{}, func() error, error)
}

// System reads the real NetworkManager.
type System struct {
	conn *dbus.Conn
}

// NewOn reads NetworkManager over conn, which stays the caller's.
func NewOn(conn *dbus.Conn) *System { return &System{conn: conn} }

func (s *System) prop(ctx context.Context, iface, name string, out any) error {
	obj := s.conn.Object(BusName, dbus.ObjectPath(nmPath))
	return obj.CallWithContext(ctx, properties+".Get", 0, iface, name).Store(out)
}

// Read collects the snapshot.
func (s *System) Read(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{}
	// WirelessEnabled is best-effort: a missing property means wifi is
	// absent, not fatal.
	_ = s.prop(ctx, nmIface, "WirelessEnabled", &snap.WifiEnabled)
	var primary string
	if err := s.prop(ctx, nmIface, "PrimaryConnectionType", &primary); err != nil {
		return snap, fmt.Errorf("network: PrimaryConnectionType: %w", err)
	}
	snap.Primary = connectionTypeOf(primary)

	var devices []dbus.ObjectPath
	obj := s.conn.Object(BusName, dbus.ObjectPath(nmPath))
	if err := obj.CallWithContext(ctx, nmIface+".GetDevices", 0).Store(&devices); err != nil {
		return snap, fmt.Errorf("network: GetDevices: %w", err)
	}
	for _, devicePath := range devices {
		if !(nm{s.conn}).managed(ctx, devicePath) {
			continue
		}
		dev := s.conn.Object(BusName, devicePath)
		var deviceType uint32
		if err := dev.CallWithContext(ctx, properties+".Get", 0, deviceIface, "DeviceType").Store(&deviceType); err != nil {
			continue
		}
		var state uint32
		if err := dev.CallWithContext(ctx, properties+".Get", 0, deviceIface, "State").Store(&state); err != nil {
			continue
		}
		status := statusOf(DeviceState(state))
		switch deviceType {
		case deviceEthernet:
			snap.HasWired = true
			if status == StatusConnected && snap.Wired != StatusConnected {
				_ = dev.CallWithContext(ctx, properties+".Get", 0, wiredIface, "Speed").Store(&snap.WiredSpeed)
				snap.WiredIP4 = s.ip4(ctx, dev)
			}
			snap.Wired = max(snap.Wired, status)
		case deviceWifi:
			snap.HasWifi = true
			if status != StatusConnected || snap.Wifi == StatusConnected {
				snap.Wifi = max(snap.Wifi, status)
				continue
			}
			snap.Wifi = StatusConnected
			var ap dbus.ObjectPath
			// ActiveAccessPoint is a Device.Wireless property, not a
			// Device one.
			if err := dev.CallWithContext(ctx, properties+".Get", 0, wirelessIface, "ActiveAccessPoint").Store(&ap); err != nil || ap == "/" {
				continue
			}
			apObj := s.conn.Object(BusName, ap)
			var ssidBytes []byte
			var strength uint8
			if err := apObj.CallWithContext(ctx, properties+".Get", 0, apIface, "Ssid").Store(&ssidBytes); err != nil {
				continue
			}
			_ = apObj.CallWithContext(ctx, properties+".Get", 0, apIface, "Strength").Store(&strength)
			_ = apObj.CallWithContext(ctx, properties+".Get", 0, apIface, "Frequency").Store(&snap.WifiFrequency)
			snap.WifiIP4 = s.ip4(ctx, dev)
			snap.WifiSSID = strings.TrimRight(string(ssidBytes), "\x00")
			snap.WifiStrength = strength
		}
	}
	return snap, nil
}

// Subscribe ticks on NM's StateChanged and on property changes of the
// manager object.
func (s *System) Subscribe(ctx context.Context) (<-chan struct{}, func() error, error) {
	ticks, stop, err := dbusx.Ticks(ctx, s.conn, dbusx.FromOwner(BusName), "network", managerRoutes)
	if err != nil {
		return nil, nil, fmt.Errorf("network: %w", err)
	}
	return ticks, stop, nil
}

// managerStateChange is what the manager's StateChanged carries.
type managerStateChange struct {
	State uint32 `dbus:"state"`
}

var managerStateChanged = dbusx.NewSignal[managerStateChange](nmIface, "StateChanged")

// managerRoutes call changed for the manager's own changes: its state,
// and the properties of its interface.
func managerRoutes(changed func()) []dbusx.Route {
	return []dbusx.Route{
		dbusx.On(managerStateChanged, nmPath, func(managerStateChange, dbus.Sequence) error {
			changed()
			return nil
		}),
		dbusx.On(dbusx.PropertiesChanged, nmPath, func(dbusx.ChangedProperties, dbus.Sequence) error {
			changed()
			return nil
		}).WithArg0(nmIface),
	}
}

// SignalIndex is the wifi strength bucketed over the icon list length. Exported for the module; pure.
func SignalIndex(strength uint8, numIcons int) int {
	if numIcons == 0 {
		return 0
	}
	clamped := min(int(strength), 100)
	bucket := max(100/numIcons, 1)
	return min(clamped/bucket, numIcons-1)
}

// Link is a kind of network device a detail read asks about.
type Link int

// Links.
const (
	LinkWired Link = iota
	LinkWifi
)

var linkDeviceTypes = map[Link]uint32{LinkWired: deviceEthernet, LinkWifi: deviceWifi}

// LinkDetails is what a connected device's link carries beyond its
// status: its hardware address, its IPv4 gateway, the DNS servers it
// was given and its first IPv6 address.
type LinkDetails struct {
	HardwareAddress string
	Gateway         string
	DNS             []string
	IP6             string
}

// Details reads the first connected device of kind; false when none
// of that kind is connected.
func (s *System) Details(ctx context.Context, kind Link) (LinkDetails, bool, error) {
	var devices []dbus.ObjectPath
	if err := s.conn.Object(BusName, dbus.ObjectPath(nmPath)).CallWithContext(ctx, nmIface+".GetDevices", 0).Store(&devices); err != nil {
		return LinkDetails{}, false, fmt.Errorf("network: GetDevices: %w", err)
	}
	n := nm{s.conn}
	for _, path := range devices {
		var deviceType, state uint32
		if n.prop(ctx, path, deviceIface, "DeviceType", &deviceType) != nil || deviceType != linkDeviceTypes[kind] || !n.managed(ctx, path) {
			continue
		}
		if n.prop(ctx, path, deviceIface, "State", &state) != nil || statusOf(DeviceState(state)) != StatusConnected {
			continue
		}
		var d LinkDetails
		if err := n.prop(ctx, path, deviceIface, "HwAddress", &d.HardwareAddress); err != nil {
			return LinkDetails{}, false, fmt.Errorf("network: the hardware address of %s: %w", path, err)
		}
		var ip4, ip6 dbus.ObjectPath
		if n.prop(ctx, path, deviceIface, "Ip4Config", &ip4) == nil && ip4 != rootPath && ip4 != "" {
			_ = n.prop(ctx, ip4, ip4ConfigIface, "Gateway", &d.Gateway)
			var servers []map[string]dbus.Variant
			if n.prop(ctx, ip4, ip4ConfigIface, "NameserverData", &servers) == nil {
				for _, srv := range servers {
					if addr, ok := srv["address"].Value().(string); ok {
						d.DNS = append(d.DNS, addr)
					}
				}
			}
		}
		if n.prop(ctx, path, deviceIface, "Ip6Config", &ip6) == nil && ip6 != rootPath && ip6 != "" {
			if addrs := n.configAddresses(ctx, ip6, ip6ConfigIface); len(addrs) > 0 {
				d.IP6 = addrs[0]
			}
		}
		return d, true, nil
	}
	return LinkDetails{}, false, nil
}

// ip4 is a device's first IPv4 address (Ip4Config's AddressData), ""
// when it has none.
func (s *System) ip4(ctx context.Context, dev dbus.BusObject) string {
	var cfg dbus.ObjectPath
	if err := dev.CallWithContext(ctx, properties+".Get", 0, deviceIface, "Ip4Config").Store(&cfg); err != nil || cfg == "/" || cfg == "" {
		return ""
	}
	for _, addr := range (nm{s.conn}).configAddresses(ctx, cfg, ip4ConfigIface) {
		if addr != "" {
			return addr
		}
	}
	return ""
}
