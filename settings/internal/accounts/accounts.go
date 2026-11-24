// Package accounts is the user accounts as lys-settings reads them
// without privilege and changes them through lys-users-helper: the
// passwd and group databases, the validated values a change is made of,
// and the requests the helper takes from pkexec.
package accounts

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/useraccount"
)

// Kind is what an account may do: a standard account, or an
// administrator, who is in the admin group.
type Kind uint8

// The kinds of account.
const (
	Standard Kind = iota
	Admin
)

// Kinds is every Kind, in the order they are offered.
var Kinds = []Kind{Standard, Admin}

func (k Kind) String() string {
	switch k {
	case Standard:
		return "standard"
	case Admin:
		return "admin"
	}
	return "kind " + strconv.Itoa(int(k))
}

// MarshalText writes k as its name.
func (k Kind) MarshalText() ([]byte, error) {
	if !slices.Contains(Kinds, k) {
		return nil, fmt.Errorf("accounts: %s", k)
	}
	return []byte(k.String()), nil
}

// UnmarshalText reads a Kind's name, refusing any other text.
func (k *Kind) UnmarshalText(text []byte) error {
	for _, c := range Kinds {
		if c.String() == string(text) {
			*k = c
			return nil
		}
	}
	return fmt.Errorf("accounts: no account kind %q", text)
}

// AdminGroups are the groups that make an administrator, the first the
// group database holds being the one: wheel where sudo is configured
// through it, sudo on Debian and Ubuntu.
var AdminGroups = []string{"wheel", "sudo"}

// ErrNoAdminGroup is the group database holding none of AdminGroups.
var ErrNoAdminGroup = errors.New("no administrators' group: the group database holds neither wheel nor sudo")

// Group is one entry of the group database.
type Group struct {
	Name    string
	GID     uint32
	Members []string
}

// ParseGroups is the well-formed entries of group text, in its order.
func ParseGroups(text string) []Group {
	var out []Group
	for line := range strings.SplitSeq(text, "\n") {
		f := strings.Split(line, ":")
		if len(f) != 4 || f[0] == "" {
			continue
		}
		gid, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil {
			continue
		}
		var members []string
		if f[3] != "" {
			members = strings.Split(f[3], ",")
		}
		out = append(out, Group{Name: f[0], GID: uint32(gid), Members: members})
	}
	return out
}

// Has reports whether e is in g, as a member or by its primary group.
func (g Group) Has(e useraccount.Entry) bool {
	return e.GID == g.GID || slices.Contains(g.Members, e.Name)
}

// Account is a passwd entry with its kind.
type Account struct {
	useraccount.Entry
	Kind Kind
}

// Ref is a's name as a request names it.
func (a Account) Ref() (Name, error) { return ParseName(a.Name) }

// Database is the accounts as the passwd and group databases and
// login.defs give them.
type Database struct {
	entries []useraccount.Entry
	admin   Group
	adminOK bool
	people  useraccount.UIDRange
}

// ParseDatabase reads the accounts from the text of the passwd and the
// group databases, people being those r and a login shell make so.
func ParseDatabase(passwd, group string, r useraccount.UIDRange) Database {
	db := Database{entries: useraccount.ParsePasswd(passwd), people: r}
	groups := ParseGroups(group)
	for _, name := range AdminGroups {
		if i := slices.IndexFunc(groups, func(g Group) bool { return g.Name == name }); i >= 0 {
			db.admin, db.adminOK = groups[i], true
			break
		}
	}
	return db
}

// ReadDatabase reads the accounts from the system's databases.
func ReadDatabase() (Database, error) {
	passwd, err := os.ReadFile(useraccount.PasswdPath)
	if err != nil {
		return Database{}, fmt.Errorf("read the accounts: %w", err)
	}
	group, err := os.ReadFile(useraccount.GroupPath)
	if err != nil {
		return Database{}, fmt.Errorf("read the groups: %w", err)
	}
	return ParseDatabase(string(passwd), string(group), useraccount.ReadUIDRange(useraccount.LoginDefsPath)), nil
}

// AdminGroup is the group that makes an administrator.
func (db Database) AdminGroup() (Group, error) {
	if !db.adminOK {
		return Group{}, ErrNoAdminGroup
	}
	return db.admin, nil
}

func (db Database) account(e useraccount.Entry) Account {
	a := Account{Entry: e, Kind: Standard}
	if db.adminOK && db.admin.Has(e) {
		a.Kind = Admin
	}
	return a
}

// ByUID is the account of uid.
func (db Database) ByUID(uid uint32) (Account, bool) {
	i := slices.IndexFunc(db.entries, func(e useraccount.Entry) bool { return e.UID == uid })
	if i < 0 {
		return Account{}, false
	}
	return db.account(db.entries[i]), true
}

// ByName is the account called name.
func (db Database) ByName(name Name) (Account, bool) {
	i := slices.IndexFunc(db.entries, func(e useraccount.Entry) bool { return e.Name == string(name) })
	if i < 0 {
		return Account{}, false
	}
	return db.account(db.entries[i]), true
}

// IsPerson reports whether a is a person's account, not a system one.
func (db Database) IsPerson(a Account) bool { return a.IsPerson(db.people) }

// People is the people's accounts, by login name.
func (db Database) People() []Account {
	var out []Account
	for _, e := range db.entries {
		if e.IsPerson(db.people) {
			out = append(out, db.account(e))
		}
	}
	slices.SortFunc(out, func(x, y Account) int { return cmp.Compare(x.Name, y.Name) })
	return out
}

// Logins is every login name the passwd database holds.
func (db Database) Logins() []string {
	out := make([]string, len(db.entries))
	for i, e := range db.entries {
		out[i] = e.Name
	}
	return out
}
