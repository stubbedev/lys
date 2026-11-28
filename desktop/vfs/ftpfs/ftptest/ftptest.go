// Package ftptest is an FTP server in the test's process, over a
// folder of the local disk: the commands an FTP client lists, makes,
// reads, writes, renames and removes with, passive data connections,
// MLSD listings, and explicit TLS (AUTH TLS) for FTPS.
package ftptest

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/vfs"
)

// Server is a running FTP server.
type Server struct {
	Addr string
	// Cert is the certificate it speaks TLS with, nil without TLS.
	Cert           *tls.Certificate
	root           string
	user, password string
	tls            *tls.Config
	ln             net.Listener
	wg             sync.WaitGroup
}

// Start serves root over FTP to user with password on 127.0.0.1 until
// the test ends; with withTLS it takes AUTH TLS, and data connections
// go over TLS once PROT P asks it.
func Start(t testing.TB, root, user, password string, withTLS bool) *Server {
	t.Helper()
	s := &Server{root: root, user: user, password: password}
	if withTLS {
		cert := httptest.NewUnstartedServer(nil)
		cert.StartTLS()
		s.tls = cert.TLS.Clone()
		s.Cert = &s.tls.Certificates[0]
		cert.Close()
	}
	var err error
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
			s.wg.Go(func() { s.serve(c) })
		}
	})
	t.Cleanup(func() {
		_ = s.ln.Close()
		s.wg.Wait()
	})
	return s
}

// Server is the vfs server that reaches s as user.
func (s *Server) Server(scheme vfs.Scheme, user string) vfs.Server {
	host, port, _ := net.SplitHostPort(s.Addr)
	n, _ := strconv.ParseUint(port, 10, 16)
	return vfs.Server{Scheme: scheme, User: user, Host: host, Port: uint16(n)}
}

// session is one control connection.
type session struct {
	s        *Server
	c        net.Conn
	r        *bufio.Reader
	user     string
	loggedIn bool
	protect  bool
	pasv     net.Listener
	renaming string
}

func (s *Server) serve(c net.Conn) {
	ss := &session{s: s, c: c, r: bufio.NewReader(c)}
	defer func() {
		_ = ss.c.Close()
		if ss.pasv != nil {
			_ = ss.pasv.Close()
		}
	}()
	ss.reply(220, "ready")
	for {
		line, err := ss.r.ReadString('\n')
		if err != nil {
			return
		}
		cmd, arg, _ := strings.Cut(strings.TrimRight(line, "\r\n"), " ")
		if !ss.handle(strings.ToUpper(cmd), arg) {
			return
		}
	}
}

func (ss *session) reply(code int, msg string) { _, _ = fmt.Fprintf(ss.c, "%d %s\r\n", code, msg) }

// local is the file the path p names under the root.
func (ss *session) local(p string) string {
	return filepath.Join(ss.s.root, filepath.FromSlash(path.Clean("/"+p)))
}

// handle runs one command; false ends the session.
func (ss *session) handle(cmd, arg string) bool {
	switch cmd {
	case "AUTH":
		if ss.s.tls == nil {
			ss.reply(502, "no TLS")
			return true
		}
		ss.reply(234, "go ahead")
		tc := tls.Server(ss.c, ss.s.tls)
		if err := tc.Handshake(); err != nil {
			return false
		}
		ss.c, ss.r = tc, bufio.NewReader(tc)
		return true
	case "USER":
		ss.user = arg
		ss.reply(331, "password")
		return true
	case "PASS":
		if ss.user != ss.s.user || arg != ss.s.password {
			ss.reply(530, "login incorrect")
			return true
		}
		ss.loggedIn = true
		ss.reply(230, "logged in")
		return true
	case "QUIT":
		ss.reply(221, "bye")
		return false
	}
	if !ss.loggedIn {
		ss.reply(530, "log in first")
		return true
	}
	switch cmd {
	case "FEAT":
		_, _ = fmt.Fprintf(ss.c, "211-Features:\r\n MLST type*;size*;modify*;\r\n UTF8\r\n211 End\r\n")
	case "OPTS", "NOOP":
		ss.reply(200, "ok")
	case "TYPE":
		ss.reply(200, "binary")
	case "PBSZ":
		ss.reply(200, "ok")
	case "PROT":
		ss.protect = arg == "P"
		ss.reply(200, "ok")
	case "PWD":
		ss.reply(257, `"/"`)
	case "EPSV":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			ss.reply(425, err.Error())
			break
		}
		ss.pasv = ln
		_, port, _ := net.SplitHostPort(ln.Addr().String())
		ss.reply(229, "entering extended passive mode (|||"+port+"|)")
	case "MLSD":
		ss.mlsd(arg)
	case "RETR":
		ss.retr(arg)
	case "STOR":
		ss.stor(arg)
	case "MKD":
		ss.done(os.Mkdir(ss.local(arg), 0o700), 257)
	case "RMD":
		ss.done(os.Remove(ss.local(arg)), 250)
	case "DELE":
		info, err := os.Stat(ss.local(arg))
		if err == nil && info.IsDir() {
			err = errors.New("a folder")
		}
		if err == nil {
			err = os.Remove(ss.local(arg))
		}
		ss.done(err, 250)
	case "RNFR":
		ss.renaming = arg
		ss.reply(350, "ready")
	case "RNTO":
		ss.done(os.Rename(ss.local(ss.renaming), ss.local(arg)), 250)
	default:
		ss.reply(502, "not implemented")
	}
	return true
}

func (ss *session) done(err error, code int) {
	if err != nil {
		ss.reply(550, err.Error())
		return
	}
	ss.reply(code, "ok")
}

// data accepts the passive data connection, over TLS when protected.
func (ss *session) data() (net.Conn, error) {
	if ss.pasv == nil {
		return nil, errors.New("no passive connection")
	}
	ln := ss.pasv
	ss.pasv = nil
	defer func() { _ = ln.Close() }()
	c, err := ln.Accept()
	if err != nil {
		return nil, err
	}
	if ss.protect {
		return tls.Server(c, ss.s.tls), nil
	}
	return c, nil
}

func (ss *session) mlsd(arg string) {
	des, err := os.ReadDir(ss.local(arg))
	if err != nil {
		ss.reply(550, err.Error())
		return
	}
	ss.reply(150, "listing")
	c, err := ss.data()
	if err != nil {
		ss.reply(425, err.Error())
		return
	}
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue
		}
		kind := "file"
		if info.IsDir() {
			kind = "dir"
		}
		_, _ = fmt.Fprintf(c, "type=%s;size=%d;modify=%s; %s\r\n", kind, info.Size(), info.ModTime().UTC().Format("20060102150405"), de.Name())
	}
	_ = c.Close()
	ss.reply(226, "done")
}

func (ss *session) retr(arg string) {
	f, err := os.Open(ss.local(arg))
	if err != nil {
		ss.reply(550, err.Error())
		return
	}
	defer func() { _ = f.Close() }()
	ss.reply(150, "sending")
	c, err := ss.data()
	if err != nil {
		ss.reply(425, err.Error())
		return
	}
	_, err = io.Copy(c, f)
	_ = c.Close()
	ss.done(err, 226)
}

func (ss *session) stor(arg string) {
	f, err := os.Create(ss.local(arg))
	if err != nil {
		ss.reply(550, err.Error())
		return
	}
	ss.reply(150, "receiving")
	c, err := ss.data()
	if err != nil {
		_ = f.Close()
		ss.reply(425, err.Error())
		return
	}
	_, err = io.Copy(f, c)
	_ = c.Close()
	ss.done(errors.Join(err, f.Close()), 226)
}
