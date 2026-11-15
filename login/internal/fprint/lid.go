package fprint

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/stubbedev/lys-lib/logind"
	dbus "github.com/stubbedev/lys-lib/third_party/godbus"
)

func VerifyWhileLidOpen(ctx context.Context, conn *dbus.Conn, user string, onResult func(Result)) error {
	lid := make(chan bool, 1)
	failures := make(chan error, 1)
	closed, w, err := logind.NewManager(conn).FollowLid(ctx,
		func(err error) {
			select {
			case failures <- err:
			default:
			}
		},
		func(closed bool) {
			select {
			case <-lid:
			default:
			}
			lid <- closed
		})
	if err != nil {
		return fmt.Errorf("fprint: %w", err)
	}
	defer func() {
		if err := w.Stop(); err != nil && !errors.Is(err, logind.ErrDisconnected) {
			log.Printf("fprint: stop following the lid: %v", err)
		}
	}()
	var scan *scanning
	defer func() { scan.stop() }()
	for {
		switch {
		case closed && scan != nil:
			scan.stop()
			scan = nil
			onResult(Stopped)
			log.Printf("fprint: the lid closed; %s verifies by password alone", user)
		case !closed && scan == nil:
			scan = startScanning(ctx, conn, user, onResult)
		case closed:
			log.Printf("fprint: the lid is closed; %s verifies by password alone", user)
		}
		select {
		case <-ctx.Done():
			return nil
		case closed = <-lid:
		case err := <-scan.ended():
			scan = nil
			if err != nil {
				onResult(Stopped)
			}
			return err
		case err := <-failures:
			return fmt.Errorf("fprint: follow the lid: %w", err)
		case <-w.Done():
			return fmt.Errorf("fprint: follow the lid: %w", w.Stop())
		}
	}
}

type scanning struct {
	cancel context.CancelFunc
	done   chan error
}

func startScanning(ctx context.Context, conn *dbus.Conn, user string, onResult func(Result)) *scanning {
	scanCtx, cancel := context.WithCancel(ctx)
	s := &scanning{cancel: cancel, done: make(chan error, 1)}
	go func() {
		s.done <- Verify(scanCtx, conn, user, func(r Result) {
			if scanCtx.Err() == nil {
				onResult(r)
			}
		})
	}()
	return s
}

func (s *scanning) ended() <-chan error {
	if s == nil {
		return nil
	}
	return s.done
}

func (s *scanning) stop() {
	if s == nil {
		return
	}
	s.cancel()
	if err := <-s.done; err != nil {
		log.Printf("fprint: end the verification: %v", err)
	}
}
