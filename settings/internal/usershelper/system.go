package usershelper

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/userdb"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/pwfiles"
)

// lockWait is how long a request waits for another program's hold on
// the account files, as long as lckpwdf(3) does.
const lockWait = 15 * time.Second

// directory is the accounts systemd's user database knows beyond the
// files: homed, LDAP and every other source, which a new account's name
// and ID must not collide with.
type directory interface {
	UserByName(ctx context.Context, name string) (userdb.User, error)
	UserByUID(ctx context.Context, uid userdb.UID) (userdb.User, error)
	GroupByName(ctx context.Context, name string) (userdb.Group, error)
	Close() error
}

func dialUserDB(ctx context.Context) (directory, error) { return userdb.Dial(ctx, userdb.SocketPath) }

// system is the machine the service changes: its /etc, the avatars the
// greeter reads, /proc, what NixOS declares, the user database and the
// clock.
type system struct {
	etc, avatars, proc string
	nixos              *nixos
	directory          func(context.Context) (directory, error)
	now                func() time.Time
}

func liveSystem(n *nixos) system {
	return system{etc: "/etc", avatars: useraccount.AvatarsDir, proc: "/proc", nixos: n, directory: dialUserDB, now: time.Now}
}

func failed(err error) *accounts.Error {
	if e, ok := errors.AsType[*accounts.Error](err); ok {
		return e
	}
	kind := accounts.ErrFailed
	switch {
	case errors.Is(err, pwfiles.ErrLocked):
		kind = accounts.ErrBusy
	case errors.Is(err, pwfiles.ErrTaken):
		kind = accounts.ErrTaken
	case errors.Is(err, pwfiles.ErrExhausted):
		kind = accounts.ErrExhausted
	}
	return accounts.Errorf(kind, "%v", err)
}

func found[T any](_ T, err error) (bool, error) {
	if errors.Is(err, userdb.NoRecordFound) {
		return false, nil
	}
	return err == nil, err
}

// change makes r for the caller of uid, whom polkit authorised for it.
func (s system) change(ctx context.Context, uid uint32, r accounts.Request) *accounts.Error {
	lockCtx, cancel := context.WithTimeout(ctx, lockWait)
	lock, err := pwfiles.TakeLock(lockCtx, s.etc)
	cancel()
	if err != nil {
		return failed(err)
	}
	defer func() { _ = lock.Unlock() }()
	files, err := pwfiles.Load(s.etc)
	if err != nil {
		return failed(err)
	}
	defs, err := readDefaults(s.etc)
	if err != nil {
		return failed(err)
	}
	db := accounts.ParseDatabase(files.Text(pwfiles.Passwd), files.Text(pwfiles.Group), useraccount.UIDRange{Min: defs.uids.Min, Max: defs.uids.Max})
	caller, ok := db.ByUID(uid)
	if !ok {
		return accounts.Errorf(accounts.ErrUnknownUser, "the caller, uid %d, is in no passwd entry", uid)
	}
	switch r := r.(type) {
	case accounts.SetPicture:
		return s.writeAvatar(caller.Name, r.Picture)
	case accounts.Rename:
		if aerr := s.nixos.refuse(caller, caller, r); aerr != nil {
			return aerr
		}
		if err := files.SetGECOS(caller.Name, string(r.RealName)); err != nil {
			return failed(err)
		}
		return commit(files)
	case accounts.Create:
		return s.create(ctx, files, db, defs, caller, r)
	case accounts.Edit:
		return s.edit(files, db, caller, r)
	case accounts.Delete:
		return s.delete(files, db, caller, r)
	}
	return accounts.Errorf(accounts.ErrInvalid, "no way to make a %T", r)
}

func commit(files *pwfiles.Files) *accounts.Error {
	if err := files.Commit(); err != nil {
		return failed(err)
	}
	return nil
}

func (s system) day() int64 { return s.now().UTC().Unix() / int64(24*time.Hour/time.Second) }

func (s system) create(ctx context.Context, files *pwfiles.Files, db accounts.Database, defs defaults, caller accounts.Account, r accounts.Create) *accounts.Error {
	login := string(r.Login)
	if aerr := s.nixos.refuse(caller, accounts.Account{}, r); aerr != nil {
		return aerr
	}
	if files.HasUser(login) {
		return accounts.Errorf(accounts.ErrTaken, "%s is already an account", login)
	}
	var admins accounts.Group
	if r.Kind == accounts.Admin {
		g, err := db.AdminGroup()
		if err != nil {
			return accounts.Errorf(accounts.ErrNoAdmins, "make %s an administrator: %v", login, err)
		}
		admins = g
	}
	dir, err := s.directory(ctx)
	if err != nil {
		return failed(fmt.Errorf("ask the user database whether %s is free: %w", login, err))
	}
	defer func() { _ = dir.Close() }()
	if taken, err := found(dir.UserByName(ctx, login)); err != nil || taken {
		return takenOrFailed(err, "the user database holds an account %s", login)
	}
	uid, err := files.FreeUID(defs.uids, func(id uint32) (bool, error) { return found(dir.UserByUID(ctx, userdb.UID(id))) })
	if err != nil {
		return failed(err)
	}
	gid, aerr := s.primaryGroup(ctx, files, dir, defs, login, uid)
	if aerr != nil {
		return aerr
	}
	home := filepath.Join(defs.homeBase, login)
	if err := files.AddUser(pwfiles.NewUser{
		Name: login, UID: uid, GID: gid, GECOS: string(r.RealName), Home: home, Shell: defs.shell,
		Hash: string(r.Hash), LastChange: s.day(), Min: defs.passMin, Max: defs.passMax, Warn: defs.passWarn,
	}); err != nil {
		return failed(err)
	}
	if admins.Name != "" {
		if err := files.AddMember(admins.Name, login); err != nil {
			return failed(err)
		}
	}
	if err := makeHome(filepath.Join(s.etc, skelName), home, int(uid), int(gid), defs.homeMode); err != nil {
		return failed(err)
	}
	if aerr := commit(files); aerr != nil {
		if err := removeTree(home); err != nil {
			aerr.Detail += "; and the new home folder stays: " + err.Error()
		}
		return aerr
	}
	return nil
}

func takenOrFailed(err error, format string, args ...any) *accounts.Error {
	if err != nil {
		return failed(err)
	}
	return accounts.Errorf(accounts.ErrTaken, format, args...)
}

func (s system) primaryGroup(ctx context.Context, files *pwfiles.Files, dir directory, defs defaults, login string, uid uint32) (uint32, *accounts.Error) {
	if !defs.userGroups {
		if defs.group == "" {
			return 0, accounts.Errorf(accounts.ErrFailed, "%s sets no USERGROUPS_ENAB and %s no GROUP: nothing says which group %s is in", loginDefsName, useraddName, login)
		}
		if gid, err := strconv.ParseUint(defs.group, 10, 32); err == nil {
			return uint32(gid), nil
		}
		gid, err := files.GroupGID(defs.group)
		if err != nil {
			return 0, failed(fmt.Errorf("%s GROUP: %w", useraddName, err))
		}
		return gid, nil
	}
	if files.HasGroup(login) {
		return 0, accounts.Errorf(accounts.ErrTaken, "%s is already a group", login)
	}
	if taken, err := found(dir.GroupByName(ctx, login)); err != nil || taken {
		return 0, takenOrFailed(err, "the user database holds a group %s", login)
	}
	gid, err := files.FreeGID(defs.gids, uid, func(uint32) (bool, error) { return false, nil })
	if err != nil {
		return 0, failed(err)
	}
	if err := files.AddGroup(login, gid); err != nil {
		return 0, failed(err)
	}
	return gid, nil
}

func person(db accounts.Database, login accounts.Name) (accounts.Account, *accounts.Error) {
	a, ok := db.ByName(login)
	if !ok {
		return accounts.Account{}, accounts.Errorf(accounts.ErrUnknownUser, "%s is no account", login)
	}
	if !db.IsPerson(a) {
		return accounts.Account{}, accounts.Errorf(accounts.ErrNotAPerson, "%s is a system account", login)
	}
	return a, nil
}

func (s system) edit(files *pwfiles.Files, db accounts.Database, caller accounts.Account, r accounts.Edit) *accounts.Error {
	a, aerr := person(db, r.Login)
	if aerr != nil {
		return aerr
	}
	if aerr := s.nixos.refuse(caller, a, r); aerr != nil {
		return aerr
	}
	if r.RealName != nil {
		if err := files.SetGECOS(a.Name, string(*r.RealName)); err != nil {
			return failed(err)
		}
	}
	if r.Kind != nil && *r.Kind != a.Kind {
		g, err := db.AdminGroup()
		if err != nil {
			return accounts.Errorf(accounts.ErrNoAdmins, "change what %s may do: %v", a.Name, err)
		}
		change := files.RemoveMember
		if *r.Kind == accounts.Admin {
			change = files.AddMember
		}
		if err := change(g.Name, a.Name); err != nil {
			return failed(err)
		}
	}
	if r.Hash != nil {
		if err := files.SetPassword(a.Name, string(*r.Hash), s.day()); err != nil {
			return failed(err)
		}
	}
	return commit(files)
}

func (s system) delete(files *pwfiles.Files, db accounts.Database, caller accounts.Account, r accounts.Delete) *accounts.Error {
	a, aerr := person(db, r.Login)
	if aerr != nil {
		return aerr
	}
	if a.UID == caller.UID {
		return accounts.Errorf(accounts.ErrOwnAccount, "%s is the account asking", a.Name)
	}
	if aerr := s.nixos.refuse(caller, a, r); aerr != nil {
		return aerr
	}
	running, err := runsAs(s.proc, a.UID)
	if err != nil {
		return failed(err)
	}
	if running {
		return accounts.Errorf(accounts.ErrInUse, "a process runs as %s: log it out first", a.Name)
	}
	removeHome := false
	if r.RemoveHome {
		if removeHome, err = checkHome(a.Home, a.UID); err != nil {
			return failed(err)
		}
	}
	if err := files.DeleteUser(a.Name); err != nil {
		return failed(err)
	}
	if aerr := commit(files); aerr != nil {
		return aerr
	}
	var errs []error
	if removeHome {
		errs = append(errs, removeTree(a.Home))
	}
	errs = append(errs, s.removeAvatar(a.Name))
	if err := errors.Join(errs...); err != nil {
		return accounts.Errorf(accounts.ErrFailed, "%s is removed, but %v", a.Name, err)
	}
	return nil
}

func (s system) writeAvatar(login string, picture accounts.Picture) *accounts.Error {
	if err := os.MkdirAll(s.avatars, 0o755); err != nil { //nolint:gosec // the greeter reads the avatars
		return accounts.Errorf(accounts.ErrFailed, "create %s: %v", s.avatars, err)
	}
	path := filepath.Join(s.avatars, login)
	if err := atomicfile.WriteFile(path, picture, 0o644); err != nil {
		return accounts.Errorf(accounts.ErrFailed, "write %s: %v", path, err)
	}
	return nil
}

func (s system) removeAvatar(login string) error {
	path := filepath.Join(s.avatars, login)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
