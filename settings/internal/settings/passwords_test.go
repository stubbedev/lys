package settings

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/secrets/client"
	"github.com/stubbedev/lys/secrets/secretstest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type callMonitor struct {
	mu    sync.Mutex
	calls map[string]int
}

func monitorCalls(t *testing.T, bus *dbustest.Bus) *callMonitor {
	t.Helper()
	conn, err := dbus.Connect(bus.Address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	m := &callMonitor{calls: map[string]int{}}
	if err := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, []string{"type='method_call'"}, uint32(0)).Err; err != nil {
		t.Fatal(err)
	}
	ch := make(chan *dbus.Message, 256)
	conn.Eavesdrop(ch)
	go func() {
		for msg := range ch {
			if msg.Type != dbus.TypeMethodCall {
				continue
			}
			member, _ := msg.Headers[dbus.FieldMember].Value().(string)
			m.mu.Lock()
			m.calls[member]++
			m.mu.Unlock()
		}
	}()
	return m
}

func (m *callMonitor) count(member string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[member]
}

type passwordsRig struct {
	srv *secretstest.Server
	bus *dbustest.Bus
	p   *passwordsPanel
	l   *loop
	mon *callMonitor
}

func startPasswords(t *testing.T, items int) *passwordsRig { return startPasswordsWith(t, items, "") }

func startPasswordsWith(t *testing.T, items int, yaml string) *passwordsRig {
	t.Helper()
	bus := dbustest.Start(t)
	srv := secretstest.Start(t, bus.Conn(t), "login-pw")
	ctx := context.Background()
	seed, err := client.Open(ctx, bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	for i := range items {
		name := strings.Repeat("x", i+1)
		if err := seed.StoreLogin(ctx, "token "+name, map[string]string{"xdg:schema": "org.git.Password", "user": name}, []byte("secret-"+name)); err != nil {
			t.Fatal(err)
		}
	}
	seed.Close()
	srv.Lock()
	before := openKeyring
	openKeyring = func(ctx context.Context) (*client.Client, func(), error) {
		conn, err := dbus.Connect(bus.Address)
		if err != nil {
			return nil, nil, err
		}
		c, err := client.OpenEncrypted(ctx, conn)
		if err != nil {
			return nil, nil, err
		}
		return c, func() { c.Close(); _ = conn.Close() }, nil
	}
	t.Cleanup(func() { openKeyring = before })
	mon := monitorCalls(t, bus)
	k := testKit(t, yaml)
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	p := passwordsKeyring(k).(*passwordsPanel)
	t.Cleanup(p.close)
	return &passwordsRig{srv: srv, bus: bus, p: p, l: l, mon: mon}
}

func (r *passwordsRig) itemRows() []*widget.Item {
	var out []*widget.Item
	for _, w := range treetest.WithClass(r.p.list, "password-item") {
		row, _ := w.(*widget.Item)
		out = append(out, row)
	}
	return out
}

func TestALockedCollectionIsListedWithoutAPromptAndARevealPromptsOnce(t *testing.T) {
	r := startPasswords(t, 1)
	if len(r.itemRows()) != 1 || r.srv.Asked() != 0 {
		t.Fatalf("%d items listed, %d prompts", len(r.itemRows()), r.srv.Asked())
	}
	r.itemRows()[0].ClickAt(widget.Point{})
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p.detail, "Reveal").OnClick()
	if r.srv.Asked() != 1 || string(r.p.revealed) != "secret-x" || r.p.secretLbl.Text() != "secret-x" {
		t.Fatalf("asked %d, revealed %q", r.srv.Asked(), r.p.revealed)
	}
	buf := r.p.revealed
	treetest.Button(t, r.p.detail, "Hide").OnClick()
	if !bytes.Equal(buf, make([]byte, len(buf))) || r.p.revealed != nil || r.p.secretLbl.Text() != hiddenText {
		t.Fatalf("after Hide the buffer holds %q", buf)
	}
}

func TestDeletingAnItemFollowsItemDeleted(t *testing.T) {
	r := startPasswords(t, 2)
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	r.itemRows()[0].ClickAt(widget.Point{})
	del := treetest.Button(t, r.p.detail, "Delete…")
	del.OnClick()
	treetest.Button(t, r.p.detail, "Delete This Item").OnClick()
	r.l.until("the list following the deletion", func() bool { return len(r.itemRows()) == 1 })
}

func TestChangePasswordRekeysThroughOnePrompt(t *testing.T) {
	r := startPasswords(t, 1)
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	file := filepath.Join(r.srv.Dir(), "login.keyring")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	asked := r.srv.Asked()
	r.srv.Answer("new-pw")
	treetest.Button(t, r.p, "Change Password…").OnClick()
	after, _ := os.ReadFile(file)
	if r.srv.Asked() != asked+1 || bytes.Equal(before[:64], after[:64]) {
		t.Fatalf("asked %d more, header changed %v", r.srv.Asked()-asked, !bytes.Equal(before[:64], after[:64]))
	}
}

func TestTheItemsLoadWithOneSearchAndOneGetAllAPage(t *testing.T) {
	r := startPasswords(t, itemsPerPage+3)
	r.l.settle()
	if r.mon.count("SearchItems") != 1 || r.mon.count("GetAll") != itemsPerPage+1 {
		t.Fatalf("SearchItems %d, GetAll %d; want one search and a GetAll for each of the first %d items plus the collection", r.mon.count("SearchItems"), r.mon.count("GetAll"), itemsPerPage)
	}
	if len(r.itemRows()) != itemsPerPage {
		t.Fatalf("%d rows on the first page", len(r.itemRows()))
	}
	treetest.Button(t, r.p, "Show More").OnClick()
	if r.mon.count("SearchItems") != 1 || r.mon.count("GetAll") != itemsPerPage+4 || len(r.itemRows()) != itemsPerPage+3 {
		t.Fatalf("after Show More: SearchItems %d, GetAll %d, rows %d", r.mon.count("SearchItems"), r.mon.count("GetAll"), len(r.itemRows()))
	}
}

func TestARevealAsksAgainOnceTheCollectionWasOpenTooLongAndACopyIsASecret(t *testing.T) {
	r := startPasswordsWith(t, 1, "passwords:\n  reveal-reprompt-minutes: 5\n")
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	r.p.now = func() time.Time { return now }
	var copied []string
	r.p.k.copySecret = func(text string) error { copied = append(copied, text); return nil }
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	r.itemRows()[0].ClickAt(widget.Point{})
	asked := r.srv.Asked()
	treetest.Button(t, r.p.detail, "Copy").OnClick()
	if r.srv.Asked() != asked || len(copied) != 1 || copied[0] != "secret-x" {
		t.Fatalf("a copy right after unlocking asked %d times and copied %q", r.srv.Asked()-asked, copied)
	}
	now = now.Add(6 * time.Minute)
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p.detail, "Reveal").OnClick()
	if r.srv.Asked() != asked+1 || string(r.p.revealed) != "secret-x" {
		t.Fatalf("six minutes on, the reveal asked %d times", r.srv.Asked()-asked)
	}
}

func TestAnItemIsSummarisedByItsAccountAndPlaceElseByWhatStoredIt(t *testing.T) {
	for _, tc := range []struct {
		attrs map[string]string
		want  string
	}{
		{map[string]string{"username": "stubbedev", "service": "gh:github.com", schemaAttribute: "org.freedesktop.Secret.Generic"}, "stubbedev · gh:github.com"},
		{map[string]string{"user": "alice", "username": "alice"}, "alice"},
		{map[string]string{"application": "chrome"}, "chrome"},
		{map[string]string{schemaAttribute: "org.git.Password"}, "git"},
		{map[string]string{schemaAttribute: "org.freedesktop.Secret.Generic"}, ""},
		{nil, ""},
	} {
		if got := itemSummary(tc.attrs); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.attrs, got, tc.want)
		}
	}
}

func TestAnItemsTimesReadInTheLocalZone(t *testing.T) {
	const at = 1762516800
	copenhagen, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		t.Fatal(err)
	}
	if got := itemTime(at, copenhagen); got != "2025-11-07 13:00" {
		t.Errorf("in Copenhagen the time reads %q", got)
	}
	if got := itemTime(at, time.UTC); got != "2025-11-07 12:00" {
		t.Errorf("in UTC the time reads %q", got)
	}
}

func TestTheSchemaShowsOnceAsStoredByNotAsAnAttribute(t *testing.T) {
	attrs := map[string]string{schemaAttribute: "org.git.Password", "user": "me", "service": "github.com"}
	if got := shownAttributes(attrs); !slices.Equal(got, []string{"service", "user"}) {
		t.Errorf("shown attributes %q", got)
	}
	if got := shownAttributes(map[string]string{"user": "me"}); !slices.Equal(got, []string{"user"}) {
		t.Errorf("without a schema the attributes are %q", got)
	}
}

func TestAnAccountNameIsCopiedAsPlainText(t *testing.T) {
	r := startPasswords(t, 1)
	var copied, secrets []string
	r.p.k.copyText = func(text string) error { copied = append(copied, text); return nil }
	r.p.k.copySecret = func(text string) error { secrets = append(secrets, text); return nil }
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	r.itemRows()[0].ClickAt(widget.Point{})
	asked := r.srv.Asked()
	treetest.Button(t, r.p.detail, "Copy Username").OnClick()
	if !slices.Equal(copied, []string{"x"}) || len(secrets) != 0 || r.srv.Asked() != asked || r.p.problem.Visible() {
		t.Fatalf("copying the username copied %q as text and %q as a secret, asked %d", copied, secrets, r.srv.Asked()-asked)
	}
	r.p.k.copyText = func(string) error { return errors.New("no data device") }
	treetest.Button(t, r.p.detail, "Copy Username").OnClick()
	if !r.p.problem.Visible() || !strings.Contains(r.p.problem.Text(), "no data device") {
		t.Errorf("a failed copy reads %q (%v)", r.p.problem.Text(), r.p.problem.Visible())
	}
}

func TestTheItemsAreGroupedByWhatStoredThemWithItsIcon(t *testing.T) {
	git := client.Item{Attributes: map[string]string{schemaAttribute: "org.git.Password"}}
	wifi := client.Item{Attributes: map[string]string{schemaAttribute: "org.freedesktop.NetworkManager.Connection"}}
	plain := client.Item{Attributes: map[string]string{"service": "example"}}
	locked := client.Item{Locked: true, Attributes: git.Attributes}
	for _, c := range []struct {
		it   client.Item
		want string
	}{{git, "ld-code-symbolic"}, {wifi, "ld-wifi-symbolic"}, {plain, itemKeyIcon}, {locked, itemLockIcon}} {
		if got := itemIcon(c.it); got != c.want {
			t.Errorf("%v (locked %v) shows %s, want %s", c.it.Attributes, c.it.Locked, got, c.want)
		}
	}
	if itemGroup(git) != "git" || itemGroup(wifi) != "NetworkManager" || itemGroup(plain) != locale.T().Get("settings-passwords-group-other") {
		t.Errorf("groups %q, %q, %q", itemGroup(git), itemGroup(wifi), itemGroup(plain))
	}
	r := startPasswords(t, 3)
	r.l.settle()
	headers := treetest.WithClass(r.p.list, "password-group")
	if len(headers) != 1 || len(r.itemRows()) != 3 {
		t.Fatalf("three git items show %d group headings over %d rows", len(headers), len(r.itemRows()))
	}
}
