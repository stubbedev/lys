package useraccount

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"
	"strings"
)

// The databases the accounts are read from.
const (
	PasswdPath    = "/etc/passwd"
	GroupPath     = "/etc/group"
	LoginDefsPath = "/etc/login.defs"
)

// UIDRange is the inclusive range of user IDs login.defs gives people's
// accounts, as UID_MIN and UID_MAX.
type UIDRange struct{ Min, Max uint32 }

// DefaultUIDRange is shadow-utils' range where login.defs says nothing.
var DefaultUIDRange = UIDRange{Min: 1000, Max: 60000}

// Contains reports whether uid is a person's account by r.
func (r UIDRange) Contains(uid uint32) bool { return uid >= r.Min && uid <= r.Max }

// ParseUIDRange reads UID_MIN and UID_MAX from login.defs text, each
// DefaultUIDRange's where unset. A malformed value or a range whose
// minimum lies above its maximum is an error.
func ParseUIDRange(text string) (UIDRange, error) {
	r := DefaultUIDRange
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 || strings.HasPrefix(f[0], "#") {
			continue
		}
		var into *uint32
		switch f[0] {
		case "UID_MIN":
			into = &r.Min
		case "UID_MAX":
			into = &r.Max
		default:
			continue
		}
		v, err := strconv.ParseUint(f[1], 0, 32)
		if err != nil {
			return DefaultUIDRange, fmt.Errorf("%s %q: %w", f[0], f[1], err)
		}
		*into = uint32(v)
	}
	if err := sc.Err(); err != nil {
		return DefaultUIDRange, err
	}
	if r.Min > r.Max {
		return DefaultUIDRange, fmt.Errorf("UID_MIN %d lies above UID_MAX %d", r.Min, r.Max)
	}
	return r, nil
}

// ReadUIDRange is the range login.defs at path gives, DefaultUIDRange
// where it is missing; an unreadable or malformed file is logged and
// gives DefaultUIDRange too.
func ReadUIDRange(path string) UIDRange {
	data, err := os.ReadFile(path) //nolint:gosec // login.defs, readable by all
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultUIDRange
	}
	if err != nil {
		log.Printf("useraccount: the user ID range is shadow-utils' default: %v", err)
		return DefaultUIDRange
	}
	r, err := ParseUIDRange(string(data))
	if err != nil {
		log.Printf("useraccount: the user ID range is shadow-utils' default: %s: %v", path, err)
	}
	return r
}

// Entry is one well-formed line of the passwd database.
type Entry struct {
	Name     string
	UID, GID uint32
	// FullName is the GECOS full name, "" where it holds none.
	FullName string
	Home     string
	Shell    string
}

// DisplayName is e's full name, else its login name.
func (e Entry) DisplayName() string { return cmp.Or(e.FullName, e.Name) }

// ParsePasswd is the well-formed entries of passwd text, in its order.
func ParsePasswd(text string) []Entry {
	var out []Entry
	for line := range strings.SplitSeq(text, "\n") {
		f := strings.Split(line, ":")
		if len(f) != 7 || f[0] == "" {
			continue
		}
		uid, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil {
			continue
		}
		gid, err := strconv.ParseUint(f[3], 10, 32)
		if err != nil {
			continue
		}
		out = append(out, Entry{Name: f[0], UID: uint32(uid), GID: uint32(gid), FullName: fullName(f[4]), Home: f[5], Shell: f[6]})
	}
	return out
}

// IsPerson reports whether e is a person's account by r: a user ID in r
// and a shell that allows an interactive login.
func (e Entry) IsPerson(r UIDRange) bool {
	return r.Contains(e.UID) && e.Shell != "" && !strings.HasSuffix(e.Shell, "nologin") &&
		!strings.HasSuffix(e.Shell, "/false") && e.Shell != "/bin/sync"
}

// People is the entries of passwd text that are people's accounts by r.
func People(passwd string, r UIDRange) []Entry {
	var out []Entry
	for _, e := range ParsePasswd(passwd) {
		if e.IsPerson(r) {
			out = append(out, e)
		}
	}
	return out
}

func fullName(gecos string) string {
	full, _, _ := strings.Cut(gecos, ",")
	return strings.TrimSpace(full)
}
