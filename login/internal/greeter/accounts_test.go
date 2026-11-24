package greeter

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
)

func accountUsers() []User {
	return []User{{Name: "ada"}, {Name: "temp_root"}, {Name: "bob"}, {Name: "carol"}}
}

func names(users []User) []string {
	got := make([]string, 0, len(users))
	for _, u := range users {
		got = append(got, u.Name)
	}
	return got
}

func accountsDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAnAccountMarkedASystemAccountIsNamed(t *testing.T) {
	dir := accountsDir(t, map[string]string{
		"temp_root": "[User]\nLanguage=\nSystemAccount=true\n",
		"bob":       "[User]\nSystemAccount=false\nIcon=/var/lib/AccountsService/icons/bob\n",
		"carol":     "[InputSource0]\nSystemAccount=true\n[User]\nXSession=sway\n",
	})
	if got, want := SystemAccounts(dir, accountUsers()), []string{"temp_root"}; !slices.Equal(got, want) {
		t.Errorf("system accounts = %v, want %v", got, want)
	}
}

func TestNoAccountIsASystemAccountWithoutTheFiles(t *testing.T) {
	if got := SystemAccounts(filepath.Join(t.TempDir(), "absent"), accountUsers()); len(got) != 0 {
		t.Errorf("system accounts = %v, want none", got)
	}
}

func TestAnUnreadableFileListsItsAccountAndSaysWhy(t *testing.T) {
	dir := accountsDir(t, map[string]string{"bob": "[User]\nSystemAccount=true\n"})
	if err := os.Mkdir(filepath.Join(dir, "temp_root"), 0o700); err != nil {
		t.Fatal(err)
	}
	logs := logtest.Capture(t)
	if got, want := SystemAccounts(dir, accountUsers()), []string{"bob"}; !slices.Equal(got, want) {
		t.Errorf("system accounts = %v, want %v", got, want)
	}
	if !strings.Contains(logs.String(), "greeter: listing temp_root:") {
		t.Errorf("log = %q, want the reason", logs.String())
	}
}

func TestSystemAccountsAreLeftOutOfTheList(t *testing.T) {
	logs := logtest.Capture(t)
	got := names(withoutSystemAccounts(accountUsers(), []string{"temp_root"}))
	if want := []string{"ada", "bob", "carol"}; !slices.Equal(got, want) {
		t.Errorf("listed = %v, want %v", got, want)
	}
	if !strings.Contains(logs.String(), "not listing temp_root:") {
		t.Errorf("log = %q, want the left-out account named", logs.String())
	}
}

func TestEveryoneIsListedWhenNoAccountIsASystemAccount(t *testing.T) {
	if got, want := names(withoutSystemAccounts(accountUsers(), nil)), names(accountUsers()); !slices.Equal(got, want) {
		t.Errorf("listed = %v, want %v", got, want)
	}
}
