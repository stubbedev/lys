package usershelper

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
)

// nscdSocket is _PATH_NSCDSOCKET, where glibc's nscd listens.
const nscdSocket = "/var/run/nscd/socket"

// The request nscd -i makes (glibc's nscd-client.h): a header of the
// protocol's version, the request's type and the key's length, each a
// host-order 32-bit integer, then the key, a cache's name and its NUL.
const (
	nscdVersion    = 2
	nscdInvalidate = 10
	nscdTimeout    = 5 * time.Second
)

// nscdCaches is what the shadow tools have nscd forget after a change
// to the account files.
var nscdCaches = []string{"passwd", "group"}

// forgetCached has the nscd at socket forget every cache of nscdCaches.
// An nscd that does not run, whose socket is missing or refuses, holds
// nothing to forget, and one that closes the connection unanswered, as
// an older nscd does once it has pruned the cache, has forgotten.
func forgetCached(ctx context.Context, socket string) error {
	for _, cache := range nscdCaches {
		if err := invalidate(ctx, socket, cache); err != nil {
			return fmt.Errorf("have nscd at %s forget its %s cache: %w", socket, cache, err)
		}
	}
	return nil
}

func invalidate(ctx context.Context, socket, cache string) error {
	ctx, cancel := context.WithTimeout(ctx, nscdTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socket)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	fail := func(err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	key := len(cache) + 1
	req := make([]byte, 0, 12+key)
	req = binary.NativeEndian.AppendUint32(req, nscdVersion)
	req = binary.NativeEndian.AppendUint32(req, nscdInvalidate)
	req = binary.NativeEndian.AppendUint32(req, intconv.Must[uint32](key))
	req = append(append(req, cache...), 0)
	if _, err := conn.Write(req); err != nil {
		return fail(err)
	}
	var resp [4]byte
	n, err := io.ReadFull(conn, resp[:])
	switch {
	case n == 0 && errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return fail(err)
	}
	if code := binary.NativeEndian.Uint32(resp[:]); code != 0 {
		return fmt.Errorf("nscd answers %w", syscall.Errno(code))
	}
	return nil
}
