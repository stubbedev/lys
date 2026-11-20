package autobright

import (
	"context"
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestDisplaySessionsResolveTheUsersDisplaySession(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Display: "2"})
	s := displaySessions{logind.NewManager(fake.Bus.Conn(t))}
	for range 2 {
		session, err := s.Display(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if session.Path() != logind.SessionPath("2") {
			t.Fatalf("the display session is %s, want %s", session.Path(), logind.SessionPath("2"))
		}
	}
	if got := fake.CallsTo("Manager.GetSession"); len(got) != 2 {
		t.Errorf("GetSession calls %v, want one per resolve", got)
	}
}

func TestDisplaySessionsReportASessionThatIsGone(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{})
	s := displaySessions{logind.NewManager(fake.Bus.Conn(t))}
	session, err := s.Display(context.Background())
	if derr, ok := errors.AsType[dbus.Error](err); !ok || derr.Name != logind.ErrNoSuchSession {
		t.Fatalf("Display with no session = %v, %v; want logind's %s", session, err, logind.ErrNoSuchSession)
	}
	if session != nil {
		t.Errorf("Display with no session returned %v", session)
	}
}
