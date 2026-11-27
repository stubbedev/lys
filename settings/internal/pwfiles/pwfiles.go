// Package pwfiles edits the account files, passwd, shadow, group,
// gshadow, subuid and subgid, as the shadow tools do: under /etc/.pwd.lock, the lock
// lckpwdf(3) takes, every line kept as it was but the ones an edit
// changes, and each changed file replaced whole with its owner, mode
// and SELinux label. The edits are pure, over a DB in memory; Load and
// Commit are the files.
package pwfiles

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/intconv"
)

// File is one of the account files.
type File uint8

// The account files, in the order Commit writes them: a new account's
// shadow entry, subordinate IDs and group before the passwd entry that
// makes it one.
const (
	Shadow File = iota
	Gshadow
	Subuid
	Subgid
	Group
	Passwd
	numFiles
)

var specs = [numFiles]struct {
	name     string
	fields   int
	optional bool
	// ranges is a file of subordinate ID ranges, whose second and
	// third fields are a start and a count.
	ranges bool
}{
	Shadow:  {name: "shadow", fields: 9},
	Gshadow: {name: "gshadow", fields: 4, optional: true},
	Subuid:  {name: "subuid", fields: 3, optional: true, ranges: true},
	Subgid:  {name: "subgid", fields: 3, optional: true, ranges: true},
	Group:   {name: "group", fields: 4},
	Passwd:  {name: "passwd", fields: 7},
}

// Name is the file's name in /etc.
func (f File) Name() string { return specs[f].name }

// The fields lys edits.
const (
	pwName, pwPassword, idField, pwGID, pwGECOS, pwHome, pwShell = 0, 1, 2, 3, 4, 5, 6
	spName, spHash, spLastChange, spMin, spMax, spWarn           = 0, 1, 2, 3, 4, 5
	grName, grPassword, grMembers                                = 0, 1, 3
	gsName, gsPassword, gsAdmins, gsMembers                      = 0, 1, 2, 3
	subOwner, subStart, subCount                                 = 0, 1, 2
)

// The defined failures of an edit or a load.
var (
	ErrMalformed = errors.New("a line that is not one entry of the file")
	ErrTaken     = errors.New("already taken")
	ErrNoEntry   = errors.New("no such entry")
	ErrExhausted = errors.New("no ID left in the range")
	ErrMember    = errors.New("membership")
)

type table struct {
	present bool
	// lines are the file's lines split into fields, nil for a blank one.
	lines   [][]string
	changed bool
}

// DB is the account files' entries in memory.
type DB struct{ tables [numFiles]table }

// parse reads the files' texts, a nil text being a file that does not
// exist, which only gshadow, subuid and subgid may be. A line with the
// wrong number of fields, or a range whose start or count is no ID, is
// ErrMalformed, naming the file and line, and nothing of it is guessed
// at.
func parse(texts [numFiles][]byte) (DB, error) {
	var db DB
	for f := range numFiles {
		if texts[f] == nil {
			if !specs[f].optional {
				return DB{}, fmt.Errorf("pwfiles: /etc/%s does not exist", f.Name())
			}
			continue
		}
		t, err := parseTable(f, string(texts[f]))
		if err != nil {
			return DB{}, err
		}
		db.tables[f] = t
	}
	return db, nil
}

func parseTable(f File, text string) (table, error) {
	t := table{present: true}
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return t, nil
	}
	for i, line := range strings.Split(text, "\n") {
		if line == "" {
			t.lines = append(t.lines, nil)
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != specs[f].fields || fields[0] == "" {
			return table{}, fmt.Errorf("pwfiles: /etc/%s line %d has %d fields, want %d: %w", f.Name(), i+1, len(fields), specs[f].fields, ErrMalformed)
		}
		if specs[f].ranges {
			if _, _, err := parseRange(fields); err != nil {
				return table{}, fmt.Errorf("pwfiles: /etc/%s line %d: %w: %w", f.Name(), i+1, err, ErrMalformed)
			}
		}
		t.lines = append(t.lines, fields)
	}
	return t, nil
}

// Text is file f as it is written, "" for one that does not exist.
func (db DB) Text(f File) string {
	var b strings.Builder
	for _, fields := range db.tables[f].lines {
		b.WriteString(strings.Join(fields, ":"))
		b.WriteByte('\n')
	}
	return b.String()
}

// Exists reports whether f exists.
func (db DB) Exists(f File) bool { return db.tables[f].present }

// Changed reports whether an edit changed f.
func (db DB) Changed(f File) bool { return db.tables[f].changed }

func (db DB) find(f File, name string) int {
	return slices.IndexFunc(db.tables[f].lines, func(fields []string) bool { return fields != nil && fields[0] == name })
}

func (db *DB) append(f File, fields []string) {
	t := &db.tables[f]
	t.lines = append(t.lines, fields)
	t.changed = true
}

func (db *DB) remove(f File, name string) {
	t := &db.tables[f]
	if i := db.find(f, name); i >= 0 {
		t.lines = slices.Delete(t.lines, i, i+1)
		t.changed = true
	}
}

func (db *DB) removeAll(f File, match func([]string) bool) {
	t := &db.tables[f]
	kept := slices.DeleteFunc(t.lines, func(fields []string) bool { return fields != nil && match(fields) })
	if len(kept) != len(t.lines) {
		t.lines = kept
		t.changed = true
	}
}

func (db *DB) set(f File, i, field int, value string) {
	t := &db.tables[f]
	if t.lines[i][field] != value {
		t.lines[i][field] = value
		t.changed = true
	}
}

// HasUser reports whether passwd or shadow holds name.
func (db DB) HasUser(name string) bool {
	return db.find(Passwd, name) >= 0 || db.find(Shadow, name) >= 0
}

// HasGroup reports whether group or gshadow holds name.
func (db DB) HasGroup(name string) bool {
	return db.find(Group, name) >= 0 || db.find(Gshadow, name) >= 0
}

func (db DB) ids(f File) map[uint32]bool {
	used := map[uint32]bool{}
	for _, fields := range db.tables[f].lines {
		if fields == nil {
			continue
		}
		if id, err := strconv.ParseUint(fields[idField], 10, 32); err == nil {
			used[uint32(id)] = true
		}
	}
	return used
}

// GroupGID is the ID of group name.
func (db DB) GroupGID(name string) (uint32, error) {
	i := db.find(Group, name)
	if i < 0 {
		return 0, fmt.Errorf("pwfiles: group %s: %w", name, ErrNoEntry)
	}
	gid, err := strconv.ParseUint(db.tables[Group].lines[i][idField], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("pwfiles: group %s has the ID %q: %w", name, db.tables[Group].lines[i][idField], ErrMalformed)
	}
	return uint32(gid), nil
}

// Range is an inclusive range of IDs.
type Range struct{ Min, Max uint32 }

// FreeUID is the user ID a new account takes in r, as useradd picks
// it: one above the highest r holds, or once that is past r's end the
// lowest r leaves free, skipping any ID taken reports taken elsewhere.
func (db DB) FreeUID(r Range, taken func(uint32) (bool, error)) (uint32, error) {
	return freeID(db.ids(Passwd), r, taken)
}

// FreeGID is FreeUID for a new group, preferring prefer, a new
// account's user ID, while it is free.
func (db DB) FreeGID(r Range, prefer uint32, taken func(uint32) (bool, error)) (uint32, error) {
	used := db.ids(Group)
	if !used[prefer] {
		t, err := taken(prefer)
		if err != nil || !t {
			return prefer, err
		}
	}
	return freeID(used, r, taken)
}

func freeID(used map[uint32]bool, r Range, taken func(uint32) (bool, error)) (uint32, error) {
	if r.Min > r.Max {
		return 0, fmt.Errorf("pwfiles: the range %d-%d is empty: %w", r.Min, r.Max, ErrExhausted)
	}
	highest, found := r.Min, false
	for id := range used {
		if id >= r.Min && id <= r.Max && (!found || id > highest) {
			highest, found = id, true
		}
	}
	try := func(id uint32) (bool, error) {
		if used[id] {
			return false, nil
		}
		t, err := taken(id)
		return err == nil && !t, err
	}
	scan := func(lo, hi uint32) (uint32, bool, error) {
		for id := lo; ; id++ {
			if ok, err := try(id); err != nil || ok {
				return id, ok, err
			}
			if id == hi {
				return 0, false, nil
			}
		}
	}
	start := r.Min
	if found && highest < r.Max {
		start = highest + 1
	} else if found {
		start = highest
	}
	if id, ok, err := scan(start, r.Max); err != nil || ok {
		return id, err
	}
	if start > r.Min {
		if id, ok, err := scan(r.Min, start-1); err != nil || ok {
			return id, err
		}
	}
	return 0, fmt.Errorf("pwfiles: every ID in %d-%d is taken: %w", r.Min, r.Max, ErrExhausted)
}

// NewUser is an account AddUser adds. The empty Min, Max and Warn are
// no limit, as useradd writes them.
type NewUser struct {
	Name, GECOS, Home, Shell string
	UID, GID                 uint32
	Hash                     string
	LastChange               int64
	Min, Max, Warn           string
}

// AddUser adds u to passwd and shadow; a name either holds is ErrTaken.
func (db *DB) AddUser(u NewUser) error {
	if db.HasUser(u.Name) {
		return fmt.Errorf("pwfiles: user %s: %w", u.Name, ErrTaken)
	}
	if db.ids(Passwd)[u.UID] {
		return fmt.Errorf("pwfiles: user ID %d: %w", u.UID, ErrTaken)
	}
	db.append(Shadow, []string{u.Name, u.Hash, strconv.FormatInt(u.LastChange, 10), u.Min, u.Max, u.Warn, "", "", ""})
	db.append(Passwd, []string{u.Name, "x", strconv.FormatUint(uint64(u.UID), 10), strconv.FormatUint(uint64(u.GID), 10), u.GECOS, u.Home, u.Shell})
	return nil
}

// AddGroup adds the group name with gid and no members to group, and to
// gshadow where it exists; a name or ID either holds is ErrTaken.
func (db *DB) AddGroup(name string, gid uint32) error {
	if db.HasGroup(name) {
		return fmt.Errorf("pwfiles: group %s: %w", name, ErrTaken)
	}
	if db.ids(Group)[gid] {
		return fmt.Errorf("pwfiles: group ID %d: %w", gid, ErrTaken)
	}
	if db.tables[Gshadow].present {
		db.append(Gshadow, []string{name, "!", "", ""})
	}
	db.append(Group, []string{name, "x", strconv.FormatUint(uint64(gid), 10), ""})
	return nil
}

func (db DB) user(f File, name string) (int, error) {
	i := db.find(f, name)
	if i < 0 {
		return 0, fmt.Errorf("pwfiles: user %s in /etc/%s: %w", name, f.Name(), ErrNoEntry)
	}
	return i, nil
}

// SetGECOS sets the GECOS field of user name.
func (db *DB) SetGECOS(name, gecos string) error {
	i, err := db.user(Passwd, name)
	if err != nil {
		return err
	}
	db.set(Passwd, i, pwGECOS, gecos)
	return nil
}

// SetPassword sets user name's shadow hash and the day it changed, and
// points its passwd entry at shadow.
func (db *DB) SetPassword(name, hash string, day int64) error {
	p, err := db.user(Passwd, name)
	if err != nil {
		return err
	}
	s, err := db.user(Shadow, name)
	if err != nil {
		return err
	}
	db.set(Shadow, s, spHash, hash)
	db.set(Shadow, s, spLastChange, strconv.FormatInt(day, 10))
	db.set(Passwd, p, pwPassword, "x")
	return nil
}

func splitList(field string) []string {
	if field == "" {
		return nil
	}
	return strings.Split(field, ",")
}

func (db *DB) editList(f File, i, field int, edit func([]string) []string) {
	db.set(f, i, field, strings.Join(edit(splitList(db.tables[f].lines[i][field])), ","))
}

// AddMember adds user to group's members, in group and in gshadow where
// it holds the group; a member already is ErrMember.
func (db *DB) AddMember(group, user string) error {
	return db.membership(group, user, true)
}

// RemoveMember takes user out of group's members; one that is not a
// member is ErrMember.
func (db *DB) RemoveMember(group, user string) error {
	return db.membership(group, user, false)
}

func (db *DB) membership(group, user string, add bool) error {
	g := db.find(Group, group)
	if g < 0 {
		return fmt.Errorf("pwfiles: group %s: %w", group, ErrNoEntry)
	}
	if slices.Contains(splitList(db.tables[Group].lines[g][grMembers]), user) == add {
		state := "is no member"
		if add {
			state = "is a member already"
		}
		return fmt.Errorf("pwfiles: %s %s of %s: %w", user, state, group, ErrMember)
	}
	edit := func(members []string) []string {
		if add {
			return append(members, user)
		}
		return slices.DeleteFunc(members, func(m string) bool { return m == user })
	}
	db.editList(Group, g, grMembers, edit)
	if s := db.find(Gshadow, group); s >= 0 {
		db.editList(Gshadow, s, gsMembers, func(members []string) []string {
			members = slices.DeleteFunc(members, func(m string) bool { return m == user })
			if add {
				members = append(members, user)
			}
			return members
		})
	}
	return nil
}

// DeleteUser removes user name from passwd and shadow, its ranges from
// subuid and subgid, it from every group's members and gshadow's
// administrators, and removes its own
// group, the one of its name that is its primary group, where no other
// account has that group as its primary one.
func (db *DB) DeleteUser(name string) error {
	p, err := db.user(Passwd, name)
	if err != nil {
		return err
	}
	gid := db.tables[Passwd].lines[p][pwGID]
	uid := db.tables[Passwd].lines[p][idField]
	db.remove(Passwd, name)
	db.remove(Shadow, name)
	for _, s := range []SubIDs{SubUIDs, SubGIDs} {
		db.removeAll(s.File(), func(fields []string) bool { return ownsRange(fields, name, uid) })
	}
	drop := func(members []string) []string {
		return slices.DeleteFunc(members, func(m string) bool { return m == name })
	}
	for i, fields := range db.tables[Group].lines {
		if fields != nil {
			db.editList(Group, i, grMembers, drop)
		}
	}
	for i, fields := range db.tables[Gshadow].lines {
		if fields != nil {
			db.editList(Gshadow, i, gsAdmins, drop)
			db.editList(Gshadow, i, gsMembers, drop)
		}
	}
	g := db.find(Group, name)
	if g < 0 || db.tables[Group].lines[g][idField] != gid {
		return nil
	}
	for _, fields := range db.tables[Passwd].lines {
		if fields != nil && fields[pwGID] == gid {
			return nil
		}
	}
	db.remove(Group, name)
	db.remove(Gshadow, name)
	return nil
}

// SubIDs is one kind of subordinate IDs: SubUIDs, in subuid, or
// SubGIDs, in subgid.
type SubIDs uint8

// The kinds of subordinate IDs.
const (
	SubUIDs SubIDs = iota
	SubGIDs
)

// File is the file that holds the kind's ranges.
func (s SubIDs) File() File { return [...]File{SubUIDs: Subuid, SubGIDs: Subgid}[s] }

func parseRange(fields []string) (start, count uint64, err error) {
	if start, err = strconv.ParseUint(fields[subStart], 10, 32); err != nil {
		return 0, 0, fmt.Errorf("the start %q is no ID", fields[subStart])
	}
	if count, err = strconv.ParseUint(fields[subCount], 10, 32); err != nil {
		return 0, 0, fmt.Errorf("the count %q is no count of IDs", fields[subCount])
	}
	return start, count, nil
}

func ownsRange(fields []string, name, uid string) bool {
	return fields[subOwner] == name || fields[subOwner] == uid
}

// HasSubIDs reports whether the account name of uid owns a range of s,
// one naming it or its user ID.
func (db DB) HasSubIDs(s SubIDs, name string, uid uint32) bool {
	id := strconv.FormatUint(uint64(uid), 10)
	return slices.ContainsFunc(db.tables[s.File()].lines, func(fields []string) bool { return fields != nil && ownsRange(fields, name, id) })
}

// FreeSubIDs is the start of the range of count IDs of s a new account
// takes in r, as useradd picks it: the lowest start in r from which
// count IDs overlap no range the file holds. A count r cannot hold is
// an error, and no such start ErrExhausted.
func (db DB) FreeSubIDs(s SubIDs, r Range, count uint32) (uint32, error) {
	lo, hi, n := uint64(r.Min), uint64(r.Max), uint64(count)
	if n == 0 || hi < lo || n > hi-lo+1 {
		return 0, fmt.Errorf("pwfiles: %d IDs of /etc/%s in %d-%d: no such range", count, s.File().Name(), r.Min, r.Max)
	}
	type span struct{ first, count uint64 }
	var held []span
	for _, fields := range db.tables[s.File()].lines {
		if fields == nil {
			continue
		}
		first, c, _ := parseRange(fields)
		if c > 0 {
			held = append(held, span{first, c})
		}
	}
	slices.SortFunc(held, func(a, b span) int { return cmp.Or(cmp.Compare(a.first, b.first), cmp.Compare(a.count, b.count)) })
	low := lo
	for _, h := range held {
		if end := min(h.first, hi+1); end >= low && end-low >= n {
			return intconv.Must[uint32](low), nil
		}
		low = max(low, h.first+h.count)
		if low > hi {
			break
		}
	}
	if low <= hi && hi-low+1 >= n {
		return intconv.Must[uint32](low), nil
	}
	return 0, fmt.Errorf("pwfiles: no %d free IDs of /etc/%s in %d-%d: %w", count, s.File().Name(), r.Min, r.Max, ErrExhausted)
}

// AddSubIDs gives the account name the range of count IDs of s from
// start; a file that does not exist is ErrNoEntry, as useradd adds
// ranges only to one that does.
func (db *DB) AddSubIDs(s SubIDs, name string, start, count uint32) error {
	f := s.File()
	if !db.tables[f].present {
		return fmt.Errorf("pwfiles: /etc/%s does not exist: %w", f.Name(), ErrNoEntry)
	}
	db.append(f, []string{name, strconv.FormatUint(uint64(start), 10), strconv.FormatUint(uint64(count), 10)})
	return nil
}
