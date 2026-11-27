package usershelper

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

func carolMachine(t *testing.T, uid uint32) machine {
	t.Helper()
	m := newMachine(t, entry("alice", me+1), "bob:x:"+id(me+2)+":"+id(myGID)+":Bob:"+filepath.Join(t.TempDir(), "bob")+":/bin/bash\n", "carol:x:"+id(uid)+":"+id(myGID)+":Carol:"+filepath.Join(t.TempDir(), "carol")+":/bin/bash\n")
	write(t, filepath.Join(m.etc, "shadow"), "root:!:20000::::::\ncarol:!:20000::::::\n", 0o600)
	return m
}

func TestADeletedAccountsHomeGoesWithItsMailSpool(t *testing.T) {
	m := carolMachine(t, me)
	spool := filepath.Join(m.mail, "carol")
	write(t, spool, "From x\n", 0o600)
	if err := m.change(t.Context(), me+1, accounts.Delete{Login: "carol", RemoveHome: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(spool); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the spool stays: %v", err)
	}
	if err := m.change(t.Context(), me+1, accounts.Delete{Login: "bob", RemoveHome: true}); err != nil {
		t.Errorf("an account with no spool: %v", err)
	}
}

func TestAMailSpoolStaysWhereTheHomeDoesOrMailLivesInIt(t *testing.T) {
	kept := carolMachine(t, me)
	spool := filepath.Join(kept.mail, "carol")
	write(t, spool, "From x\n", 0o600)
	if err := kept.change(t.Context(), me+1, accounts.Delete{Login: "carol"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(spool); err != nil {
		t.Errorf("the spool of an account whose home stays went: %v", err)
	}
	inHome := carolMachine(t, me)
	write(t, filepath.Join(inHome.etc, loginDefsName), "UID_MIN "+id(me)+"\nUID_MAX "+id(me+100)+"\nMAIL_FILE .mail\n", 0o644)
	spool = filepath.Join(inHome.mail, "carol")
	write(t, spool, "From x\n", 0o600)
	if err := inHome.change(t.Context(), me+1, accounts.Delete{Login: "carol", RemoveHome: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(spool); err != nil {
		t.Errorf("a spool where MAIL_FILE keeps mail in the home went: %v", err)
	}
}

func TestAMailSpoolThatIsNotTheAccountsFileRefusesTheDeletion(t *testing.T) {
	target := filepath.Join(t.TempDir(), "elsewhere")
	write(t, target, "x", 0o600)
	for name, c := range map[string]struct {
		uid   uint32
		setup func(t *testing.T, spool string)
	}{
		"a link": {uid: me, setup: func(t *testing.T, spool string) {
			if err := os.Symlink(target, spool); err != nil {
				t.Fatal(err)
			}
		}},
		"a folder": {uid: me, setup: func(t *testing.T, spool string) {
			if err := os.Mkdir(spool, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		"another's file": {uid: me + 5, setup: func(t *testing.T, spool string) { write(t, spool, "From x\n", 0o600) }},
	} {
		m := carolMachine(t, c.uid)
		spool := filepath.Join(m.mail, "carol")
		c.setup(t, spool)
		before := m.snapshot(t)
		err := m.change(t.Context(), me+1, accounts.Delete{Login: "carol", RemoveHome: true})
		if err == nil || err.Kind != accounts.ErrFailed || !strings.Contains(err.Detail, spool) {
			t.Errorf("%s: %v, want %s naming %s", name, err, accounts.ErrFailed, spool)
		}
		if _, err := os.Lstat(spool); err != nil || m.snapshot(t) != before {
			t.Errorf("%s: the refusal changed something: %v", name, err)
		}
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("the link's target went: %v", err)
	}
}
