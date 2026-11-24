// Command lys-users-helper changes a user account for lys-settings: run
// as root through pkexec, it reads one request from its standard input
// and makes it with the shadow tools.
//
//	pkexec lys-users-helper self|others < REQUEST
package main

import (
	"os"

	"github.com/stubbedev/lys/settings/internal/usershelper"
)

func main() {
	os.Exit(usershelper.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv("PKEXEC_UID"), os.Geteuid()))
}
