package network

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// EAPMethod is the outer 802.1X authentication an enterprise network
// runs.
type EAPMethod int

// EAP methods.
const (
	EAPPEAP EAPMethod = iota
	EAPTTLS
	EAPTLS
)

// EAPMethods are the methods in the order the join form offers them.
var EAPMethods = []EAPMethod{EAPPEAP, EAPTTLS, EAPTLS}

// String is the method's protocol name.
func (m EAPMethod) String() string {
	switch m {
	case EAPPEAP:
		return "PEAP"
	case EAPTTLS:
		return "TTLS"
	case EAPTLS:
		return "TLS"
	}
	return fmt.Sprintf("EAPMethod(%d)", int(m))
}

func (m EAPMethod) nmName() string { return strings.ToLower(m.String()) }

// Phase2s are the inner authentications the method tunnels, in the
// order the join form offers them; TLS tunnels none.
func (m EAPMethod) Phase2s() []Phase2Auth {
	switch m {
	case EAPPEAP:
		return []Phase2Auth{Phase2MSCHAPv2, Phase2GTC, Phase2MD5}
	case EAPTTLS:
		return []Phase2Auth{Phase2MSCHAPv2, Phase2PAP, Phase2MSCHAP, Phase2CHAP}
	}
	return nil
}

func (m EAPMethod) fits(p Phase2Auth) bool {
	if m.UsesPassword() {
		return slices.Contains(m.Phase2s(), p)
	}
	return p == Phase2None
}

// UsesPassword reports whether the method signs in with a password; TLS
// signs in with a certificate and its key.
func (m EAPMethod) UsesPassword() bool { return m == EAPPEAP || m == EAPTTLS }

// Phase2Auth is the inner authentication a tunneled EAP method runs.
type Phase2Auth int

// Inner authentications; Phase2None is TLS's.
const (
	Phase2None Phase2Auth = iota
	Phase2MSCHAPv2
	Phase2GTC
	Phase2MD5
	Phase2PAP
	Phase2CHAP
	Phase2MSCHAP
)

// String is the inner authentication's protocol name.
func (p Phase2Auth) String() string {
	switch p {
	case Phase2None:
		return "none"
	case Phase2MSCHAPv2:
		return "MSCHAPv2"
	case Phase2GTC:
		return "GTC"
	case Phase2MD5:
		return "MD5"
	case Phase2PAP:
		return "PAP"
	case Phase2CHAP:
		return "CHAP"
	case Phase2MSCHAP:
		return "MSCHAP"
	}
	return fmt.Sprintf("Phase2Auth(%d)", int(p))
}

// Enterprise sign-in refusals.
var (
	ErrEAPMethodUnknown     = errors.New("unknown EAP method")
	ErrEAPPhase2Mismatch    = errors.New("the inner authentication does not fit the EAP method")
	ErrEAPIdentityEmpty     = errors.New("an enterprise network needs an identity")
	ErrEAPPasswordEmpty     = errors.New("this EAP method needs a password")
	ErrEAPServerUnchecked   = errors.New("an enterprise network needs a CA certificate or a server domain to check its server against")
	ErrEAPDomainInvalid     = errors.New("a server domain has no spaces")
	ErrEAPPathNotAbsolute   = errors.New("a certificate or key path must be absolute")
	ErrEAPClientCertMissing = errors.New("EAP-TLS needs a user certificate")
	ErrEAPPrivateKeyMissing = errors.New("EAP-TLS needs a private key")
	ErrEAPFieldUnused       = errors.New("a field the EAP method does not use is set")
	ErrEAPPathInvalid       = errors.New("a certificate or key path has a control character")
	ErrNotEnterprise        = errors.New("the network is not an enterprise network")
	ErrEnterprisePassword   = errors.New("an enterprise network signs in with EAP settings, not a password alone")
)

// EAPInput is an enterprise sign-in as typed: the method and what it
// needs. A tunneled method (PEAP, TTLS) takes its inner authentication
// and a password; TLS takes a user certificate and private key
// instead. Either checks the server against a CA certificate, a server
// domain, or both.
type EAPInput struct {
	Method             EAPMethod
	Phase2             Phase2Auth
	Identity           string
	AnonymousIdentity  string
	Password           string
	CACert             string
	Domain             string
	ClientCert         string
	PrivateKey         string
	PrivateKeyPassword string
}

// EAPSettings is a validated enterprise sign-in, the 802-1x setting a
// NetworkManager profile carries.
type EAPSettings struct{ in EAPInput }

// NewEAPSettings validates an enterprise sign-in, refusing the first
// thing that is missing or does not fit the method.
func NewEAPSettings(in EAPInput) (EAPSettings, error) {
	if !slices.Contains(EAPMethods, in.Method) {
		return EAPSettings{}, fmt.Errorf("%w: %d", ErrEAPMethodUnknown, int(in.Method))
	}
	if !in.Method.fits(in.Phase2) {
		return EAPSettings{}, fmt.Errorf("%w: %v over %v", ErrEAPPhase2Mismatch, in.Phase2, in.Method)
	}
	if strings.TrimSpace(in.Identity) == "" {
		return EAPSettings{}, ErrEAPIdentityEmpty
	}
	if err := in.checkCredentials(); err != nil {
		return EAPSettings{}, err
	}
	if in.CACert == "" && in.Domain == "" {
		return EAPSettings{}, ErrEAPServerUnchecked
	}
	if strings.ContainsFunc(in.Domain, unicode.IsSpace) {
		return EAPSettings{}, fmt.Errorf("%w: %q", ErrEAPDomainInvalid, in.Domain)
	}
	for _, path := range []string{in.CACert, in.ClientCert, in.PrivateKey} {
		if err := checkPath(path); err != nil {
			return EAPSettings{}, err
		}
	}
	return EAPSettings{in: in}, nil
}

func (in EAPInput) checkCredentials() error {
	if in.Method.UsesPassword() {
		switch {
		case in.Password == "":
			return fmt.Errorf("%w: %v", ErrEAPPasswordEmpty, in.Method)
		case in.ClientCert != "" || in.PrivateKey != "" || in.PrivateKeyPassword != "":
			return fmt.Errorf("%w: a certificate or key over %v", ErrEAPFieldUnused, in.Method)
		}
		return nil
	}
	switch {
	case in.ClientCert == "":
		return ErrEAPClientCertMissing
	case in.PrivateKey == "":
		return ErrEAPPrivateKeyMissing
	case in.Password != "" || in.AnonymousIdentity != "":
		return fmt.Errorf("%w: a password or anonymous identity over %v", ErrEAPFieldUnused, in.Method)
	}
	return nil
}

func checkPath(path string) error {
	switch {
	case path == "":
		return nil
	case strings.ContainsFunc(path, unicode.IsControl):
		return fmt.Errorf("%w: %q", ErrEAPPathInvalid, path)
	case !filepath.IsAbs(path):
		return fmt.Errorf("%w: %q", ErrEAPPathNotAbsolute, path)
	}
	return nil
}

// Method is the sign-in's EAP method.
func (e EAPSettings) Method() EAPMethod { return e.in.Method }

// certPath is a certificate or key path in NetworkManager's path
// scheme.
func certPath(path string) []byte { return []byte("file://" + path + "\x00") }

// section is the 802-1x setting.
func (e EAPSettings) section() map[string]dbus.Variant {
	in := e.in
	s := map[string]dbus.Variant{
		"eap":      dbus.MakeVariant([]string{in.Method.nmName()}),
		"identity": dbus.MakeVariant(in.Identity),
	}
	set := func(key, value string) {
		if value != "" {
			s[key] = dbus.MakeVariant(value)
		}
	}
	setPath := func(key, path string) {
		if path != "" {
			s[key] = dbus.MakeVariant(certPath(path))
		}
	}
	set("anonymous-identity", in.AnonymousIdentity)
	if in.Method.UsesPassword() {
		s["phase2-auth"] = dbus.MakeVariant(strings.ToLower(in.Phase2.String()))
		set("password", in.Password)
	}
	setPath("ca-cert", in.CACert)
	if in.CACert == "" {
		s["system-ca-certs"] = dbus.MakeVariant(true)
	}
	set("domain-suffix-match", in.Domain)
	setPath("client-cert", in.ClientCert)
	setPath("private-key", in.PrivateKey)
	set("private-key-password", in.PrivateKeyPassword)
	return s
}

// secure puts the sign-in into a wifi profile: WPA-EAP key management
// and the whole 802-1x setting, replacing whatever was there.
func (e EAPSettings) secure(dict ConnectionDict) {
	securitySection(dict)["key-mgmt"] = dbus.MakeVariant(SecurityEnterprise.keyMgmt())
	dict["802-1x"] = e.section()
}

// ConnectEnterprise joins an enterprise access point with eap: a saved
// profile for its SSID gets eap as its sign-in; otherwise a new profile
// is made. It returns the active connection.
func (w *Wifi) ConnectEnterprise(ctx context.Context, apPath dbus.ObjectPath, eap EAPSettings) (dbus.ObjectPath, error) {
	ap, err := w.accessPoint(ctx, apPath)
	if err != nil {
		return "", fmt.Errorf("cannot get ssid: %w", err)
	}
	if ap.Security != SecurityEnterprise {
		return "", fmt.Errorf("%w: %q is %v", ErrNotEnterprise, ap.Name(), ap.Security)
	}
	return w.join(ctx, ap.SSID, apPath, newWifiProfile(ap), eap.secure)
}

// ConnectEnterpriseAndWait joins an enterprise access point and follows
// the device as ConnectAndWait does.
func (w *Wifi) ConnectEnterpriseAndWait(ctx context.Context, apPath dbus.ObjectPath, eap EAPSettings, progress func(DeviceState)) error {
	return w.watchJoin(ctx, progress, func(ctx context.Context) error {
		_, err := w.ConnectEnterprise(ctx, apPath, eap)
		return err
	})
}

// JoinHiddenEnterprise joins an enterprise network that does not
// broadcast its name with eap: a saved profile for it gets eap as its
// sign-in; otherwise a new hidden profile is made. It returns the
// active connection.
func (w *Wifi) JoinHiddenEnterprise(ctx context.Context, net HiddenNetwork, eap EAPSettings) (dbus.ObjectPath, error) {
	if net.security != SecurityEnterprise {
		return "", fmt.Errorf("%w: %q is %v", ErrNotEnterprise, net.Name(), net.security)
	}
	return w.join(ctx, net.ssid, rootPath, hiddenWifiProfile(net), eap.secure)
}

// JoinHiddenEnterpriseAndWait joins a hidden enterprise network and
// follows the device as ConnectAndWait does.
func (w *Wifi) JoinHiddenEnterpriseAndWait(ctx context.Context, net HiddenNetwork, eap EAPSettings, progress func(DeviceState)) error {
	return w.watchJoin(ctx, progress, func(ctx context.Context) error {
		_, err := w.JoinHiddenEnterprise(ctx, net, eap)
		return err
	})
}
