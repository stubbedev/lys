// Package smbfs is a network location over SMB 2 and 3 (smb://) through
// github.com/hirochachacha/go-smb2, behind vfs.FS. The location's root
// is the server: its shares are the root's folders, each mounted the
// first time a path below it is used.
package smbfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/hirochachacha/go-smb2"

	"github.com/stubbedev/lys/desktop/vfs"
)

// The NTSTATUS codes a connection types.
const (
	statusLogonFailure        = 0xC000006D
	statusAccountRestriction  = 0xC000006E
	statusPasswordExpired     = 0xC0000071
	statusAccountDisabled     = 0xC0000072
	statusBadNetworkName      = 0xC00000CC
	statusObjectNameCollision = 0xC0000035
	statusDirectoryNotEmpty   = 0xC0000101
)

// Dial connects to the SMB server srv as cred says, through NTLM:
// refused credentials, or none, are vfs.ErrAuth, a server not reached a
// *vfs.UnreachableError.
func Dial(ctx context.Context, srv vfs.Server, cred vfs.Credentials) (vfs.Conn, error) {
	user := cmpOr(cred.User, srv.User)
	if user == "" {
		return nil, fmt.Errorf("connect to %s: no user: %w", srv, vfs.ErrAuth)
	}
	tcp, err := vfs.Dial(ctx, srv.Address())
	if err != nil {
		return nil, err
	}
	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: user, Password: cred.Password, Domain: cmpOr(cred.Domain, srv.Domain)}}
	s, err := d.DialContext(ctx, tcp)
	if err != nil {
		_ = tcp.Close()
		return nil, sessionError(srv, err)
	}
	return &conn{s: s, shares: map[string]*smb2.Share{}}, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// sessionError is why a session could not be set up, typed.
func sessionError(srv vfs.Server, err error) error {
	if re, ok := errors.AsType[*smb2.ResponseError](err); ok {
		switch re.Code {
		case statusLogonFailure, statusAccountRestriction, statusPasswordExpired, statusAccountDisabled:
			return fmt.Errorf("log in to %s: %w: %w", srv, vfs.ErrAuth, err)
		}
	}
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("log in to %s: %w: %w", srv, vfs.ErrAuth, err)
	}
	if _, ok := errors.AsType[*smb2.TransportError](err); ok {
		return &vfs.UnreachableError{Host: srv.Address(), Err: err}
	}
	return fmt.Errorf("connect to %s: %w", srv, err)
}

// conn is an SMB session and the shares mounted on it.
type conn struct {
	s      *smb2.Session
	mu     sync.Mutex
	shares map[string]*smb2.Share
}

// Close unmounts the shares and logs off.
func (c *conn) Close() error {
	c.mu.Lock()
	shares := c.shares
	c.shares = map[string]*smb2.Share{}
	c.mu.Unlock()
	var errs []error
	for _, sh := range shares {
		errs = append(errs, sh.Umount())
	}
	return errors.Join(append(errs, c.s.Logoff())...)
}

// split cuts a path into its share and the path within it, "" for the
// server's root or the share's.
func split(p string) (share, rest string) {
	p = strings.TrimPrefix(path.Clean(p), "/")
	share, rest, _ = strings.Cut(p, "/")
	return share, rest
}

// share is the share name mounted, under ctx.
func (c *conn) share(ctx context.Context, name string) (*smb2.Share, error) {
	c.mu.Lock()
	sh, ok := c.shares[name]
	c.mu.Unlock()
	if !ok {
		var err error
		sh, err = c.s.WithContext(ctx).Mount(name)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		if prev, ok := c.shares[name]; ok {
			c.mu.Unlock()
			_ = sh.Umount()
			sh = prev
		} else {
			c.shares[name] = sh
			c.mu.Unlock()
		}
	}
	return sh.WithContext(ctx), nil
}

// pathErr types what the server answered of op on p.
func pathErr(op, p string, err error) error {
	if err == nil {
		return nil
	}
	if re, ok := errors.AsType[*smb2.ResponseError](err); ok {
		switch re.Code {
		case statusBadNetworkName:
			err = fs.ErrNotExist
		case statusObjectNameCollision:
			err = fs.ErrExist
		case statusDirectoryNotEmpty:
			err = errors.New("directory not empty")
		}
	}
	var pe *fs.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		err = pe.Err
	case errors.As(err, &le):
		err = le.Err
	}
	return &fs.PathError{Op: op, Path: p, Err: err}
}

func infoOf(name string, fi os.FileInfo) vfs.Info {
	return vfs.Info{Name: name, IsDir: fi.IsDir(), Link: fi.Mode()&os.ModeSymlink != 0, Size: fi.Size(), Modified: fi.ModTime()}
}

// within is the share p lies on and p's path in it; the server's root
// is ErrUnsupported for op.
func (c *conn) within(ctx context.Context, op, p string) (*smb2.Share, string, error) {
	share, rest := split(p)
	if share == "" {
		return nil, "", &fs.PathError{Op: op, Path: p, Err: vfs.ErrUnsupported}
	}
	sh, err := c.share(ctx, share)
	if err != nil {
		return nil, "", pathErr(op, p, err)
	}
	return sh, rest, nil
}

// ReadDir implements vfs.FS: the server's root lists its shares.
func (c *conn) ReadDir(ctx context.Context, dir string) ([]vfs.Info, error) {
	if share, _ := split(dir); share == "" {
		names, err := c.s.WithContext(ctx).ListSharenames()
		if err != nil {
			return nil, pathErr("readdir", dir, err)
		}
		infos := make([]vfs.Info, 0, len(names))
		for _, n := range names {
			if !strings.HasSuffix(n, "$") {
				infos = append(infos, vfs.Info{Name: n, IsDir: true})
			}
		}
		return infos, nil
	}
	sh, rest, err := c.within(ctx, "readdir", dir)
	if err != nil {
		return nil, err
	}
	fis, err := sh.ReadDir(rest)
	if err != nil {
		return nil, pathErr("readdir", dir, err)
	}
	infos := make([]vfs.Info, 0, len(fis))
	for _, fi := range fis {
		infos = append(infos, infoOf(fi.Name(), fi))
	}
	return infos, nil
}

// Stat implements vfs.FS.
func (c *conn) Stat(ctx context.Context, p string) (vfs.Info, error) {
	share, rest := split(p)
	if share == "" {
		return vfs.Info{Name: "/", IsDir: true}, nil
	}
	sh, _, err := c.within(ctx, "stat", p)
	if err != nil {
		return vfs.Info{}, err
	}
	if rest == "" {
		return vfs.Info{Name: share, IsDir: true}, nil
	}
	fi, err := sh.Stat(rest)
	if err != nil {
		return vfs.Info{}, pathErr("stat", p, err)
	}
	return infoOf(path.Base(rest), fi), nil
}

// Open implements vfs.FS: an *smb2.File, a reader at any offset.
func (c *conn) Open(ctx context.Context, p string) (vfs.File, error) {
	sh, rest, err := c.within(ctx, "open", p)
	if err != nil {
		return nil, err
	}
	f, err := sh.Open(rest)
	if err != nil {
		return nil, pathErr("open", p, err)
	}
	return f, nil
}

// Create implements vfs.FS.
func (c *conn) Create(ctx context.Context, p string) (io.WriteCloser, error) {
	sh, rest, err := c.within(ctx, "create", p)
	if err != nil {
		return nil, err
	}
	f, err := sh.OpenFile(rest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, pathErr("create", p, err)
	}
	return f, nil
}

// Mkdir implements vfs.FS.
func (c *conn) Mkdir(ctx context.Context, p string) error {
	sh, rest, err := c.within(ctx, "mkdir", p)
	if err != nil {
		return err
	}
	return pathErr("mkdir", p, sh.Mkdir(rest, 0o755))
}

// Rename implements vfs.FS, within one share.
func (c *conn) Rename(ctx context.Context, from, to string) error {
	sh, rest, err := c.within(ctx, "rename", from)
	if err != nil {
		return err
	}
	toShare, toRest := split(to)
	if fromShare, _ := split(from); toShare != fromShare {
		return &fs.PathError{Op: "rename", Path: from, Err: fmt.Errorf("to another share: %w", vfs.ErrUnsupported)}
	}
	return pathErr("rename", from, sh.Rename(rest, toRest))
}

// Remove implements vfs.FS.
func (c *conn) Remove(ctx context.Context, p string) error {
	sh, rest, err := c.within(ctx, "remove", p)
	if err != nil {
		return err
	}
	if rest == "" {
		return &fs.PathError{Op: "remove", Path: p, Err: vfs.ErrUnsupported}
	}
	return pathErr("remove", p, sh.Remove(rest))
}
