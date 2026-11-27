package busauth

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/peercred"
	"github.com/stubbedev/lys/lib/peercred/peercredtest"
	"github.com/stubbedev/lys/lib/polkit"
	"github.com/stubbedev/lys/lib/polkit/polkittest"
	"github.com/stubbedev/lys/lib/procstat"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestMain(m *testing.M) {
	os.Exit(testhome.Main(func() int { return peercredtest.Main(m.Run) }))
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

const (
	action        = polkit.Action("org.example.busauth.act")
	notAuthorized = "org.example.Busauth.Error.NotAuthorized"
	serviceName   = "org.example.Busauth"
	servicePath   = dbus.ObjectPath("/org/example/Busauth")
	serviceIface  = "org.example.Busauth"
	byPIDFD       = "(by the pidfd the bus gave)"
	byPID         = "(by pid and start time: the bus gave no pidfd)"
)

func ids() (uid, pid uint32) {
	return intconv.Must[uint32](os.Getuid()), intconv.Must[uint32](os.Getpid())
}

type service struct {
	conn    *dbus.Conn
	callers chan Caller
}

func (s service) Act(sender dbus.Sender) *dbus.Error {
	var caller Caller
	err := Answer(s.conn, sender, notAuthorized, func(ctx context.Context, c Caller) error {
		caller = c
		return Polkit(s.conn, action)(ctx, c)
	})
	s.callers <- caller
	return err
}

func systemConn(t *testing.T) *dbus.Conn {
	t.Helper()
	conn, err := dbusx.DialSystem(t.Context(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func serveOnSystemBus(t *testing.T) (client *dbus.Conn, callers <-chan Caller) {
	t.Helper()
	conn := systemConn(t)
	s := service{conn: conn, callers: make(chan Caller, 1)}
	release, err := dbusx.Serve(conn, dbusx.Service{Name: serviceName, Path: servicePath, Interface: serviceIface, Methods: s})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return systemConn(t), s.callers
}

func act(t *testing.T, client *dbus.Conn) error {
	t.Helper()
	return client.Object(serviceName, servicePath).CallWithContext(t.Context(), serviceIface+".Act", 0).Err
}

func busError(t *testing.T, err error) dbus.Error {
	t.Helper()
	e, ok := errors.AsType[dbus.Error](err)
	if !ok || len(e.Body) != 1 {
		t.Fatalf("the reply %v is no D-Bus error with a message", err)
	}
	return e
}

func TestPolkitDecidesAboutTheCallerByThePIDFDTheBusGivesWhichIsClosedAfterTheCall(t *testing.T) {
	me, mePID := ids()
	a := polkittest.Start(t, polkittest.Authorized)
	client, callers := serveOnSystemBus(t)
	if err := act(t, client); err != nil {
		t.Fatalf("an authorized call: %v", err)
	}
	c := <-callers
	if c.UID != me || c.PID != mePID || c.Sender != client.Names()[0] || c.pidfd == nil {
		t.Fatalf("the caller %+v, want uid %d pid %d %s with the pidfd the bus gave", c, me, mePID, client.Names()[0])
	}
	if !strings.Contains(c.String(), byPIDFD) {
		t.Errorf("the caller is named %q, want it to say it is %s", c, byPIDFD)
	}
	checks := a.Checks.Load()
	if len(checks) != 1 || checks[0].Action != action || checks[0].Flags != polkit.AllowUserInteraction {
		t.Fatalf("polkit was asked %+v", checks)
	}
	if checks[0].PIDFD != os.Getpid() {
		t.Errorf("the subject's pidfd names pid %d, want the caller %d", checks[0].PIDFD, os.Getpid())
	}
	if p, ok := checks[0].Subject.(polkit.UnixProcess); !ok || p.UID != me || p.PID != mePID {
		t.Errorf("polkit was asked about %v", checks[0].Subject)
	}
	if _, err := c.pidfd.FD(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("the caller's pidfd after the call: %v, want it closed", err)
	}
}

func TestARefusalIsTheNotAuthorizedErrorWithPolkitsReason(t *testing.T) {
	for verdict, want := range map[string]struct {
		result polkit.AuthorizationResult
		reason string
	}{
		"denied":    {polkittest.Denied, "polkit does not authorize uid"},
		"dismissed": {polkittest.Dismissed, "the authentication for " + string(action) + " was dismissed"},
		"challenge": {polkittest.Challenge, "needs authentication and no polkit agent answered"},
	} {
		polkittest.Start(t, want.result)
		client, callers := serveOnSystemBus(t)
		e := busError(t, act(t, client))
		<-callers
		if reason, _ := e.Body[0].(string); e.Name != notAuthorized || !strings.Contains(reason, want.reason) {
			t.Errorf("%s: %s %q, want %s saying %q", verdict, e.Name, e.Body[0], notAuthorized, want.reason)
		}
	}
}

func TestAPolkitThatCannotBeAskedIsAFailureSayingHowTheCallerWasNamed(t *testing.T) {
	bus := dbustest.Start(t)
	bus.UseAsSystemBus(t)
	client, callers := serveOnSystemBus(t)
	e := busError(t, act(t, client))
	<-callers
	if msg, _ := e.Body[0].(string); e.Name != dbusx.ErrFailed || !strings.Contains(msg, string(action)) || !strings.Contains(msg, byPIDFD) {
		t.Errorf("%s %q, want %s naming %s and %s", e.Name, e.Body[0], dbusx.ErrFailed, action, byPIDFD)
	}
}

func TestACallerTheBusGivesNoPIDFDForIsNamedToPolkitByPIDAndStartTime(t *testing.T) {
	me, mePID := ids()
	c, err := callerOf(":1.7", map[string]dbus.Variant{uidKey: dbus.MakeVariant(me), pidKey: dbus.MakeVariant(mePID)})
	if err != nil {
		t.Fatal(err)
	}
	if c.pidfd != nil || c.Sender != ":1.7" || c.UID != me || c.PID != mePID {
		t.Fatalf("the caller %+v, want uid %d pid %d with no pidfd", c, me, mePID)
	}
	if !strings.Contains(c.String(), byPID) {
		t.Errorf("the caller is named %q, want it to say it is %s", c, byPID)
	}
	a := polkittest.Start(t, polkittest.Authorized)
	if err := Polkit(systemConn(t), action)(t.Context(), c); err != nil {
		t.Fatalf("an authorized caller: %v", err)
	}
	st, err := procstat.Of(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	checks := a.Checks.Load()
	if len(checks) != 1 || checks[0].PIDFD != 0 {
		t.Fatalf("polkit was asked %+v, want one check with no pidfd", checks)
	}
	if p, ok := checks[0].Subject.(polkit.UnixProcess); !ok || p.UID != me || p.PID != mePID || p.StartTime != st.StartTime {
		t.Errorf("polkit was asked about %+v, want uid %d pid %d started at %d", checks[0].Subject, me, mePID, st.StartTime)
	}
}

func TestAPolkitThatCannotBeAskedAboutACallerWithoutAPIDFDSaysSo(t *testing.T) {
	me, mePID := ids()
	bus := dbustest.Start(t)
	bus.UseAsSystemBus(t)
	err := Polkit(systemConn(t), action)(t.Context(), Caller{UID: me, PID: mePID})
	if _, refused := errors.AsType[Refusal](err); err == nil || refused || !strings.Contains(err.Error(), byPID) {
		t.Errorf("%v, want a failure, not a refusal, saying it was %s", err, byPID)
	}
}

func TestACallerWhosePIDFDNamesAnExitedProcessIsNotAskedAbout(t *testing.T) {
	me, _ := ids()
	a := polkittest.Start(t, polkittest.Authorized)
	peer := peercredtest.Start(t)
	raw, err := unix.PidfdOpen(peer.PID, 0)
	if err != nil {
		t.Fatal(err)
	}
	pid := intconv.Must[int32](peer.PID)
	pidfd := peercred.Adopt(raw, peercred.Credentials{UID: me, PID: pid})
	t.Cleanup(func() { _ = pidfd.Close() })
	peer.Reap(t)
	err = Polkit(systemConn(t), action)(t.Context(), Caller{UID: me, PID: intconv.Must[uint32](pid), pidfd: pidfd})
	if _, refused := errors.AsType[Refusal](err); err == nil || refused || !strings.Contains(err.Error(), "pid "+strconv.Itoa(peer.PID)) || !strings.Contains(err.Error(), byPIDFD) {
		t.Errorf("%v, want a failure naming pid %d %s", err, peer.PID, byPIDFD)
	}
	if checks := a.Checks.Load(); len(checks) != 0 {
		t.Errorf("polkit was asked about an exited caller: %+v", checks)
	}
}

func openPIDFD(t *testing.T) (fd int, watch fdtest.Watch) {
	t.Helper()
	fd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		t.Fatal(err)
	}
	watch = fdtest.WatchFD(t, fd)
	t.Cleanup(func() {
		if watch.Open(t) {
			_ = unix.Close(fd)
		}
	})
	return fd, watch
}

func TestCredentialsTheBusCannotNameACallerByAreAnErrorThatClosesTheirPIDFD(t *testing.T) {
	me, mePID := ids()
	for name, creds := range map[string]func(fd int) map[string]dbus.Variant{
		"no uid": func(fd int) map[string]dbus.Variant {
			return map[string]dbus.Variant{pidKey: dbus.MakeVariant(mePID), processFDKey: dbus.MakeVariant(dbus.UnixFD(fd))}
		},
		"no pid": func(fd int) map[string]dbus.Variant {
			return map[string]dbus.Variant{uidKey: dbus.MakeVariant(me), processFDKey: dbus.MakeVariant(dbus.UnixFD(fd))}
		},
		"a pid past int32": func(fd int) map[string]dbus.Variant {
			return map[string]dbus.Variant{uidKey: dbus.MakeVariant(me), pidKey: dbus.MakeVariant(uint32(1 << 31)), processFDKey: dbus.MakeVariant(dbus.UnixFD(fd))}
		},
	} {
		fd, watch := openPIDFD(t)
		if c, err := callerOf(":1.7", creds(fd)); err == nil || !strings.Contains(err.Error(), ":1.7") {
			t.Errorf("%s: %+v, %v; want an error naming the sender", name, c, err)
		}
		if watch.Open(t) {
			t.Errorf("%s: the ProcessFD stays open", name)
		}
	}
}

func TestAProcessFDThatIsNoDescriptorIsAnError(t *testing.T) {
	me, mePID := ids()
	for name, v := range map[string]dbus.Variant{
		"a string":            dbus.MakeVariant("3"),
		"a negative":          dbus.MakeVariant(dbus.UnixFD(-1)),
		"an unresolved index": dbus.MakeVariant(dbus.UnixFDIndex(0)),
	} {
		creds := map[string]dbus.Variant{uidKey: dbus.MakeVariant(me), pidKey: dbus.MakeVariant(mePID), processFDKey: v}
		if c, err := callerOf(":1.7", creds); err == nil || !strings.Contains(err.Error(), "not a pidfd") {
			t.Errorf("%s: %+v, %v; want an error saying it is not a pidfd", name, c, err)
		}
	}
}

func TestRunOfASenderTheBusDoesNotKnowRunsNothing(t *testing.T) {
	bus := dbustest.Start(t)
	bus.UseAsSystemBus(t)
	ran := false
	err := Run(t.Context(), systemConn(t), ":1.4242", func(context.Context, Caller) error { ran = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "read the credentials of :1.4242") || ran {
		t.Errorf("%v (ran: %v), want an error reading the credentials and nothing run", err, ran)
	}
}

func TestReplyAnswersARefusalAsNotAuthorizedAndAnyOtherErrorAsFailed(t *testing.T) {
	if e := Reply(nil, notAuthorized); e != nil {
		t.Errorf("Reply of nil = %v", e)
	}
	refusal := errors.Join(errors.New("closing"), Refusal{Reason: "no"})
	if e := Reply(refusal, notAuthorized); e == nil || e.Name != notAuthorized || e.Body[0] != "no" {
		t.Errorf("Reply of a refusal = %v", e)
	}
	if e := Reply(errors.New("broke"), notAuthorized); e == nil || e.Name != dbusx.ErrFailed || e.Body[0] != "broke" {
		t.Errorf("Reply of a failure = %v", e)
	}
}
