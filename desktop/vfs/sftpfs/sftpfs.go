// Package sftpfs is a network location over SFTP (sftp:// and ssh://):
// an SSH connection through golang.org/x/crypto/ssh, its host key
// checked against known_hosts, and the SFTP subsystem through
// github.com/pkg/sftp, behind vfs.FS.
package sftpfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/stubbedev/lys/desktop/vfs"
)

// Options are where a connection finds what it authenticates with
// besides the password: the SSH agent's socket, "" for none.
type Options struct {
	AgentSocket string
}

// Dial connects to the SFTP server srv as cred says, its host key
// checked against trust's known_hosts: an unknown or changed key is a
// *vfs.HostKeyError, refused credentials vfs.ErrAuth, a server not
// reached a *vfs.UnreachableError.
func Dial(ctx context.Context, srv vfs.Server, cred vfs.Credentials, trust *vfs.Trust, o Options) (vfs.Conn, error) {
	user := cmpOr(cred.User, srv.User)
	if user == "" {
		return nil, fmt.Errorf("connect to %s: no user: %w", srv, vfs.ErrAuth)
	}
	addr := srv.Address()
	hostKey, err := hostKeyCallback(trust.KnownHosts)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", srv, err)
	}
	auths := []ssh.AuthMethod{}
	var agentConn net.Conn
	if o.AgentSocket != "" {
		if c, err := net.Dial("unix", o.AgentSocket); err == nil {
			agentConn = c
			auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(c).Signers))
		}
	}
	if cred.Password != "" {
		pw := cred.Password
		auths = append(auths, ssh.Password(pw), ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = pw
			}
			return answers, nil
		}))
	}
	defer func() {
		if agentConn != nil {
			_ = agentConn.Close()
		}
	}()
	tcp, err := vfs.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = tcp.SetDeadline(deadline)
	}
	config := &ssh.ClientConfig{User: user, Auth: auths, HostKeyCallback: hostKey, Timeout: vfs.DialTimeout}
	sc, chans, reqs, err := ssh.NewClientConn(tcp, addr, config)
	if err != nil {
		_ = tcp.Close()
		return nil, handshakeError(srv, err)
	}
	_ = tcp.SetDeadline(time.Time{})
	client := ssh.NewClient(sc, chans, reqs)
	sf, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start SFTP on %s: %w", srv, err)
	}
	return &conn{ssh: client, c: sf}, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// handshakeError is why the SSH handshake with srv failed, typed: the
// host key, the credentials, or the connection.
func handshakeError(srv vfs.Server, err error) error {
	var hk *vfs.HostKeyError
	switch {
	case errors.As(err, &hk):
		return hk
	case strings.Contains(err.Error(), "unable to authenticate"):
		return fmt.Errorf("log in to %s: %w: %w", srv, vfs.ErrAuth, err)
	}
	return &vfs.UnreachableError{Host: srv.Address(), Err: err}
}

// hostKeyCallback checks a host key against the known_hosts file: a
// key it lists for the host passes, an unknown one or one that differs
// is a *vfs.HostKeyError. A file not there yet knows no host.
func hostKeyCallback(file string) (ssh.HostKeyCallback, error) {
	var known ssh.HostKeyCallback
	switch _, err := os.Stat(file); {
	case file == "", errors.Is(err, fs.ErrNotExist):
		known = func(string, net.Addr, ssh.PublicKey) error { return &knownhosts.KeyError{} }
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", file, err)
	default:
		cb, err := knownhosts.New(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		known = cb
	}
	return func(host string, remote net.Addr, key ssh.PublicKey) error {
		err := known(host, remote, key)
		var ke *knownhosts.KeyError
		if !errors.As(err, &ke) {
			return err
		}
		return &vfs.HostKeyError{
			Host: host, Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key),
			Line: knownhosts.Line([]string{knownhosts.Normalize(host)}, key), Changed: len(ke.Want) > 0,
		}
	}, nil
}

// conn is an SFTP session over its SSH connection.
type conn struct {
	ssh *ssh.Client
	c   *sftp.Client
}

// Close ends the SFTP session and the SSH connection.
func (c *conn) Close() error { return errors.Join(c.c.Close(), c.ssh.Close()) }

func pathErr(op, p string, err error) error {
	if err == nil {
		return nil
	}
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		err = pe.Err
	}
	return &fs.PathError{Op: op, Path: p, Err: err}
}

// infoOf is what fi says of the entry name; a link is stated through
// to what it leads to.
func (c *conn) infoOf(name, p string, fi os.FileInfo) vfs.Info {
	if fi.Mode()&os.ModeSymlink == 0 {
		return vfs.Info{Name: name, IsDir: fi.IsDir(), Size: fi.Size(), Modified: fi.ModTime()}
	}
	target, err := c.c.Stat(p)
	if err != nil {
		return vfs.Info{Name: name, Link: true}
	}
	return vfs.Info{Name: name, IsDir: target.IsDir(), Link: true, Size: target.Size(), Modified: target.ModTime()}
}

// ReadDir implements vfs.FS.
func (c *conn) ReadDir(ctx context.Context, dir string) ([]vfs.Info, error) {
	return vfs.Call(ctx, func() ([]vfs.Info, error) {
		fis, err := c.c.ReadDirContext(ctx, dir)
		if err != nil {
			return nil, pathErr("readdir", dir, err)
		}
		infos := make([]vfs.Info, 0, len(fis))
		for _, fi := range fis {
			infos = append(infos, c.infoOf(fi.Name(), joinPath(dir, fi.Name()), fi))
		}
		return infos, nil
	})
}

func joinPath(dir, name string) string { return strings.TrimSuffix(dir, "/") + "/" + name }

// Stat implements vfs.FS.
func (c *conn) Stat(ctx context.Context, p string) (vfs.Info, error) {
	return vfs.Call(ctx, func() (vfs.Info, error) {
		fi, err := c.c.Lstat(p)
		if err != nil {
			return vfs.Info{}, pathErr("stat", p, err)
		}
		return c.infoOf(baseName(p), p, fi), nil
	})
}

func baseName(p string) string { return p[strings.LastIndexByte(p, '/')+1:] }

// Open implements vfs.FS: an *sftp.File, a reader at any offset.
func (c *conn) Open(ctx context.Context, p string) (vfs.File, error) {
	return vfs.Call(ctx, func() (vfs.File, error) {
		f, err := c.c.Open(p)
		if err != nil {
			return nil, pathErr("open", p, err)
		}
		return f, nil
	})
}

// Create implements vfs.FS.
func (c *conn) Create(ctx context.Context, p string) (io.WriteCloser, error) {
	return vfs.Call(ctx, func() (io.WriteCloser, error) {
		if err := c.free("create", p); err != nil {
			return nil, err
		}
		f, err := c.c.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return nil, pathErr("create", p, err)
		}
		return f, nil
	})
}

// free fails with fs.ErrExist when something is at p: SFTP v3 says
// nothing more than "failure" of a name taken.
func (c *conn) free(op, p string) error {
	if _, err := c.c.Lstat(p); err == nil {
		return &fs.PathError{Op: op, Path: p, Err: fs.ErrExist}
	}
	return nil
}

// Mkdir implements vfs.FS.
func (c *conn) Mkdir(ctx context.Context, p string) error {
	_, err := vfs.Call(ctx, func() (struct{}, error) {
		if err := c.free("mkdir", p); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, pathErr("mkdir", p, c.c.Mkdir(p))
	})
	return err
}

// Rename implements vfs.FS.
func (c *conn) Rename(ctx context.Context, from, to string) error {
	_, err := vfs.Call(ctx, func() (struct{}, error) {
		if err := c.free("rename", to); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, pathErr("rename", from, c.c.Rename(from, to))
	})
	return err
}

// Remove implements vfs.FS.
func (c *conn) Remove(ctx context.Context, p string) error {
	_, err := vfs.Call(ctx, func() (struct{}, error) {
		return struct{}{}, pathErr("remove", p, c.c.Remove(p))
	})
	return err
}
