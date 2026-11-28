// Package sftptest is an SFTP server in the test's process: an SSH
// server through golang.org/x/crypto/ssh serving github.com/pkg/sftp's
// subsystem over the local disk, logging one user in by password.
package sftptest

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/stubbedev/lys/desktop/vfs"
)

// Server is a running SFTP server.
type Server struct {
	// Addr is where it listens; Key is its host key.
	Addr string
	Key  ssh.PublicKey
	ln   net.Listener
	wg   sync.WaitGroup
	mu   sync.Mutex
	// logins counts the logins that passed; conns are the connections
	// open, closed when the test ends.
	logins int
	conns  map[net.Conn]bool
}

// Start serves SFTP to user with password on 127.0.0.1 until the test
// ends.
func Start(t testing.TB, user, password string) *Server {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Key: signer.PublicKey(), conns: map[net.Conn]bool{}}
	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if meta.User() == user && string(pw) == password {
				s.mu.Lock()
				s.logins++
				s.mu.Unlock()
				return nil, nil
			}
			return nil, errors.New("wrong password")
		},
	}
	config.AddHostKey(signer)
	s.ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Addr = s.ln.Addr().String()
	s.wg.Go(func() {
		for {
			c, err := s.ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns[c] = true
			s.mu.Unlock()
			s.wg.Go(func() {
				serve(c, config)
				s.mu.Lock()
				delete(s.conns, c)
				s.mu.Unlock()
			})
		}
	})
	t.Cleanup(func() {
		_ = s.ln.Close()
		s.mu.Lock()
		for c := range s.conns {
			_ = c.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return s
}

// Logins is how many logins passed.
func (s *Server) Logins() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logins
}

// Server is the vfs server that reaches s as user.
func (s *Server) Server(user string) vfs.Server {
	host, port, _ := net.SplitHostPort(s.Addr)
	n, _ := strconv.ParseUint(port, 10, 16)
	return vfs.Server{Scheme: vfs.SFTP, User: user, Host: host, Port: uint16(n)}
}

func serve(c net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = c.Close() }()
	sc, chans, reqs, err := ssh.NewServerConn(c, config)
	if err != nil {
		return
	}
	defer func() { _ = sc.Close() }()
	go ssh.DiscardRequests(reqs)
	var wg sync.WaitGroup
	defer wg.Wait()
	for nc := range chans {
		if nc.ChannelType() != "session" {
			_ = nc.Reject(ssh.UnknownChannelType, "sessions alone")
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			return
		}
		wg.Go(func() {
			defer func() { _ = ch.Close() }()
			for req := range requests {
				ok := req.Type == "subsystem" && len(req.Payload) > 4 && string(req.Payload[4:]) == "sftp"
				_ = req.Reply(ok, nil)
				if !ok {
					continue
				}
				srv, err := sftp.NewServer(ch)
				if err != nil {
					return
				}
				_ = srv.Serve()
				_ = srv.Close()
				return
			}
		})
	}
}
