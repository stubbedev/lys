//go:build !darwin

package dbus_test

import (
	"bufio"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/sockdir"
	"github.com/stubbedev/lys/lib/waittest"
)

// serveAuth accepts one connection and answers its handshake, replying
// to NEGOTIATE_UNIX_FD with negotiated only once BEGIN has arrived, as
// sd-bus has read BEGIN by the time it answers. It reports why it
// stopped, nil once it has replied.
func serveAuth(l *net.UnixListener, negotiated string) <-chan error {
	done := make(chan error, 1)
	go func() {
		c, err := l.AcceptUnix()
		if err != nil {
			done <- fmt.Errorf("accept: %w", err)
			return
		}
		defer c.Close()
		done <- answer(bufio.NewReader(c), c, negotiated)
	}()
	return done
}

func answer(in *bufio.Reader, out net.Conn, negotiated string) error {
	if b, err := in.ReadByte(); err != nil || b != 0 {
		return fmt.Errorf("read the null byte: got %d: %w", b, err)
	}
	steps := []struct{ want, reply string }{
		{"AUTH", "REJECTED EXTERNAL"},
		{"AUTH EXTERNAL", "DATA"},
		{"DATA", "OK 0123456789abcdef0123456789abcdef"},
		{"NEGOTIATE_UNIX_FD", ""},
		{"BEGIN", negotiated},
	}
	for _, s := range steps {
		line, err := in.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read %s: %w", s.want, err)
		}
		if got := strings.TrimSuffix(line, "\r\n"); got != s.want {
			return fmt.Errorf("read %q, want %s", line, s.want)
		}
		if s.reply == "" {
			continue
		}
		if _, err := out.Write([]byte(s.reply + "\r\n")); err != nil {
			return fmt.Errorf("reply %q: %w", s.reply, err)
		}
	}
	return nil
}

// TestBeginGoesOutWithTheUnixFDNegotiation is the BEGIN patch: upstream
// sent BEGIN after reading the answer to NEGOTIATE_UNIX_FD, right before
// the first message, so a server could read both at once. sd-bus,
// serving a systemd manager's private socket, then held that message
// unread until more bytes came, and a lone ThawUnit never got its reply.
// BEGIN now goes out with NEGOTIATE_UNIX_FD, whether the server agrees
// to pass descriptors or not.
func TestBeginGoesOutWithTheUnixFDNegotiation(t *testing.T) {
	for _, negotiated := range []string{"AGREE_UNIX_FD", "ERROR"} {
		t.Run(negotiated, func(t *testing.T) {
			sock := filepath.Join(sockdir.Dir(t), "private")
			l, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = l.Close() })
			served := serveAuth(l, negotiated)

			conn, err := dbus.Dial("unix:path=" + sock)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			authed := make(chan error, 1)
			go func() { authed <- conn.Auth(nil) }()

			if err := waittest.For(t, served, "the server to read BEGIN before it answers NEGOTIATE_UNIX_FD"); err != nil {
				t.Fatal(err)
			}
			if err := waittest.For(t, authed, "the client to finish authenticating"); err != nil {
				t.Fatalf("authenticate: %v", err)
			}
		})
	}
}
