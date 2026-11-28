package vfs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Credentials are what a connection logs in with: the user, the domain
// (SMB alone), and the password; an empty password logs in without one
// (a key, an anonymous login).
type Credentials struct {
	User, Domain, Password string
}

// DialTimeout bounds reaching a server.
const DialTimeout = 15 * time.Second

// Dial reaches addr over TCP within DialTimeout; a server that cannot
// be reached is an *UnreachableError.
func Dial(ctx context.Context, addr string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, &UnreachableError{Host: addr, Err: err}
	}
	return c, nil
}

// Trust is what connections trust beyond the system's roots: the SSH
// hosts a known_hosts file lists, and the TLS certificates the user
// accepted for this session. It is safe for concurrent use.
type Trust struct {
	// KnownHosts is the known_hosts file SSH hosts are checked against
	// and accepted into.
	KnownHosts string
	// Roots are the certificate authorities TLS servers are verified
	// against; nil for the system's.
	Roots *x509.CertPool
	mu    sync.Mutex
	certs map[string]string
}

// AcceptCertificate trusts cert for host for as long as the Trust
// lives, as the user answered a CertificateError.
func (t *Trust) AcceptCertificate(host string, cert *x509.Certificate) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.certs == nil {
		t.certs = map[string]string{}
	}
	t.certs[host] = CertFingerprint(cert)
}

// accepted reports whether the user accepted cert for host.
func (t *Trust) accepted(host string, cert *x509.Certificate) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.certs[host] != "" && t.certs[host] == CertFingerprint(cert)
}

// TLSConfig is the TLS a connection to host speaks: the server's chain
// verified against the roots and its name, else its certificate one the
// user accepted, else a *CertificateError to ask about.
func (t *Trust) TLSConfig(host string) *tls.Config {
	return &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // VerifyConnection verifies the chain, or the certificate the user accepted
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return &CertificateError{Host: host, Err: errors.New("the server sent no certificate")}
			}
			leaf := cs.PeerCertificates[0]
			inter := x509.NewCertPool()
			for _, c := range cs.PeerCertificates[1:] {
				inter.AddCert(c)
			}
			_, err := leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: t.Roots, Intermediates: inter})
			if err == nil || t.accepted(host, leaf) {
				return nil
			}
			return &CertificateError{Host: host, Cert: leaf, Err: err}
		},
	}
}

// AcceptHostKey trusts the SSH host key e is about from now on,
// appending its line to the known_hosts file; a changed key is never
// accepted.
func (t *Trust) AcceptHostKey(e *HostKeyError) error {
	if e.Changed {
		return fmt.Errorf("accept the host key of %s: %w", e.Host, e)
	}
	if err := os.MkdirAll(filepath.Dir(t.KnownHosts), 0o700); err != nil {
		return fmt.Errorf("accept the host key of %s: %w", e.Host, err)
	}
	f, err := os.OpenFile(t.KnownHosts, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("accept the host key of %s: %w", e.Host, err)
	}
	_, err = f.WriteString(e.Line + "\n")
	if err = errors.Join(err, f.Close()); err != nil {
		return fmt.Errorf("accept the host key of %s into %s: %w", e.Host, t.KnownHosts, err)
	}
	return nil
}

// Call runs fn, giving up with ctx's error once ctx ends first, for a
// client whose calls take no context: what fn asked of the server is
// answered, and dropped, while the connection lives.
func Call[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := fn()
		done <- result{v, err}
	}()
	select {
	case r := <-done:
		return r.v, r.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
