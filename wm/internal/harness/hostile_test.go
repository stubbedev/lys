package harness

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wmipc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/ipc"
	cxdg "github.com/stubbedev/lys/wm/internal/protocol/client/xdgshell"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/testenv"
	"github.com/stubbedev/lys/wm/internal/wire"
)

func unresponsive(id int, want bool) func(*headless.Backend, *compositor.Compositor) bool {
	return func(_ *headless.Backend, c *compositor.Compositor) bool {
		for _, w := range c.Snapshot().Windows {
			if int(w.ID) == id {
				return w.Unresponsive == want
			}
		}
		return false
	}
}

func TestHungClientsAreMarkedUnresponsive(t *testing.T) {
	t.Parallel()
	h := Start(t, small)
	lines := h.watch()
	hung := testclient.Connect(t, h.Socket())
	w := hung.Window("hung", quadrants)
	hung.DispatchUntil("the window to map", w.Mapped)
	h.Eventually("the window focused", func(_ *headless.Backend, c *compositor.Compositor) bool {
		ws := c.Snapshot().Windows
		return len(ws) == 1 && ws[0].Focused
	})
	h.Eventually("the hung window marked unresponsive", unresponsive(1, true))
	expectEvent(t, lines, `{"event":"unresponsive","window":1,"unresponsive":true}`)

	alive := testclient.Connect(t, h.Socket())
	other := alive.Window("alive", quadrants)
	alive.DispatchUntil("another client's window to map", other.Mapped)
	h.Eventually("the other window responsive", unresponsive(2, false))

	hung.Sync()
	h.Eventually("the recovered window marked responsive", unresponsive(1, false))
	expectEvent(t, lines, `{"event":"unresponsive","window":1,"unresponsive":false}`)
}

func (r *rawClient) closedByServer() bool {
	deadline := time.Now().Add(testenv.HangLimit)
	for {
		m, err := r.receive(deadline)
		if err != nil {
			return !errors.Is(err, errTimedOut)
		}
		_ = m.Finish()
	}
}

func survives(t *testing.T, h *Harness) {
	t.Helper()
	c := testclient.Connect(t, h.Socket())
	w := c.Window("survivor", quadrants)
	c.DispatchUntil("a window mapped beside the hostile client", w.Mapped)
}

func serverEnds(t *testing.T, h *Harness, n int) []fdtest.Watch {
	t.Helper()
	var fds []int
	h.Eventually(fmt.Sprintf("the compositor serving %d clients", n), func(_ *headless.Backend, c *compositor.Compositor) bool {
		fds = fds[:0]
		for _, client := range c.Display().Clients() {
			fds = append(fds, client.Conn().FD())
		}
		return len(fds) == n
	})
	ends := make([]fdtest.Watch, len(fds))
	for i, fd := range fds {
		ends[i] = fdtest.WatchFD(t, fd)
	}
	return ends
}

func awaitClosed(t *testing.T, end fdtest.Watch, what string) {
	t.Helper()
	waittest.Until(t, func() bool { return !end.Open(t) }, what)
}

func heldPools(t *testing.T, c *testclient.Client) []int {
	t.Helper()
	var held []int
	for i, pool := range c.Pools() {
		if len(pool.Holders(t)) != 0 {
			held = append(held, i)
		}
	}
	return held
}

func TestGarbageDisconnectsOnlyItsSender(t *testing.T) {
	t.Parallel()
	h := StartUntraced(t, small)
	r := dialRaw(t, h.Socket())
	if _, err := unix.Write(r.conn.FD(), []byte{1, 0, 0, 0, 0, 0, 4, 0, 0xde, 0xad, 0xbe, 0xef}); err != nil {
		t.Fatal(err)
	}
	if !r.closedByServer() {
		t.Fatal("a client that sent a 4-byte message was not disconnected")
	}
	survives(t, h)
}

func TestIDsOutsideTheClientRangeAreErrors(t *testing.T) {
	t.Parallel()
	for name, send := range map[string]func(r *rawClient) error{
		"a server-side new id": func(r *rawClient) error {
			return r.send(displayID, getRegistry, func(e *wire.Encoder) { e.Uint(0xff000005) })
		},
		"an unknown object": func(r *rawClient) error {
			return r.send(0x7fffffff, 0, func(*wire.Encoder) {})
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := StartUntraced(t, small)
			r := dialRaw(t, h.Socket())
			if err := errors.Join(send(r), r.conn.Flush()); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(testenv.HangLimit)
			m, err := r.receive(deadline)
			if err != nil || m.ID != displayID || m.Opcode != 0 {
				t.Fatalf("got %v, %v instead of a wl_display.error", m, err)
			}
			_ = m.Finish()
			if !r.closedByServer() {
				t.Fatal("the client was not disconnected after its error")
			}
			survives(t, h)
		})
	}
}

func TestFDFloodsAreDisconnectedAndClosed(t *testing.T) {
	h := StartUntraced(t, small)
	r := dialRaw(t, h.Socket())
	pipe := make([]int, 2)
	if err := unix.Pipe2(pipe, unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = errors.Join(unix.Close(pipe[0]), unix.Close(pipe[1])) }()
	end := serverEnds(t, h, 1)[0]
	fds := make([]int, 250)
	for i := range fds {
		fds[i] = pipe[0]
	}
	rights := unix.UnixRights(fds...)
	for range 8 {
		if err := unix.Sendmsg(r.conn.FD(), []byte{0}, rights, nil, unix.MSG_NOSIGNAL); err != nil {
			break
		}
	}
	awaitClosed(t, end, "the compositor to close the connection of a client that queued 2000 file descriptors")
	h.Do(func(*headless.Backend, *compositor.Compositor) error { return nil })
	if held := fdtest.Holders(t, pipe[0]); len(held) != 0 {
		t.Fatalf("%d descriptors still hold the pipe the flood sent after the server closed its end", len(held))
	}
	survives(t, h)
}

func TestClientsThatNeverReadAreDisconnected(t *testing.T) {
	t.Parallel()
	h := StartUntraced(t, small)
	r := dialRaw(t, h.Socket())
	waittest.Until(t, func() bool {
		for range 1000 {
			cb := r.newID()
			err := r.send(displayID, displaySync, func(e *wire.Encoder) { e.Uint(uint32(cb)) })
			if err == nil {
				err = r.conn.Flush()
			}
			if errors.Is(err, wire.ErrWouldBlock) || errors.Is(err, wire.ErrOutputFull) {
				if _, err := r.wait(unix.POLLOUT, time.Now().Add(100*time.Millisecond)); err != nil {
					t.Fatal(err)
				}
				return false
			}
			if err != nil {
				return true
			}
		}
		return false
	}, "a client that never reads its events to be disconnected")
	survives(t, h)
}

func TestStalledWindowsDoNotStallOthers(t *testing.T) {
	t.Parallel()
	h := Start(t, small)
	hung := testclient.Connect(t, h.Socket())
	w := hung.Window("stalled", quadrants)
	hung.DispatchUntil("the window to map", w.Mapped)
	var done bool
	waittest.Until(t, func() bool {
		var err error
		for range 1000 {
			if _, err = w.Surface.Frame(frameDone{&done}); err != nil {
				break
			}
		}
		if err == nil {
			err = errors.Join(w.Surface.Attach(hung.Buffer(w.Logical, 1, quadrants), 0, 0), w.Surface.Damage(0, 0, 10, 10), w.Surface.Commit(), hung.Display.Flush())
		}
		return err != nil && !errors.Is(err, wire.ErrWouldBlock) && !errors.Is(err, wire.ErrOutputFull)
	}, "a client that never reads its frame callbacks to be disconnected")
	for range 3 {
		survives(t, h)
	}
	h.Eventually("the other clients tiled", mappedToplevels(3))
}

func busyClient(t *testing.T, h *Harness, name string) *testclient.Client {
	t.Helper()
	c := testclient.Connect(t, h.Socket())
	c.WatchPools()
	seat, in := c.Seat()
	w := c.Window(name, quadrants)
	c.DispatchUntil(name+" to map", w.Mapped)
	cb := c.Clipboard(seat)
	src := c.DataSource(map[string]string{"text/plain": name})
	if err := errors.Join(cb.Device.SetSelection(src.Data, in.Serial), c.Display.Roundtrip()); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDisconnectedClientsLeaveNothingBehind(t *testing.T) {
	h := StartUntraced(t, small)
	settled := func(what string) {
		t.Helper()
		h.Eventually(what, func(_ *headless.Backend, c *compositor.Compositor) bool { return len(c.Toplevels()) == 0 })
	}
	warm := busyClient(t, h, "warm")
	if err := warm.Display.Close(); err != nil {
		t.Fatal(err)
	}
	settled("the warm-up client gone")
	goroutines := runtime.NumGoroutine()
	clients := make([]*testclient.Client, 0, 5)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		clients = append(clients, busyClient(t, h, name))
	}
	ends := serverEnds(t, h, len(clients))
	for _, c := range clients {
		if err := c.Display.Close(); err != nil {
			t.Fatal(err)
		}
	}
	settled("every client gone")
	h.Do(func(*headless.Backend, *compositor.Compositor) error { return nil })
	for _, end := range ends {
		if end.Open(t) {
			t.Errorf("after five clients came and went the compositor still holds the connection on descriptor %d", end.FD())
		}
	}
	for i, c := range clients {
		if held := heldPools(t, c); len(held) != 0 {
			t.Errorf("after five clients came and went the compositor still holds pools %v of client %d's %d", held, i, len(c.Pools()))
		}
	}
	if g := runtime.NumGoroutine(); g != goroutines {
		t.Errorf("after five clients came and went: %d goroutines, %d before", g, goroutines)
	}
}

func TestClientsLeavingABacklogDoNotStopTheCompositor(t *testing.T) {
	t.Parallel()
	h := Start(t, small)
	gone := testclient.Connect(t, h.Socket())
	w := gone.Window("leaving", quadrants)
	gone.DispatchUntil("the window to map", w.Mapped)
	var done bool
	for range 20000 {
		if _, err := w.Surface.Frame(frameDone{&done}); err != nil || errors.Is(gone.Display.Flush(), wire.ErrWouldBlock) {
			break
		}
	}
	if err := errors.Join(w.Surface.Attach(gone.Buffer(w.Logical, 1, quadrants), 0, 0), w.Surface.Commit(), gone.Display.Flush()); err != nil {
		t.Fatal(err)
	}
	if err := gone.Display.Close(); err != nil {
		t.Fatal(err)
	}
	survives(t, h)
	h.Eventually("the leaving client's window gone", mappedToplevels(1))
}

func TestAClientLeavingWithAnOlderWindowFocusedDoesNotStopTheCompositor(t *testing.T) {
	t.Parallel()
	h := Start(t, small)
	gone := testclient.Connect(t, h.Socket())
	for _, name := range []string{"older", "newer"} {
		w := gone.Window(name, quadrants)
		gone.DispatchUntil(name+" to map", w.Mapped)
	}
	if _, err := ipc.Do(h.IPCSocket(), ipc.Request{Request: wmipc.KindDispatch, Action: "focus left"}); err != nil {
		t.Fatal(err)
	}
	if w := query[ipc.Window](h, wmipc.KindActiveWindow); w.Title != "older" {
		t.Fatalf("focus left made %q active", w.Title)
	}
	for range 2 {
		if err := gone.Display.Roundtrip(); err != nil {
			t.Fatal(err)
		}
	}
	if err := gone.Display.Close(); err != nil {
		t.Fatal(err)
	}
	h.Eventually("the leaving client's windows gone", func(_ *headless.Backend, c *compositor.Compositor) bool { return len(c.Toplevels()) == 0 })
	survives(t, h)
}

type deaf struct{}

func (deaf) Configure(*cxdg.XdgSurface, uint32) error { return nil }

type deafToplevel struct{}

func (deafToplevel) Configure(*cxdg.XdgToplevel, int32, int32, []byte) error { return nil }
func (deafToplevel) Close(*cxdg.XdgToplevel) error                           { return nil }
func (deafToplevel) ConfigureBounds(*cxdg.XdgToplevel, int32, int32) error   { return nil }
func (deafToplevel) WmCapabilities(*cxdg.XdgToplevel, []byte) error          { return nil }

func TestUnackedConfiguresDoNotStallOthers(t *testing.T) {
	t.Parallel()
	h := Start(t, small)
	c := testclient.Connect(t, h.Socket())
	for range 3 {
		s := c.Surface()
		xs, err := c.WM.GetXdgSurface(deaf{}, s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := xs.GetToplevel(deafToplevel{}); err != nil {
			t.Fatal(err)
		}
		if err := s.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Display.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		survives(t, h)
	}
	h.Eventually("the windows that acked mapped", mappedToplevels(2))
}

func TestAnObjectThatCannotBeCreatedClosesItsFDs(t *testing.T) {
	h := Start(t, small)
	r := dialRaw(t, h.Socket())
	end := serverEnds(t, h, 1)[0]
	registry := r.newID()
	if err := r.send(displayID, getRegistry, func(e *wire.Encoder) { e.Uint(uint32(registry)) }); err != nil {
		t.Fatal(err)
	}
	var shmName, shmVersion uint32
	if err := r.sync(time.Now().Add(testenv.HangLimit), func(m *wire.Message) {
		if m.ID == registry && m.Opcode == registryGlobal {
			if name, iface, version := m.Uint(), m.Str(), m.Uint(); iface == "wl_shm" {
				shmName, shmVersion = name, version
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	shm := r.newID()
	if err := r.send(registry, registryBind, func(e *wire.Encoder) {
		e.Uint(shmName)
		e.Str("wl_shm")
		e.Uint(shmVersion)
		e.Uint(uint32(shm))
	}); err != nil {
		t.Fatal(err)
	}
	pool, err := unix.MemfdCreate("pool", unix.MFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Ftruncate(pool, 4096); err != nil {
		t.Fatal(err)
	}
	sent := fdtest.WatchFD(t, pool)
	if err := errors.Join(r.send(shm, 0, func(e *wire.Encoder) {
		e.Uint(uint32(registry))
		e.FD(pool)
		e.Int(4096)
	}), r.conn.Flush(), unix.Close(pool)); err != nil {
		t.Fatal(err)
	}
	awaitClosed(t, end, "the compositor to close the connection of a create_pool reusing a live id")
	if err := r.conn.Close(); err != nil {
		t.Fatal(err)
	}
	h.Do(func(*headless.Backend, *compositor.Compositor) error { return nil })
	if held := sent.Holders(t); len(held) != 0 {
		t.Fatalf("descriptors %v still hold the pool of the refused create_pool", held)
	}
}

func TestAWindowLetsGoOfTheBuffersItOutgrew(t *testing.T) {
	h := Start(t, small)
	c := testclient.Connect(t, h.Socket())
	c.WatchPools()
	other := c.Window("other", testclient.Fill(0, 0, 0xff))
	c.DispatchUntil("the other window to map", other.Mapped)
	beside := c.Window("resized", testclient.Fill(0xff, 0, 0))
	c.DispatchUntil("the resized window to map", beside.Mapped)
	half := beside.Logical
	c.Sync()
	h.Do(func(*headless.Backend, *compositor.Compositor) error { return nil })
	baseline := len(heldPools(t, c))
	for range 5 {
		if err := errors.Join(beside.Toplevel.SetFullscreen(nil), c.Display.Flush()); err != nil {
			t.Fatal(err)
		}
		c.DispatchUntil("the window to fill the output", func() bool { return beside.Logical == small.Size })
		if err := errors.Join(beside.Toplevel.UnsetFullscreen(), c.Display.Flush()); err != nil {
			t.Fatal(err)
		}
		c.DispatchUntil("the window back beside the other", func() bool { return beside.Logical == half })
	}
	c.Sync()
	waittest.Until(t, func() bool { return len(heldPools(t, c)) <= baseline }, "no more pools held than before the resizes")
	if err := beside.Destroy(); err != nil {
		t.Fatal(err)
	}
	c.Sync()
	waittest.Until(t, func() bool { return len(heldPools(t, c)) < baseline }, "the destroyed window's pool closed")
}
