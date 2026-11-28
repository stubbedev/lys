// Package ftpfs is a network location over FTP (ftp://) and explicit
// FTPS (ftps://, AUTH TLS on the control port) through
// github.com/jlaffaye/ftp, behind vfs.FS. An FTP control connection
// does one thing at a time, so the location keeps a few, logged in as
// they are needed: a file read or written holds one until it is closed.
package ftpfs

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/textproto"
	"path"
	"sync"

	"github.com/jlaffaye/ftp"
	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/desktop/vfs"
)

// maxConns bounds the control connections a location keeps.
const maxConns = 4

// Dial connects to the FTP server srv as cred says, an anonymous login
// without a user: refused credentials are vfs.ErrAuth, a certificate
// that does not verify a *vfs.CertificateError, a server not reached a
// *vfs.UnreachableError.
func Dial(ctx context.Context, srv vfs.Server, cred vfs.Credentials, trust *vfs.Trust) (vfs.Conn, error) {
	c := &conn{srv: srv, user: cmpOr(cmpOr(cred.User, srv.User), "anonymous"), password: cred.Password, slots: make(chan struct{}, maxConns)}
	if srv.Scheme == vfs.FTPS {
		c.tls = trust.TLSConfig(srv.Host)
	}
	sc, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.slots <- struct{}{}
	c.put(sc)
	return c, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// conn is an FTP server and the control connections idle on it.
type conn struct {
	srv            vfs.Server
	user, password string
	tls            *tls.Config
	slots          chan struct{}
	mu             sync.Mutex
	idle           []*ftp.ServerConn
	closed         bool
}

// dial opens and logs in one control connection.
func (c *conn) dial(ctx context.Context) (*ftp.ServerConn, error) {
	opts := []ftp.DialOption{ftp.DialWithContext(ctx), ftp.DialWithDialer(net.Dialer{Timeout: vfs.DialTimeout})}
	if c.tls != nil {
		opts = append(opts, ftp.DialWithExplicitTLS(c.tls))
	}
	sc, err := vfs.Call(ctx, func() (*ftp.ServerConn, error) { return ftp.Dial(c.srv.Address(), opts...) })
	if err != nil {
		return nil, c.connectError(err)
	}
	if _, err := vfs.Call(ctx, func() (struct{}, error) { return struct{}{}, sc.Login(c.user, c.password) }); err != nil {
		_ = sc.Quit()
		return nil, c.connectError(err)
	}
	return sc, nil
}

// connectError is why connecting failed, typed.
func (c *conn) connectError(err error) error {
	var ce *vfs.CertificateError
	var te *textproto.Error
	var oe *net.OpError
	switch {
	case errors.As(err, &ce):
		return ce
	case errors.As(err, &te) && te.Code == ftp.StatusNotLoggedIn:
		return fmt.Errorf("log in to %s: %w: %w", c.srv, vfs.ErrAuth, err)
	case errors.As(err, &oe), errors.Is(err, context.DeadlineExceeded):
		return &vfs.UnreachableError{Host: c.srv.Address(), Err: err}
	}
	return fmt.Errorf("connect to %s: %w", c.srv, err)
}

// get is a control connection to use alone, an idle one or a new one
// once fewer than maxConns are out.
func (c *conn) get(ctx context.Context) (*ftp.ServerConn, error) {
	select {
	case c.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		<-c.slots
		return nil, fs.ErrClosed
	}
	if n := len(c.idle); n > 0 {
		sc := c.idle[n-1]
		c.idle = c.idle[:n-1]
		c.mu.Unlock()
		return sc, nil
	}
	c.mu.Unlock()
	sc, err := c.dial(ctx)
	if err != nil {
		<-c.slots
		return nil, err
	}
	return sc, nil
}

// put hands a control connection back.
func (c *conn) put(sc *ftp.ServerConn) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = sc.Quit()
	} else {
		c.idle = append(c.idle, sc)
		c.mu.Unlock()
	}
	<-c.slots
}

// drop lets go of a control connection a failure left in a state
// nothing can trust.
func (c *conn) drop(sc *ftp.ServerConn) {
	_ = sc.Quit()
	<-c.slots
}

// attempt runs fn on sc under ctx: false when ctx ended first, and sc
// is no longer the caller's, dropped once fn is done with it.
func attempt[T any](ctx context.Context, c *conn, sc *ftp.ServerConn, fn func(*ftp.ServerConn) (T, error)) (T, bool, error) {
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := fn(sc)
		done <- result{v, err}
	}()
	select {
	case r := <-done:
		return r.v, true, r.err
	case <-ctx.Done():
		go func() {
			<-done
			c.drop(sc)
		}()
		var zero T
		return zero, false, ctx.Err()
	}
}

// with runs fn on a control connection of its own, under ctx.
func with[T any](ctx context.Context, c *conn, fn func(*ftp.ServerConn) (T, error)) (T, error) {
	sc, err := c.get(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	v, ours, err := attempt(ctx, c, sc, fn)
	if ours {
		c.put(sc)
	}
	return v, err
}

// Close quits every control connection.
func (c *conn) Close() error {
	c.mu.Lock()
	c.closed = true
	idle := c.idle
	c.idle = nil
	c.mu.Unlock()
	var errs []error
	for _, sc := range idle {
		errs = append(errs, sc.Quit())
	}
	return errors.Join(errs...)
}

// pathErr types what the server answered of op on p.
func pathErr(op, p string, err error) error {
	if err == nil {
		return nil
	}
	var te *textproto.Error
	if errors.As(err, &te) && te.Code == ftp.StatusFileUnavailable {
		err = fs.ErrNotExist
	}
	return &fs.PathError{Op: op, Path: p, Err: err}
}

func infoOf(e *ftp.Entry) vfs.Info {
	return vfs.Info{Name: e.Name, IsDir: e.Type == ftp.EntryTypeFolder, Link: e.Type == ftp.EntryTypeLink, Size: intconv.Clamp[int64](e.Size), Modified: e.Time}
}

func list(sc *ftp.ServerConn, dir string) ([]vfs.Info, error) {
	entries, err := sc.List(dir)
	if err != nil {
		return nil, err
	}
	infos := make([]vfs.Info, 0, len(entries))
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		infos = append(infos, infoOf(e))
	}
	return infos, nil
}

// ReadDir implements vfs.FS.
func (c *conn) ReadDir(ctx context.Context, dir string) ([]vfs.Info, error) {
	return with(ctx, c, func(sc *ftp.ServerConn) ([]vfs.Info, error) {
		if _, err := stat(sc, dir); err != nil {
			return nil, pathErr("readdir", dir, err)
		}
		infos, err := list(sc, dir)
		return infos, pathErr("readdir", dir, err)
	})
}

// stat states p from its folder's listing, which every server has.
func stat(sc *ftp.ServerConn, p string) (vfs.Info, error) {
	p = path.Clean(p)
	if p == "/" {
		return vfs.Info{Name: "/", IsDir: true}, nil
	}
	infos, err := list(sc, path.Dir(p))
	if err != nil {
		return vfs.Info{}, err
	}
	for _, info := range infos {
		if info.Name == path.Base(p) {
			return info, nil
		}
	}
	return vfs.Info{}, fs.ErrNotExist
}

// Stat implements vfs.FS.
func (c *conn) Stat(ctx context.Context, p string) (vfs.Info, error) {
	return with(ctx, c, func(sc *ftp.ServerConn) (vfs.Info, error) {
		info, err := stat(sc, p)
		return info, pathErr("stat", p, err)
	})
}

// Open implements vfs.FS: the file read as it comes, holding a control
// connection until it is closed.
func (c *conn) Open(ctx context.Context, p string) (vfs.File, error) {
	sc, err := c.get(ctx)
	if err != nil {
		return nil, err
	}
	resp, ours, err := attempt(ctx, c, sc, func(sc *ftp.ServerConn) (*ftp.Response, error) {
		if info, err := stat(sc, p); err != nil || info.IsDir {
			return nil, errors.Join(err, errors.New("not a file"))
		}
		return sc.Retr(p)
	})
	if err != nil {
		if ours {
			c.put(sc)
		}
		return nil, pathErr("open", p, err)
	}
	return &download{c: c, sc: sc, resp: resp}, nil
}

type download struct {
	c    *conn
	sc   *ftp.ServerConn
	resp *ftp.Response
	once sync.Once
	// whole is set once the file was read to its end.
	whole bool
}

func (d *download) Read(b []byte) (int, error) {
	n, err := d.resp.Read(b)
	if errors.Is(err, io.EOF) {
		d.whole = true
	}
	return n, err
}

// Close ends the transfer: the server's failure is the file's when it
// was read whole, and the abort of a transfer stopped early otherwise.
func (d *download) Close() error {
	var err error
	d.once.Do(func() {
		err = d.resp.Close()
		switch {
		case err == nil:
			d.c.put(d.sc)
		case d.whole:
			d.c.drop(d.sc)
		default:
			d.c.drop(d.sc)
			err = nil
		}
	})
	return err
}

// Create implements vfs.FS: what is written streams to the server,
// holding a control connection until it is closed.
func (c *conn) Create(ctx context.Context, p string) (io.WriteCloser, error) {
	sc, err := c.get(ctx)
	if err != nil {
		return nil, err
	}
	if _, ours, err := attempt(ctx, c, sc, func(sc *ftp.ServerConn) (struct{}, error) { return struct{}{}, c.free(sc, p) }); err != nil {
		if ours {
			c.put(sc)
		}
		return nil, pathErr("create", p, err)
	}
	pr, pw := io.Pipe()
	u := &upload{pw: pw, done: make(chan error, 1)}
	go func() {
		err := sc.Stor(p, pr)
		_ = pr.CloseWithError(err)
		if err != nil {
			c.drop(sc)
		} else {
			c.put(sc)
		}
		u.done <- pathErr("create", p, err)
	}()
	return u, nil
}

// free fails with fs.ErrExist when something is at p, and with
// fs.ErrNotExist when its folder is not there: STOR replaces, and says
// nothing of why it fails.
func (c *conn) free(sc *ftp.ServerConn, p string) error {
	if dir, err := stat(sc, path.Dir(p)); err != nil || !dir.IsDir {
		return fs.ErrNotExist
	}
	if _, err := stat(sc, p); err == nil {
		return fs.ErrExist
	}
	return nil
}

type upload struct {
	pw   *io.PipeWriter
	done chan error
}

func (u *upload) Write(b []byte) (int, error) { return u.pw.Write(b) }

func (u *upload) Close() error {
	_ = u.pw.Close()
	return <-u.done
}

// Mkdir implements vfs.FS.
func (c *conn) Mkdir(ctx context.Context, p string) error {
	_, err := with(ctx, c, func(sc *ftp.ServerConn) (struct{}, error) {
		if err := c.free(sc, p); err != nil {
			return struct{}{}, pathErr("mkdir", p, err)
		}
		return struct{}{}, pathErr("mkdir", p, sc.MakeDir(p))
	})
	return err
}

// Rename implements vfs.FS: RNFR and RNTO, once nothing is at to.
func (c *conn) Rename(ctx context.Context, from, to string) error {
	_, err := with(ctx, c, func(sc *ftp.ServerConn) (struct{}, error) {
		if err := c.free(sc, to); err != nil {
			return struct{}{}, pathErr("rename", to, err)
		}
		return struct{}{}, pathErr("rename", from, sc.Rename(from, to))
	})
	return err
}

// Remove implements vfs.FS.
func (c *conn) Remove(ctx context.Context, p string) error {
	_, err := with(ctx, c, func(sc *ftp.ServerConn) (struct{}, error) {
		info, err := stat(sc, p)
		if err != nil {
			return struct{}{}, pathErr("remove", p, err)
		}
		if info.IsDir {
			if held, err := list(sc, p); err != nil || len(held) > 0 {
				return struct{}{}, pathErr("remove", p, errors.Join(err, errors.New("directory not empty")))
			}
			return struct{}{}, pathErr("remove", p, sc.RemoveDir(p))
		}
		return struct{}{}, pathErr("remove", p, sc.Delete(p))
	})
	return err
}
