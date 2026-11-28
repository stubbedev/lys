package vfs

import (
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

// ErrUnsupported is what a filesystem cannot do: a trash on a network
// share, a rename on a server that has none.
var ErrUnsupported = errors.ErrUnsupported

// ErrAuth is a server refusing the credentials, or asking for some
// none were given for.
var ErrAuth = errors.New("the server refused the credentials")

// ErrUnreachable is a server that cannot be reached: no such host, no
// route, nothing listening, no answer in time.
var ErrUnreachable = errors.New("the server cannot be reached")

// ErrNotMounted is a location on a server that is not connected.
var ErrNotMounted = errors.New("the location is not connected")

// UnreachableError is a server that cannot be reached, and why.
type UnreachableError struct {
	Host string
	Err  error
}

func (e *UnreachableError) Error() string { return fmt.Sprintf("reach %s: %v", e.Host, e.Err) }

// Unwrap is ErrUnreachable and the cause.
func (e *UnreachableError) Unwrap() []error { return []error{ErrUnreachable, e.Err} }

// NotMountedError is a location on a server that is not connected.
type NotMountedError struct{ Server Server }

func (e *NotMountedError) Error() string { return e.Server.String() + " is not connected" }

// Unwrap is ErrNotMounted.
func (e *NotMountedError) Unwrap() error { return ErrNotMounted }

// HostKeyError is an SSH server whose key known_hosts does not list:
// a host never seen (Changed false), to trust or not, or one whose key
// differs from the one listed (Changed true), never to connect to.
type HostKeyError struct {
	Host string
	// Algorithm and Fingerprint name the key offered: its type and its
	// SHA256 fingerprint as ssh-keygen prints it.
	Algorithm, Fingerprint string
	// Line is the known_hosts line that trusts the key.
	Line    string
	Changed bool
}

func (e *HostKeyError) Error() string {
	if e.Changed {
		return fmt.Sprintf("the %s host key of %s changed to %s", e.Algorithm, e.Host, e.Fingerprint)
	}
	return fmt.Sprintf("the %s host key %s of %s is not known", e.Algorithm, e.Fingerprint, e.Host)
}

// CertificateError is a TLS server whose certificate does not verify,
// to trust or not: the server's leaf certificate and why it failed.
type CertificateError struct {
	Host string
	Cert *x509.Certificate
	Err  error
}

func (e *CertificateError) Error() string {
	return fmt.Sprintf("the certificate of %s does not verify: %v", e.Host, e.Err)
}

func (e *CertificateError) Unwrap() error { return e.Err }

// Fingerprint is the certificate's SHA-256 fingerprint, colon-separated
// hex as browsers show it.
func (e *CertificateError) Fingerprint() string { return CertFingerprint(e.Cert) }

// CertFingerprint is cert's SHA-256 fingerprint, colon-separated hex.
func CertFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	var b strings.Builder
	for i, c := range sum {
		if i > 0 {
			b.WriteByte(':')
		}
		fmt.Fprintf(&b, "%02X", c)
	}
	return b.String()
}
