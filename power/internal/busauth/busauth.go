// Package busauth names who calls lys-power over the system bus and asks
// polkit whether they may.
package busauth

import (
	"context"
	"fmt"

	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/peercred"
	"github.com/stubbedev/lys/lib/polkit"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// Caller is the process behind a method call.
type Caller struct {
	Sender string
	UID    uint32
	PID    uint32
}

// Refusal is a call polkit did not authorize, and why.
type Refusal struct{ Reason string }

func (r Refusal) Error() string { return r.Reason }

// Of asks the bus who sender is.
func Of(ctx context.Context, conn *dbus.Conn, sender dbus.Sender) (Caller, error) {
	var creds map[string]dbus.Variant
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionCredentials", 0, string(sender)).Store(&creds); err != nil {
		return Caller{}, fmt.Errorf("read the credentials of %s: %w", sender, err)
	}
	uid, uidOK := creds["UnixUserID"].Value().(uint32)
	pid, pidOK := creds["ProcessID"].Value().(uint32)
	if !uidOK || !pidOK {
		return Caller{}, fmt.Errorf("the bus gives %s no uid or pid: %v", sender, creds)
	}
	return Caller{Sender: string(sender), UID: uid, PID: pid}, nil
}

// Authorizer decides whether a caller may: nil when it may, a Refusal
// when polkit says no.
type Authorizer func(ctx context.Context, c Caller) error

// Polkit asks polkit on conn whether a caller may take action, letting it
// authenticate.
func Polkit(conn *dbus.Conn, action polkit.Action) Authorizer {
	return func(ctx context.Context, c Caller) error {
		pid, err := intconv.Exact[int32](c.PID)
		if err != nil {
			return fmt.Errorf("ask polkit about pid %d of uid %d: %w", c.PID, c.UID, err)
		}
		process, err := polkit.ProcessOf(peercred.Credentials{UID: c.UID, PID: pid})
		if err != nil {
			return err
		}
		result, err := polkit.NewAuthority(conn).CheckAuthorization(ctx, process, action, polkit.AllowUserInteraction)
		if err != nil {
			return err
		}
		switch verdict := result.Verdict(); verdict {
		case polkit.Authorized:
			return nil
		case polkit.Dismissed:
			return Refusal{Reason: fmt.Sprintf("the authentication for %s was dismissed", action)}
		case polkit.Challenge:
			return Refusal{Reason: fmt.Sprintf("%s needs authentication and no polkit agent answered for uid %d", action, c.UID)}
		case polkit.Denied:
			return Refusal{Reason: fmt.Sprintf("polkit does not authorize uid %d for %s", c.UID, action)}
		default:
			return fmt.Errorf("polkit: unknown verdict %s on %s", verdict, action)
		}
	}
}
