package usershelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/useraccount"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

const passwd = `root:x:0:0:root:/root:/bin/bash
alice:x:1000:1000:Alice:/home/alice:/bin/bash
bob:x:2000:2000:Bob:/home/bob:/bin/bash
svc:x:998:998:service:/var/lib/svc:/usr/sbin/nologin
`

const group = "wheel:x:10:alice\n"

func database() accounts.Database {
	return accounts.ParseDatabase(passwd, group, useraccount.DefaultUIDRange)
}

func caller(t *testing.T, name string) accounts.Account {
	t.Helper()
	a, ok := database().ByName(accounts.Name(name))
	if !ok {
		t.Fatalf("no %s", name)
	}
	return a
}

func commands(p plan) []string {
	var out []string
	for _, st := range p.steps {
		if c, ok := st.(command); ok {
			out = append(out, string(c.tool)+" "+strings.Join(c.args, " "))
		}
	}
	return out
}

func TestEachRequestIsTheShadowToolsWithArgv(t *testing.T) {
	hash := accounts.HashPassword([]byte("pw"))
	name, admin, standard := accounts.RealName("Robert; rm -rf /"), accounts.Admin, accounts.Standard
	for _, c := range []struct {
		by   string
		r    accounts.Request
		want []string
	}{
		{"alice", accounts.Rename{RealName: "Alice Liddell"}, []string{"usermod -c Alice Liddell -- alice"}},
		{"bob", accounts.Rename{RealName: ""}, []string{"usermod -c  -- bob"}},
		{"alice", accounts.Create{Login: "zoe", RealName: "Zoë", Kind: accounts.Standard, Hash: hash}, []string{"useradd -m -c Zoë -- zoe", "chpasswd -e"}},
		{"alice", accounts.Create{Login: "zoe", Kind: accounts.Admin, Hash: hash}, []string{"useradd -m -c  -G wheel -- zoe", "chpasswd -e"}},
		{"alice", accounts.Edit{Login: "bob", RealName: &name, Kind: &admin, Hash: &hash}, []string{"usermod -c Robert; rm -rf / -- bob", "gpasswd -a bob wheel", "chpasswd -e"}},
		{"alice", accounts.Edit{Login: "bob", Kind: &standard}, nil},
		{"bob", accounts.Edit{Login: "alice", Kind: &standard}, []string{"gpasswd -d alice wheel"}},
		{"alice", accounts.Delete{Login: "bob"}, []string{"userdel -- bob"}},
		{"alice", accounts.Delete{Login: "bob", RemoveHome: true}, []string{"userdel -r -- bob"}},
	} {
		p, err := makePlan(database(), caller(t, c.by), c.r)
		if err != nil {
			t.Errorf("%#v: %v", c.r, err)
			continue
		}
		if got := commands(p); !slices.Equal(got, c.want) {
			t.Errorf("%#v: %q, want %q", c.r, got, c.want)
		}
	}
}

func TestAPasswordGoesToChpasswdOnItsInputNeverItsArgv(t *testing.T) {
	hash := accounts.HashPassword([]byte("pw"))
	p, err := makePlan(database(), caller(t, "alice"), accounts.Edit{Login: "bob", Hash: &hash})
	if err != nil {
		t.Fatal(err)
	}
	c := p.steps[0].(command)
	if string(c.stdin) != "bob:"+string(hash)+"\n" || slices.ContainsFunc(c.args, func(a string) bool { return strings.Contains(a, string(hash)) }) {
		t.Errorf("chpasswd gets %q on stdin and argv %q", c.stdin, c.args)
	}
}

func TestARequestTheDatabaseDoesNotAllowIsRefusedWithItsKind(t *testing.T) {
	hash := accounts.HashPassword([]byte("pw"))
	noAdmins := accounts.ParseDatabase(passwd, "users:x:100:\n", useraccount.DefaultUIDRange)
	admin := accounts.Admin
	for _, c := range []struct {
		db   accounts.Database
		by   string
		r    accounts.Request
		want accounts.ErrorKind
	}{
		{database(), "alice", accounts.Create{Login: "bob", Hash: hash}, accounts.ErrTaken},
		{database(), "alice", accounts.Create{Login: "svc", Hash: hash}, accounts.ErrTaken},
		{database(), "alice", accounts.Edit{Login: "zoe"}, accounts.ErrUnknownUser},
		{database(), "alice", accounts.Edit{Login: "root"}, accounts.ErrNotAPerson},
		{database(), "alice", accounts.Edit{Login: "svc"}, accounts.ErrNotAPerson},
		{database(), "alice", accounts.Delete{Login: "root"}, accounts.ErrNotAPerson},
		{database(), "alice", accounts.Delete{Login: "alice"}, accounts.ErrOwnAccount},
		{noAdmins, "alice", accounts.Create{Login: "zoe", Kind: accounts.Admin, Hash: hash}, accounts.ErrNoAdmins},
		{noAdmins, "alice", accounts.Edit{Login: "bob", Kind: &admin}, accounts.ErrNoAdmins},
	} {
		if _, err := makePlan(c.db, caller(t, c.by), c.r); err == nil || err.Kind != c.want {
			t.Errorf("%#v: %v, want %s", c.r, err, c.want)
		}
	}
}

func encode(t *testing.T, r accounts.Request) []byte {
	t.Helper()
	data, err := accounts.Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readDB() (accounts.Database, error) { return database(), nil }

func TestOnlyRootThroughPkexecWithAValidRequestIsServed(t *testing.T) {
	sys := fakeSystem(t, nil)
	good := encode(t, accounts.Delete{Login: "bob"})
	for _, c := range []struct {
		name  string
		class accounts.Class
		data  []byte
		uid   string
		euid  int
		want  accounts.ErrorKind
	}{
		{"not root", accounts.Others, good, "1000", 1000, accounts.ErrInvalid},
		{"no PKEXEC_UID", accounts.Others, good, "", 0, accounts.ErrInvalid},
		{"a PKEXEC_UID that is no uid", accounts.Others, good, "alice", 0, accounts.ErrInvalid},
		{"a caller with no account", accounts.Others, good, "4242", 0, accounts.ErrUnknownUser},
		{"a request of the other class", accounts.Self, good, "1000", 0, accounts.ErrInvalid},
		{"an oversized request", accounts.Others, bytes.Repeat([]byte(" "), maxRequest+1), "1000", 0, accounts.ErrInvalid},
	} {
		err := serve(c.class, bytes.NewReader(c.data), c.uid, c.euid, readDB, sys.system)
		if err == nil || err.Kind != c.want {
			t.Errorf("%s: %v, want %s", c.name, err, c.want)
		}
	}
	if got := sys.calls(t); len(got) != 0 {
		t.Fatalf("a refused request ran %q", got)
	}
	if err := serve(accounts.Others, bytes.NewReader(good), "1000", 0, readDB, sys.system); err != nil {
		t.Fatal(err)
	}
	if got := sys.calls(t); !slices.Equal(got, []string{"userdel -- bob"}) {
		t.Fatalf("ran %q", got)
	}
}

type fake struct {
	system
	log string
}

func fakeSystem(t *testing.T, failing []tool) fake {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh to stand in for the shadow tools")
	}
	dir := t.TempDir()
	f := fake{sbin: filepath.Join(dir, "sbin"), bin: filepath.Join(dir, "bin"), avatars: filepath.Join(dir, "avatars"), log: filepath.Join(dir, "log")}
	for _, d := range []string{f.sbin, f.bin} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tl := range []tool{useradd, usermod, userdel, chpasswd, gpasswd} {
		status := 0
		if slices.Contains(failing, tl) {
			status = 3
		}
		script := "#!" + sh + "\nprintf '%s' " + string(tl) + " >> " + f.log + "\nfor a in \"$@\"; do printf ' %s' \"$a\" >> " + f.log + "; done\nprintf '\\n' >> " + f.log + "\nwhile IFS= read -r _; do :; done\n" +
			"[ " + strconv.Itoa(status) + " = 0 ] || { echo " + string(tl) + " refused >&2; exit " + strconv.Itoa(status) + "; }\n"
		if err := os.WriteFile(f.path(tl), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f fake) calls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(f.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestAPictureIsWrittenWhereTheGreeterReadsIt(t *testing.T) {
	sys := fakeSystem(t, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	pic, err := accounts.ParsePicture(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := serve(accounts.Self, bytes.NewReader(encode(t, accounts.SetPicture{Picture: pic})), "2000", 0, readDB, sys.system); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sys.avatars, "bob")
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o644 {
		t.Fatalf("bob's picture: %v, %v", st, err)
	}
	if dir, err := os.Stat(sys.avatars); err != nil || dir.Mode().Perm() != 0o755 {
		t.Fatalf("the avatars directory: %v, %v", dir, err)
	}
	if err := serve(accounts.Others, bytes.NewReader(encode(t, accounts.Delete{Login: "bob"})), "1000", 0, readDB, sys.system); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a removed account's picture stays: %v", err)
	}
}

func TestANewAccountWhosePasswordCannotBeSetIsRemovedAgain(t *testing.T) {
	sys := fakeSystem(t, []tool{chpasswd})
	r := accounts.Create{Login: "zoe", RealName: "Zoë", Hash: accounts.HashPassword([]byte("pw"))}
	err := serve(accounts.Others, bytes.NewReader(encode(t, r)), "1000", 0, readDB, sys.system)
	if err == nil || err.Kind != accounts.ErrFailed || !strings.Contains(err.Detail, "chpasswd refused") {
		t.Fatalf("got %v, want chpasswd's failure", err)
	}
	if got, want := sys.calls(t), []string{"useradd -m -c Zoë -- zoe", "chpasswd -e", "userdel -r -- zoe"}; !slices.Equal(got, want) {
		t.Fatalf("ran %q, want %q", got, want)
	}
}

func TestAFailedFirstStepUndoesNothingAndSaysWhy(t *testing.T) {
	sys := fakeSystem(t, []tool{useradd})
	r := accounts.Create{Login: "zoe", Hash: accounts.HashPassword([]byte("pw"))}
	err := serve(accounts.Others, bytes.NewReader(encode(t, r)), "1000", 0, readDB, sys.system)
	if err == nil || err.Kind != accounts.ErrFailed || !strings.Contains(err.Detail, "useradd refused") {
		t.Fatalf("got %v", err)
	}
	if got := sys.calls(t); !slices.Equal(got, []string{"useradd -m -c  -- zoe"}) {
		t.Fatalf("ran %q", got)
	}
}

func TestMainAnswersARefusalInJSONAndAUsageErrorWithTwo(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Main([]string{"others"}, strings.NewReader("{}"), &out, &errOut, "", 1000); code != 1 {
		t.Fatalf("exit %d", code)
	}
	var e accounts.Error
	if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.Kind != accounts.ErrInvalid {
		t.Fatalf("answer %q: %v", out.String(), err)
	}
	if err := accounts.Outcome(1, out.Bytes(), errOut.Bytes()); err == nil {
		t.Fatal("the answer reads as success")
	}
	for _, args := range [][]string{nil, {"root"}, {"self", "others"}} {
		if code := Main(args, strings.NewReader(""), &out, &errOut, "1000", 0); code != 2 {
			t.Errorf("%q: exit %d, want 2", args, code)
		}
	}
}
