// Package netfs connects to a network location by its scheme: SFTP,
// WebDAV, FTP and FTPS, or SMB, each through its own vfs client.
package netfs

import (
	"context"
	"fmt"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/davfs"
	"github.com/stubbedev/lys/desktop/vfs/ftpfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs"
	"github.com/stubbedev/lys/desktop/vfs/smbfs"
)

// Options are what a connection reaches beyond its credentials: the
// SSH agent's socket an SFTP login tries first, "" for none.
type Options struct {
	AgentSocket string
}

// Connect connects to the server of loc as cred says, trusting what
// trust holds; the errors are the clients' own, typed by vfs.
func Connect(ctx context.Context, loc vfs.Location, cred vfs.Credentials, trust *vfs.Trust, o Options) (vfs.Conn, error) {
	srv := loc.Server
	switch srv.Scheme {
	case vfs.SFTP:
		return sftpfs.Dial(ctx, srv, cred, trust, sftpfs.Options{AgentSocket: o.AgentSocket})
	case vfs.DAV, vfs.DAVS:
		return davfs.Dial(ctx, srv, cred, trust, loc.Path)
	case vfs.FTP, vfs.FTPS:
		return ftpfs.Dial(ctx, srv, cred, trust)
	case vfs.SMB:
		return smbfs.Dial(ctx, srv, cred)
	}
	return nil, fmt.Errorf("connect to %s: no client for %q: %w", srv, srv.Scheme, vfs.ErrUnsupported)
}

// NeedsDomain reports a scheme whose login names a domain too.
func NeedsDomain(s vfs.Scheme) bool { return s == vfs.SMB }
