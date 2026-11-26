// Package passwd names the users polkit asks to authenticate as, through
// systemd's user database, so every account source systemd-userdbd
// knows answers: nsswitch.conf's, drop-in records and systemd-homed.
package passwd

import (
	"context"

	"github.com/stubbedev/lys/lib/userdb"
)

var socket = userdb.SocketPath

// Name is the login name of uid.
func Name(ctx context.Context, uid uint32) (string, error) {
	u, err := userdb.LookupUID(ctx, socket, userdb.UID(uid))
	return u.Name, err
}
