package polkit

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/peercred"
	"github.com/stubbedev/lys/lib/procstat"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestProcessOfReadsTheKernelsStartTime(t *testing.T) {
	me := peercred.Credentials{UID: uint32(os.Getuid()), GID: uint32(os.Getgid()), PID: int32(os.Getpid())}
	p, err := ProcessOf(me)
	if err != nil {
		t.Fatal(err)
	}
	self, err := procstat.Read("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	start := self.StartTime
	if want := (UnixProcess{PID: uint32(os.Getpid()), StartTime: start, UID: me.UID}); p != want || start == 0 {
		t.Errorf("ProcessOf = %+v, want %+v", p, want)
	}
	for name, c := range map[string]peercred.Credentials{
		"a peer outside the pid namespace": {UID: me.UID, PID: 0},
		"a pid that is none":               {UID: me.UID, PID: -1},
	} {
		if p, err := ProcessOf(c); err == nil {
			t.Errorf("%s: ProcessOf = %+v, want an error", name, p)
		}
	}
}

func TestSubjectsTravelAsPolkitReadsThem(t *testing.T) {
	session, err := UnixSession{ID: "c2"}.Wire()
	if err != nil {
		t.Fatal(err)
	}
	if want := (WireSubject{Kind: "unix-session", Details: map[string]dbus.Variant{"session-id": dbus.MakeVariant("c2")}}); !reflect.DeepEqual(session, want) {
		t.Errorf("unix-session travels as %+v, want %+v", session, want)
	}
	process, err := UnixProcess{PID: 4242, StartTime: 987654, UID: 1000}.Wire()
	if err != nil {
		t.Fatal(err)
	}
	want := WireSubject{Kind: "unix-process", Details: map[string]dbus.Variant{
		"pid":        dbus.MakeVariant(uint32(4242)),
		"start-time": dbus.MakeVariant(uint64(987654)),
		"uid":        dbus.MakeVariant(int32(1000)),
	}}
	if !reflect.DeepEqual(process, want) {
		t.Errorf("unix-process travels as %+v, want %+v", process, want)
	}
	for _, s := range []Subject{UnixSession{ID: "c2"}, UnixProcess{PID: 4242, StartTime: 987654, UID: 1000}, UnixProcess{PID: 1, StartTime: 1, UID: 4294967294}} {
		w, err := s.Wire()
		if err != nil {
			t.Fatal(err)
		}
		if got, err := w.Subject(); err != nil || got != s {
			t.Errorf("%s read back as %+v, %v", s, got, err)
		}
		if fd, ok := w.PIDFD(); ok {
			t.Errorf("%s carries the pidfd %d it was not given", s, fd)
		}
	}
	if got := (UnixSession{ID: "c2"}).String(); got != "unix-session c2" {
		t.Errorf("String = %q", got)
	}
	if got := (UnixProcess{PID: 4242, UID: 1000}).String(); got != "unix-process 4242 of uid 1000" {
		t.Errorf("String = %q", got)
	}
}

func TestSubjectsThatNameNobodyAreRefused(t *testing.T) {
	for name, s := range map[string]Subject{
		"a session without an id": UnixSession{},
		"a process without a pid": UnixProcess{StartTime: 1, UID: 1000},
	} {
		if w, err := s.Wire(); err == nil {
			t.Errorf("%s travels as %+v, want an error", name, w)
		}
	}
	for name, tc := range map[string]struct {
		wire WireSubject
		want string
	}{
		"another kind":             {WireSubject{Kind: "system-bus-name", Details: map[string]dbus.Variant{"name": dbus.MakeVariant(":1.2")}}, `subject kind "system-bus-name"`},
		"a session without its id": {WireSubject{Kind: "unix-session"}, "without session-id"},
		"a session id of a number": {WireSubject{Kind: "unix-session", Details: map[string]dbus.Variant{"session-id": dbus.MakeVariant(uint32(2))}}, "session-id of type u, want s"},
		"a process without a pid": {WireSubject{Kind: "unix-process", Details: map[string]dbus.Variant{
			"start-time": dbus.MakeVariant(uint64(1)), "uid": dbus.MakeVariant(int32(1000)),
		}}, "without pid"},
		"a process with an unsigned uid": {WireSubject{Kind: "unix-process", Details: map[string]dbus.Variant{
			"pid": dbus.MakeVariant(uint32(1)), "start-time": dbus.MakeVariant(uint64(1)), "uid": dbus.MakeVariant(uint32(1000)),
		}}, "uid of type u, want i"},
	} {
		got, err := tc.wire.Subject()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Subject = %+v, %v; want an error with %q", name, got, err, tc.want)
		}
	}
}

func TestVerdictReadsPolkitsAnswer(t *testing.T) {
	for want, r := range map[Verdict]AuthorizationResult{
		Authorized: {IsAuthorized: true},
		Denied:     {},
		Challenge:  {IsChallenge: true},
		Dismissed:  {IsChallenge: true, Details: map[string]string{DismissedDetail: "true"}},
	} {
		if got := r.Verdict(); got != want {
			t.Errorf("%+v is %s, want %s", r, got, want)
		}
	}
	if got := (AuthorizationResult{IsAuthorized: true, Details: map[string]string{DismissedDetail: "true"}}).Verdict(); got != Authorized {
		t.Errorf("an authorized result with a dismissal is %s", got)
	}
	var zero Verdict
	if got := zero.String(); got != "Verdict(0)" {
		t.Errorf("the zero Verdict is %q: it must not read as an answer", got)
	}
}
