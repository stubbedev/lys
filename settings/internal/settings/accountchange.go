package settings

import (
	"context"
	"fmt"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

func changeAccountOver(bus func() (*dbus.Conn, error)) func(context.Context, accounts.Request) error {
	return func(ctx context.Context, r accounts.Request) error {
		conn, err := bus()
		if err != nil {
			return fmt.Errorf("reach %s: %w", accounts.BusName, err)
		}
		defer func() { _ = conn.Close() }()
		return accounts.Change(ctx, conn, r)
	}
}
