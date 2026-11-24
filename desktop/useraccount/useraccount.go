// Package useraccount describes a user account the way the desktop
// shows it: login name, real name, home, and avatar.
package useraccount

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
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

// Current is the account the process runs as, from its passwd entry by
// uid, so an empty or spoofed $USER changes nothing. An account the
// passwd file does not list (userdb, LDAP) falls back to what the
// environment says, with a warning.
func Current() (Account, error) {
	u, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil {
		log.Printf("useraccount: uid %d is not in the passwd file (%v); using the environment's account", os.Getuid(), err)
		if u, err = user.Current(); err != nil {
			return Account{}, fmt.Errorf("look up the current user: %w", err)
		}
	}
	return Account{Name: u.Username, RealName: cmp.Or(fullName(u.Name), u.Username), Home: u.HomeDir, Avatar: FindAvatar(u.Username, u.HomeDir)}, nil
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
