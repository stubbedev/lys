package settings

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/secrets/client"
	"github.com/stubbedev/lys/secrets/secretstest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

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
	pickers *fakePickers
	srv     *secretstest.Server
	bus     *dbustest.Bus
	p       *passwordsPanel
	l       *loop
	mon     *callMonitor
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
	pickers := &fakePickers{}
	k.pickers = pickers
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	p := passwordsKeyring(k).(*passwordsPanel)
	t.Cleanup(p.close)
	return &passwordsRig{pickers: pickers, srv: srv, bus: bus, p: p, l: l, mon: mon}
}

func (r *passwordsRig) itemRows() []*settingRow {
	var out []*settingRow
	for _, w := range treetest.WithClass(r.p.list, "password-item") {
		row, _ := w.(*settingRow)
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
	treetest.Button(t, r.p.detail, "Delete…").OnClick()
	if len(r.pickers.dialogs) != 1 || r.pickers.dialogs[0].Title != locale.T().Get("settings-passwords-delete-item-title", i18n.Str("label", "token x")) {
		t.Fatalf("the delete asked %+v", r.pickers.dialogs)
	}
	r.pickers.respond(responseCancel)
	r.l.drain()
	if len(r.itemRows()) != 2 {
		t.Fatalf("a cancelled delete left %d items", len(r.itemRows()))
	}
	treetest.Button(t, r.p.detail, "Delete…").OnClick()
	r.pickers.respond(responseConfirm)
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
	if !r.p.problem.Visible() || r.p.problem.Text() != locale.T().Get("settings-passwords-failed") || !strings.Contains(r.p.problem.detail.Text(), "no data device") {
		t.Errorf("a failed copy reads %q over %q (%v)", r.p.problem.Text(), r.p.problem.detail.Text(), r.p.problem.Visible())
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

func (r *passwordsRig) collectionRows() []*settingRow {
	var out []*settingRow
	for _, w := range treetest.WithClass(r.p.sidebar, "password-collection") {
		row, _ := w.(*settingRow)
		out = append(out, row)
	}
	return out
}

// sidebarText is what the collection rows read: each title, then its
// count.
func (r *passwordsRig) sidebarText() []string {
	var out []string
	for _, row := range r.collectionRows() {
		out = append(out, row.Title(), row.Description())
	}
	return out
}

func countText(n int64) string {
	return locale.T().Get("settings-passwords-count", i18n.Int("count", n))
}

func TestTheCollectionsAreASidebarCountingTheirItems(t *testing.T) {
	r := startPasswords(t, 3)
	r.l.settle()
	rows := r.collectionRows()
	if len(rows) != 1 || !rows[0].HasState(widget.StateSelected) {
		t.Fatalf("%d collection rows, the login one selected: %v", len(rows), len(rows) == 1 && rows[0].HasState(widget.StateSelected))
	}
	labels := r.sidebarText()
	if !slices.Equal(labels, []string{locale.T().Get("settings-passwords-locked-collection"), countText(3)}) {
		t.Fatalf("the sidebar reads %q", labels)
	}
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	if labels := r.sidebarText(); !slices.Equal(labels, []string{"Login", countText(3)}) {
		t.Fatalf("unlocked, the sidebar reads %q", labels)
	}
	if got := countText(1); got != "One item" {
		t.Errorf("one item reads %q", got)
	}
	if got := countText(0); got != "No items" {
		t.Errorf("an empty collection reads %q", got)
	}
}

func TestAnItemsMenuCopiesItsPasswordAndUsernameAndDeletesItAfterAsking(t *testing.T) {
	r := startPasswords(t, 2)
	root, host := hosted(r.p)
	var copied, secrets []string
	r.p.k.copyText = func(text string) error { copied = append(copied, text); return nil }
	r.p.k.copySecret = func(text string) error { secrets = append(secrets, text); return nil }
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	root.Arrange(render.Rect{W: 800, H: 3000})
	more := treetest.WithClass(r.itemRows()[0], css.Icon)
	if len(more) != 1 {
		t.Fatalf("the item row holds %d icon buttons, want its … menu", len(more))
	}
	more[0].(*widget.Button).OnClick()
	if len(host.menus) != 1 {
		t.Fatal("the … button opened no menu")
	}
	run := func(label string) {
		t.Helper()
		for _, it := range host.menus[len(host.menus)-1] {
			if it.Label == label {
				it.OnClick()
				return
			}
		}
		t.Fatalf("no %q in the menu", label)
	}
	run("Copy Password")
	run("Copy Username")
	if !slices.Equal(secrets, []string{"secret-x"}) || !slices.Equal(copied, []string{"x"}) {
		t.Fatalf("copied the secrets %q and the texts %q", secrets, copied)
	}
	run("Show Details")
	if r.p.expanded != r.p.items[0].Path {
		t.Fatal("Show Details did not open the item")
	}
	run("Delete…")
	if len(r.pickers.dialogs) != 1 {
		t.Fatalf("Delete… asked %d questions", len(r.pickers.dialogs))
	}
	r.pickers.respond(responseConfirm)
	r.l.until("the deleted item gone", func() bool { return len(r.itemRows()) == 1 })
}

func TestAnItemWithoutAnAccountHasNoCopyUsername(t *testing.T) {
	p := &passwordsPanel{}
	for _, it := range p.itemMenu(client.Item{Attributes: map[string]string{"service": "mail"}}) {
		if it.Label == "Copy Username" {
			t.Fatal("an item without an account offers Copy Username")
		}
	}
}

func TestANewItemNeedsANameAndAPasswordAndKeepsWhatWasGiven(t *testing.T) {
	item, problem := parseNewItem("  Mail ", " me@example.org ", "", []byte("pw"))
	if problem != "" {
		t.Fatal(problem.text())
	}
	want := map[string]string{schemaAttribute: genericSchema, usernameAttr: "me@example.org"}
	if item.label != "Mail" || !maps.Equal(item.attrs, want) || string(item.secret) != "pw" {
		t.Fatalf("parsed %+v", item)
	}
	if _, problem := parseNewItem(" ", "me", "mail", []byte("pw")); problem != itemNoName {
		t.Errorf("a blank name: %q", problem)
	}
	if _, problem := parseNewItem("Mail", "me", "mail", nil); problem != itemNoSecret {
		t.Errorf("no password: %q", problem)
	}
}

func TestAddItemStoresANewItemInTheShownCollectionAndStaysOpenOverAMissingName(t *testing.T) {
	r := startPasswords(t, 1)
	r.srv.Answer("login-pw")
	treetest.Button(t, r.p, "Unlock…").OnClick()
	treetest.Button(t, r.p, "Add Item…").OnClick()
	if len(r.pickers.dialogs) != 1 || r.pickers.dialogs[0].Title != locale.T().Get("settings-passwords-add-title", i18n.Str("collection", "Login")) {
		t.Fatalf("Add Item opened %+v", r.pickers.dialogs)
	}
	f := r.pickers.dialogs[0].Content.(*keyringItemForm)
	f.secret.SetText("hunter2")
	if r.pickers.respond(responseConfirm) || r.pickers.dialogsClosed != 0 || !f.messages.Visible() || f.messages.Text() != "Give the item a name" {
		t.Fatalf("an unnamed item closed the dialog or reads %q", f.messages.Text())
	}
	f.label.SetText("Mail")
	f.username.SetText("me@example.org")
	f.service.SetText("imap.example.org")
	r.pickers.respond(responseConfirm)
	if r.pickers.dialogsClosed != 1 {
		t.Fatalf("the dialog stayed open: %q", f.messages.Text())
	}
	r.l.until("the new item listed", func() bool { return len(r.itemRows()) == 2 })
	i := slices.IndexFunc(r.p.items, func(it client.Item) bool { return it.Label == "Mail" })
	if i < 0 || r.p.items[i].Attributes["service"] != "imap.example.org" || r.p.items[i].Attributes[schemaAttribute] != genericSchema {
		t.Fatalf("items %+v", r.p.items)
	}
	if f.secret.Text() != "" {
		t.Error("the password stayed in the closed form")
	}
	if !slices.Contains(r.sidebarText(), countText(2)) {
		t.Errorf("the count did not follow: %q", r.sidebarText())
	}
}

// A keyring that could not be read still offered Add Item, Lock and
// Change Password, pressed against the card's top edge: the actions
// show, inset in the card, once a collection is shown, and not before.
func TestTheKeyringActionsShowOnlyOverAShownCollection(t *testing.T) {
	before := openKeyring
	openKeyring = func(context.Context) (*client.Client, func(), error) {
		return nil, nil, errors.New("no keyring daemon")
	}
	t.Cleanup(func() { openKeyring = before })
	k := testKit(t, "")
	p := passwordsKeyring(k).(*passwordsPanel)
	t.Cleanup(p.close)
	if p.head.Visible() {
		t.Error("an unreadable keyring offers its collection's actions")
	}
	if !treetest.Contains(p.list, p.add) {
		t.Error("the actions sit outside the card's inset rows")
	}
	r := startPasswords(t, 1)
	r.l.until("the collection shown", func() bool { return r.p.head.Visible() })
}
