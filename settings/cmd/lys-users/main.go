// Command lys-users is the accounts service lys-settings changes user
// accounts through: activated as root on the system bus, it asks polkit
// whether each caller may and edits the account files itself.
//
//	lys-users [--nixos FILE]
package main

import (
	"os"

	"github.com/stubbedev/lys/settings/internal/usershelper"
)

func main() {
	os.Exit(usershelper.Main(os.Args[1:], os.Stderr))
}
