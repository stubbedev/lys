package filebrowser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/netfs"

	"github.com/stubbedev/lys/shell/internal/statefile"
)

// Keyring keeps passwords in the user's keyring through the Secret
// Service (secrets/client.Keychain).
type Keyring interface {
	Lookup(ctx context.Context, attrs map[string]string) (value []byte, found bool, err error)
	Store(ctx context.Context, label string, attrs map[string]string, value []byte) error
}

// Remember is how long a password typed to connect is kept.
type Remember uint8

// The ways a password is remembered.
const (
	// RememberNever forgets it once connected.
	RememberNever Remember = iota
	// RememberSession keeps it in memory until the process ends.
	RememberSession
	// RememberKeyring stores it in the keyring.
	RememberKeyring
)

// Dialer connects to the server of a location (netfs.Connect with its
// options bound).
type Dialer func(ctx context.Context, loc vfs.Location, cred vfs.Credentials, trust *vfs.Trust) (vfs.Conn, error)

// Network is how browsers connect to network locations, shared by every
// browse of a process: the dialer, what it trusts, where passwords are
// kept, and the servers connected to lately. It is safe for concurrent
// use.
type Network struct {
	dial    Dialer
	trust   *vfs.Trust
	keyring Keyring
	recent  *statefile.File
	mu      sync.Mutex
	session map[vfs.Server]vfs.Credentials
}

// maxRecentServers bounds the servers the dialog offers.
const maxRecentServers = 10

// NewNetwork connects through dial trusting trust, keeps passwords in
// keyring (nil offers no keyring) and the servers connected to in
// recentFile ("" keeps them in memory alone).
func NewNetwork(dial Dialer, trust *vfs.Trust, keyring Keyring, recentFile string) *Network {
	return &Network{dial: dial, trust: trust, keyring: keyring, recent: &statefile.File{Path: recentFile}, session: map[vfs.Server]vfs.Credentials{}}
}

// NewSystemNetwork connects through lys's own clients (netfs), trying
// the SSH agent at agentSocket ("" for none) first and trusting the
// hosts in home's ~/.ssh/known_hosts, keeping passwords in keyring (nil
// offers none) and the servers connected to in recentFile ("" keeps
// them in memory alone).
func NewSystemNetwork(home, agentSocket string, keyring Keyring, recentFile string) *Network {
	dial := func(ctx context.Context, loc vfs.Location, cred vfs.Credentials, trust *vfs.Trust) (vfs.Conn, error) {
		return netfs.Connect(ctx, loc, cred, trust, netfs.Options{AgentSocket: agentSocket})
	}
	return NewNetwork(dial, &vfs.Trust{KnownHosts: filepath.Join(home, ".ssh", "known_hosts")}, keyring, recentFile)
}

// keyringAttrs are the attributes a server's password is stored under.
func keyringAttrs(srv vfs.Server, cred vfs.Credentials) map[string]string {
	srv.User, srv.Domain = cmpOr(cred.User, srv.User), cmpOr(cred.Domain, srv.Domain)
	return map[string]string{"xdg:schema": "dev.stubbe.lys.NetworkPassword", "server": srv.String()}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// keyringTimeout bounds a keyring lookup or store, which may wait on
// the keyring's unlock prompt.
const keyringTimeout = 2 * time.Minute

// remembered is what is kept for srv: the session's credentials, else
// the keyring's password for srv's user; found false for none. Safe off
// the loop.
func (n *Network) remembered(srv vfs.Server) (vfs.Credentials, bool) {
	n.mu.Lock()
	cred, ok := n.session[srv]
	n.mu.Unlock()
	if ok {
		return cred, true
	}
	if n.keyring == nil || srv.User == "" {
		return vfs.Credentials{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyringTimeout)
	defer cancel()
	pw, found, err := n.keyring.Lookup(ctx, keyringAttrs(srv, vfs.Credentials{}))
	if err != nil || !found {
		return vfs.Credentials{}, false
	}
	return vfs.Credentials{User: srv.User, Domain: srv.Domain, Password: string(pw)}, true
}

// remember keeps cred for srv as how says; safe off the loop.
func (n *Network) remember(srv vfs.Server, cred vfs.Credentials, how Remember) error {
	switch how {
	case RememberSession:
		n.mu.Lock()
		n.session[srv] = cred
		n.mu.Unlock()
	case RememberKeyring:
		if n.keyring == nil {
			return errors.New("remember the password: no keyring")
		}
		ctx, cancel := context.WithTimeout(context.Background(), keyringTimeout)
		defer cancel()
		label := "Network password for " + srv.String()
		if err := n.keyring.Store(ctx, label, keyringAttrs(srv, cred), []byte(cred.Password)); err != nil {
			return fmt.Errorf("remember the password for %s: %w", srv, err)
		}
	}
	return nil
}

// recentServers are the locations connected to lately, newest first;
// none without a file. Safe off the loop.
func (n *Network) recentServers() []string {
	if n.recent.Path == "" {
		return nil
	}
	data, err := os.ReadFile(n.recent.Path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("file browser: the recent servers: %v", err)
		}
		return nil
	}
	return parseRecent(string(data))
}

// parseRecent reads the recent servers file, a location a line, the
// lines no location parses from left out.
func parseRecent(data string) []string {
	var locs []string
	for line := range strings.SplitSeq(data, "\n") {
		if l, err := vfs.Parse(strings.TrimSpace(line)); err == nil && !l.IsLocal() && !slices.Contains(locs, l.String()) {
			locs = append(locs, l.String())
		}
	}
	return locs
}

// addRecent puts loc first among recent, at most maxRecentServers, and
// keeps them.
func (n *Network) addRecent(recent []string, loc string) []string {
	next := append([]string{loc}, slices.DeleteFunc(slices.Clone(recent), func(r string) bool { return r == loc })...)
	if len(next) > maxRecentServers {
		next = next[:maxRecentServers]
	}
	n.recent.Write([]byte(strings.Join(next, "\n") + "\n"))
	return next
}

// Close waits for the recent servers' last write.
func (n *Network) Close() { n.recent.Wait() }
