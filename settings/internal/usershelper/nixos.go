package usershelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

// nixos is what the lys-settings NixOS module says of the system's
// users: whether users.mutableUsers holds, and the accounts
// users.users declares. NixOS rewrites passwd, shadow and group from
// the configuration on every activation: with mutableUsers false
// whole, and otherwise every field of a declared account but its
// password, so a change the configuration would undo is refused.
type nixos struct {
	// configured is false on a NixOS where the service runs without
	// the module's word, which refuses every change to the files.
	configured bool
	mutable    bool
	declared   map[string]bool
}

type nixosFile struct {
	MutableUsers *bool    `json:"mutableUsers"`
	Users        []string `json:"users"`
}

func parseNixOS(data []byte) (*nixos, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var f nixosFile
	if err := d.Decode(&f); err != nil {
		return nil, fmt.Errorf("the NixOS users file: %w", err)
	}
	if f.MutableUsers == nil {
		return nil, errors.New("the NixOS users file says nothing of mutableUsers")
	}
	n := &nixos{configured: true, mutable: *f.MutableUsers, declared: make(map[string]bool, len(f.Users))}
	for _, u := range f.Users {
		n.declared[u] = true
	}
	return n, nil
}

// readNixOS is the policy for the system whose /etc is etc: the file
// at path, which the NixOS module names; with none, nil off NixOS, and
// an unconfigured policy on it.
func readNixOS(etc, path string) (*nixos, error) {
	if path != "" {
		data, err := os.ReadFile(path) //nolint:gosec // the file the service's command line names
		if err != nil {
			return nil, fmt.Errorf("read the NixOS users file: %w", err)
		}
		return parseNixOS(data)
	}
	_, err := os.Stat(filepath.Join(etc, nixosName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tell whether this is NixOS: %w", err)
	}
	return &nixos{}, nil
}

func (n *nixos) declares(login string, field string) *accounts.Error {
	if !n.declared[login] {
		return nil
	}
	return accounts.Errorf(accounts.ErrDeclarative, "the NixOS configuration declares %s, and sets its %s on every activation: change users.users.%s there", login, field, login)
}

// refuse is the refusal of r, made by caller about target (the caller
// for a request of its own, nobody yet for a new account), that the
// configuration would undo; nil where it would not, and always off
// NixOS.
func (n *nixos) refuse(caller, target accounts.Account, r accounts.Request) *accounts.Error {
	if n == nil {
		return nil
	}
	if _, ok := r.(accounts.SetPicture); ok {
		return nil
	}
	if !n.configured {
		return accounts.Errorf(accounts.ErrDeclarative, "this is NixOS, and lys-users runs without the lys-settings NixOS module, which tells it what the configuration declares: enable programs.lys-settings")
	}
	if !n.mutable {
		return accounts.Errorf(accounts.ErrDeclarative, "users.mutableUsers is false: the NixOS configuration declares every account, so change it there")
	}
	switch r := r.(type) {
	case accounts.Rename:
		return n.declares(caller.Name, "full name")
	case accounts.Create:
		return n.declares(string(r.Login), "account")
	case accounts.Edit:
		if r.RealName != nil {
			return n.declares(string(r.Login), "full name")
		}
		if r.Kind != nil && *r.Kind != target.Kind {
			return n.declares(string(r.Login), "groups")
		}
	case accounts.Delete:
		return n.declares(string(r.Login), "account")
	}
	return nil
}
