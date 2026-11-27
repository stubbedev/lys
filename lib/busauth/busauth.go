// Package busauth names who calls a service over the system bus and
// asks polkit whether they may: by the pidfd the bus names the caller
// by where it offers one (dbus-broker, dbus-daemon 1.15 and later), so
// that polkit 124 and later decide about that very process, and by its
// pid and start time where the bus offers none.
package busauth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/peercred"
	"github.com/stubbedev/lys/lib/polkit"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// The credentials GetConnectionCredentials gives that a Caller is made
// of.
const (
	credentialsMethod = "org.freedesktop.DBus.GetConnectionCredentials"
	uidKey            = "UnixUserID"
	pidKey            = "ProcessID"
	processFDKey      = "ProcessFD"
)

// Caller is the process behind a method call. Within Run it also holds
// the pidfd the bus named it by, when the bus offered one.
type Caller struct {
	Sender string
	UID    uint32
	PID    uint32
	pidfd  *peercred.PIDFD
}

// String names the caller and how polkit is told which process it is.
func (c Caller) String() string {
	who := "uid " + strconv.FormatUint(uint64(c.UID), 10) + " pid " + strconv.FormatUint(uint64(c.PID), 10)
	if c.pidfd != nil {
		return who + " (by the pidfd the bus gave)"
	}
	return who + " (by pid and start time: the bus gave no pidfd)"
}

func (c Caller) process() (polkit.UnixProcess, error) {
	if c.pidfd != nil {
		return polkit.ProcessOfPIDFD(c.pidfd)
	}
	pid, err := intconv.Exact[int32](c.PID)
	if err != nil {
		return polkit.UnixProcess{}, err
	}
	return polkit.ProcessOf(peercred.Credentials{UID: c.UID, PID: pid})
}

// Refusal is a call polkit did not authorize, and why.
type Refusal struct{ Reason string }

func (r Refusal) Error() string { return r.Reason }

// Run asks the bus who sender is and runs do for that caller, closing
// the pidfd the bus gave once do returns.
func Run(ctx context.Context, conn *dbus.Conn, sender dbus.Sender, do func(context.Context, Caller) error) error {
	var creds map[string]dbus.Variant
	if err := conn.BusObject().CallWithContext(ctx, credentialsMethod, 0, string(sender)).Store(&creds); err != nil {
		return fmt.Errorf("read the credentials of %s: %w", sender, err)
	}
	c, err := callerOf(sender, creds)
	if err != nil {
		return err
	}
	err = do(ctx, c)
	if c.pidfd == nil {
		return err
	}
	if cerr := c.pidfd.Close(); cerr != nil {
		return errors.Join(err, fmt.Errorf("after the call of %s: %w", c, cerr))
	}
	return err
}

func callerOf(sender dbus.Sender, creds map[string]dbus.Variant) (Caller, error) {
	uid, uidOK := creds[uidKey].Value().(uint32)
	pid, pidOK := creds[pidKey].Value().(uint32)
	v, offered := creds[processFDKey]
	fd, fdOK := v.Value().(dbus.UnixFD)
	switch {
	case !uidOK || !pidOK:
		return Caller{}, errors.Join(fmt.Errorf("the bus gives %s no uid or pid: %v", sender, creds), dbus.CloseFDs([]any{creds}))
	case !offered:
		return Caller{Sender: string(sender), UID: uid, PID: pid}, nil
	case !fdOK || fd < 0:
		return Caller{}, errors.Join(fmt.Errorf("the bus gives %s a %s of %s %v, not a pidfd", sender, processFDKey, v.Signature(), v.Value()), dbus.CloseFDs([]any{creds}))
	}
	peerPID, err := intconv.Exact[int32](pid)
	if err != nil {
		return Caller{}, errors.Join(fmt.Errorf("the bus gives %s pid %d: %w", sender, pid, err), dbus.CloseFDs([]any{creds}))
	}
	return Caller{Sender: string(sender), UID: uid, PID: pid, pidfd: peercred.Adopt(int(fd), peercred.Credentials{UID: uid, PID: peerPID})}, nil
}

// AuthTimeout bounds a call that may wait for the caller to authenticate.
const AuthTimeout = 2 * time.Minute

// Reply is the answer to a method call whose work ended in err: none
// for nil, the error named notAuthorized for a Refusal, and a Failed
// for any other.
func Reply(err error, notAuthorized string) *dbus.Error {
	if r, ok := errors.AsType[Refusal](err); ok {
		return dbus.NewError(notAuthorized, []any{r.Reason})
	}
	if err != nil {
		return dbusx.Failed(err.Error())
	}
	return nil
}

// Answer runs do for the caller behind sender, within AuthTimeout, and
// answers the method call as Reply does.
func Answer(conn *dbus.Conn, sender dbus.Sender, notAuthorized string, do func(context.Context, Caller) error) *dbus.Error {
	ctx, cancel := context.WithTimeout(context.Background(), AuthTimeout)
	defer cancel()
	return Reply(Run(ctx, conn, sender, do), notAuthorized)
}

func check(ctx context.Context, conn *dbus.Conn, c Caller, action polkit.Action) (polkit.AuthorizationResult, error) {
	process, err := c.process()
	if err != nil {
		return polkit.AuthorizationResult{}, err
	}
	return polkit.NewAuthority(conn).CheckAuthorization(ctx, process, action, polkit.AllowUserInteraction)
}

// Authorizer decides whether a caller may: nil when it may, a Refusal
// when polkit says no.
type Authorizer func(ctx context.Context, c Caller) error

// Polkit asks polkit on conn whether a caller may take action, letting it
// authenticate.
func Polkit(conn *dbus.Conn, action polkit.Action) Authorizer {
	return func(ctx context.Context, c Caller) error {
		result, err := check(ctx, conn, c, action)
		if err != nil {
			return fmt.Errorf("ask polkit whether %s may %s: %w", c, action, err)
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
			return fmt.Errorf("polkit: unknown verdict %s on %s for %s", verdict, action, c)
		}
	}
}
