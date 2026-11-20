package bar

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"
)

// nineTags is nine tags as a mango frame fragment.
func nineTags(active, occupied int) string {
	parts := make([]string, 0, 9)
	for i := 1; i <= 9; i++ {
		count := 0
		if i == occupied {
			count = 1
		}
		parts = append(parts, fmt.Sprintf(`{"index":%d,"is_active":%v,"is_urgent":false,"layout":"T","client_count":%d}`, i, i == active, count))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// fakeMangoSocket streams one monitors frame and one clients frame and
// records dispatches.
func fakeMangoSocket(t *testing.T, monitors, clients string) chan string {
	t.Helper()
	dispatched := make(chan string, 8)
	path := serveUnix(t, "mango.sock", func(conn net.Conn, _ <-chan struct{}) {
		r := bufio.NewReader(conn)
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		switch line {
		case "watch all-monitors\n":
			_, _ = conn.Write([]byte(`{"monitors":` + monitors + "}\n"))
			_, _ = io.Copy(io.Discard, r)
		case "watch all-clients\n":
			_, _ = conn.Write([]byte(`{"clients":` + clients + "}\n"))
			_, _ = io.Copy(io.Discard, r)
		default:
			dispatched <- strings.TrimSuffix(line, "\n")
			_, _ = conn.Write([]byte("{}\n"))
		}
	})
	clearCompositorEnv(t)
	t.Setenv("MANGO_INSTANCE_SIGNATURE", path)
	return dispatched
}

// newTestMango builds the module and waits until both frames landed.
func newTestMango(t *testing.T, cfg *config.Config, connector string) *cwsModule {
	t.Helper()
	ctx := newTestContext(t, cfg)
	ctx.Connector = connector
	module, err := createOnHeadlessLoop("workspaces", ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	m := module.(*cwsModule)
	t.Cleanup(m.Stop)
	backend := m.backend.(*connectingBackend).conn.(*mangoBackend)
	waittest.Until(t, backend.watch.Ready, "the first frames")
	if err := onHeadlessLoop(func() error { return readAndApply(m.read, m.apply) }); err != nil {
		t.Fatal(err)
	}
	return m
}

func tagIndices(m *cwsModule) []uint64 {
	out := make([]uint64, 0, len(m.models))
	for _, model := range m.models {
		out = append(out, model.WS.ID)
	}
	return out
}

func TestMangoHideEmptyAndMinTagCount(t *testing.T) {
	monitors := `[{"name":"DP-1","active":true,"tags":` + nineTags(1, 1) + `,"active_tags":[1],"active_client":{"id":null},"keymode":"","keyboardlayout":""}]`
	for _, tc := range []struct {
		hide bool
		min  int
		want []uint64
	}{
		{false, 0, []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{true, 0, []uint64{1}},
		{true, 5, []uint64{1, 2, 3, 4, 5}},
	} {
		fakeMangoSocket(t, monitors, "[]")
		cfg := config.Defaults()
		cfg.Workspaces.HideEmpty, cfg.Workspaces.MinTagCount = tc.hide, uint8(tc.min)
		m := newTestMango(t, cfg, "DP-1")
		if got := tagIndices(m); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("hide=%v min=%d: tags %v, want %v", tc.hide, tc.min, got, tc.want)
		}
	}
}

func TestMangoChoosesTheBarMonitorThenTheActiveOne(t *testing.T) {
	mon := func(name string, active bool, activeTag int) string {
		return fmt.Sprintf(`{"name":%q,"active":%v,"tags":%s,"active_tags":[%d],"active_client":{"id":null},"keymode":"","keyboardlayout":""}`, name, active, nineTags(activeTag, 0), activeTag)
	}
	monitors := "[" + mon("DP-1", false, 2) + "," + mon("DP-2", true, 7) + "]"
	fakeMangoSocket(t, monitors, "[]")
	if got := tagIndices(newTestMango(t, config.Defaults(), "DP-1")); !reflect.DeepEqual(got, []uint64{2}) {
		t.Errorf("bar DP-1 = %v, want its own active tag 2", got)
	}
	fakeMangoSocket(t, monitors, "[]")
	if got := tagIndices(newTestMango(t, config.Defaults(), "HDMI-A-1")); !reflect.DeepEqual(got, []uint64{7}) {
		t.Errorf("unknown bar monitor = %v, want the active DP-2's tag 7", got)
	}
}

func TestMangoTagButtonsAndClients(t *testing.T) {
	monitors := `[{"name":"DP-1","active":true,"tags":[{"index":1,"is_active":true,"is_urgent":false,"layout":"T","client_count":2},{"index":3,"is_active":false,"is_urgent":false,"layout":"T","client_count":1}],"active_tags":[1],"active_client":{"id":null},"keymode":"","keyboardlayout":""}]`
	clients := `[{"id":6,"title":"a","appid":"foot","monitor":"DP-1","tags":[1,3],"is_urgent":false,"is_focused":true},` +
		`{"id":3,"title":"b","appid":"firefox","monitor":"DP-2","tags":[1],"is_urgent":false,"is_focused":false},` +
		`{"id":2,"title":"c","appid":"kitty","monitor":"DP-1","tags":[1],"is_urgent":false,"is_focused":false}]`
	fakeMangoSocket(t, monitors, clients)
	cfg := config.Defaults()
	cfg.Workspaces.AppIconsShow = true
	cfg.Workspaces.AppIconsDedupe = false
	cfg.Workspaces.WorkspaceMap = map[string]config.WorkspaceStyle{"3": {Label: new("web")}}
	m := newTestMango(t, cfg, "DP-1")
	if len(m.models) != 2 {
		t.Fatalf("models = %+v", m.models)
	}
	one, three := m.models[0], m.models[1]
	if one.Label != "1" || three.Label != "web" {
		t.Errorf("labels = %q, %q, want the index and the tag-map label", one.Label, three.Label)
	}
	// DP-2's client is not this monitor's; client 6 sits on both tags.
	var ids []uint64
	for _, icon := range one.AppIcons {
		ids = append(ids, icon.WindowIDs...)
	}
	if !reflect.DeepEqual(ids, []uint64{6, 2}) {
		t.Errorf("tag 1 clients = %v, want [6 2] in list order", ids)
	}
	if len(three.AppIcons) != 1 || three.AppIcons[0].WindowIDs[0] != 6 {
		t.Errorf("tag 3 clients = %+v", three.AppIcons)
	}
	want := []string{"workspace", "active", "indicator-background", "tag-1"}
	if !reflect.DeepEqual(one.Classes, want) {
		t.Errorf("classes = %v, want %v (no focused, tag-N)", one.Classes, want)
	}
	if !widget.HasClass(m.strip.Segments(), "mango") {
		t.Error("container misses the mango class")
	}
}

func TestMangoClicksDispatch(t *testing.T) {
	monitors := `[{"name":"DP-1","active":true,"tags":` + nineTags(1, 1) + `,"active_tags":[1],"active_client":{"id":null},"keymode":"","keyboardlayout":""}]`
	dispatched := fakeMangoSocket(t, monitors, "[]")
	cfg := config.Defaults()
	cfg.Workspaces.MiddleClick = config.ParseWorkspaceClickAction("focus:last")
	m := newTestMango(t, cfg, "DP-1")
	b := m.strip.Segments().Segment(0)
	expect := func(want string) {
		t.Helper()
		if got := waittest.For(t, dispatched, "the dispatch "+want); got != want {
			t.Fatalf("dispatched %q, want %q", got, want)
		}
	}
	b.ClickAt(widget.Point{})
	expect("dispatch view,1")
	router, p := treetest.Pointer(m.strip.Segments(), b, 500, 40)
	router.Axis(0, 1, 0)
	expect("dispatch viewtoright")
	router.Axis(0, -1, 0)
	expect("dispatch viewtoleft")
	// focus:last has no mango action: nothing is sent.
	router.Press(widget.BTNMiddle, p, 0)
	select {
	case got := <-dispatched:
		t.Fatalf("focus:last dispatched %q", got)
	case <-time.After(quietWindow):
	}
}
