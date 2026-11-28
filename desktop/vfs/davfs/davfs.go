// Package davfs is a network location over WebDAV (dav:// over HTTP,
// davs:// over HTTPS): PROPFIND lists and states, GET reads, with a
// Range for a read at an offset, PUT writes, MKCOL makes a folder, MOVE
// renames and DELETE removes, through net/http, behind vfs.FS.
package davfs

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/fileuri"

	"github.com/stubbedev/lys/desktop/vfs"
)

// headerTimeout bounds how long a server may take to answer a request.
const headerTimeout = 30 * time.Second

// Dial connects to the WebDAV server srv as cred says, stating probe,
// the location first opened, to check it answers: refused credentials
// are vfs.ErrAuth, a certificate that does not verify a
// *vfs.CertificateError, a server not reached a *vfs.UnreachableError.
func Dial(ctx context.Context, srv vfs.Server, cred vfs.Credentials, trust *vfs.Trust, probe string) (vfs.Conn, error) {
	scheme := "http"
	transport := &http.Transport{
		DialContext:           func(ctx context.Context, _, addr string) (net.Conn, error) { return vfs.Dial(ctx, addr) },
		ResponseHeaderTimeout: headerTimeout,
		ForceAttemptHTTP2:     true,
	}
	if srv.Scheme == vfs.DAVS {
		scheme = "https"
		transport.TLSClientConfig = trust.TLSConfig(srv.Host)
	}
	c := &conn{
		base:   url.URL{Scheme: scheme, Host: srv.Address()},
		client: &http.Client{Transport: transport},
		user:   cmpOr(cred.User, srv.User), password: cred.Password,
		srv: srv,
	}
	if _, err := c.Stat(ctx, probe); err != nil && !errors.Is(err, fs.ErrNotExist) {
		transport.CloseIdleConnections()
		return nil, fmt.Errorf("connect to %s: %w", srv, err)
	}
	return c, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// conn is a WebDAV server.
type conn struct {
	base           url.URL
	client         *http.Client
	user, password string
	srv            vfs.Server
}

// Close drops the idle connections.
func (c *conn) Close() error {
	c.client.CloseIdleConnections()
	return nil
}

// href is p's URL on the server, percent-encoded.
func (c *conn) href(p string) string {
	u := c.base
	u.RawPath = fileuri.Encode(p, "!$&'()*+,/:=@")
	u.Path = p
	return u.String()
}

// do sends a request for p and returns its response; a status the
// request may not answer with is an error, typed as io/fs and vfs
// type them.
func (c *conn) do(ctx context.Context, op, method, p string, body io.Reader, header http.Header, ok ...int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.href(p), body)
	if err != nil {
		return nil, &fs.PathError{Op: op, Path: p, Err: err}
	}
	maps.Copy(req.Header, header)
	if c.user != "" || c.password != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ue, ok := errors.AsType[*url.Error](err); ok {
			err = ue.Err
		}
		var ce *vfs.CertificateError
		var unreachable *vfs.UnreachableError
		if errors.As(err, &ce) || errors.As(err, &unreachable) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, &vfs.UnreachableError{Host: c.srv.Address(), Err: err}
	}
	if slices.Contains(ok, resp.StatusCode) {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	return nil, statusError(op, p, c.srv, resp.StatusCode)
}

// statusError is what an HTTP status means for op on p.
func statusError(op, p string, srv vfs.Server, code int) error {
	var err error
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s %s on %s: %w", op, p, srv, vfs.ErrAuth)
	case http.StatusForbidden:
		err = fs.ErrPermission
	case http.StatusNotFound, http.StatusConflict:
		err = fs.ErrNotExist
	case http.StatusMethodNotAllowed, http.StatusPreconditionFailed:
		err = fs.ErrExist
	case http.StatusNotImplemented:
		err = vfs.ErrUnsupported
	default:
		err = fmt.Errorf("the server answered %d %s", code, http.StatusText(code))
	}
	return &fs.PathError{Op: op, Path: p, Err: err}
}

// propfindBody asks for what a listing needs.
const propfindBody = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/></d:prop></d:propfind>`

type multistatus struct {
	Responses []response `xml:"DAV: response"`
}

type response struct {
	Href     string     `xml:"DAV: href"`
	Propstat []propstat `xml:"DAV: propstat"`
}

type propstat struct {
	Status string `xml:"DAV: status"`
	Prop   struct {
		ResourceType struct {
			Collection *struct{} `xml:"DAV: collection"`
		} `xml:"DAV: resourcetype"`
		Length   string `xml:"DAV: getcontentlength"`
		Modified string `xml:"DAV: getlastmodified"`
	} `xml:"DAV: prop"`
}

// maxListing bounds a PROPFIND answer read.
const maxListing = 64 << 20

// propfind states p, and with depth 1 what the folder p holds, each by
// its path.
func (c *conn) propfind(ctx context.Context, op, p string, depth int) (map[string]vfs.Info, error) {
	h := http.Header{"Depth": {strconv.Itoa(depth)}, "Content-Type": {"application/xml; charset=utf-8"}}
	resp, err := c.do(ctx, op, "PROPFIND", p, strings.NewReader(propfindBody), h, http.StatusMultiStatus)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var ms multistatus
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxListing)).Decode(&ms); err != nil {
		return nil, &fs.PathError{Op: op, Path: p, Err: fmt.Errorf("the server's listing: %w", err)}
	}
	infos := make(map[string]vfs.Info, len(ms.Responses))
	for _, r := range ms.Responses {
		rp, ok := c.hrefPath(r.Href)
		if !ok {
			continue
		}
		info := vfs.Info{Name: path.Base(rp)}
		for _, ps := range r.Propstat {
			if !strings.Contains(ps.Status, " 200") {
				continue
			}
			info.IsDir = ps.Prop.ResourceType.Collection != nil
			info.Size, _ = strconv.ParseInt(strings.TrimSpace(ps.Prop.Length), 10, 64)
			if t, err := http.ParseTime(strings.TrimSpace(ps.Prop.Modified)); err == nil {
				info.Modified = t
			}
		}
		infos[rp] = info
	}
	return infos, nil
}

// hrefPath is the path a response's href names, a path or a URL,
// percent-decoded and cleaned.
func (c *conn) hrefPath(href string) (string, bool) {
	u, err := url.Parse(href)
	if err != nil || u.Path == "" {
		return "", false
	}
	return path.Clean(u.Path), true
}

// ReadDir implements vfs.FS.
func (c *conn) ReadDir(ctx context.Context, dir string) ([]vfs.Info, error) {
	infos, err := c.propfind(ctx, "readdir", dir, 1)
	if err != nil {
		return nil, err
	}
	self := path.Clean(dir)
	if info, ok := infos[self]; ok && !info.IsDir {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: errors.New("not a folder")}
	}
	list := make([]vfs.Info, 0, len(infos))
	for p, info := range infos {
		if p != self {
			list = append(list, info)
		}
	}
	return list, nil
}

// Stat implements vfs.FS.
func (c *conn) Stat(ctx context.Context, p string) (vfs.Info, error) {
	infos, err := c.propfind(ctx, "stat", p, 0)
	if err != nil {
		return vfs.Info{}, err
	}
	if info, ok := infos[path.Clean(p)]; ok {
		return info, nil
	}
	if len(infos) == 1 {
		for _, info := range infos {
			return info, nil
		}
	}
	return vfs.Info{}, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
}

// Open implements vfs.FS: a GET read as it comes, and a Range GET for
// a read at an offset.
func (c *conn) Open(ctx context.Context, p string) (vfs.File, error) {
	resp, err := c.do(ctx, "open", http.MethodGet, p, nil, nil, http.StatusOK) //nolint:bodyclose // the file closes it
	if err != nil {
		return nil, err
	}
	return &file{c: c, ctx: ctx, p: p, body: resp.Body}, nil
}

type file struct {
	c    *conn
	ctx  context.Context
	p    string
	body io.ReadCloser
}

func (f *file) Read(b []byte) (int, error) { return f.body.Read(b) }

func (f *file) Close() error { return f.body.Close() }

// ReadAt reads len(b) bytes from off through a Range request.
func (f *file) ReadAt(b []byte, off int64) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	h := http.Header{"Range": {fmt.Sprintf("bytes=%d-%d", off, off+int64(len(b))-1)}}
	resp, err := f.c.do(f.ctx, "read", http.MethodGet, f.p, nil, h, http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return 0, io.EOF
	}
	n, err := io.ReadFull(resp.Body, b)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

// Create implements vfs.FS: a PUT whose body streams what is written,
// asking the server to refuse it when the name is taken.
func (c *conn) Create(ctx context.Context, p string) (io.WriteCloser, error) {
	if _, err := c.Stat(ctx, p); err == nil {
		return nil, &fs.PathError{Op: "create", Path: p, Err: fs.ErrExist}
	}
	if _, err := c.Stat(ctx, path.Dir(p)); err != nil {
		return nil, &fs.PathError{Op: "create", Path: p, Err: fs.ErrNotExist}
	}
	pr, pw := io.Pipe()
	w := &upload{pw: pw, done: make(chan error, 1)}
	go func() {
		resp, err := c.do(ctx, "create", http.MethodPut, p, pr, http.Header{"If-None-Match": {"*"}}, http.StatusCreated, http.StatusNoContent, http.StatusOK)
		if err == nil {
			_ = resp.Body.Close()
		}
		_ = pr.CloseWithError(err)
		w.done <- err
	}()
	return w, nil
}

type upload struct {
	pw   *io.PipeWriter
	done chan error
}

func (u *upload) Write(b []byte) (int, error) { return u.pw.Write(b) }

// Close ends the body and waits for the server to take the file.
func (u *upload) Close() error {
	_ = u.pw.Close()
	return <-u.done
}

// Mkdir implements vfs.FS.
func (c *conn) Mkdir(ctx context.Context, p string) error {
	resp, err := c.do(ctx, "mkdir", "MKCOL", p, nil, nil, http.StatusCreated)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Rename implements vfs.FS: a MOVE that does not overwrite.
func (c *conn) Rename(ctx context.Context, from, to string) error {
	h := http.Header{"Destination": {c.href(to)}, "Overwrite": {"F"}}
	resp, err := c.do(ctx, "rename", "MOVE", from, nil, h, http.StatusCreated, http.StatusNoContent)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Remove implements vfs.FS: a folder only once it is empty, as a
// DELETE would remove all it holds.
func (c *conn) Remove(ctx context.Context, p string) error {
	info, err := c.Stat(ctx, p)
	if err != nil {
		return err
	}
	if info.IsDir {
		held, err := c.ReadDir(ctx, p)
		if err != nil {
			return err
		}
		if len(held) > 0 {
			return &fs.PathError{Op: "remove", Path: p, Err: errors.New("directory not empty")}
		}
	}
	resp, err := c.do(ctx, "remove", http.MethodDelete, p, nil, nil, http.StatusNoContent, http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
