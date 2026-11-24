package greeter

import (
	"log"
	"os"
	"sort"
	"strings"

	"github.com/stubbedev/lys/desktop/useraccount"
)

// User is one login account offered in the user list (users.rs).
type User struct {
	// Name is the login name greetd authenticates.
	Name string
	// UID keys the cursor record the account's last session left.
	UID uint32
	// DisplayName is the GECOS full name, else Name.
	DisplayName string
	// Home seeds ~/.face and the cursor read from dotfiles (it may be
	// unreadable).
	Home string
	// Icon is a readable avatar path, "" when none.
	Icon string
}

// ReadUsers reads the people's accounts from the passwd database, by the
// user ID range of login.defs.
func ReadUsers() []User {
	data, err := os.ReadFile(useraccount.PasswdPath)
	if err != nil {
		log.Printf("greeter: listing nobody: %v", err)
	}
	return usersOf(string(data), useraccount.ReadUIDRange(useraccount.LoginDefsPath))
}

// LoadUsers is what the user list offers: ReadUsers without the system
// accounts named, with the avatars resolved.
func LoadUsers(systemAccounts []string) []User {
	users := withoutSystemAccounts(ReadUsers(), systemAccounts)
	names := make([]string, len(users))
	for i := range users {
		users[i].Icon = useraccount.FindAvatar(users[i].Name, users[i].Home)
		names[i] = users[i].Name
	}
	log.Printf("greeter: listing %d login users: %s", len(users), strings.Join(names, " "))
	return users
}

func usersOf(passwd string, r useraccount.UIDRange) []User {
	people := useraccount.People(passwd, r)
	users := make([]User, len(people))
	for i, e := range people {
		users[i] = User{Name: e.Name, UID: e.UID, DisplayName: e.DisplayName(), Home: e.Home}
	}
	sort.SliceStable(users, func(i, j int) bool {
		return strings.ToLower(users[i].DisplayName) < strings.ToLower(users[j].DisplayName)
	})
	return users
}
