package vfs

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/fileuri"
)

// Scheme is the protocol a location is reached through; the local
// disk has none.
type Scheme string

// The schemes a location names. ssh:// is read as sftp://.
const (
	Local Scheme = ""
	SMB   Scheme = "smb"
	SFTP  Scheme = "sftp"
	DAV   Scheme = "dav"
	DAVS  Scheme = "davs"
	FTP   Scheme = "ftp"
	FTPS  Scheme = "ftps"
	MTP   Scheme = "mtp"
)

// schemes are the schemes a typed location may name, each with the
// scheme it is read as.
var schemes = map[string]Scheme{
	"smb": SMB, "sftp": SFTP, "ssh": SFTP, "dav": DAV, "davs": DAVS, "ftp": FTP, "ftps": FTPS, "mtp": MTP,
}

// DefaultPort is the port a server of scheme listens on unless told
// otherwise; 0 for one that has no port.
func (s Scheme) DefaultPort() uint16 {
	switch s {
	case SMB:
		return 445
	case SFTP:
		return 22
	case DAV:
		return 80
	case DAVS:
		return 443
	case FTP, FTPS:
		return 21
	}
	return 0
}

// Server is what a connection is made to, and what a connected
// location is mounted as: the scheme, the user (and for SMB the
// domain), the host and the port, 0 for the scheme's own.
type Server struct {
	Scheme Scheme
	User   string
	Domain string
	Host   string
	Port   uint16
}

// Address is host:port, the scheme's port when none is given.
func (s Server) Address() string {
	port := s.Port
	if port == 0 {
		port = s.Scheme.DefaultPort()
	}
	return net.JoinHostPort(s.Host, strconv.Itoa(int(port)))
}

// String is the server's root without its trailing slash:
// scheme://[domain;][user@]host[:port].
func (s Server) String() string {
	var b strings.Builder
	b.WriteString(string(s.Scheme))
	b.WriteString("://")
	if s.Domain != "" {
		b.WriteString(fileuri.Encode(s.Domain, userinfoKeep))
		b.WriteByte(';')
	}
	if s.User != "" {
		b.WriteString(fileuri.Encode(s.User, userinfoKeep))
		b.WriteByte('@')
	}
	if strings.Contains(s.Host, ":") {
		b.WriteString("[" + s.Host + "]")
	} else {
		b.WriteString(s.Host)
	}
	if s.Port != 0 && s.Port != s.Scheme.DefaultPort() {
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(int(s.Port)))
	}
	return b.String()
}

// userinfoKeep are the bytes a user or a domain keeps unescaped: the
// sub-delimiters but ; which ends a domain.
const userinfoKeep = "!$&'()*+,="

// Location is a place on a filesystem: a server (the zero Server for
// the local disk) and an absolute, clean path on it.
type Location struct {
	Server Server
	Path   string
}

// IsLocal reports a location on the local disk.
func (l Location) IsLocal() bool { return l.Server.Scheme == Local }

// String is the location as the browser shows and keeps it: the path
// for the local disk, else the server's root and the path, unescaped.
func (l Location) String() string {
	if l.IsLocal() {
		return l.Path
	}
	return l.Server.String() + l.Path
}

// Errors parsing a location.
var (
	ErrScheme   = errors.New("no supported scheme")
	ErrPassword = errors.New("a location carries no password: the connection asks for it")
)

// Parse reads a location: an absolute local path, a file:// URI, or
// scheme://[domain;][user@]host[:port][/path] for a scheme this package
// knows, its user and domain percent-decoded and its path taken as
// typed.
func Parse(s string) (Location, error) {
	switch {
	case strings.HasPrefix(s, "/"):
		return Location{Path: filepath.Clean(s)}, nil
	case strings.HasPrefix(s, "file://"):
		p, ok := fileuri.Path(s)
		if !ok {
			return Location{}, fmt.Errorf("parse the location %q: not a local file URI", s)
		}
		return Location{Path: filepath.Clean(p)}, nil
	}
	name, rest, ok := strings.Cut(s, "://")
	if !ok {
		return Location{}, fmt.Errorf("parse the location %q: neither a path nor scheme://host", s)
	}
	scheme, known := schemes[strings.ToLower(name)]
	if !known {
		return Location{}, fmt.Errorf("parse the location %q: %w (%s)", s, ErrScheme, name)
	}
	authority, p := rest, "/"
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		authority, p = rest[:i], path.Clean(rest[i:])
	}
	srv, err := parseAuthority(scheme, authority)
	if err != nil {
		return Location{}, fmt.Errorf("parse the location %q: %w", s, err)
	}
	return Location{Server: srv, Path: p}, nil
}

// parseAuthority reads [domain;][user@]host[:port].
func parseAuthority(scheme Scheme, authority string) (Server, error) {
	srv := Server{Scheme: scheme}
	hostport := authority
	if userinfo, host, ok := strings.CutLast(authority, "@"); ok {
		hostport = host
		if strings.Contains(userinfo, ":") {
			return Server{}, ErrPassword
		}
		if domain, user, ok := strings.Cut(userinfo, ";"); ok {
			d, err := url.PathUnescape(domain)
			if err != nil {
				return Server{}, fmt.Errorf("the domain %q: %w", domain, err)
			}
			srv.Domain, userinfo = d, user
		}
		u, err := url.PathUnescape(userinfo)
		if err != nil {
			return Server{}, fmt.Errorf("the user %q: %w", userinfo, err)
		}
		srv.User = u
	}
	host, port := hostport, ""
	switch {
	case strings.HasPrefix(hostport, "["):
		end := strings.IndexByte(hostport, ']')
		if end < 0 {
			return Server{}, fmt.Errorf("the host %q has no closing ]", hostport)
		}
		host, port = hostport[1:end], strings.TrimPrefix(hostport[end+1:], ":")
	case strings.Count(hostport, ":") == 1:
		host, port, _ = strings.Cut(hostport, ":")
	}
	if host == "" {
		return Server{}, errors.New("no host")
	}
	srv.Host = strings.ToLower(host)
	if port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil || n == 0 {
			return Server{}, fmt.Errorf("the port %q is not a port number", port)
		}
		srv.Port = uint16(n)
	}
	return srv, nil
}

// IsRemote reports a location string that names a server, not a local
// path.
func IsRemote(loc string) bool {
	return strings.Contains(loc, "://") && !strings.HasPrefix(loc, "file://")
}

// splitRoot cuts a location string into its server's root
// (scheme://authority, "" for a local path) and its path.
func splitRoot(loc string) (root, p string) {
	if !IsRemote(loc) {
		return "", loc
	}
	i := strings.Index(loc, "://") + 3
	j := strings.IndexByte(loc[i:], '/')
	if j < 0 {
		return loc, "/"
	}
	return loc[:i+j], loc[i+j:]
}

// Root is the location of the root of loc's filesystem: / for a local
// path, the server's root for a remote one.
func Root(loc string) string {
	root, _ := splitRoot(loc)
	return root + "/"
}

// Dir is the folder holding loc; the root holds itself.
func Dir(loc string) string {
	root, p := splitRoot(loc)
	if root == "" {
		return filepath.Dir(p)
	}
	return root + path.Dir(p)
}

// Base is loc's last element; a server's root is its host.
func Base(loc string) string {
	root, p := splitRoot(loc)
	if root != "" && (p == "/" || p == "") {
		return root[strings.Index(root, "://")+3:]
	}
	return filepath.Base(p)
}

// Join is name in the folder loc.
func Join(loc string, name ...string) string {
	root, p := splitRoot(loc)
	if root == "" {
		return filepath.Join(append([]string{p}, name...)...)
	}
	return root + path.Join(append([]string{p}, name...)...)
}

// Clean is loc with its path cleaned.
func Clean(loc string) string {
	root, p := splitRoot(loc)
	if root == "" {
		return filepath.Clean(p)
	}
	return root + path.Clean("/"+p)
}

// Within reports whether loc is dir or lies under it.
func Within(loc, dir string) bool {
	return loc == dir || strings.HasPrefix(loc, strings.TrimSuffix(dir, "/")+"/")
}

// URI is loc as a URI, its path percent-encoded: a file:// URI for a
// local path.
func URI(loc string) string {
	root, p := splitRoot(loc)
	if root == "" {
		return fileuri.FromPath(p)
	}
	return root + fileuri.Encode(p, "!$&'()*+,/:=@")
}

// FromURI is the location a URI names, its path percent-decoded: a
// local path for a file:// URI, false for a URI of no scheme this
// package knows.
func FromURI(uri string) (string, bool) {
	if p, ok := fileuri.Path(uri); ok {
		return p, true
	}
	name, _, ok := strings.Cut(uri, "://")
	if _, known := schemes[strings.ToLower(name)]; !ok || !known {
		return "", false
	}
	root, p := splitRoot(uri)
	l, err := Parse(root + fileuri.DecodeString(p))
	if err != nil {
		return "", false
	}
	return l.String(), true
}
