package sftpfs_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs/sftptest"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// trusting is a Trust whose known_hosts lists s's key.
func trusting(t *testing.T, s *sftptest.Server) *vfs.Trust {
	t.Helper()
	file := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(s.Addr)}, s.Key)
	if err := os.WriteFile(file, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &vfs.Trust{KnownHosts: file}
}

// A connection to a known host with the right password is a
// filesystem that behaves as vfs.FS says.
func TestAConnectionConforms(t *testing.T) {
	s := sftptest.Start(t, "bob", "pw")
	c, err := sftpfs.Dial(context.Background(), s.Server("bob"), vfs.Credentials{Password: "pw"}, trusting(t, s), sftpfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	vfstest.Conform(t, c, t.TempDir())
}

// A wrong password, or none, is vfs.ErrAuth; no user at all is too.
func TestRefusedCredentialsAreAuthErrors(t *testing.T) {
	s := sftptest.Start(t, "bob", "pw")
	trust := trusting(t, s)
	for _, cred := range []vfs.Credentials{{Password: "wrong"}, {}, {User: "eve", Password: "pw"}} {
		_, err := sftpfs.Dial(context.Background(), s.Server("bob"), cred, trust, sftpfs.Options{})
		if !errors.Is(err, vfs.ErrAuth) {
			t.Errorf("logging in with %+v: %v, want %v", cred, err, vfs.ErrAuth)
		}
	}
	srv := s.Server("")
	if _, err := sftpfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, trust, sftpfs.Options{}); !errors.Is(err, vfs.ErrAuth) {
		t.Errorf("logging in with no user: %v", err)
	}
	if s.Logins() != 0 {
		t.Errorf("%d logins passed", s.Logins())
	}
}

// A host known_hosts does not list is a question, accepted into the
// file, after which it connects; a host whose key changed is refused
// and cannot be accepted.
func TestHostKeysAreCheckedAgainstKnownHosts(t *testing.T) {
	s := sftptest.Start(t, "bob", "pw")
	trust := &vfs.Trust{KnownHosts: filepath.Join(t.TempDir(), "ssh", "known_hosts")}
	_, err := sftpfs.Dial(context.Background(), s.Server("bob"), vfs.Credentials{Password: "pw"}, trust, sftpfs.Options{})
	var hk *vfs.HostKeyError
	if !errors.As(err, &hk) || hk.Changed || hk.Fingerprint != ssh.FingerprintSHA256(s.Key) {
		t.Fatalf("an unknown host: %v, want an unknown host key %s", err, ssh.FingerprintSHA256(s.Key))
	}
	if s.Logins() != 0 {
		t.Error("an unknown host was logged in to")
	}
	if err := trust.AcceptHostKey(hk); err != nil {
		t.Fatal(err)
	}
	c, err := sftpfs.Dial(context.Background(), s.Server("bob"), vfs.Credentials{Password: "pw"}, trust, sftpfs.Options{})
	if err != nil {
		t.Fatalf("the accepted host: %v", err)
	}
	_ = c.Close()

	other := sftptest.Start(t, "bob", "pw")
	_, port, _ := net.SplitHostPort(other.Addr)
	line := knownhosts.Line([]string{knownhosts.Normalize(other.Addr)}, s.Key)
	if err := os.WriteFile(trust.KnownHosts, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = sftpfs.Dial(context.Background(), other.Server("bob"), vfs.Credentials{Password: "pw"}, trust, sftpfs.Options{})
	if !errors.As(err, &hk) || !hk.Changed {
		t.Fatalf("a changed key on port %s: %v, want a changed host key", port, err)
	}
	if err := trust.AcceptHostKey(hk); err == nil {
		t.Error("a changed host key was accepted")
	}
	if other.Logins() != 0 {
		t.Error("a host whose key changed was logged in to")
	}
}

// A server nothing listens for is unreachable.
func TestAServerNotListeningIsUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()
	srv := vfs.Server{Scheme: vfs.SFTP, User: "bob", Host: "127.0.0.1", Port: uint16(addr.Port)}
	_, err = sftpfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, &vfs.Trust{}, sftpfs.Options{})
	if !errors.Is(err, vfs.ErrUnreachable) {
		t.Errorf("a closed port: %v, want %v", err, vfs.ErrUnreachable)
	}
}
