// Package usershelper is lys-users, the accounts service: run as root
// and activated on the system bus as accounts.BusName, it takes one
// change to the user accounts per call, asks polkit whether the caller
// may make it, and makes it in the account files itself, answering a
// refusal or a failure with an accounts.Error.
package usershelper

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/peercred"
	"github.com/stubbedev/lys/lib/polkit"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

const (
	// callTimeout bounds one call, the caller's authentication included.
	callTimeout = 10 * time.Minute
	maxRequest  = 2*accounts.MaxPictureBytes + 64<<10
	busTimeout  = 10 * time.Second
)

type caller struct{ uid, pid uint32 }

func callerOf(ctx context.Context, conn *dbus.Conn, sender dbus.Sender) (caller, error) {
	var creds map[string]dbus.Variant
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionCredentials", 0, string(sender)).Store(&creds); err != nil {
		return caller{}, fmt.Errorf("read the credentials of %s: %w", sender, err)
	}
	uid, uidOK := creds["UnixUserID"].Value().(uint32)
	pid, pidOK := creds["ProcessID"].Value().(uint32)
	if !uidOK || !pidOK {
		return caller{}, fmt.Errorf("the bus gives %s no uid or pid", sender)
	}
	return caller{uid: uid, pid: pid}, nil
}

func authorize(ctx context.Context, conn *dbus.Conn, c caller, action polkit.Action) *accounts.Error {
	pid, err := intconv.Exact[int32](c.pid)
	if err != nil {
		return accounts.Errorf(accounts.ErrFailed, "ask polkit about pid %d of uid %d: %v", c.pid, c.uid, err)
	}
	process, err := polkit.ProcessOf(peercred.Credentials{UID: c.uid, PID: pid})
	if err != nil {
		return accounts.Errorf(accounts.ErrFailed, "%v", err)
	}
	result, err := polkit.NewAuthority(conn).CheckAuthorization(ctx, process, action, polkit.AllowUserInteraction)
	if err != nil {
		return accounts.Errorf(accounts.ErrFailed, "%v", err)
	}
	switch v := result.Verdict(); v {
	case polkit.Authorized:
		return nil
	case polkit.Dismissed:
		return accounts.Errorf(accounts.ErrNotAuthorized, "the authentication for %s was dismissed", action)
	case polkit.Challenge:
		return accounts.Errorf(accounts.ErrNotAuthorized, "%s needs authentication and no polkit agent answered for uid %d", action, c.uid)
	case polkit.Denied:
		return accounts.Errorf(accounts.ErrNotAuthorized, "polkit does not authorize uid %d for %s", c.uid, action)
	default:
		return accounts.Errorf(accounts.ErrFailed, "polkit's verdict %s on %s", v, action)
	}
}

// service is the exported object; Change is its one method.
type service struct {
	conn *dbus.Conn
	sys  system
	log  *log.Logger
}

// Change makes the encoded accounts.Request for the caller polkit
// authorises for its class.
func (s service) Change(sender dbus.Sender, request []byte) *dbus.Error {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if aerr := s.change(ctx, sender, request); aerr != nil {
		s.log.Printf("refused a request of %s: %v", sender, aerr)
		return aerr.BusError()
	}
	return nil
}

func (s service) change(ctx context.Context, sender dbus.Sender, request []byte) *accounts.Error {
	if len(request) > maxRequest {
		return accounts.Errorf(accounts.ErrInvalid, "the request is over %d bytes", maxRequest)
	}
	r, err := accounts.Decode(request)
	if err != nil {
		return accounts.Errorf(accounts.ErrInvalid, "%v", err)
	}
	c, err := callerOf(ctx, s.conn, sender)
	if err != nil {
		return accounts.Errorf(accounts.ErrFailed, "%v", err)
	}
	if aerr := authorize(ctx, s.conn, c, polkit.Action(r.Class().Action())); aerr != nil {
		return aerr
	}
	return s.sys.change(ctx, c.uid, r)
}

// serve exports the service on conn and takes its name.
func serve(conn *dbus.Conn, sys system, logger *log.Logger) (func(), error) {
	release, err := dbusx.Serve(conn, dbusx.Service{Name: accounts.BusName, Path: accounts.BusPath, Interface: accounts.BusInterface, Methods: service{conn: conn, sys: sys, log: logger}})
	if err != nil {
		return nil, fmt.Errorf("serve %s: %w", accounts.BusName, err)
	}
	return release, nil
}

var errUsage = errors.New("usage")

func parseArgs(args []string, stderr io.Writer) (nixosFile string, err error) {
	fs := flag.NewFlagSet("lys-users", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&nixosFile, "nixos", "", "the `file` in which the lys-settings NixOS module says what the configuration declares")
	if err := fs.Parse(args); err != nil {
		return "", errUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "lys-users takes no arguments, only flags: %q\n", fs.Args())
		return "", errUsage
	}
	return nixosFile, nil
}

// Main is lys-users with args after the program name: it serves the
// system bus until SIGTERM or SIGINT. It exits 0 once stopped, 1 when
// it cannot serve, and 2 on a malformed command line.
func Main(args []string, stderr io.Writer) int {
	logger := log.New(stderr, "lys-users: ", 0)
	nixosFile, err := parseArgs(args, stderr)
	if err != nil {
		return 2
	}
	sys := liveSystem(nil)
	if sys.nixos, err = readNixOS(sys.etc, nixosFile); err != nil {
		logger.Print(err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := run(ctx, sys, logger); err != nil {
		logger.Print(err)
		return 1
	}
	return 0
}

func run(ctx context.Context, sys system, logger *log.Logger) error {
	conn, err := dbusx.DialSystem(ctx, busTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	release, err := serve(conn, sys, logger)
	if err != nil {
		return err
	}
	defer release()
	<-ctx.Done()
	return nil
}
