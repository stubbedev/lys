package useraccount

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/userdb"
	"github.com/stubbedev/lys/lib/userdb/userdbtest"
)

// lys's avatar wins over the one AccountsService left, which wins over
// ~/.face; an unreadable or missing file is skipped; none at all is "".
func TestFindAvatarOrder(t *testing.T) {
	dir := t.TempDir()
	lys := filepath.Join(dir, "lys", "ada")
	service := filepath.Join(dir, "icons", "ada")
	home := filepath.Join(dir, "home")
	face := filepath.Join(home, ".face")
	prev := avatarCandidates
	avatarCandidates = func(string, string) []string { return []string{lys, service, face} }
	t.Cleanup(func() { avatarCandidates = prev })
	if got := FindAvatar("ada", home); got != "" {
		t.Errorf("no avatar = %q", got)
	}
	for _, p := range []string{face, service, lys} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct{ want, then string }{{lys, lys}, {service, service}, {face, ""}} {
		if got := FindAvatar("ada", home); got != step.want {
			t.Errorf("avatar = %q, want %q", got, step.want)
		}
		if step.then != "" {
			if err := os.Remove(step.then); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestAvatarsAreLookedForInLysThenAccountsServiceThenHome(t *testing.T) {
	want := []string{"/var/lib/lys/avatars/ada", "/var/lib/AccountsService/icons/ada", "/home/ada/.face"}
	if got := avatarCandidates("ada", "/home/ada"); !slices.Equal(got, want) {
		t.Errorf("candidates = %q, want %q", got, want)
	}
	if got := AvatarPath("ada"); got != want[0] {
		t.Errorf("AvatarPath = %q, want %q", got, want[0])
	}
}

// The account comes from the user database's record for the uid, an
// account outside /etc/passwd among them: an empty or spoofed $USER does
// not change it.
func TestLookupResolvesTheUIDsRecord(t *testing.T) {
	socket := userdbtest.Serve(t, userdbtest.DB{Users: []userdb.User{
		{Name: "carol", UID: 70001, GID: 70001, RealName: "Carol Directory,Room 4", Home: "/home/carol"},
		{Name: "dave", UID: 70002, GID: 70002, Home: "/home/dave"},
	}})
	prev := avatarCandidates
	avatarCandidates = func(string, string) []string { return nil }
	t.Cleanup(func() { avatarCandidates = prev })
	for _, env := range []string{"", "mallory"} {
		t.Setenv("USER", env)
		acct, err := Lookup(t.Context(), socket, 70001)
		if want := (Account{Name: "carol", RealName: "Carol Directory", Home: "/home/carol"}); err != nil || acct != want {
			t.Errorf("$USER=%q: %+v, %v; want %+v", env, acct, err, want)
		}
	}
	if acct, err := Lookup(t.Context(), socket, 70002); err != nil || acct.RealName != "dave" {
		t.Errorf("no real name: %+v, %v; want the login name", acct, err)
	}
}

func TestLookupOfAnUnknownUIDFails(t *testing.T) {
	socket := userdbtest.Serve(t, userdbtest.DB{})
	if acct, err := Lookup(t.Context(), socket, 70001); !errors.Is(err, userdb.NoRecordFound) || !strings.Contains(err.Error(), "uid 70001") {
		t.Errorf("Lookup = %+v, %v; want NoRecordFound for uid 70001", acct, err)
	}
}

func TestLookupWithoutTheUserDatabaseFails(t *testing.T) {
	socket := userdbtest.Missing(t)
	if acct, err := Lookup(t.Context(), socket, 70001); err == nil || !strings.Contains(err.Error(), socket) {
		t.Errorf("Lookup = %+v, %v; want an error naming %s", acct, err, socket)
	}
}
