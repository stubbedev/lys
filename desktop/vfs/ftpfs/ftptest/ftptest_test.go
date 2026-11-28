package ftptest

import (
	"crypto/tls"
	"net"
	"testing"
)

// A protected data connection is handed out with its TLS handshake
// done, so closing it after writing nothing (an empty listing) is a
// clean TLS close. Handed out before the handshake, the close left the
// client's unread ClientHello in the socket and the kernel answered
// with a reset, which failed the FTPS tests now and then.
func TestAProtectedDataConnectionIsHandshakenBeforeUse(t *testing.T) {
	s := Start(t, t.TempDir(), "u", "p", true)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ss := &session{s: s, pasv: ln, protect: true}
	client := make(chan error, 1)
	go func() {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			client <- err
			return
		}
		defer func() { _ = c.Close() }()
		tc := tls.Client(c, &tls.Config{InsecureSkipVerify: true})
		client <- tc.Handshake()
	}()
	c, err := ss.data()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	tc, ok := c.(*tls.Conn)
	if !ok || !tc.ConnectionState().HandshakeComplete {
		t.Fatalf("the protected data connection %T was handed out before its handshake", c)
	}
	if err := <-client; err != nil {
		t.Fatalf("the client's handshake: %v", err)
	}
}

// An unprotected data connection is the plain socket.
func TestAnUnprotectedDataConnectionIsPlain(t *testing.T) {
	s := Start(t, t.TempDir(), "u", "p", false)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ss := &session{s: s, pasv: ln}
	go func() {
		if c, err := net.Dial("tcp", ln.Addr().String()); err == nil {
			_ = c.Close()
		}
	}()
	c, err := ss.data()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, ok := c.(*tls.Conn); ok {
		t.Fatal("an unprotected data connection speaks TLS")
	}
}
