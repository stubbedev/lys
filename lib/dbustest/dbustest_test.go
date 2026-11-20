package dbustest

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/fdtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

type echo struct{}

func (echo) Echo(s string) (string, *dbus.Error) { return s, nil }

// The name is long on purpose: t.TempDir() embeds it, and a socket
// under that path would overflow sun_path.
func TestAPrivateBusStartsEvenWhenTheTestNameIsFarLongerThanAUnixSocketPathMayBeWithoutBeingTruncated(t *testing.T) {
	bus := Start(t)
	if bus.Address == "" {
		t.Fatal("no address")
	}
	_ = bus.Conn(t)
}

func TestPrivateBusCarriesCallsBetweenPeers(t *testing.T) {
	bus := Start(t)
	server := bus.Conn(t)
	if err := server.Export(echo{}, "/test", "com.example.Echo"); err != nil {
		t.Fatal(err)
	}
	if reply, err := server.RequestName("com.example.Echo", dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("RequestName = %v, %v", reply, err)
	}
	client := bus.Conn(t)
	var got string
	if err := client.Object("com.example.Echo", "/test").Call("com.example.Echo.Echo", 0, "hi").Store(&got); err != nil {
		t.Fatal(err)
	}
	if got != "hi" {
		t.Errorf("echo = %q", got)
	}
	// An unowned name is an error, not a hang.
	if err := client.Object("com.example.Missing", "/test").Call("com.example.Echo.Echo", 0, "hi").Err; err == nil {
		t.Error("call to an unowned name: want an error")
	}
}

// A bus stopped and started again at the same socket is a new bus at
// the same address: connections to the old one are gone, a fresh dial
// of the path reaches the new one, and the cleanup's second Stop is
// harmless.
func TestABusRestartsAtTheSameSocket(t *testing.T) {
	sock := SocketPath(t)
	first := StartAt(t, sock)
	old := first.Conn(t)
	first.Stop(t)
	if err := old.BusObject().Call("org.freedesktop.DBus.Peer.Ping", 0).Err; err == nil {
		t.Error("a connection to the stopped bus still answers")
	}
	second := StartAt(t, sock)
	if second.Address == first.Address {
		t.Errorf("the restarted bus kept the address %s, guid included", first.Address)
	}
	conn, err := dbus.Connect("unix:path=" + sock)
	if err != nil {
		t.Fatalf("dial the restarted bus at %s: %v", sock, err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.BusObject().Call("org.freedesktop.DBus.Peer.Ping", 0).Err; err != nil {
		t.Errorf("ping the restarted bus: %v", err)
	}
}

// A missing dbus-daemon is an error that names it, which StartAt turns
// into a failed test rather than a skip.
func TestLaunchWithoutADaemonFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := launch(t.TempDir(), SocketPath(t), time.After)
	if !errors.Is(err, exec.ErrNotFound) || !strings.Contains(err.Error(), "dbus-daemon") {
		t.Fatalf("launch without dbus-daemon = %v, want a not-found error naming it", err)
	}
}

type fakeDaemon struct {
	dir, started, release string
}

func installFakeDaemon(t *testing.T, script string) fakeDaemon {
	t.Helper()
	d := fakeDaemon{dir: t.TempDir()}
	d.started, d.release = filepath.Join(d.dir, "started"), filepath.Join(d.dir, "release")
	for _, fifo := range []string{d.started, d.release} {
		if err := unix.Mkfifo(fifo, 0o600); err != nil {
			t.Fatalf("mkfifo %s: %v", fifo, err)
		}
	}
	bin := filepath.Join(d.dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nSTARTED=" + d.started + "\nRELEASE=" + d.release + "\n" + script
	fdtest.WriteExecutable(t, filepath.Join(bin, "dbus-daemon"), []byte(body))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return d
}

type launched struct {
	bus *Bus
	err error
}

func launchWithGuard(t *testing.T, d fakeDaemon) (results <-chan launched, armed <-chan time.Duration, fire chan<- time.Time) {
	t.Helper()
	out := make(chan launched, 1)
	arm := make(chan time.Duration, 1)
	trigger := make(chan time.Time, 1)
	after := func(d time.Duration) <-chan time.Time {
		arm <- d
		return trigger
	}
	go func() {
		b, err := launch(d.dir, SocketPath(t), after)
		out <- launched{b, err}
	}()
	return out, arm, trigger
}

func (d fakeDaemon) awaitStart(t *testing.T) {
	t.Helper()
	f, err := os.Open(d.started)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := bufio.NewReader(f).ReadString('\n'); err != nil {
		t.Fatalf("the fake daemon's start signal: %v", err)
	}
}

func TestLaunchWaitsForTheAddressHoweverLongTheDaemonTakes(t *testing.T) {
	d := installFakeDaemon(t, "echo started > \"$STARTED\"\nread line < \"$RELEASE\"\necho \"$line\"\nexec sleep 600\n")
	results, armed, _ := launchWithGuard(t, d)
	d.awaitStart(t)
	if guard := <-armed; guard != hangGuard || guard < time.Minute {
		t.Fatalf("launch armed a %v guard, want the %v hang guard", guard, hangGuard)
	}
	select {
	case r := <-results:
		t.Fatalf("launch returned %+v before the daemon printed its address", r)
	default:
	}
	const address = "unix:path=/nowhere,guid=0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(d.release, []byte(address+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := <-results
	if r.err != nil || r.bus.Address != address {
		t.Fatalf("launch = %+v, want the printed address %s", r, address)
	}
	r.bus.Stop(t)
}

func TestLaunchStopsADaemonThatHangsWithoutAnAddress(t *testing.T) {
	d := installFakeDaemon(t, "echo stuck >&2\necho started > \"$STARTED\"\nread line < \"$RELEASE\"\n")
	results, armed, fire := launchWithGuard(t, d)
	d.awaitStart(t)
	<-armed
	fire <- time.Time{}
	r := <-results
	if r.err == nil || !strings.Contains(r.err.Error(), "hung without printing its address") || !strings.Contains(r.err.Error(), "stuck") {
		t.Fatalf("launch of a hung daemon = %v, want the hang named with its stderr", r.err)
	}
}

func TestLaunchFailsAtOnceWhenTheDaemonExitsWithoutAnAddress(t *testing.T) {
	d := installFakeDaemon(t, "echo 'Failed to start message bus: no config' >&2\nexit 1\n")
	results, _, _ := launchWithGuard(t, d)
	r := <-results
	if r.err == nil || !strings.Contains(r.err.Error(), "printed no address") || !strings.Contains(r.err.Error(), "no config") {
		t.Fatalf("launch of a failing daemon = %v, want its stderr", r.err)
	}
}

func TestVarIsOrderedAcrossGoroutines(t *testing.T) {
	var v Var[int]
	if v.Load() != 0 {
		t.Fatal("the zero Var holds the zero value")
	}
	done := make(chan struct{})
	for range 8 {
		go func() {
			v.Update(func(n int) int { return n + 1 })
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
	if got := v.Load(); got != 8 {
		t.Fatalf("after 8 updates = %d, want 8 (an update was lost)", got)
	}
	v.Store(3)
	if got := v.Load(); got != 3 {
		t.Fatalf("after Store(3) = %d", got)
	}
}
