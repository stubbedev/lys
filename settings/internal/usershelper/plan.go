// Package usershelper is lys-users-helper: run as root by pkexec, it
// makes one change to the user accounts that lys-settings asked for,
// with the shadow tools and never a shell, and answers a refusal or a
// failure with an accounts.Error.
package usershelper

import (
	"github.com/stubbedev/lys/settings/internal/accounts"
)

type tool string

const (
	useradd  tool = "useradd"
	usermod  tool = "usermod"
	userdel  tool = "userdel"
	chpasswd tool = "chpasswd"
	gpasswd  tool = "gpasswd"
)

type step interface{ isStep() }

type command struct {
	tool  tool
	args  []string
	stdin []byte
}

type writeAvatar struct {
	login   string
	picture accounts.Picture
}

type removeAvatar struct{ login string }

func (command) isStep()      {}
func (writeAvatar) isStep()  {}
func (removeAvatar) isStep() {}

type plan struct {
	steps    []step
	rollback []step
}

func passwordLine(login string, hash accounts.Hash) []byte {
	return []byte(login + ":" + string(hash) + "\n")
}

func setPassword(login string, hash accounts.Hash) command {
	return command{tool: chpasswd, args: []string{"-e"}, stdin: passwordLine(login, hash)}
}

func makePlan(db accounts.Database, caller accounts.Account, r accounts.Request) (plan, *accounts.Error) {
	switch r := r.(type) {
	case accounts.Rename:
		return plan{steps: []step{command{tool: usermod, args: []string{"-c", string(r.RealName), "--", caller.Name}}}}, nil
	case accounts.SetPicture:
		return plan{steps: []step{writeAvatar{login: caller.Name, picture: r.Picture}}}, nil
	case accounts.Create:
		return planCreate(db, r)
	case accounts.Edit:
		return planEdit(db, r)
	case accounts.Delete:
		return planDelete(db, caller, r)
	}
	return plan{}, accounts.Errorf(accounts.ErrInvalid, "no plan for a %T", r)
}

func planCreate(db accounts.Database, r accounts.Create) (plan, *accounts.Error) {
	login := string(r.Login)
	if _, ok := db.ByName(accounts.Name(r.Login)); ok {
		return plan{}, accounts.Errorf(accounts.ErrTaken, "%s is already an account", login)
	}
	args := []string{"-m", "-c", string(r.RealName)}
	if r.Kind == accounts.Admin {
		g, err := db.AdminGroup()
		if err != nil {
			return plan{}, accounts.Errorf(accounts.ErrNoAdmins, "make %s an administrator: %v", login, err)
		}
		args = append(args, "-G", g.Name)
	}
	return plan{
		steps:    []step{command{tool: useradd, args: append(args, "--", login)}, setPassword(login, r.Hash)},
		rollback: []step{command{tool: userdel, args: []string{"-r", "--", login}}, removeAvatar{login: login}},
	}, nil
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

func planEdit(db accounts.Database, r accounts.Edit) (plan, *accounts.Error) {
	a, aerr := person(db, r.Login)
	if aerr != nil {
		return plan{}, aerr
	}
	var steps []step
	if r.RealName != nil {
		steps = append(steps, command{tool: usermod, args: []string{"-c", string(*r.RealName), "--", a.Name}})
	}
	if r.Kind != nil && *r.Kind != a.Kind {
		g, err := db.AdminGroup()
		if err != nil {
			return plan{}, accounts.Errorf(accounts.ErrNoAdmins, "change what %s may do: %v", a.Name, err)
		}
		flag := "-d"
		if *r.Kind == accounts.Admin {
			flag = "-a"
		}
		steps = append(steps, command{tool: gpasswd, args: []string{flag, a.Name, g.Name}})
	}
	if r.Hash != nil {
		steps = append(steps, setPassword(a.Name, *r.Hash))
	}
	return plan{steps: steps}, nil
}

func planDelete(db accounts.Database, caller accounts.Account, r accounts.Delete) (plan, *accounts.Error) {
	a, aerr := person(db, r.Login)
	if aerr != nil {
		return plan{}, aerr
	}
	if a.UID == caller.UID {
		return plan{}, accounts.Errorf(accounts.ErrOwnAccount, "%s is the account asking", a.Name)
	}
	args := []string{"--", a.Name}
	if r.RemoveHome {
		args = append([]string{"-r"}, args...)
	}
	return plan{steps: []step{command{tool: userdel, args: args}, removeAvatar{login: a.Name}}}, nil
}
