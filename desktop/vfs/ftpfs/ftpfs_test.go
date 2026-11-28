package ftpfs_test

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"testing"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/ftpfs"
	"github.com/stubbedev/lys/desktop/vfs/ftpfs/ftptest"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// An FTP server, plain and over explicit TLS once its certificate is
// accepted, behaves as vfs.FS says.
func TestAConnectionConforms(t *testing.T) {
	s := ftptest.Start(t, t.TempDir(), "bob", "pw", false)
	c, err := ftpfs.Dial(context.Background(), s.Server(vfs.FTP, "bob"), vfs.Credentials{Password: "pw"}, &vfs.Trust{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	vfstest.Conform(t, c, "/")
}

func TestFTPSAsksAboutItsCertificateAndConforms(t *testing.T) {
	s := ftptest.Start(t, t.TempDir(), "bob", "pw", true)
	srv := s.Server(vfs.FTPS, "bob")
	trust := &vfs.Trust{Roots: x509.NewCertPool()}
	_, err := ftpfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, trust)
	var ce *vfs.CertificateError
	if !errors.As(err, &ce) || ce.Cert == nil {
		t.Fatalf("an unknown certificate: %v, want a certificate question", err)
	}
	trust.AcceptCertificate(srv.Host, ce.Cert)
	c, err := ftpfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, trust)
	if err != nil {
		t.Fatalf("the accepted certificate: %v", err)
	}
	defer func() { _ = c.Close() }()
	vfstest.Conform(t, c, "/")
}

// A wrong password is vfs.ErrAuth; a server nothing listens for is
// unreachable.
func TestRefusedAndUnreachable(t *testing.T) {
	s := ftptest.Start(t, t.TempDir(), "bob", "pw", false)
	if _, err := ftpfs.Dial(context.Background(), s.Server(vfs.FTP, "bob"), vfs.Credentials{Password: "wrong"}, &vfs.Trust{}); !errors.Is(err, vfs.ErrAuth) {
		t.Errorf("a wrong password: %v, want %v", err, vfs.ErrAuth)
	}
	if _, err := ftpfs.Dial(context.Background(), s.Server(vfs.FTP, ""), vfs.Credentials{}, &vfs.Trust{}); !errors.Is(err, vfs.ErrAuth) {
		t.Errorf("an anonymous login the server refuses: %v, want %v", err, vfs.ErrAuth)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	srv := vfs.Server{Scheme: vfs.FTP, Host: "127.0.0.1", Port: uint16(port)}
	if _, err := ftpfs.Dial(context.Background(), srv, vfs.Credentials{}, &vfs.Trust{}); !errors.Is(err, vfs.ErrUnreachable) {
		t.Errorf("a closed port: %v, want %v", err, vfs.ErrUnreachable)
	}
}
