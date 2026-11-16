package pulse

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/waittest"
)

func fastReconnect(t *testing.T) {
	t.Helper()
	min, max := reconnectMin, reconnectMax
	reconnectMin, reconnectMax = 5*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { reconnectMin, reconnectMax = min, max })
}

func hasDefaultSink(s *Service) bool {
	_, err := s.DefaultSink(context.Background())
	return err == nil
}

func TestLostConnectionEmptiesThenRefills(t *testing.T) {
	fastReconnect(t)
	srv := fixture(t)
	s := connect(t, srv)
	ticks, stop, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	srv.Close()
	waittest.Until(t, func() bool {
		return len(s.OutputDevices()) == 0 && len(s.InputDevices()) == 0 && len(s.PlaybackStreams()) == 0
	}, "the state to empty with the connection")
	if _, err := s.DefaultSink(context.Background()); !errors.Is(err, ErrNoDefaultDevice) {
		t.Errorf("DefaultSink while down = %v, want ErrNoDefaultDevice", err)
	}
	if err := s.SetMuted(context.Background(), true); !errors.Is(err, ErrNoDefaultDevice) {
		t.Errorf("SetMuted while down = %v, want ErrNoDefaultDevice", err)
	}
	if err := s.SetDeviceMute(context.Background(), DeviceKey{Index: 1, Type: DeviceOutput}, true); !errors.Is(err, ErrDisconnected) {
		t.Errorf("SetDeviceMute while down = %v, want ErrDisconnected", err)
	}
	open := false
	waittest.Until(t, func() bool {
		select {
		case _, open = <-ticks:
			return true
		default:
			return false
		}
	}, "a tick for the lost connection")
	if !open {
		t.Fatal("a lost connection closed the subscriber")
	}

	srv.Listen(t)
	waittest.Until(t, func() bool { return len(s.OutputDevices()) == 2 && hasDefaultSink(s) }, "the state to refill on reconnect")
	if err := s.SetMuted(context.Background(), true); err != nil {
		t.Fatalf("SetMuted after reconnect: %v", err)
	}
	waittest.Until(t, func() bool {
		d, err := s.DefaultSink(context.Background())
		return err == nil && d.Muted
	}, "events from the new connection")
}

func TestStartWaitsForTheServer(t *testing.T) {
	fastReconnect(t)
	srv := fixture(t)
	srv.Close()
	s := Start(srv.Addr())
	t.Cleanup(func() { _ = s.Close() })
	time.Sleep(quietWindow)
	if len(s.OutputDevices()) != 0 || hasDefaultSink(s) {
		t.Fatal("state before the server is up")
	}
	srv.Listen(t)
	waittest.Until(t, func() bool { return len(s.OutputDevices()) == 2 && hasDefaultSink(s) }, "the first connection")
}

func TestCloseStopsReconnecting(t *testing.T) {
	fastReconnect(t)
	srv := fixture(t)
	srv.Close()
	s := Start(srv.Addr())
	done := make(chan struct{})
	go func() {
		_ = s.Close()
		close(done)
	}()
	waittest.For(t, done, "Close while reconnecting")
	srv.Listen(t)
	time.Sleep(quietWindow)
	if srv.Clients() != 0 || len(s.OutputDevices()) != 0 {
		t.Error("a closed service connected")
	}
}

func TestStaleConnectionEventsAreIgnored(t *testing.T) {
	srv := fixture(t)
	s := connect(t, srv)
	old := s.sess
	s.drop(old)
	changed, err := foldObject(s, old, native.SubscribeEvent{Facility: native.FacilitySink, Operation: native.OpRemove, Index: 1}, s.outputs, old.conn.Sink, fromSink)
	if err != nil || changed {
		t.Errorf("a replaced connection's event applied: changed %v err %v", changed, err)
	}
}
