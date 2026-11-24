package greeter

import (
	"bufio"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// AccountsServiceUsersDir is where AccountsService left a file per
// account it knew, readable by root alone.
const AccountsServiceUsersDir = "/var/lib/AccountsService/users"

// SystemAccounts is the users whose file in dir marks them a system
// account: on Ubuntu an administrator's spare account has a login shell
// and a regular UID, and only this tells it from a person's. An account
// without a file is a person's; one whose file cannot be read is
// logged and listed, as the list is a convenience and a wrongly short
// one would hide a person. Only root reads dir, so the display manager
// asks and hands the answer to the greeter.
func SystemAccounts(dir string, users []User) []string {
	var system []string
	for _, u := range users {
		marked, err := markedSystemAccount(filepath.Join(dir, u.Name))
		if err != nil {
			log.Printf("greeter: listing %s: %v", u.Name, err)
			continue
		}
		if marked {
			system = append(system, u.Name)
		}
	}
	return system
}

func markedSystemAccount(path string) (bool, error) {
	f, err := os.Open(path) //nolint:gosec // a file under AccountsServiceUsersDir, named by a passwd entry
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && section == "User" && strings.TrimSpace(key) == "SystemAccount" {
			v := strings.TrimSpace(value)
			return v == "true" || v == "1", nil
		}
	}
	return false, sc.Err()
}

func withoutSystemAccounts(users []User, system []string) []User {
	listed := make([]User, 0, len(users))
	for _, u := range users {
		if slices.Contains(system, u.Name) {
			log.Printf("greeter: not listing %s: AccountsService marked it a system account", u.Name)
			continue
		}
		listed = append(listed, u)
	}
	return listed
}
