package launcher

import (
	"context"
	"fmt"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/fileuri"
)

// fileManagerTimeout bounds the call to the file manager, which the bus
// may have to start first.
const fileManagerTimeout = 5 * time.Second

// ShowItem asks the desktop's file manager (lys-files, through
// org.freedesktop.FileManager1) to show path selected in its folder.
func ShowItem(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, fileManagerTimeout)
	defer cancel()
	conn, err := dbusx.DialSession(ctx, fileManagerTimeout)
	if err != nil {
		return fmt.Errorf("show %s in the file manager: %w", path, err)
	}
	defer func() { _ = conn.Close() }()
	call := conn.Object("org.freedesktop.FileManager1", "/org/freedesktop/FileManager1").
		CallWithContext(ctx, "org.freedesktop.FileManager1.ShowItems", 0, []string{fileuri.FromPath(path)}, "")
	if call.Err != nil {
		return fmt.Errorf("show %s in the file manager: %w", path, call.Err)
	}
	return nil
}
