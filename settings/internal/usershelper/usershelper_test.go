package usershelper

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/polkit"
	"github.com/stubbedev/lys/lib/polkit/polkittest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/userdb"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/pwfiles"
)

var (
	me    = uint32(os.Getuid())
	myGID = uint32(os.Getgid())
)

func id(n uint32) string { return strconv.FormatUint(uint64(n), 10) }

type fakeDirectory struct {
	users  map[string]bool
	uids   map[uint32]bool
	groups map[string]bool
}

func (f fakeDirectory) UserByName(_ context.Context, name string) (userdb.User, error) {
	if f.users[name] {
		return userdb.User{Name: name}, nil
	}
	return userdb.User{}, userdb.NoRecordFound
}

func (f fakeDirectory) UserByUID(_ context.Context, uid userdb.UID) (userdb.User, error) {
	if f.uids[uint32(uid)] {
		return userdb.User{UID: uid}, nil
	}
	return userdb.User{}, userdb.NoRecordFound
}

func (f fakeDirectory) GroupByName(_ context.Context, name string) (userdb.Group, error) {
	if f.groups[name] {
		return userdb.Group{Name: name}, nil
	}
	return userdb.Group{}, userdb.NoRecordFound
}

func (fakeDirectory) Close() error { return nil }

type machine struct {
	system
	home string
}

func write(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}

// newMachine is an /etc whose people are uids me to me+100, holding
// root, admin alice (uid me+1) and bob (me+2) and the system account
// svc, whose new accounts go in the group of my gid, and a /proc with
// no process.
func newMachine(t *testing.T, people ...string) machine {
	t.Helper()
	dir := t.TempDir()
	m := machine{
		etc: filepath.Join(dir, "etc"), avatars: filepath.Join(dir, "avatars"), proc: filepath.Join(dir, "proc"),
		directory: func(context.Context) (directory, error) { return fakeDirectory{}, nil },
		now:       func() time.Time { return time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC) },
		home:      filepath.Join(dir, "home"),
	}
	passwd := "root:x:0:0:root:/root:/bin/bash\n" + strings.Join(people, "")
	write(t, filepath.Join(m.etc, "passwd"), passwd, 0o644)
	write(t, filepath.Join(m.etc, "shadow"), "root:!:20000::::::\n", 0o600)
	write(t, filepath.Join(m.etc, "group"), "root:x:0:\nwheel:x:10:\nmine:x:"+id(myGID)+":\n", 0o644)
	write(t, filepath.Join(m.etc, "login.defs"), "UID_MIN "+id(me)+"\nUID_MAX "+id(me+100)+"\nPASS_MAX_DAYS 99999\nPASS_WARN_AGE 7\nHOME_MODE 0750\n", 0o644)
	write(t, filepath.Join(m.etc, useraddName), "GROUP="+id(myGID)+"\nHOME="+m.home+"\nSHELL=/bin/zsh\n", 0o644)
	for _, d := range []string{m.proc, m.home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func (m machine) read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(m.etc, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (m machine) snapshot(t *testing.T) string {
	t.Helper()
	return m.read(t, "passwd") + m.read(t, "shadow") + m.read(t, "group")
}

func entry(name string, uid uint32) string {
	return name + ":x:" + id(uid) + ":" + id(myGID) + ":" + name + ":/home/" + name + ":/bin/bash\n"
}

func hash(t *testing.T) accounts.Hash {
	t.Helper()
	return accounts.HashPassword([]byte("pw"))
}

func TestANewAccountIsInTheFilesWithAHomeFromTheSkeleton(t *testing.T) {
	m := newMachine(t)
	write(t, filepath.Join(m.etc, "skel", ".profile"), "export EDITOR=vi\n", 0o640)
	write(t, filepath.Join(m.etc, "skel", ".config", "app.conf"), "x\n", 0o600)
	if err := os.Symlink(".profile", filepath.Join(m.etc, "skel", ".bash_profile")); err != nil {
		t.Fatal(err)
	}
	h := hash(t)
	if err := m.change(t.Context(), 0, accounts.Create{Login: "zoe", RealName: "Zoë", Kind: accounts.Admin, Hash: h}); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(m.home, "zoe")
	if got, want := m.read(t, "passwd"), "root:x:0:0:root:/root:/bin/bash\nzoe:x:"+id(me)+":"+id(myGID)+":Zoë:"+home+":/bin/zsh\n"; got != want {
		t.Errorf("passwd %q, want %q", got, want)
	}
	day := strconv.FormatInt(time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC).Unix()/86400, 10)
	if got, want := m.read(t, "shadow"), "root:!:20000::::::\nzoe:"+string(h)+":"+day+"::99999:7:::\n"; got != want {
		t.Errorf("shadow %q, want %q", got, want)
	}
	if !strings.Contains(m.read(t, "group"), "wheel:x:10:zoe\n") {
		t.Errorf("zoe is no administrator: %q", m.read(t, "group"))
	}
	for path, mode := range map[string]os.FileMode{home: os.ModeDir | 0o750, filepath.Join(home, ".profile"): 0o640, filepath.Join(home, ".config"): os.ModeDir | 0o755, filepath.Join(home, ".config", "app.conf"): 0o600, filepath.Join(home, ".bash_profile"): os.ModeSymlink | 0o777} {
		st, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		owner := st.Sys().(*syscall.Stat_t)
		if st.Mode() != mode || owner.Uid != me || owner.Gid != myGID {
			t.Errorf("%s: %v of %d:%d, want %v of %d:%d", path, st.Mode(), owner.Uid, owner.Gid, mode, me, myGID)
		}
	}
	if data, err := os.ReadFile(filepath.Join(home, ".bash_profile")); err != nil || string(data) != "export EDITOR=vi\n" {
		t.Errorf("the linked profile: %q, %v", data, err)
	}
}

func TestANewAccountWhoseNameIDOrHomeIsTakenChangesNothing(t *testing.T) {
	for name, c := range map[string]struct {
		setup func(*testing.T, *machine)
		login accounts.Login
		want  accounts.ErrorKind
	}{
		"a name the files hold": {login: "root", want: accounts.ErrTaken},
		"a name the user database holds": {setup: func(_ *testing.T, m *machine) {
			m.directory = directoryOf(fakeDirectory{users: map[string]bool{"zoe": true}})
		}, login: "zoe", want: accounts.ErrTaken},
		"a home that exists": {setup: func(t *testing.T, m *machine) { write(t, filepath.Join(m.home, "zoe", "old"), "x", 0o600) }, login: "zoe", want: accounts.ErrTaken},
		"every user ID taken": {setup: func(_ *testing.T, m *machine) {
			taken := map[uint32]bool{}
			for u := me; u <= me+100; u++ {
				taken[u] = true
			}
			m.directory = directoryOf(fakeDirectory{uids: taken})
		}, login: "zoe", want: accounts.ErrExhausted},
		"a malformed passwd": {setup: func(t *testing.T, m *machine) {
			write(t, filepath.Join(m.etc, "passwd"), "root:x:0:0:root:/root:/bin/bash\nbroken:x:1\n", 0o644)
		}, login: "zoe", want: accounts.ErrFailed},
		"a user database that does not answer": {setup: func(_ *testing.T, m *machine) {
			m.directory = func(context.Context) (directory, error) { return nil, errors.New("no socket") }
		}, login: "zoe", want: accounts.ErrFailed},
	} {
		m := newMachine(t)
		if c.setup != nil {
			c.setup(t, &m)
		}
		before := m.snapshot(t)
		err := m.change(t.Context(), 0, accounts.Create{Login: c.login, Hash: hash(t)})
		if err == nil || err.Kind != c.want {
			t.Errorf("%s: %v, want %s", name, err, c.want)
		}
		if m.snapshot(t) != before {
			t.Errorf("%s changed the files", name)
		}
		if name != "a home that exists" {
			if _, err := os.Stat(filepath.Join(m.home, string(c.login))); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s left a home: %v", name, err)
			}
		}
	}
}

func directoryOf(d fakeDirectory) func(context.Context) (directory, error) {
	return func(context.Context) (directory, error) { return d, nil }
}

func TestWithUserGroupsANewAccountGetsAGroupOfItsOwnByItsUserID(t *testing.T) {
	m := newMachine(t)
	files, err := pwfiles.Load(m.etc)
	if err != nil {
		t.Fatal(err)
	}
	defs := defaults{userGroups: true, gids: pwfiles.Range{Min: 1000, Max: 60000}}
	gid, aerr := m.primaryGroup(t.Context(), files, fakeDirectory{}, defs, "zoe", 4242)
	if aerr != nil || gid != 4242 || !strings.Contains(files.Text(pwfiles.Group), "zoe:x:4242:\n") {
		t.Fatalf("gid %d, %v, group %q", gid, aerr, files.Text(pwfiles.Group))
	}
	if _, aerr := m.primaryGroup(t.Context(), files, fakeDirectory{}, defs, "zoe", 4243); aerr == nil || aerr.Kind != accounts.ErrTaken {
		t.Errorf("a group of the name: %v", aerr)
	}
	if _, aerr := m.primaryGroup(t.Context(), files, fakeDirectory{groups: map[string]bool{"yan": true}}, defs, "yan", 4243); aerr == nil || aerr.Kind != accounts.ErrTaken {
		t.Errorf("a group the user database holds: %v", aerr)
	}
	if _, aerr := m.primaryGroup(t.Context(), files, fakeDirectory{}, defaults{}, "xia", 4244); aerr == nil || aerr.Kind != accounts.ErrFailed {
		t.Errorf("neither user groups nor GROUP: %v", aerr)
	}
}

func TestAnEditSetsTheFieldsItNames(t *testing.T) {
	m := newMachine(t, entry("alice", me+1), entry("bob", me+2))
	write(t, filepath.Join(m.etc, "shadow"), "root:!:20000::::::\nalice:!:20000::::::\nbob:!:20000::::::\n", 0o600)
	write(t, filepath.Join(m.etc, "group"), "root:x:0:\nwheel:x:10:alice\n", 0o644)
	name, admin, standard, h := accounts.RealName("Robert"), accounts.Admin, accounts.Standard, hash(t)
	if err := m.change(t.Context(), me+1, accounts.Edit{Login: "bob", RealName: &name, Kind: &admin, Hash: &h}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.read(t, "passwd"), "bob:x:"+id(me+2)+":"+id(myGID)+":Robert:") || !strings.Contains(m.read(t, "group"), "wheel:x:10:alice,bob\n") || !strings.Contains(m.read(t, "shadow"), "bob:"+string(h)+":") {
		t.Fatalf("after the edit:\n%s", m.snapshot(t))
	}
	if err := m.change(t.Context(), me+2, accounts.Edit{Login: "alice", Kind: &standard}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.read(t, "group"), "wheel:x:10:bob\n") {
		t.Fatalf("alice is still an administrator: %q", m.read(t, "group"))
	}
	if err := m.change(t.Context(), me+1, accounts.Rename{RealName: "Alice Liddell"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.read(t, "passwd"), ":Alice Liddell:") {
		t.Fatalf("the rename: %q", m.read(t, "passwd"))
	}
}

func TestARequestTheAccountsDoNotAllowIsRefusedWithItsKind(t *testing.T) {
	m := newMachine(t, entry("alice", me+1), entry("bob", me+2), "svc:x:"+id(me+3)+":1:svc:/var/lib/svc:/usr/sbin/nologin\n")
	write(t, filepath.Join(m.etc, "group"), "root:x:0:\n", 0o644)
	before := m.snapshot(t)
	admin := accounts.Admin
	for _, c := range []struct {
		by   uint32
		r    accounts.Request
		want accounts.ErrorKind
	}{
		{me + 1, accounts.Edit{Login: "zoe"}, accounts.ErrUnknownUser},
		{me + 1, accounts.Edit{Login: "root"}, accounts.ErrNotAPerson},
		{me + 1, accounts.Delete{Login: "svc"}, accounts.ErrNotAPerson},
		{me + 1, accounts.Delete{Login: "alice"}, accounts.ErrOwnAccount},
		{me + 1, accounts.Create{Login: "zoe", Kind: accounts.Admin, Hash: hash(t)}, accounts.ErrNoAdmins},
		{me + 1, accounts.Edit{Login: "bob", Kind: &admin}, accounts.ErrNoAdmins},
		{4242, accounts.Rename{RealName: "x"}, accounts.ErrUnknownUser},
	} {
		if err := m.change(t.Context(), c.by, c.r); err == nil || err.Kind != c.want {
			t.Errorf("%#v: %v, want %s", c.r, err, c.want)
		}
	}
	if m.snapshot(t) != before {
		t.Error("a refused request changed the files")
	}
}

func TestADeletedAccountLosesItsHomeAndPictureButNotWhileItRuns(t *testing.T) {
	carol := filepath.Join(t.TempDir(), "carol")
	m := newMachine(t, entry("alice", me+1), "carol:x:"+id(me)+":"+id(myGID)+":Carol:"+carol+":/bin/bash\n")
	write(t, filepath.Join(carol, "notes"), "x", 0o600)
	write(t, filepath.Join(m.avatars, "carol"), "png", 0o644)
	write(t, filepath.Join(m.etc, "shadow"), "root:!:20000::::::\ncarol:!:20000::::::\n", 0o600)
	if err := os.Mkdir(filepath.Join(m.proc, "4242"), 0o555); err != nil {
		t.Fatal(err)
	}
	before := m.snapshot(t)
	if err := m.change(t.Context(), me+1, accounts.Delete{Login: "carol", RemoveHome: true}); err == nil || err.Kind != accounts.ErrInUse {
		t.Fatalf("deleting a running account: %v", err)
	}
	if m.snapshot(t) != before {
		t.Fatal("the refused delete changed the files")
	}
	if err := os.Remove(filepath.Join(m.proc, "4242")); err != nil {
		t.Fatal(err)
	}
	if err := m.change(t.Context(), me+1, accounts.Delete{Login: "carol", RemoveHome: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.snapshot(t), "carol") {
		t.Fatalf("carol stays:\n%s", m.snapshot(t))
	}
	for _, path := range []string{carol, filepath.Join(m.avatars, "carol")} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s stays: %v", path, err)
		}
	}
}

func TestAHomeThatIsALinkIsNeverRemoved(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "dave")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, entry("alice", me+1), "dave:x:"+id(me)+":"+id(myGID)+":Dave:"+link+":/bin/bash\n")
	before := m.snapshot(t)
	if err := m.change(t.Context(), me+1, accounts.Delete{Login: "dave", RemoveHome: true}); err == nil || err.Kind != accounts.ErrFailed {
		t.Fatalf("a linked home: %v", err)
	}
	if _, err := os.Stat(target); err != nil || m.snapshot(t) != before {
		t.Fatalf("the refusal touched something: %v", err)
	}
}

func TestAPictureIsWrittenWhereTheGreeterReadsIt(t *testing.T) {
	m := newMachine(t, entry("alice", me+1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	pic, err := accounts.ParsePicture(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.change(t.Context(), me+1, accounts.SetPicture{Picture: pic}); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(m.avatars, "alice")); err != nil || st.Mode().Perm() != 0o644 {
		t.Fatalf("alice's picture: %v, %v", st, err)
	}
}

func TestLockedAccountFilesAreBusyAndChangeNothing(t *testing.T) {
	m := newMachine(t, entry("alice", me+1))
	held, err := pwfiles.TakeLock(t.Context(), m.etc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Unlock() }()
	before := m.snapshot(t)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	if err := m.change(ctx, me+1, accounts.Rename{RealName: "A"}); err == nil || err.Kind != accounts.ErrBusy {
		t.Fatalf("a change while locked: %v", err)
	}
	if m.snapshot(t) != before {
		t.Fatal("the busy change changed the files")
	}
}

func TestNixOSRefusesWhatItsConfigurationWouldUndo(t *testing.T) {
	declared, err := parseNixOS([]byte(`{"mutableUsers":true,"users":["alice","root"]}`))
	if err != nil {
		t.Fatal(err)
	}
	immutable, err := parseNixOS([]byte(`{"mutableUsers":false,"users":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, entry("alice", me+1), entry("bob", me+2))
	write(t, filepath.Join(m.etc, "shadow"), "root:!:20000::::::\nalice:!:20000::::::\nbob:!:20000::::::\n", 0o600)
	before := m.snapshot(t)
	name, admin, standard, h := accounts.RealName("X"), accounts.Admin, accounts.Standard, hash(t)
	for _, c := range []struct {
		policy *nixos
		by     uint32
		r      accounts.Request
	}{
		{&nixos{}, me + 1, accounts.Edit{Login: "bob", Hash: &h}},
		{immutable, me + 1, accounts.Edit{Login: "bob", Hash: &h}},
		{immutable, me + 2, accounts.Rename{RealName: "B"}},
		{declared, me + 1, accounts.Rename{RealName: "A"}},
		{declared, me + 2, accounts.Edit{Login: "alice", RealName: &name}},
		{declared, me + 2, accounts.Edit{Login: "alice", Kind: &admin}},
		{declared, me + 2, accounts.Delete{Login: "alice"}},
		{declared, me + 2, accounts.Create{Login: "root", Hash: h}},
	} {
		m.nixos = c.policy
		if err := m.change(t.Context(), c.by, c.r); err == nil || err.Kind != accounts.ErrDeclarative {
			t.Errorf("%#v under %+v: %v, want %s", c.r, c.policy, err, accounts.ErrDeclarative)
		}
	}
	if m.snapshot(t) != before {
		t.Fatal("a declarative refusal changed the files")
	}
	m.nixos = declared
	for _, r := range []accounts.Request{
		accounts.Edit{Login: "alice", Hash: &h},
		accounts.Edit{Login: "alice", Kind: &standard},
		accounts.Rename{RealName: "Bob B"},
	} {
		if err := m.change(t.Context(), me+2, r); err != nil {
			t.Errorf("%#v of a declared account, which NixOS leaves alone: %v", r, err)
		}
	}
}

func TestTheNixOSFileSaysWhetherAndWhichAccountsAreDeclared(t *testing.T) {
	etc := t.TempDir()
	if n, err := readNixOS(etc, ""); err != nil || n != nil {
		t.Fatalf("off NixOS: %+v, %v", n, err)
	}
	write(t, filepath.Join(etc, nixosName), "", 0o644)
	if n, err := readNixOS(etc, ""); err != nil || n == nil || n.configured {
		t.Fatalf("NixOS without the module: %+v, %v", n, err)
	}
	for _, bad := range []string{`{}`, `{"mutableUsers":true,"user":[]}`, `not json`} {
		if _, err := parseNixOS([]byte(bad)); err == nil {
			t.Errorf("%s was taken", bad)
		}
	}
	if _, err := readNixOS(etc, filepath.Join(etc, "absent.json")); err == nil {
		t.Error("a missing file named on the command line was taken")
	}
}

func TestTheDefaultsAreWhatLoginDefsAndUseraddSay(t *testing.T) {
	d, err := parseDefaults("# c\nUID_MIN 2000\nUID_MAX 3000\nGID_MIN 2000\nUSERGROUPS_ENAB yes\nUMASK 027\nPASS_MIN_DAYS -1\nPASS_MAX_DAYS 90\n", "GROUP=100\nHOME=/srv/home\nSHELL=/bin/fish\n")
	if err != nil {
		t.Fatal(err)
	}
	want := defaults{uids: pwfiles.Range{Min: 2000, Max: 3000}, gids: pwfiles.Range{Min: 2000, Max: 60000}, userGroups: true, group: "100", homeBase: "/srv/home", shell: "/bin/fish", homeMode: 0o750, passMax: "90"}
	if d != want {
		t.Errorf("%+v, want %+v", d, want)
	}
	if d, err := parseDefaults("", ""); err != nil || d.homeBase != "/home" || d.shell != "/bin/sh" || d.homeMode != 0o755 || d.userGroups {
		t.Errorf("no files: %+v, %v", d, err)
	}
	for _, c := range [][2]string{{"UID_MIN x\n", ""}, {"UMASK 999\n", ""}, {"HOME_MODE 7777\n", ""}, {"PASS_MAX_DAYS soon\n", ""}, {"GID_MAX -1\n", ""}, {"", "HOME=home\n"}} {
		if _, err := parseDefaults(c[0], c[1]); err == nil {
			t.Errorf("%q %q was taken", c[0], c[1])
		}
	}
}

func TestASkeletonWithADeviceOrFifoLeavesNoHome(t *testing.T) {
	skel := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(skel, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "zoe")
	if err := makeHome(skel, home, int(me), int(myGID), 0o700); err == nil || !strings.Contains(err.Error(), "neither a file") {
		t.Fatalf("a fifo in the skeleton: %v", err)
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the half-made home stays: %v", err)
	}
}

type bus struct {
	authority *polkittest.Authority
	client    *dbus.Conn
}

func serveOnBus(t *testing.T, m machine, verdict polkit.AuthorizationResult) bus {
	t.Helper()
	a := polkittest.Start(t, verdict)
	conn, err := dbusx.DialSystem(t.Context(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	release, err := serve(conn, m.system, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	client, err := dbusx.DialSystem(t.Context(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return bus{authority: a, client: client}
}

func TestACallPolkitRefusesIsNotAuthorizedAndChangesNothing(t *testing.T) {
	for name, verdict := range map[string]polkit.AuthorizationResult{"denied": polkittest.Denied, "dismissed": polkittest.Dismissed, "a challenge": polkittest.Challenge} {
		m := newMachine(t, entry("alice", me+1), "me:x:"+id(me)+":"+id(myGID)+":Me:/home/me:/bin/bash\n")
		b := serveOnBus(t, m, verdict)
		before := m.snapshot(t)
		err := accounts.Change(t.Context(), b.client, accounts.Delete{Login: "alice"})
		if e, ok := errors.AsType[*accounts.Error](err); !ok || e.Kind != accounts.ErrNotAuthorized {
			t.Errorf("%s: %v, want %s", name, err, accounts.ErrNotAuthorized)
		}
		if m.snapshot(t) != before {
			t.Errorf("%s: the refused call changed the files", name)
		}
		checks := b.authority.Checks.Load()
		if len(checks) != 1 || checks[0].Action != polkit.Action(accounts.Others.Action()) || checks[0].Flags != polkit.AllowUserInteraction {
			t.Errorf("%s: polkit was asked %+v", name, checks)
		}
		if p, ok := checks[0].Subject.(polkit.UnixProcess); !ok || p.UID != me || int(p.PID) != os.Getpid() {
			t.Errorf("%s: polkit was asked about %v", name, checks[0].Subject)
		}
	}
}

func TestACallPolkitAuthorizesIsMadeForTheCallerAndAMalformedOneIsInvalid(t *testing.T) {
	m := newMachine(t, "me:x:"+id(me)+":"+id(myGID)+":Me:/home/me:/bin/bash\n")
	b := serveOnBus(t, m, polkittest.Authorized)
	if err := accounts.Change(t.Context(), b.client, accounts.Rename{RealName: "Myself"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.read(t, "passwd"), "me:x:"+id(me)+":"+id(myGID)+":Myself:") {
		t.Fatalf("passwd %q", m.read(t, "passwd"))
	}
	if checks := b.authority.Checks.Load(); len(checks) != 1 || checks[0].Action != polkit.Action(accounts.Self.Action()) {
		t.Errorf("polkit was asked %+v", checks)
	}
	call := b.client.Object(accounts.BusName, accounts.BusPath).CallWithContext(t.Context(), accounts.BusInterface+"."+accounts.ChangeMethod, 0, []byte(`{"op":"delete","request":{"login":"-r"}}`))
	if reply, ok := errors.AsType[dbus.Error](call.Err); !ok || reply.Name != accounts.BusErrorName(accounts.ErrInvalid) {
		t.Fatalf("a malformed request: %v", call.Err)
	}
	if len(b.authority.Checks.Load()) != 1 {
		t.Error("polkit was asked about a request that did not decode")
	}
}

func TestTheCommandLineTakesOnlyTheNixOSFile(t *testing.T) {
	for _, args := range [][]string{{"self"}, {"--bogus"}, {"--nixos"}} {
		if code := Main(args, io.Discard); code != 2 {
			t.Errorf("%q: exit %d, want 2", args, code)
		}
	}
	if file, err := parseArgs([]string{"--nixos", "/nix/store/x-users.json"}, io.Discard); err != nil || file != "/nix/store/x-users.json" {
		t.Errorf("--nixos: %q, %v", file, err)
	}
}
