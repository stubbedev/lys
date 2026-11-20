package network

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// This file is the wifi half of the service: the device's access
// points, scanning, and connecting.

// DeviceState is NMDeviceState.
type DeviceState uint32

// Device states.
const (
	DeviceUnknown      DeviceState = 0
	DeviceUnmanaged    DeviceState = 10
	DeviceUnavailable  DeviceState = 20
	DeviceDisconnected DeviceState = 30
	DevicePrepare      DeviceState = 40
	DeviceConfig       DeviceState = 50
	DeviceNeedAuth     DeviceState = 60
	DeviceIPConfig     DeviceState = 70
	DeviceIPCheck      DeviceState = 80
	DeviceSecondaries  DeviceState = 90
	DeviceActivated    DeviceState = 100
	DeviceDeactivating DeviceState = 110
	DeviceFailed       DeviceState = 120
)

// DeviceReason is NMDeviceStateReason (the values the connect watch
// reads).
type DeviceReason uint32

// Device state reasons.
const (
	DeviceReasonNone                   DeviceReason = 0
	DeviceReasonUnknown                DeviceReason = 1
	DeviceReasonIPConfigUnavailable    DeviceReason = 5
	DeviceReasonIPConfigExpired        DeviceReason = 6
	DeviceReasonNoSecrets              DeviceReason = 7
	DeviceReasonSupplicantDisconnect   DeviceReason = 8
	DeviceReasonSupplicantConfigFailed DeviceReason = 9
	DeviceReasonSupplicantFailed       DeviceReason = 10
	DeviceReasonSupplicantTimeout      DeviceReason = 11
	DeviceReasonUserRequested          DeviceReason = 39
	DeviceReasonSSIDNotFound           DeviceReason = 53
	DeviceReasonNewActivation          DeviceReason = 60
)

// SecurityType is an access point's security classification.
type SecurityType int

// Security types.
const (
	SecurityNone SecurityType = iota
	SecurityWEP
	SecurityWPA
	SecurityWPA2
	SecurityWPA3
	SecurityEnterprise
)

// String is the English name.
func (s SecurityType) String() string {
	switch s {
	case SecurityWEP:
		return "WEP"
	case SecurityWPA:
		return "WPA"
	case SecurityWPA2:
		return "WPA2"
	case SecurityWPA3:
		return "WPA3"
	case SecurityEnterprise:
		return "Enterprise"
	}
	return "Open"
}

// RequiresPassword reports whether connecting asks for a password
// first; an enterprise network needs a whole sign-in (EAPSettings)
// instead.
func (s SecurityType) RequiresPassword() bool {
	return s != SecurityNone && s != SecurityEnterprise
}

// keyMgmt is the 802-11-wireless-security key-mgmt for a security type.
func (s SecurityType) keyMgmt() string {
	switch s {
	case SecurityWPA, SecurityWPA2:
		return "wpa-psk"
	case SecurityWPA3:
		return "sae"
	case SecurityEnterprise:
		return "wpa-eap"
	}
	return "none"
}

// secretKey is where the password goes: wep-key0 for WEP, psk otherwise.
func (s SecurityType) secretKey() string {
	if s == SecurityWEP {
		return "wep-key0"
	}
	return "psk"
}

// NM80211ApFlags / NM80211ApSecurityFlags bits the classification reads.
const (
	apFlagPrivacy       = 0x1
	secPairWEP40        = 0x1
	secPairWEP104       = 0x2
	secGroupWEP40       = 0x10
	secGroupWEP104      = 0x20
	secKeyMgmtPSK       = 0x100
	secKeyMgmt8021X     = 0x200
	secKeyMgmtSAE       = 0x400
	secKeyMgmtOWE       = 0x800
	secKeyMgmtOWETM     = 0x1000
	secKeyMgmtEAPSuiteB = 0x2000
)

// securityFromFlags derives the strongest security an AP supports.
func securityFromFlags(flags, wpa, rsn uint32) SecurityType {
	const (
		enterprise = secKeyMgmt8021X | secKeyMgmtEAPSuiteB
		wpa3       = secKeyMgmtSAE | secKeyMgmtOWE | secKeyMgmtOWETM
		wep        = secPairWEP40 | secPairWEP104 | secGroupWEP40 | secGroupWEP104
	)
	switch {
	case rsn&enterprise != 0 || wpa&enterprise != 0:
		return SecurityEnterprise
	case rsn&wpa3 != 0:
		return SecurityWPA3
	case rsn&secKeyMgmtPSK != 0:
		return SecurityWPA2
	case wpa&secKeyMgmtPSK != 0:
		return SecurityWPA
	case wpa&wep != 0 || rsn&wep != 0:
		return SecurityWEP
	case flags&apFlagPrivacy != 0 && wpa == 0 && rsn == 0:
		return SecurityWEP
	}
	return SecurityNone
}

// manufacturerDefaultSSIDs are router default names locked to the BSSID
// on a new profile, so a neighbour's router of the same make is not
// joined by name (nm-applet's approach).
var manufacturerDefaultSSIDs = []string{
	"linksys", "linksys-a", "linksys-g", "default", "belkin54g",
	"NETGEAR", "o2DSL", "WLAN", "ALICE-WLAN",
}

// AccessPoint is one visible network.
type AccessPoint struct {
	// Path is the AccessPoint object, which a connect names.
	Path dbus.ObjectPath
	// SSID is the raw SSID; empty for a hidden network.
	SSID []byte
	// Strength is 0-100.
	Strength uint8
	// Security is the classification from the AP's flags.
	Security SecurityType
	// Frequency is in MHz.
	Frequency uint32
	// BSSID is the hardware address.
	BSSID string
}

// Name is the SSID as text, invalid UTF-8 replaced.
func (ap AccessPoint) Name() string { return strings.ToValidUTF8(string(ap.SSID), "�") }

// Wifi is the wireless device and its controls.
type Wifi struct {
	nm       nm
	settings *Settings
	// Device is the wireless device's object path.
	Device dbus.ObjectPath
}

// findWifiDevice is the first wireless device NM manages.
func findWifiDevice(ctx context.Context, n nm) (dbus.ObjectPath, bool) {
	var devices []dbus.ObjectPath
	if err := n.manager().CallWithContext(ctx, nmIface+".GetDevices", 0).Store(&devices); err != nil {
		return "", false
	}
	for _, path := range devices {
		var deviceType uint32
		if err := n.prop(ctx, path, deviceIface, "DeviceType", &deviceType); err == nil && deviceType == deviceWifi && n.managed(ctx, path) {
			return path, true
		}
	}
	return "", false
}

// Enabled is NM's WirelessEnabled.
func (w *Wifi) Enabled(ctx context.Context) (bool, error) {
	var on bool
	err := w.nm.prop(ctx, nmPath, nmIface, "WirelessEnabled", &on)
	return on, err
}

// SetEnabled turns the radio on or off system-wide; off terminates
// every wifi connection.
func (w *Wifi) SetEnabled(ctx context.Context, on bool) error {
	err := w.nm.manager().CallWithContext(ctx, properties+".Set", 0, nmIface, "WirelessEnabled", dbus.MakeVariant(on)).Err
	if err != nil {
		return fmt.Errorf("cannot set wireless enabled: %w", err)
	}
	return nil
}

// Scan asks NM for a fresh scan. A refusal is usually NM saying one is
// already running, which still ends with a LastScan bump.
func (w *Wifi) Scan(ctx context.Context) error {
	err := w.nm.object(w.Device).CallWithContext(ctx, wirelessIface+".RequestScan", 0, map[string]dbus.Variant{}).Err
	if err != nil {
		return fmt.Errorf("cannot request wifi scan: %w", err)
	}
	return nil
}

// LastScan is the device's LastScan stamp; NM bumps it when a scan
// finishes, the only real "done" signal.
func (w *Wifi) LastScan(ctx context.Context) int64 {
	var at int64
	_ = w.nm.prop(ctx, w.Device, wirelessIface, "LastScan", &at)
	return at
}

// ScanAndWait scans and returns once LastScan moves past its value
// before the request, or after timeout.
func (w *Wifi) ScanAndWait(ctx context.Context, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	before := w.LastScan(ctx)
	_ = w.Scan(ctx)
	for w.LastScan(ctx) == before {
		if !sleepCtx(ctx, 200*time.Millisecond) {
			return
		}
	}
}

// AccessPoints reads every visible access point.
func (w *Wifi) AccessPoints(ctx context.Context) ([]AccessPoint, error) {
	var paths []dbus.ObjectPath
	if err := w.nm.object(w.Device).CallWithContext(ctx, wirelessIface+".GetAllAccessPoints", 0).Store(&paths); err != nil {
		return nil, fmt.Errorf("cannot get all access points: %w", err)
	}
	aps := make([]AccessPoint, 0, len(paths))
	for _, path := range paths {
		if ap, err := w.accessPoint(ctx, path); err == nil {
			aps = append(aps, ap)
		}
	}
	return aps, nil
}

func (w *Wifi) accessPoint(ctx context.Context, path dbus.ObjectPath) (AccessPoint, error) {
	var props map[string]dbus.Variant
	if err := w.nm.object(path).CallWithContext(ctx, properties+".GetAll", 0, apIface).Store(&props); err != nil {
		return AccessPoint{}, err
	}
	u32 := func(key string) uint32 { v, _ := props[key].Value().(uint32); return v }
	ssid, _ := props["Ssid"].Value().([]byte)
	strength, _ := props["Strength"].Value().(byte)
	bssid, _ := props["HwAddress"].Value().(string)
	return AccessPoint{
		Path:      path,
		SSID:      ssid,
		Strength:  strength,
		Security:  securityFromFlags(u32("Flags"), u32("WpaFlags"), u32("RsnFlags")),
		Frequency: u32("Frequency"),
		BSSID:     bssid,
	}, nil
}

// ActiveSSID is the SSID of the access point the device is on, "" when
// it is on none.
func (w *Wifi) ActiveSSID(ctx context.Context) string {
	var ap dbus.ObjectPath
	if err := w.nm.prop(ctx, w.Device, wirelessIface, "ActiveAccessPoint", &ap); err != nil || ap == rootPath || ap == "" {
		return ""
	}
	point, err := w.accessPoint(ctx, ap)
	if err != nil {
		return ""
	}
	return point.Name()
}

// KnownSSIDs are the SSIDs a saved profile exists for.
func (w *Wifi) KnownSSIDs() map[string]bool {
	known := map[string]bool{}
	for _, p := range w.settings.Profiles() {
		if p.SSID != nil {
			known[strings.ToValidUTF8(string(p.SSID), "�")] = true
		}
	}
	return known
}

// Forget deletes every saved profile of a network.
func (w *Wifi) Forget(ctx context.Context, ssid string) error {
	return w.settings.DeleteForSSID(ctx, []byte(ssid))
}

// Connect joins an access point. A saved profile for its SSID is reused
// (its password updated when one is given); otherwise a new profile is
// made, locked to the BSSID for a manufacturer-default SSID. An
// enterprise network takes no password here: it connects by its saved
// profile, or through ConnectEnterprise. It returns the active
// connection.
func (w *Wifi) Connect(ctx context.Context, apPath dbus.ObjectPath, password string) (dbus.ObjectPath, error) {
	ap, err := w.accessPoint(ctx, apPath)
	if err != nil {
		return "", fmt.Errorf("cannot get ssid: %w", err)
	}
	if ap.Security == SecurityEnterprise && password != "" {
		return "", fmt.Errorf("%w: %q", ErrEnterprisePassword, ap.Name())
	}
	return w.join(ctx, ap.SSID, apPath, newWifiProfile(ap), passwordSecurity(ap.Security, password))
}

// MaxSSIDBytes is the longest SSID 802.11 carries.
const MaxSSIDBytes = 32

// Hidden network refusals.
var (
	ErrHiddenNameEmpty      = errors.New("a hidden network needs a name")
	ErrHiddenNameTooLong    = errors.New("a network name is at most 32 bytes")
	ErrHiddenEnterprise     = errors.New("an unsaved hidden enterprise network signs in with EAP settings")
	ErrHiddenPasswordNeeded = errors.New("a secured hidden network needs a password")
)

// HiddenNetwork is a network that does not broadcast its name, as the
// user named it: 1 to MaxSSIDBytes bytes, secured by nothing, by a
// password, or by an enterprise sign-in.
type HiddenNetwork struct {
	ssid     []byte
	security SecurityType
}

// NewHiddenNetwork validates a hidden network the user named.
func NewHiddenNetwork(name string, security SecurityType) (HiddenNetwork, error) {
	switch {
	case name == "":
		return HiddenNetwork{}, ErrHiddenNameEmpty
	case len(name) > MaxSSIDBytes:
		return HiddenNetwork{}, fmt.Errorf("%w: %q is %d", ErrHiddenNameTooLong, name, len(name))
	}
	return HiddenNetwork{ssid: []byte(name), security: security}, nil
}

// Name is the network's name.
func (h HiddenNetwork) Name() string { return string(h.ssid) }

// Security is how the network is secured.
func (h HiddenNetwork) Security() SecurityType { return h.security }

// JoinHidden joins a network that does not broadcast its name: a saved
// profile for it is reused (its password updated when one is given);
// otherwise a new hidden profile is made, which a password-secured
// network needs a password for. An enterprise network takes no
// password here: it joins by its saved profile, or through
// JoinHiddenEnterprise. It returns the active connection.
func (w *Wifi) JoinHidden(ctx context.Context, net HiddenNetwork, password string) (dbus.ObjectPath, error) {
	saved := len(w.settings.ForSSID(net.ssid)) > 0
	switch {
	case net.security == SecurityEnterprise && password != "":
		return "", fmt.Errorf("%w: %q", ErrEnterprisePassword, net.Name())
	case net.security == SecurityEnterprise && !saved:
		return "", fmt.Errorf("%w: %q", ErrHiddenEnterprise, net.Name())
	case net.security.RequiresPassword() && password == "" && !saved:
		return "", fmt.Errorf("%w: %q", ErrHiddenPasswordNeeded, net.Name())
	}
	return w.join(ctx, net.ssid, rootPath, hiddenWifiProfile(net), passwordSecurity(net.security, password))
}

// join activates the saved profile for ssid, secured anew by secure
// when that is not nil, or adds profile secured by secure and
// activates it.
func (w *Wifi) join(ctx context.Context, ssid []byte, specific dbus.ObjectPath, profile ConnectionDict, secure func(ConnectionDict)) (dbus.ObjectPath, error) {
	if saved := w.settings.ForSSID(ssid); len(saved) > 0 {
		path := saved[0].Path
		if secure != nil {
			if err := w.resecure(ctx, path, secure); err != nil {
				return "", err
			}
		}
		active, err := w.nm.activate(ctx, path, w.Device, specific)
		if err != nil {
			return "", fmt.Errorf("cannot activate existing connection: %w", err)
		}
		return active, nil
	}
	if secure != nil {
		secure(profile)
	}
	var settingsPath, active dbus.ObjectPath
	if err := w.nm.manager().CallWithContext(ctx, nmIface+".AddAndActivateConnection", 0, profile, w.Device, specific).Store(&settingsPath, &active); err != nil {
		return "", fmt.Errorf("cannot add and activate connection: %w", err)
	}
	return active, nil
}

// newWifiProfile is a new profile for an access point.
func newWifiProfile(ap AccessPoint) ConnectionDict {
	wireless := map[string]dbus.Variant{"ssid": dbus.MakeVariant(ap.SSID)}
	if slices.Contains(manufacturerDefaultSSIDs, ap.Name()) {
		if mac, ok := parseMAC(ap.BSSID); ok {
			wireless["bssid"] = dbus.MakeVariant(mac)
		}
	}
	return wifiProfile(ap.Name(), wireless)
}

func hiddenWifiProfile(net HiddenNetwork) ConnectionDict {
	wireless := map[string]dbus.Variant{"ssid": dbus.MakeVariant(net.ssid), "hidden": dbus.MakeVariant(true)}
	return wifiProfile(net.Name(), wireless)
}

func wifiProfile(name string, wireless map[string]dbus.Variant) ConnectionDict {
	return ConnectionDict{
		"connection": {
			"type": dbus.MakeVariant("802-11-wireless"),
			"id":   dbus.MakeVariant(name),
		},
		"802-11-wireless": wireless,
	}
}

// passwordSecurity secures a profile with a password, nil when there
// is none to set.
func passwordSecurity(security SecurityType, password string) func(ConnectionDict) {
	if password == "" {
		return nil
	}
	return func(dict ConnectionDict) {
		section := securitySection(dict)
		section["key-mgmt"] = dbus.MakeVariant(security.keyMgmt())
		section[security.secretKey()] = dbus.MakeVariant(password)
	}
}

// securitySection is the profile's 802-11-wireless-security setting,
// added when it has none.
func securitySection(dict ConnectionDict) map[string]dbus.Variant {
	section := dict["802-11-wireless-security"]
	if section == nil {
		section = map[string]dbus.Variant{}
		dict["802-11-wireless-security"] = section
	}
	return section
}

func parseMAC(s string) ([]byte, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return nil, false
	}
	mac := make([]byte, 0, 6)
	for _, part := range parts {
		b, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return nil, false
		}
		mac = append(mac, byte(b))
	}
	return mac, true
}

func (w *Wifi) resecure(ctx context.Context, profile dbus.ObjectPath, secure func(ConnectionDict)) error {
	dict, err := w.settings.Get(ctx, profile)
	if err != nil {
		return err
	}
	secure(dict)
	return w.settings.Update(ctx, profile, dict)
}

// Disconnect deactivates the device's connection; none is a no-op.
func (w *Wifi) Disconnect(ctx context.Context) error {
	var active dbus.ObjectPath
	if err := w.nm.prop(ctx, w.Device, deviceIface, "ActiveConnection", &active); err != nil {
		return fmt.Errorf("cannot get active connection: %w", err)
	}
	if active == rootPath || active == "" {
		return nil
	}
	if err := w.nm.deactivate(ctx, active); err != nil {
		return fmt.Errorf("cannot deactivate connection: %w", err)
	}
	return nil
}

// ConnectFailure is why a wifi connect did not come up, in the
// dropdown's vocabulary.
type ConnectFailure int

// Connect failures.
const (
	// FailureGeneric is "Connection failed".
	FailureGeneric ConnectFailure = iota
	// FailureAuth is a refused password.
	FailureAuth
	// FailureTimeout is the attempt timing out.
	FailureTimeout
	// FailureNotFound is the network gone.
	FailureNotFound
	// FailureIPConfig is no IP address.
	FailureIPConfig
	// FailureSuperseded is the attempt replaced by another (a user
	// request or a new activation): not an error to show.
	FailureSuperseded
)

// ConnectError is a wifi connect that ended without the device coming
// up.
type ConnectError struct {
	Failure ConnectFailure
	Reason  DeviceReason
}

func (e *ConnectError) Error() string {
	return "wifi connection failed (reason " + strconv.FormatUint(uint64(e.Reason), 10) + ")"
}

// ConnectTimeout is how long a connect is watched (CONNECTION_TIMEOUT).
const ConnectTimeout = 30 * time.Second

// deviceTransition is what a device's StateChanged carries.
type deviceTransition struct {
	New    uint32 `dbus:"new_state"`
	Old    uint32 `dbus:"old_state"`
	Reason uint32 `dbus:"reason"`
}

var deviceStateChanged = dbusx.NewSignal[deviceTransition](deviceIface, "StateChanged")

// ConnectAndWait connects to an access point and follows the device
// until it is up, fails, or ConnectTimeout passes (watchJoin).
func (w *Wifi) ConnectAndWait(ctx context.Context, apPath dbus.ObjectPath, password string, progress func(DeviceState)) error {
	return w.watchJoin(ctx, progress, func(ctx context.Context) error {
		_, err := w.Connect(ctx, apPath, password)
		return err
	})
}

// JoinHiddenAndWait joins a hidden network and follows the device as
// ConnectAndWait does.
func (w *Wifi) JoinHiddenAndWait(ctx context.Context, net HiddenNetwork, password string, progress func(DeviceState)) error {
	return w.watchJoin(ctx, progress, func(ctx context.Context) error {
		_, err := w.JoinHidden(ctx, net, password)
		return err
	})
}

// watchJoin runs join and follows the device until it is up, fails, or
// ConnectTimeout passes, reporting each intermediate state to progress
// (the connection watcher) from the caller's goroutine. Subscribed
// before joining, so a fast activation is not missed.
func (w *Wifi) watchJoin(ctx context.Context, progress func(DeviceState), join func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, ConnectTimeout)
	defer cancel()
	transitions := make(chan deviceTransition, 32)
	watch, err := dbusx.WatchSignals(ctx, w.nm.conn, dbusx.FromOwner(BusName),
		func(err error) { log.Printf("network: %v", err) },
		dbusx.On(deviceStateChanged, w.Device, func(change deviceTransition, _ dbus.Sequence) error {
			select {
			case transitions <- change:
			case <-ctx.Done():
			}
			return nil
		}))
	if err != nil {
		return fmt.Errorf("cannot watch the device: %w", err)
	}
	defer func() {
		cancel()
		if err := watch.Stop(); err != nil && !errors.Is(err, dbusx.ErrDisconnected) {
			log.Printf("network: %v", err)
		}
	}()
	if err := join(ctx); err != nil {
		return err
	}
	j := joinWatch{progress: progress}
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return &ConnectError{Failure: FailureTimeout}
			}
			return ctx.Err()
		case change := <-transitions:
			done, err := j.step(DeviceState(change.New), DeviceReason(change.Reason))
			if done {
				return err
			}
		}
	}
}

// joinWatch follows the device through one join. Transitions before
// the join's activation starts belong to the connection it replaces
// (that connection's late Deactivating and Disconnected, reason
// user-requested), so they neither end the watch nor count as progress.
type joinWatch struct {
	progress func(DeviceState)
	started  bool
}

// step handles one device transition: whether the watch is over, and
// how it ended.
func (j *joinWatch) step(state DeviceState, reason DeviceReason) (bool, error) {
	if !j.started {
		if state < DevicePrepare || state > DeviceActivated {
			return false, nil
		}
		j.started = true
	}
	switch state {
	case DeviceActivated:
		return true, nil
	case DeviceFailed:
		return true, &ConnectError{Failure: classifyDeviceFailure(reason), Reason: reason}
	case DeviceDisconnected:
		if isTransientDisconnect(reason) {
			return false, nil
		}
		return true, &ConnectError{Failure: classifyDeviceFailure(reason), Reason: reason}
	}
	if j.progress != nil {
		j.progress(state)
	}
	return false, nil
}

func isTransientDisconnect(reason DeviceReason) bool {
	return reason == DeviceReasonNone || reason == DeviceReasonUnknown || reason == DeviceReasonNewActivation
}

func classifyDeviceFailure(reason DeviceReason) ConnectFailure {
	switch reason {
	case DeviceReasonNoSecrets, DeviceReasonSupplicantDisconnect, DeviceReasonSupplicantConfigFailed, DeviceReasonSupplicantFailed:
		return FailureAuth
	case DeviceReasonSupplicantTimeout:
		return FailureTimeout
	case DeviceReasonSSIDNotFound:
		return FailureNotFound
	case DeviceReasonIPConfigUnavailable, DeviceReasonIPConfigExpired:
		return FailureIPConfig
	case DeviceReasonUserRequested, DeviceReasonNewActivation:
		return FailureSuperseded
	}
	return FailureGeneric
}
