package usershelper

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/sockdir"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

type nscdRequest struct {
	version, kind uint32
	key           string
}

// fakeNSCD serves nscd's socket until the test ends, answering each
// request with answer's reply, none for nil, and is the socket's path
// and the requests it took.
func fakeNSCD(t *testing.T, answer func(nscdRequest) []byte) (string, func() []nscdRequest) {
	t.Helper()
	path := filepath.Join(sockdir.Dir(t), "socket")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu   sync.Mutex
		got  []nscdRequest
		wg   sync.WaitGroup
		done = make(chan struct{})
	)
	t.Cleanup(func() {
		close(done)
		_ = ln.Close()
		wg.Wait()
	})
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				var head [12]byte
				if _, err := io.ReadFull(conn, head[:]); err != nil {
					return
				}
				key := make([]byte, binary.NativeEndian.Uint32(head[8:]))
				if _, err := io.ReadFull(conn, key); err != nil {
					return
				}
				r := nscdRequest{version: binary.NativeEndian.Uint32(head[0:]), kind: binary.NativeEndian.Uint32(head[4:]), key: string(key)}
				mu.Lock()
				got = append(got, r)
				mu.Unlock()
				reply := answer(r)
				if reply == nil {
					<-done
					return
				}
				_, _ = conn.Write(reply)
			})
		}
	})
	return path, func() []nscdRequest {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

func nscdReply(code uint32) []byte { return binary.NativeEndian.AppendUint32(nil, code) }

func TestAChangeHasNSCDForgetItsPasswdAndGroupCaches(t *testing.T) {
	m := newMachine(t, entry("alice", me+1))
	path, requests := fakeNSCD(t, func(nscdRequest) []byte { return nscdReply(0) })
	m.nscd = path
	if err := m.change(t.Context(), me+1, accounts.Rename{RealName: "A"}); err != nil {
		t.Fatal(err)
	}
	want := []nscdRequest{{2, 10, "passwd\x00"}, {2, 10, "group\x00"}}
	if got := requests(); !slices.Equal(got, want) {
		t.Errorf("requests %+v, want %+v", got, want)
	}
}

func TestAnNSCDThatDoesNotRunOrClosesUnansweredHasNothingToForget(t *testing.T) {
	stale := filepath.Join(sockdir.Dir(t), "socket")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: stale, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	_ = ln.Close()
	closing, _ := fakeNSCD(t, func(nscdRequest) []byte { return []byte{} })
	for name, path := range map[string]string{"no socket": filepath.Join(sockdir.Dir(t), "socket"), "a stale socket": stale, "an older nscd": closing} {
		if err := forgetCached(t.Context(), path); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAnNSCDThatRefusesOrHangsFailsTheChangeAfterTheFiles(t *testing.T) {
	refusing, _ := fakeNSCD(t, func(nscdRequest) []byte { return nscdReply(uint32(syscall.EINVAL)) })
	if err := forgetCached(t.Context(), refusing); !errors.Is(err, syscall.EINVAL) || !strings.Contains(err.Error(), "passwd") {
		t.Errorf("a refusal: %v, want EINVAL naming the passwd cache", err)
	}
	hanging, _ := fakeNSCD(t, func(nscdRequest) []byte { return nil })
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := forgetCached(ctx, hanging); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("no answer: %v, want the deadline", err)
	}
	m := newMachine(t, entry("alice", me+1))
	m.nscd = refusing
	err := m.change(t.Context(), me+1, accounts.Rename{RealName: "Alice B"})
	if err == nil || err.Kind != accounts.ErrFailed || !strings.Contains(err.Detail, "changed, but") {
		t.Errorf("a change nscd refuses to forget: %v", err)
	}
	if !strings.Contains(m.read(t, "passwd"), ":Alice B:") {
		t.Errorf("the change was not made: %q", m.read(t, "passwd"))
	}
	created := newMachine(t)
	created.nscd = refusing
	if err := created.change(t.Context(), 0, accounts.Create{Login: "zoe", Hash: hash(t)}); err == nil || !strings.Contains(err.Detail, "zoe is created, but") {
		t.Errorf("a new account nscd refuses to forget: %v", err)
	}
	if !strings.Contains(created.read(t, "passwd"), "zoe:") {
		t.Errorf("the new account is gone: %q", created.read(t, "passwd"))
	}
	if err := created.change(t.Context(), 0, accounts.Delete{Login: "zoe", RemoveHome: true}); err == nil || !strings.Contains(err.Detail, "zoe is removed, but") {
		t.Errorf("a deletion nscd refuses to forget: %v", err)
	}
	if strings.Contains(created.read(t, "passwd"), "zoe:") || dirExists(filepath.Join(created.home, "zoe")) {
		t.Errorf("the deletion was not made whole: %q", created.read(t, "passwd"))
	}
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
