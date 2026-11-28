package davfs_test

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"golang.org/x/net/webdav"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/davfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// serve is a WebDAV server over a scratch folder, over TLS when tls is
// set, letting bob in with pw.
func serve(t *testing.T, tls bool) (*httptest.Server, vfs.Server) {
	t.Helper()
	dav := &webdav.Handler{FileSystem: webdav.Dir(t.TempDir()), LockSystem: webdav.NewMemLS()}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "bob" || p != "pw" {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		dav.ServeHTTP(w, r)
	})
	var s *httptest.Server
	scheme := vfs.DAV
	if tls {
		s, scheme = httptest.NewTLSServer(h), vfs.DAVS
	} else {
		s = httptest.NewServer(h)
	}
	t.Cleanup(s.Close)
	u, _ := url.Parse(s.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	n, _ := strconv.ParseUint(port, 10, 16)
	return s, vfs.Server{Scheme: scheme, User: "bob", Host: host, Port: uint16(n)}
}

// A WebDAV server behaves as vfs.FS says.
func TestAConnectionConforms(t *testing.T) {
	_, srv := serve(t, false)
	c, err := davfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, &vfs.Trust{}, "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	vfstest.Conform(t, c, "/")
}

// A wrong password, or none, is vfs.ErrAuth.
func TestRefusedCredentialsAreAuthErrors(t *testing.T) {
	_, srv := serve(t, false)
	for _, cred := range []vfs.Credentials{{Password: "wrong"}, {}} {
		if _, err := davfs.Dial(context.Background(), srv, cred, &vfs.Trust{}, "/"); !errors.Is(err, vfs.ErrAuth) {
			t.Errorf("logging in with %+v: %v, want %v", cred, err, vfs.ErrAuth)
		}
	}
}

// A certificate no root vouches for is a question; once accepted the
// server connects, and a server another root vouches for asks nothing.
func TestCertificatesThatDoNotVerifyAreAsked(t *testing.T) {
	s, srv := serve(t, true)
	trust := &vfs.Trust{Roots: x509.NewCertPool()}
	_, err := davfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, trust, "/")
	var ce *vfs.CertificateError
	if !errors.As(err, &ce) || ce.Cert == nil || ce.Fingerprint() != vfs.CertFingerprint(s.Certificate()) {
		t.Fatalf("an unknown certificate: %v, want a certificate question", err)
	}
	trust.AcceptCertificate(srv.Host, ce.Cert)
	c, err := davfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, trust, "/")
	if err != nil {
		t.Fatalf("the accepted certificate: %v", err)
	}
	vfstest.Conform(t, c, "/")
	_ = c.Close()

	roots := x509.NewCertPool()
	roots.AddCert(s.Certificate())
	if _, err := davfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, &vfs.Trust{Roots: roots}, "/"); err != nil {
		t.Errorf("a server a root vouches for: %v", err)
	}
}

// A server nothing listens for is unreachable.
func TestAServerNotListeningIsUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	srv := vfs.Server{Scheme: vfs.DAV, Host: "127.0.0.1", Port: uint16(port)}
	if _, err := davfs.Dial(context.Background(), srv, vfs.Credentials{}, &vfs.Trust{}, "/"); !errors.Is(err, vfs.ErrUnreachable) {
		t.Errorf("a closed port: %v, want %v", err, vfs.ErrUnreachable)
	}
}
