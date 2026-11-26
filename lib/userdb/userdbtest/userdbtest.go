// Package userdbtest serves a fake systemd user database on a Varlink
// socket of the test's own, so code that resolves accounts through
// lib/userdb runs against known records instead of the host's.
package userdbtest

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/sockdir"
	"github.com/stubbedev/lys/lib/userdb"
)

// Call is one call a client made.
type Call struct {
	Method     string                     `json:"method"`
	Parameters map[string]json.RawMessage `json:"parameters"`
	More       bool                       `json:"more"`
}

// String is the call's string parameter name, "" without one.
func (c Call) String(name string) string {
	var s string
	_ = json.Unmarshal(c.Parameters[name], &s)
	return s
}

// Reply is one reply to a call: an error name, or parameters.
type Reply struct {
	Parameters any              `json:"parameters,omitempty"`
	Continues  bool             `json:"continues,omitempty"`
	Error      userdb.ErrorName `json:"error,omitempty"`
}

// ServeFunc serves answer's replies on a socket until the test ends and
// is the socket's path. Each reply is sent as answer gives it, raw.
func ServeFunc(t testing.TB, answer func(Call) [][]byte) string {
	t.Helper()
	path := filepath.Join(sockdir.Dir(t), "userdb")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("userdbtest: listen on %s: %v", path, err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
	})
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() { serveConn(conn, answer) })
		}
	})
	return path
}

func serveConn(conn net.Conn, answer func(Call) [][]byte) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	for {
		msg, err := r.ReadBytes(0)
		if err != nil {
			return
		}
		var c Call
		if err := json.Unmarshal(msg[:len(msg)-1], &c); err != nil {
			return
		}
		for _, frame := range answer(c) {
			if _, err := conn.Write(append(frame, 0)); err != nil {
				return
			}
		}
	}
}

// DB is the records a fake user database holds.
type DB struct {
	Users  []userdb.User
	Groups []userdb.Group
	// Memberships is each user's groups besides its primary one.
	Memberships map[string][]string
}

// Serve serves db as the multiplexer does until the test ends and is the
// socket's path. A call that does not name the multiplexer's service is
// refused with BadService.
func Serve(t testing.TB, db DB) string {
	t.Helper()
	return ServeFunc(t, func(c Call) [][]byte {
		replies := db.answer(c)
		frames := make([][]byte, len(replies))
		for i, r := range replies {
			frame, err := json.Marshal(r)
			if err != nil {
				panic(err)
			}
			frames[i] = frame
		}
		return frames
	})
}

func (db DB) answer(c Call) []Reply {
	if c.String("service") != userdb.Service {
		return []Reply{{Error: userdb.BadService}}
	}
	switch c.Method {
	case userdb.MethodGetUserRecord:
		for _, u := range db.Users {
			if matches(c, "userName", u.Name) && matchesID(c, "uid", uint32(u.UID)) {
				return []Reply{{Parameters: map[string]any{"record": u, "incomplete": false}}}
			}
		}
	case userdb.MethodGetGroupRecord:
		for _, g := range db.Groups {
			if matches(c, "groupName", g.Name) && matchesID(c, "gid", uint32(g.GID)) {
				return []Reply{{Parameters: map[string]any{"record": g, "incomplete": false}}}
			}
		}
	case userdb.MethodGetMemberships:
		user := c.String("userName")
		groups := db.Memberships[user]
		replies := make([]Reply, len(groups))
		for i, g := range groups {
			replies[i] = Reply{Parameters: map[string]string{"userName": user, "groupName": g}, Continues: c.More && i < len(groups)-1}
		}
		if len(replies) > 0 {
			return replies
		}
	default:
		return []Reply{{Error: "org.varlink.service.MethodNotFound"}}
	}
	return []Reply{{Error: userdb.NoRecordFound}}
}

func matches(c Call, name, value string) bool {
	_, set := c.Parameters[name]
	return !set || c.String(name) == value
}

func matchesID(c Call, name string, id uint32) bool {
	raw, set := c.Parameters[name]
	if !set {
		return true
	}
	var got uint32
	return json.Unmarshal(raw, &got) == nil && got == id
}

// Missing is a socket path nothing serves, in a directory of the test's own.
func Missing(t testing.TB) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "absent")
}
