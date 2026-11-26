// Package useraccount describes a user account the way the desktop
// shows it: login name, real name, home, and avatar.
package useraccount

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stubbedev/lys/lib/userdb"
)

// AvatarsDir is where lys keeps avatars, one file per login name,
// readable by the greeter where home directories are not.
const AvatarsDir = "/var/lib/lys/avatars"

const accountsServiceIcons = "/var/lib/AccountsService/icons"

// AvatarPath is the file in AvatarsDir that holds login's avatar.
func AvatarPath(login string) string { return filepath.Join(AvatarsDir, login) }

// Account is one user account.
type Account struct {
	// Name is the login name; RealName the GECOS full name, else Name.
	Name, RealName string
	Home           string
	// Avatar is a readable avatar path, "" when none.
	Avatar string
}

// Current is the account the process runs as, from systemd's user
// database by uid, so an empty or spoofed $USER changes nothing and an
// account outside /etc/passwd (LDAP, systemd-homed) resolves too.
func Current() (Account, error) {
	return Lookup(context.Background(), userdb.SocketPath, userdb.UID(os.Getuid())) //nolint:gosec // a uid is never negative
}

// Lookup is uid's account, from the user database at socket.
func Lookup(ctx context.Context, socket string, uid userdb.UID) (Account, error) {
	u, err := userdb.LookupUID(ctx, socket, uid)
	if err != nil {
		return Account{}, fmt.Errorf("look up the account of uid %d: %w", uid, err)
	}
	return Account{Name: u.Name, RealName: cmp.Or(fullName(u.RealName), u.Name), Home: u.Home, Avatar: FindAvatar(u.Name, u.Home)}, nil
}

// FindAvatar is the first readable avatar for a login: lys's own, the
// one AccountsService left behind, then ~/.face.
func FindAvatar(name, home string) string {
	for _, candidate := range avatarCandidates(name, home) {
		if f, err := os.Open(candidate); err == nil { //nolint:gosec // avatar lookup
			_ = f.Close()
			return candidate
		}
	}
	return ""
}

var avatarCandidates = func(name, home string) []string {
	return []string{AvatarPath(name), filepath.Join(accountsServiceIcons, name), filepath.Join(home, ".face")}
}
