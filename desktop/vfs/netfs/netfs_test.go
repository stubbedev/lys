package netfs_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/webdav"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/ftpfs/ftptest"
	"github.com/stubbedev/lys/desktop/vfs/netfs"
)

// A location connects through the client its scheme names; a scheme no
// client connects by address is unsupported.
func TestConnectGoesByScheme(t *testing.T) {
	dav := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(t.TempDir()), LockSystem: webdav.NewMemLS()})
	t.Cleanup(dav.Close)
	ftp := ftptest.Start(t, t.TempDir(), "bob", "pw", false)
	for _, loc := range []string{"dav://" + dav.Listener.Addr().String() + "/", ftp.Server(vfs.FTP, "bob").String() + "/"} {
		l, err := vfs.Parse(loc)
		if err != nil {
			t.Fatal(err)
		}
		c, err := netfs.Connect(context.Background(), l, vfs.Credentials{Password: "pw"}, &vfs.Trust{}, netfs.Options{})
		if err != nil {
			t.Errorf("connect to %s: %v", loc, err)
			continue
		}
		if _, err := c.ReadDir(context.Background(), "/"); err != nil {
			t.Errorf("list %s: %v", loc, err)
		}
		_ = c.Close()
	}
	l, _ := vfs.Parse("mtp://1-2/")
	if _, err := netfs.Connect(context.Background(), l, vfs.Credentials{}, &vfs.Trust{}, netfs.Options{}); !errors.Is(err, vfs.ErrUnsupported) {
		t.Errorf("an mtp address: %v, want %v", err, vfs.ErrUnsupported)
	}
}
