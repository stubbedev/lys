package main

import (
	"context"

	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/internal/cli"
	"github.com/stubbedev/lys/shell/service/sessionctl"
)

// logoutCommand ends the user's session as the power menu's built-in
// Log out does, dialling the system bus only when logind ends it.
func logoutCommand() *cli.Command {
	return &cli.Command{
		Name:  "logout",
		About: "Log out of the session through lys-wm or logind",
		Run: func(*cli.Matches) error {
			ctx := context.Background()
			var conn *dbus.Conn
			defer func() {
				if conn != nil {
					_ = conn.Close()
				}
			}()
			return sessionctl.Logout(ctx, func() (*dbus.Conn, error) {
				c, err := logind.Dial(ctx)
				conn = c
				return c, err
			})
		},
	}
}
