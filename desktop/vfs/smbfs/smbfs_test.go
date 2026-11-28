package smbfs

import (
	"context"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"net"
	"strconv"
	"testing"

	"github.com/hirochachacha/go-smb2"

	"github.com/stubbedev/lys/desktop/vfs"
)

// No pure-Go SMB server exists to test against: these tests cover what
// the client does before and around a session, against a fake server
// that speaks just enough SMB2 to negotiate and refuse a login, and the
// paths and errors a session maps.

// The SMB2 commands the fake answers.
const (
	cmdNegotiate    = 0
	cmdSessionSetup = 1
)

// statusMoreProcessing asks the client for the next leg of its login.
const statusMoreProcessing = 0xC0000016

// challengeBody is a session setup response whose SPNEGO token carries
// an NTLM challenge, the login's first leg as a server answers it.
func challengeBody() []byte {
	challenge := make([]byte, 56, 60)
	copy(challenge, "NTLMSSP\x00")
	binary.LittleEndian.PutUint32(challenge[8:], 2)
	binary.LittleEndian.PutUint32(challenge[16:], 56)
	binary.LittleEndian.PutUint32(challenge[20:], 0xffffffff)
	copy(challenge[24:32], "8bytes!!")
	binary.LittleEndian.PutUint16(challenge[40:], 4)
	binary.LittleEndian.PutUint16(challenge[42:], 4)
	binary.LittleEndian.PutUint32(challenge[44:], 56)
	challenge = append(challenge, 0, 0, 0, 0)
	field := func(tag int, inner []byte) []byte {
		b, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: tag, IsCompound: true, Bytes: inner})
		return b
	}
	state, _ := asn1.Marshal(asn1.Enumerated(1))
	mech, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 2, 10})
	token, _ := asn1.Marshal(challenge)
	seq, _ := asn1.Marshal(asn1.RawValue{Tag: asn1.TagSequence, IsCompound: true, Bytes: append(append(field(0, state), field(1, mech)...), field(2, token)...)})
	spnego := field(1, seq)
	body := make([]byte, 8, 8+len(spnego))
	binary.LittleEndian.PutUint16(body[0:], 9)
	binary.LittleEndian.PutUint16(body[4:], 64+8)
	binary.LittleEndian.PutUint16(body[6:], uint16(len(spnego)))
	return append(body, spnego...)
}

// fakeServer negotiates SMB 2.1 and answers every session setup with
// status, until the test ends; it reports the logins it refused.
func fakeServer(t *testing.T, status uint32) (vfs.Server, <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refused := make(chan struct{}, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go answer(c, status, refused)
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	n, _ := strconv.ParseUint(port, 10, 16)
	return vfs.Server{Scheme: vfs.SMB, User: "ann", Host: host, Port: uint16(n)}, refused
}

func answer(c net.Conn, status uint32, refused chan<- struct{}) {
	defer func() { _ = c.Close() }()
	legs := 0
	for {
		var frame [4]byte
		if _, err := io.ReadFull(c, frame[:]); err != nil {
			return
		}
		req := make([]byte, binary.BigEndian.Uint32(frame[:])&0xffffff)
		if _, err := io.ReadFull(c, req); err != nil || len(req) < 64 {
			return
		}
		cmd := binary.LittleEndian.Uint16(req[12:])
		var body []byte
		var st uint32
		switch cmd {
		case cmdNegotiate:
			body = make([]byte, 65)
			binary.LittleEndian.PutUint16(body[0:], 65)
			binary.LittleEndian.PutUint16(body[2:], 1)
			binary.LittleEndian.PutUint16(body[4:], 0x0210)
			for _, at := range []int{28, 32, 36} {
				binary.LittleEndian.PutUint32(body[at:], 1<<16)
			}
			binary.LittleEndian.PutUint16(body[56:], 128)
		case cmdSessionSetup:
			legs++
			if legs == 1 {
				body, st = challengeBody(), statusMoreProcessing
				break
			}
			body = make([]byte, 9)
			binary.LittleEndian.PutUint16(body[0:], 9)
			st = status
			refused <- struct{}{}
		default:
			return
		}
		resp := make([]byte, 64, 64+len(body))
		copy(resp, []byte{0xfe, 'S', 'M', 'B'})
		binary.LittleEndian.PutUint16(resp[4:], 64)
		binary.LittleEndian.PutUint16(resp[6:], 1)
		binary.LittleEndian.PutUint32(resp[8:], st)
		binary.LittleEndian.PutUint16(resp[12:], cmd)
		binary.LittleEndian.PutUint16(resp[14:], 64)
		binary.LittleEndian.PutUint32(resp[16:], 1)
		copy(resp[24:32], req[24:32])
		resp = append(resp, body...)
		binary.BigEndian.PutUint32(frame[:], uint32(len(resp)))
		if _, err := c.Write(append(frame[:], resp...)); err != nil {
			return
		}
	}
}

// A server that refuses the login is vfs.ErrAuth, whatever the reason
// it gives; one that is no SMB server is not.
func TestARefusedLoginIsAnAuthError(t *testing.T) {
	for _, status := range []uint32{statusLogonFailure, statusAccountDisabled, statusPasswordExpired} {
		srv, refused := fakeServer(t, status)
		_, err := Dial(context.Background(), srv, vfs.Credentials{Password: "wrong"})
		if !errors.Is(err, vfs.ErrAuth) {
			t.Errorf("a login refused with %#x: %v, want %v", status, err, vfs.ErrAuth)
		}
		select {
		case <-refused:
		default:
			t.Errorf("the login refused with %#x never reached the server", status)
		}
	}
	srv, _ := fakeServer(t, 0xC0000001)
	if _, err := Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}); err == nil || errors.Is(err, vfs.ErrAuth) {
		t.Errorf("a server failing the setup otherwise: %v, want an error that is no auth error", err)
	}
}

// No user is vfs.ErrAuth before anything is sent; a server nothing
// listens for, or one that hangs up, is unreachable.
func TestNoUserAndNoServer(t *testing.T) {
	srv, refused := fakeServer(t, statusLogonFailure)
	srv.User = ""
	if _, err := Dial(context.Background(), srv, vfs.Credentials{}); !errors.Is(err, vfs.ErrAuth) {
		t.Errorf("no user: %v, want %v", err, vfs.ErrAuth)
	}
	if len(refused) != 0 {
		t.Error("a login with no user reached the server")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	hangsUp := vfs.Server{Scheme: vfs.SMB, User: "ann", Host: "127.0.0.1", Port: uint16(port)}
	if _, err := Dial(context.Background(), hangsUp, vfs.Credentials{Password: "pw"}); !errors.Is(err, vfs.ErrUnreachable) {
		t.Errorf("a server that hangs up: %v, want %v", err, vfs.ErrUnreachable)
	}
	_ = ln.Close()
	if _, err := Dial(context.Background(), hangsUp, vfs.Credentials{Password: "pw"}); !errors.Is(err, vfs.ErrUnreachable) {
		t.Errorf("a closed port: %v, want %v", err, vfs.ErrUnreachable)
	}
}

// A path is a share and the path within it; the server's root is no
// share, and what a server answers is typed as io/fs types it.
func TestPathsAndErrorsMapOntoShares(t *testing.T) {
	for p, want := range map[string][2]string{
		"/": {"", ""}, "/docs": {"docs", ""}, "/docs/": {"docs", ""}, "/docs/a/b.txt": {"docs", "a/b.txt"},
	} {
		if share, rest := split(p); share != want[0] || rest != want[1] {
			t.Errorf("split(%q) = %q %q, want %q %q", p, share, rest, want[0], want[1])
		}
	}
	for code, want := range map[uint32]error{statusBadNetworkName: fs.ErrNotExist, statusObjectNameCollision: fs.ErrExist} {
		if err := pathErr("open", "/x", &smb2.ResponseError{Code: code}); !errors.Is(err, want) {
			t.Errorf("status %#x: %v, want %v", code, err, want)
		}
	}
	c := &conn{}
	if _, err := c.Open(context.Background(), "/"); !errors.Is(err, vfs.ErrUnsupported) {
		t.Errorf("opening the server's root: %v", err)
	}
}
