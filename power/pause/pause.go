// Package pause is lys-power's pausing of chosen background units on
// battery: the units one may choose, what lys-power holds paused and
// what failed, and the system-bus contract that publishes that as the
// State property of dev.stubbe.lys.Pause1, with the methods that choose
// the units and resume them until the next unplug.
package pause

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Scope uint8

const (
	ScopeSystem Scope = iota + 1
	ScopeUser
)

func (s Scope) String() string {
	switch s {
	case ScopeSystem:
		return "system"
	case ScopeUser:
		return "user"
	}
	return fmt.Sprintf("Scope(%d)", uint8(s))
}

const (
	suffixService = ".service"
	suffixScope   = ".scope"
	suffixTimer   = ".timer"
	maxNameLen    = 255
)

var suffixes = []string{suffixService, suffixScope, suffixTimer}

var essential = []string{
	"accounts-daemon", "app-", "at-spi", "autovt@", "bluetooth", "dbus", "display-manager", "gdm",
	"getty@", "gnome-keyring", "greetd", "init.scope", "iwd", "lightdm", "lys-", "NetworkManager",
	"pipewire", "polkit", "power-profiles-daemon", "rtkit-daemon", "sddm", "serial-getty@",
	"session-", "systemd-", "udisks2", "upower", "user-runtime-dir@", "user@", "wireplumber",
	"wpa_supplicant", "xdg-",
}

var (
	ErrNotPausable = errors.New("not a unit lys-power pauses")
	ErrEssential   = errors.New("the session or the system needs it running")
)

func CheckName(name string) error {
	if name == "" || len(name) > maxNameLen {
		return fmt.Errorf("unit %q: %w: a unit name has 1 to %d bytes", name, ErrNotPausable, maxNameLen)
	}
	for _, r := range name {
		if !nameRune(r) {
			return fmt.Errorf("unit %q: %w: %q is not allowed in a unit name", name, ErrNotPausable, r)
		}
	}
	stem := ""
	for _, s := range suffixes {
		if before, ok := strings.CutSuffix(name, s); ok {
			stem = before
		}
	}
	if stem == "" {
		return fmt.Errorf("unit %q: %w: only services, scopes and timers are paused", name, ErrNotPausable)
	}
	for _, e := range essential {
		if strings.HasPrefix(name, e) {
			return fmt.Errorf("unit %q: %w", name, ErrEssential)
		}
	}
	return nil
}

func nameRune(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return true
	}
	return strings.ContainsRune(":_.-@\\", r)
}

type Unit struct {
	scope Scope
	uid   uint32
	name  string
}

func System(name string) (Unit, error) {
	if err := CheckName(name); err != nil {
		return Unit{}, err
	}
	return Unit{scope: ScopeSystem, name: name}, nil
}

func User(uid uint32, name string) (Unit, error) {
	if err := CheckName(name); err != nil {
		return Unit{}, err
	}
	return Unit{scope: ScopeUser, uid: uid, name: name}, nil
}

func (u Unit) Scope() Scope { return u.scope }

func (u Unit) UID() uint32 { return u.uid }

func (u Unit) Name() string { return u.name }

func (u Unit) Held() bool { return strings.HasSuffix(u.name, suffixTimer) }

func (u Unit) String() string {
	if u.scope == ScopeUser {
		return u.scope.String() + "/" + strconv.FormatUint(uint64(u.uid), 10) + "/" + u.name
	}
	return u.scope.String() + "/" + u.name
}

func Parse(text string) (Unit, error) {
	scope, rest, _ := strings.Cut(text, "/")
	switch scope {
	case ScopeSystem.String():
		return System(rest)
	case ScopeUser.String():
		id, name, ok := strings.Cut(rest, "/")
		uid, err := strconv.ParseUint(id, 10, 32)
		if !ok || err != nil {
			return Unit{}, fmt.Errorf("unit %q: want user/<uid>/<name>", text)
		}
		return User(uint32(uid), name)
	}
	return Unit{}, fmt.Errorf("unit %q: want system/<name> or user/<uid>/<name>", text)
}

func (u Unit) MarshalText() ([]byte, error) {
	if u.scope == 0 {
		return nil, errors.New("marshal a unit that was never made")
	}
	return []byte(u.String()), nil
}

func (u *Unit) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

func Compare(a, b Unit) int {
	return cmp.Or(cmp.Compare(a.scope, b.scope), cmp.Compare(a.uid, b.uid), cmp.Compare(a.name, b.name))
}

func Sorted(units []Unit) []Unit {
	out := slices.Clone(units)
	slices.SortFunc(out, Compare)
	return slices.CompactFunc(out, func(a, b Unit) bool { return a == b })
}

func (u Unit) VisibleTo(uid uint32) bool { return u.scope == ScopeSystem || u.uid == uid }

type Status uint8

const (
	StatusPaused Status = iota + 1
	StatusPauseFailed
	StatusResumeFailed
)

func (s Status) String() string {
	switch s {
	case StatusPaused:
		return "paused"
	case StatusPauseFailed:
		return "pause-failed"
	case StatusResumeFailed:
		return "resume-failed"
	}
	return fmt.Sprintf("Status(%d)", uint8(s))
}

func ParseStatus(wire string) (Status, error) {
	for _, s := range []Status{StatusPaused, StatusPauseFailed, StatusResumeFailed} {
		if s.String() == wire {
			return s, nil
		}
	}
	return 0, fmt.Errorf("status %q: want paused, pause-failed or resume-failed", wire)
}

type Entry struct {
	Unit   Unit
	Status Status
	Err    string
}

type State struct {
	Picks   []Unit
	Entries []Entry
	Resumed bool
}

func (s State) VisibleTo(uid uint32) State {
	out := State{Resumed: s.Resumed}
	for _, u := range s.Picks {
		if u.VisibleTo(uid) {
			out.Picks = append(out.Picks, u)
		}
	}
	for _, e := range s.Entries {
		if e.Unit.VisibleTo(uid) {
			out.Entries = append(out.Entries, e)
		}
	}
	return out
}

func (s State) Paused() []Unit {
	var out []Unit
	for _, e := range s.Entries {
		if e.Status != StatusPauseFailed {
			out = append(out, e.Unit)
		}
	}
	return out
}

func NewlyPaused(prev, cur State) []Unit {
	before := prev.Paused()
	var out []Unit
	for _, u := range cur.Paused() {
		if !slices.Contains(before, u) {
			out = append(out, u)
		}
	}
	return out
}
