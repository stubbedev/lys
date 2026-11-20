// Package workspacestest fakes the IPC sockets of Hyprland, niri and sway,
// each answering its workspace query with a fixed JSON reply, and
// advertises the fake in the test's environment.
package workspacestest

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/sockdir"

	"github.com/stubbedev/lys/desktop/compositor"
)

const (
	swayMagic         = "i3-ipc"
	swayHeader        = 14
	swayGetWorkspaces = 1
	hyprlandSignature = "workspacestest"
)

// Sway advertises a sway whose GET_WORKSPACES answers workspaces and
// which drops the connection on any other message.
func Sway(t testing.TB, workspaces string) {
	t.Helper()
	size, err := intconv.Exact[uint32](len(workspaces))
	if err != nil {
		t.Fatalf("workspacestest: sway reply of %d bytes: %v", len(workspaces), err)
	}
	path := filepath.Join(sockdir.Dir(t), "sway.sock")
	serve(t, path, func(conn net.Conn) {
		for {
			head := make([]byte, swayHeader)
			if _, err := io.ReadFull(conn, head); err != nil {
				return
			}
			if _, err := io.CopyN(io.Discard, conn, int64(binary.LittleEndian.Uint32(head[6:]))); err != nil {
				return
			}
			if string(head[:len(swayMagic)]) != swayMagic || binary.LittleEndian.Uint32(head[10:]) != swayGetWorkspaces {
				return
			}
			reply := make([]byte, swayHeader, swayHeader+len(workspaces))
			copy(reply, swayMagic)
			binary.LittleEndian.PutUint32(reply[6:], size)
			binary.LittleEndian.PutUint32(reply[10:], swayGetWorkspaces)
			if _, err := conn.Write(append(reply, workspaces...)); err != nil {
				return
			}
		}
	})
	t.Setenv(compositor.SwaySocketEnv, path)
}

// Niri advertises a niri whose Workspaces request answers workspaces,
// folded onto the one line niri's protocol carries, and which refuses
// every other request.
func Niri(t testing.TB, workspaces string) {
	t.Helper()
	var line bytes.Buffer
	if err := json.Compact(&line, []byte(workspaces)); err != nil {
		t.Fatalf("workspacestest: niri answers one JSON line: %v", err)
	}
	path := filepath.Join(sockdir.Dir(t), "niri.sock")
	serve(t, path, func(conn net.Conn) {
		lines := bufio.NewScanner(conn)
		for lines.Scan() {
			reply := `{"Err":"workspacestest answers only Workspaces"}`
			if lines.Text() == `"Workspaces"` {
				reply = `{"Ok":{"Workspaces":` + line.String() + `}}`
			}
			if _, err := io.WriteString(conn, reply+"\n"); err != nil {
				return
			}
		}
	})
	t.Setenv(compositor.NiriSocketEnv, path)
}

// Hyprland advertises a Hyprland whose j/workspaces answers workspaces and
// which answers every other request as unknown.
func Hyprland(t testing.TB, workspaces string) {
	t.Helper()
	runtime := sockdir.Dir(t)
	dir := filepath.Join(runtime, "hypr", hyprlandSignature)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	serve(t, filepath.Join(dir, ".socket.sock"), func(conn net.Conn) {
		request := make([]byte, 4096)
		n, err := conn.Read(request)
		if err != nil {
			return
		}
		reply := "unknown request"
		if string(request[:n]) == "j/workspaces" {
			reply = workspaces
		}
		_, _ = io.WriteString(conn, reply+"\x04")
	})
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv(compositor.HyprlandSocketEnv, hyprlandSignature)
}

func serve(t testing.TB, path string, handle func(net.Conn)) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu    sync.Mutex
		open  = map[net.Conn]bool{}
		conns sync.WaitGroup
	)
	conns.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			open[conn] = true
			mu.Unlock()
			conns.Go(func() {
				defer func() {
					mu.Lock()
					delete(open, conn)
					mu.Unlock()
					_ = conn.Close()
				}()
				handle(conn)
			})
		}
	})
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		for conn := range open {
			_ = conn.Close()
		}
		mu.Unlock()
		conns.Wait()
	})
}
