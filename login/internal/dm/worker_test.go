package dm

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/userdb"
	"github.com/stubbedev/lys/lib/userdb/userdbtest"
	"github.com/stubbedev/lys/lib/waittest"
)

// useUserDB looks accounts up in the user database at path until the
// test ends.
func useUserDB(t *testing.T, path string) {
	t.Helper()
	prev := userDB
	userDB = path
	t.Cleanup(func() { userDB = prev })
}

func TestStopGroupStopsTheCompositorBeforeItsClients(t *testing.T) {
	dir := t.TempDir()
	childPid, verdict := filepath.Join(dir, "child"), filepath.Join(dir, "verdict")
	script := `trap 'if kill -0 "$(cat "$1")" 2>/dev/null; then echo alive; else echo gone; fi > "$2"; exit 0' TERM; sleep 30 & echo $! > "$1"; wait`
	cmd := exec.Command("sh", "-c", script, "compositor", childPid, verdict)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	var child int
	waittest.Until(t, func() bool {
		if b, err := os.ReadFile(childPid); err == nil {
			child, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		return child != 0
	}, "the stand-in compositor to start its client")
	if err := stopGroup("the test", cmd, exited); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(verdict); err != nil || strings.TrimSpace(string(b)) != "alive" {
		t.Fatalf("the client was %q when the compositor got SIGTERM (%v), want still alive", b, err)
	}
	waittest.Until(t, func() bool { return syscall.Kill(child, 0) != nil }, "the compositor's client to end with the stopped group")
}

func TestALoginRegistersTheTypeOfTheSessionItStarts(t *testing.T) {
	o := WorkerOptions{VT: 7}
	for _, sessionType := range []string{"wayland", "x11"} {
		got, err := o.loginEnv([]string{"LANG=C", "XDG_SESSION_TYPE=" + sessionType})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"XDG_SESSION_TYPE=" + sessionType, "XDG_SESSION_CLASS=user", "XDG_SEAT=seat0", "XDG_VTNR=7"}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", sessionType, got, want)
		}
	}
}

func TestTheLastSessionTypeInAStartRequestWins(t *testing.T) {
	got, err := WorkerOptions{VT: 1}.loginEnv([]string{"XDG_SESSION_TYPE=wayland", "XDG_SESSION_TYPE=x11"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, "XDG_SESSION_TYPE=x11") {
		t.Errorf("%q takes the first type, not the --env override after it", got)
	}
}

func TestAStartRequestWithoutASessionTypeIsRefused(t *testing.T) {
	if _, err := (WorkerOptions{VT: 1}).loginEnv([]string{"LANG=C"}); err == nil || !strings.Contains(err.Error(), "XDG_SESSION_TYPE") {
		t.Fatalf("err = %v", err)
	}
}

func TestAnAccountIsItsUserRecordWithItsGroups(t *testing.T) {
	useUserDB(t, userdbtest.Serve(t, userdbtest.DB{
		Users:       []userdb.User{{Name: "carol", UID: 70001, GID: 100, Home: "/home/carol", Shell: "/bin/zsh"}, {Name: "dave", UID: 70002, GID: 70002, Home: "/home/dave"}},
		Groups:      []userdb.Group{{Name: "users", GID: 100}, {Name: "wheel", GID: 1}, {Name: "video", GID: 26}},
		Memberships: map[string][]string{"carol": {"wheel", "users", "video"}},
	}))
	a, err := lookupAccount("carol")
	want := account{name: "carol", home: "/home/carol", shell: "/bin/zsh", uid: 70001, gid: 100, groups: []uint32{100, 1, 26}}
	if err != nil || !reflect.DeepEqual(a, want) {
		t.Errorf("carol = %+v, %v; want %+v", a, err, want)
	}
	a, err = lookupAccount("dave")
	want = account{name: "dave", home: "/home/dave", shell: defaultShell, uid: 70002, gid: 70002, groups: []uint32{70002}}
	if err != nil || !reflect.DeepEqual(a, want) {
		t.Errorf("dave = %+v, %v; want %+v, the default shell and only his primary group", a, err, want)
	}
}

func TestAnAccountThatCannotBeResolvedIsRefused(t *testing.T) {
	useUserDB(t, userdbtest.Serve(t, userdbtest.DB{
		Users:       []userdb.User{{Name: "carol", UID: 70001, GID: 70001, Home: "/home/carol"}, {Name: "homeless", UID: 70003, GID: 70003}},
		Memberships: map[string][]string{"carol": {"gone"}},
	}))
	for name, want := range map[string]string{
		"mallory":  string(userdb.NoRecordFound),
		"homeless": "no home directory",
		"carol":    "carol's group gone",
	} {
		if a, err := lookupAccount(name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s = %+v, %v; want an error with %q", name, a, err, want)
		}
	}
}

func TestAnAccountWithoutTheUserDatabaseIsRefused(t *testing.T) {
	path := userdbtest.Missing(t)
	useUserDB(t, path)
	if a, err := lookupAccount("carol"); err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("lookupAccount = %+v, %v; want an error naming %s", a, err, path)
	}
}

const workerSession logind.SessionID = "42"

// watchedSession is a fake logind with workerSession, the manager a
// worker waits on, and the activations it watches from before PAM opens
// the session.
func watchedSession(t *testing.T) (*logindtest.Logind, logind.Manager, *logind.Activations) {
	t.Helper()
	fake := logindtest.Start(t, logindtest.Config{Own: workerSession})
	m := logind.NewManager(fake.Bus.Conn(t))
	activations, err := m.WatchActivations(t.Context())
	if err != nil {
		t.Fatalf("WatchActivations: %v", err)
	}
	t.Cleanup(func() { _ = activations.Stop() })
	return fake, m, activations
}

func TestTheCompositorWaitsUntilItsSessionIsActive(t *testing.T) {
	fake, m, activations := watchedSession(t)
	if !fake.SetActive(t, workerSession, false) {
		t.FailNow()
	}
	done := make(chan error, 1)
	go func() { done <- awaitActive(t.Context(), m, activations, workerSession, time.Hour) }()
	fake.SetActive(t, workerSession, true)
	if err := waittest.For(t, done, "the wait to end once the session is active"); err != nil {
		t.Fatalf("awaitActive = %v, want nil once the session is active", err)
	}
}

// logind answers Active true for the seat's new foreground session
// before udev has granted it the seat's devices, and announces it
// active only once udev has: a compositor started on the answer found
// /dev/udmabuf still closed to it.
func TestTheCompositorWaitsForLogindToAnnounceItsSessionActive(t *testing.T) {
	fake, m, activations := watchedSession(t)
	err := awaitActive(t.Context(), m, activations, workerSession, 50*time.Millisecond)
	want := "logind session 42 was not announced active within 50ms: it is active"
	if err == nil || err.Error() != want {
		t.Fatalf("awaitActive before logind announced the session active = %v, want %q", err, want)
	}
	done := make(chan error, 1)
	go func() { done <- awaitActive(t.Context(), m, activations, workerSession, time.Hour) }()
	fake.AnnounceProperty(t, workerSession, "Active", true)
	if err := waittest.For(t, done, "the wait to end once logind announces the session active"); err != nil {
		t.Fatalf("awaitActive = %v, want nil once the session is announced active", err)
	}
}

func TestASessionThatNeverBecomesActiveFailsNamingItsState(t *testing.T) {
	fake, m, activations := watchedSession(t)
	if !fake.SetActive(t, workerSession, false) {
		t.FailNow()
	}
	err := awaitActive(t.Context(), m, activations, workerSession, 50*time.Millisecond)
	want := "logind session 42 was not announced active within 50ms: it is online"
	if err == nil || err.Error() != want {
		t.Fatalf("awaitActive = %v, want %q", err, want)
	}
}

func TestAWaitForAnUnknownSessionFails(t *testing.T) {
	_, m, activations := watchedSession(t)
	err := awaitActive(t.Context(), m, activations, "9", time.Hour)
	if derr, ok := errors.AsType[dbus.Error](err); !ok || derr.Name != logind.ErrNoSuchSession {
		t.Fatalf("awaitActive of an unknown session = %v, want logind's %s", err, logind.ErrNoSuchSession)
	}
}

func TestTheOpenedSessionIsTheOnePAMNames(t *testing.T) {
	got, err := openedSession([]string{"XDG_SESSION_ID=3", "LANG=C", "XDG_SESSION_ID=42"})
	if err != nil || got != workerSession {
		t.Errorf("openedSession = %q, %v; want %q, the last XDG_SESSION_ID", got, err, workerSession)
	}
	if got, err := openedSession([]string{"LANG=C"}); err == nil || !strings.Contains(err.Error(), "XDG_SESSION_ID") {
		t.Errorf("openedSession without XDG_SESSION_ID = %q, %v; want an error naming it", got, err)
	}
}
