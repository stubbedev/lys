package settings

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	secrets "github.com/stubbedev/lys/secrets/control"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeAccount struct {
	name, full string
	uid        uint32
	kind       accounts.Kind
}

var (
	alice = fakeAccount{name: "alice", full: "Alice", uid: uint32(os.Getuid())}
	bob   = fakeAccount{name: "bob", full: "Bob", uid: 50002}
)

// fakeAccounts is the passwd and group databases lys-settings reads, and
// an accounts service that takes every request the real one decodes,
// records it and, unless refused, makes it.
type fakeAccounts struct {
	mu       sync.Mutex
	accounts []fakeAccount
	requests []accounts.Request
	refuse   error
}

func (f *fakeAccounts) read() (accounts.Database, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var passwd strings.Builder
	var admins []string
	passwd.WriteString("root:x:0:0:root:/root:/bin/sh\nsvc:x:998:998:service:/var/lib/svc:/usr/sbin/nologin\n")
	for _, a := range f.accounts {
		fmt.Fprintf(&passwd, "%s:x:%d:%d:%s:/home/%s:/bin/sh\n", a.name, a.uid, a.uid, a.full, a.name)
		if a.kind == accounts.Admin {
			admins = append(admins, a.name)
		}
	}
	return accounts.ParseDatabase(passwd.String(), "wheel:x:10:"+strings.Join(admins, ",")+"\n", useraccount.UIDRange{Min: 1, Max: 60000}), nil
}

func (f *fakeAccounts) change(_ context.Context, r accounts.Request) error {
	data, err := accounts.Encode(r)
	if err != nil {
		return err
	}
	decoded, err := accounts.Decode(data)
	if err != nil {
		return accounts.Errorf(accounts.ErrInvalid, "%v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, decoded)
	if f.refuse != nil {
		return f.refuse
	}
	switch r := decoded.(type) {
	case accounts.Rename:
		f.edit(alice.name, func(a *fakeAccount) { a.full = string(r.RealName) })
	case accounts.Create:
		f.accounts = append(f.accounts, fakeAccount{name: string(r.Login), full: string(r.RealName), uid: 50100, kind: r.Kind})
	case accounts.Edit:
		f.edit(string(r.Login), func(a *fakeAccount) {
			if r.RealName != nil {
				a.full = string(*r.RealName)
			}
			if r.Kind != nil {
				a.kind = *r.Kind
			}
		})
	case accounts.Delete:
		f.accounts = slices.DeleteFunc(f.accounts, func(a fakeAccount) bool { return a.name == string(r.Login) })
	}
	return nil
}

func (f *fakeAccounts) edit(name string, fn func(*fakeAccount)) {
	for i := range f.accounts {
		if f.accounts[i].name == name {
			fn(&f.accounts[i])
		}
	}
}

func (f *fakeAccounts) refuseWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse = err
}

func (f *fakeAccounts) made() []accounts.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}

func (f *fakeAccounts) named(name string) fakeAccount {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if a.name == name {
			return a
		}
	}
	return fakeAccount{}
}

func serveAccounts(k *kit, self fakeAccount, others ...fakeAccount) *fakeAccounts {
	f := &fakeAccounts{accounts: append([]fakeAccount{self}, others...)}
	k.readAccounts, k.changeAccount = f.read, f.change
	return f
}

var errNotAuthorized = accounts.Errorf(accounts.ErrNotAuthorized, "polkit does not authorize uid 1000 for dev.stubbe.lys.settings.users.others")

func usersFixture(t *testing.T, bus *dbustest.Bus, accts ...fakeAccount) (*usersPanel, *loop, *fakeAccounts) {
	t.Helper()
	k, l := systemKit(t, bus)
	var f *fakeAccounts
	if len(accts) > 0 {
		f = serveAccounts(k, accts[0], accts[1:]...)
	}
	p := usersFooter(k).(*usersPanel)
	t.Cleanup(p.close)
	return p, l, f
}

func TestAStockAvatarIsOnePictureRequestAndTheNameOneRename(t *testing.T) {
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, alice, bob)
	l.until("the account", func() bool { return p.realName.Enabled() })
	if p.realName.Text() != "Alice" || p.admin.Visible() {
		t.Fatalf("name %q, admin section shown %v for a standard user", p.realName.Text(), p.admin.Visible())
	}
	stockAvatarChoice(t, p, 0).OnClick()
	l.until("the picture", func() bool { return len(f.made()) == 1 })
	pic, ok := f.made()[0].(accounts.SetPicture)
	if !ok {
		t.Fatalf("%#v", f.made())
	}
	img, err := png.Decode(bytes.NewReader(pic.Picture))
	if err != nil || img.Bounds().Dx() != avatarSide || img.Bounds().Dy() != avatarSide {
		t.Fatalf("the avatar handed over is %v, %v; want a %dpx PNG", img.Bounds(), err, avatarSide)
	}
	p.realName.SetText("Alice Liddell")
	p.realName.OnActivate("Alice Liddell")
	l.until("the rename", func() bool { return len(f.made()) == 2 })
	if r, ok := f.made()[1].(accounts.Rename); !ok || r.RealName != "Alice Liddell" {
		t.Fatalf("%#v", f.made()[1])
	}
	if got := f.named("alice").full; got != "Alice Liddell" || p.problem.Visible() {
		t.Fatalf("alice is called %q, the page reads %q", got, p.problem.Text())
	}
}

func TestARefusedRenameOrPictureLeavesTheAccountAndSaysWhy(t *testing.T) {
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, alice)
	l.until("the account", func() bool { return p.realName.Enabled() })
	f.refuseWith(errNotAuthorized)
	p.realName.SetText("Mallory")
	p.realName.OnActivate("Mallory")
	l.until("the refusal", func() bool { return p.problem.Visible() })
	if p.problem.Text() != locale.T().Get("settings-users-failed") || p.problem.detail.Text() != errNotAuthorized.Error() {
		t.Fatalf("the page reads %q over %q", p.problem.Text(), p.problem.detail.Text())
	}
	if got := f.named("alice").full; got != "Alice" || p.realName.Text() != "Alice" || p.face.Initials() != "A" {
		t.Fatalf("after a refused rename alice is %q, the field reads %q, the avatar %q", got, p.realName.Text(), p.face.Initials())
	}
	p.problem.Clear()
	stockAvatarChoice(t, p, 1).OnClick()
	l.until("the second refusal", func() bool { return p.problem.Visible() })
	if _, ok := f.made()[1].(accounts.SetPicture); !ok || p.problem.detail.Text() != errNotAuthorized.Error() {
		t.Fatalf("asked %#v, the page reads %q", f.made(), p.problem.detail.Text())
	}
}

func TestANameTheGECOSFieldCannotHoldIsRefusedBeforeTheHelper(t *testing.T) {
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, alice)
	l.until("the account", func() bool { return p.realName.Enabled() })
	if err := p.rename("Alice:x:0:0"); !errors.Is(err, accounts.ErrRealName) {
		t.Fatalf("rename: %v, want %v", err, accounts.ErrRealName)
	}
	l.drain()
	if len(f.made()) != 0 {
		t.Fatalf("the helper was asked: %#v", f.made())
	}
}

func TestTheHeaderShowsWhoYouAreAndKeepsANameTypedAndLeft(t *testing.T) {
	self := alice
	self.kind = accounts.Admin
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, self)
	l.until("the account", func() bool { return p.realName.Enabled() })
	tr := locale.T()
	if want := tr.Get("settings-users-identity", i18n.Str("login", "alice"), i18n.Str("type", tr.Get("settings-users-type-admin"))); p.identity.Text() != want {
		t.Errorf("the header reads %q, want %q", p.identity.Text(), want)
	}
	if p.face.Initials() != "A" || treetest.First[*widget.Avatar](t, p) != p.face {
		t.Errorf("the header's avatar is not the first picture on the page, or reads %q", p.face.Initials())
	}
	p.realName.SetText("Alice Liddell")
	p.realName.OnChanged("Alice Liddell")
	if !p.realName.dirtyBadge().Visible() {
		t.Fatal("a name typed and not saved shows no unsaved badge")
	}
	p.realName.SetText("  Alice Liddell ")
	p.realName.OnActivate(p.realName.Text())
	l.until("the rename", func() bool { return len(f.made()) == 1 })
	if r := f.made()[0].(accounts.Rename); r.RealName != "Alice Liddell" || p.realName.dirtyBadge().Visible() {
		t.Fatalf("the name saved %#v, badge still shown %v", r, p.realName.dirtyBadge().Visible())
	}
	if p.face.Initials() != "AL" {
		t.Errorf("after the rename the avatar reads %q", p.face.Initials())
	}
}

func TestAnAccountPictureThatCannotBeReadSaysSo(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "alice")
	put(t, broken, "not a picture")
	if img, err := loadIcon(broken); err == nil || img != nil {
		t.Fatalf("a broken picture loaded %v, %v", img, err)
	}
	for _, none := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if img, err := loadIcon(none); err != nil || img != nil {
			t.Errorf("no picture at %q gave %v, %v", none, img, err)
		}
	}
}

func TestWithoutTheAccountsTheCardSaysSoAndNothingIsChanged(t *testing.T) {
	bus := dbustest.Start(t)
	p, l, _ := usersFixture(t, bus)
	l.drain()
	if p.realName.Enabled() || p.admin.Visible() {
		t.Fatal("the name or the admin section without the accounts")
	}
	if st, want := p.accounts, locale.T().Get("settings-users-unavailable"); st.kind != stateUnavailable || st.Title() != want || st.detail.Text() == "" || p.realName.Placeholder() != "" {
		t.Fatalf("without the accounts the card shows %v %q over %q, the name reads %q; want %q", st.kind, st.Title(), st.detail.Text(), p.realName.Placeholder(), want)
	}
	if err := p.rename("Mallory"); err == nil {
		t.Fatal("a rename without the accounts was taken")
	}
}

func TestAnAccountNotInThePasswdDatabaseIsUnavailable(t *testing.T) {
	bus := dbustest.Start(t)
	p, l, _ := usersFixture(t, bus, fakeAccount{name: "ldap", uid: uint32(os.Getuid()) + 1})
	l.drain()
	if p.realName.Enabled() || p.accounts.kind != stateUnavailable || !strings.Contains(p.accounts.detail.Text(), "no entry") {
		t.Fatalf("an account the passwd database does not hold shows %v %q", p.accounts.kind, p.accounts.detail.Text())
	}
}

func TestAnAdministratorSeesTheOtherUsers(t *testing.T) {
	admin := alice
	admin.kind = accounts.Admin
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, admin, bob)
	l.until("the admin section", func() bool { return p.admin.Visible() && len(p.others) == 2 })
	if got := len(linksTitled(p.admin, locale.T().Get("settings-users-edit"))); got != 1 {
		t.Fatalf("the list offers %d users' details, want only the other user's", got)
	}
	for _, want := range []string{"Alice", "Bob", locale.T().Get("settings-users-identity", i18n.Str("login", "bob"), i18n.Str("type", locale.T().Get("settings-users-type-standard")))} {
		if !slices.Contains(treetest.Labels(p.admin), want) {
			t.Errorf("the list reads %v, want %q", treetest.Labels(p.admin), want)
		}
	}
	if slices.Contains(treetest.Labels(p.admin), "service") || !slices.Contains(p.taken, "svc") {
		t.Errorf("a system account is listed, or its name is free: %v, %v", treetest.Labels(p.admin), p.taken)
	}
	pickers := &fakePickers{}
	p.k.pickers = pickers
	remove := func() app.DialogConfig {
		linksTitled(p.admin, locale.T().Get("settings-users-edit"))[0].ClickAt(widget.Point{})
		if !pickers.respond(responseRemove) {
			t.Fatal("Remove User kept the details open")
		}
		return pickers.dialogs[len(pickers.dialogs)-1]
	}
	ask := remove()
	if ask.Title != locale.T().Get("settings-users-remove-title", i18n.Str("name", "Bob")) || !ask.Modal || ask.CancelResponse != responseCancel || ask.DefaultResponse != "" {
		t.Fatalf("dialog %q modal %v cancel %q default %q", ask.Title, ask.Modal, ask.CancelResponse, ask.DefaultResponse)
	}
	ask.OnResponse(responseCancel)
	l.drain()
	if len(f.made()) != 0 {
		t.Fatalf("cancelling removed a user: %#v", f.made())
	}
	f.refuseWith(errNotAuthorized)
	ask.OnResponse(responseKeepHome)
	l.until("the refusal", func() bool { return p.problem.Visible() })
	if d, ok := f.made()[0].(accounts.Delete); !ok || d != (accounts.Delete{Login: "bob"}) || f.named("bob").name != "bob" {
		t.Fatalf("keeping the home folder asked %#v and left %+v", f.made(), f.named("bob"))
	}
	l.until("the list again", func() bool { return len(linksTitled(p.admin, locale.T().Get("settings-users-edit"))) == 1 })
	f.refuseWith(nil)
	remove().OnResponse(responseDeleteFiles)
	l.until("the removal", func() bool { return len(f.made()) == 2 })
	if d := f.made()[1].(accounts.Delete); d != (accounts.Delete{Login: "bob", RemoveHome: true}) {
		t.Fatalf("deleting the files: %#v", d)
	}
	l.until("the list without bob", func() bool { return len(linksTitled(p.admin, locale.T().Get("settings-users-edit"))) == 0 })
}

func TestWithoutAWindowRemovingAUserIsRefused(t *testing.T) {
	admin := alice
	admin.kind = accounts.Admin
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, admin, bob)
	l.until("the admin section", func() bool { return len(linksTitled(p.admin, locale.T().Get("settings-users-edit"))) == 1 })
	p.k.pickers = nil
	p.confirmRemove(p.others[1])
	l.drain()
	if !p.problem.Visible() || len(f.made()) != 0 {
		t.Fatalf("problem shown %v, requests %#v", p.problem.Visible(), f.made())
	}
}

func TestTheUsersPageOfARealAccountFitsTheWindow(t *testing.T) {
	self := fakeAccount{name: "alexanderstage", full: "Alexander Stage", uid: uint32(os.Getuid()), kind: accounts.Admin}
	others := []fakeAccount{
		{name: "margaret.hamilton", full: "Margaret Hamilton-Whitfield", uid: 50001},
		{name: "jonas.kristoffersen", full: "Jonas Bjørn Kristoffersen", uid: 50002, kind: accounts.Admin},
		{name: "soeren", full: "Søren Østergaard Nielsen", uid: 50003},
		{name: "backup-operator", full: "Nightly Backup Operator Account", uid: 50004},
		{name: "guest", uid: 50005},
	}
	bus := dbustest.Start(t)
	enrolled := []string{"right-index-finger", "left-index-finger"}
	serveFprint(t, bus, true, &enrolled, new(atomic.Bool))
	k, _ := systemKit(t, bus)
	serveAccounts(k, self, others...)
	w := newWindow(k, k.store.svc.Config(), func(_ time.Duration, fn func()) { fn() })
	w.show(usersID)
	if got := len(linksTitled(w.page, locale.T().Get("settings-users-edit"))); got != len(others) {
		t.Fatalf("the users page offers %d other accounts, want %d", got, len(others))
	}
	hands := treetest.All[*fingerHands](w.page)
	offered := 0
	for _, f := range fingers {
		if len(hands) == 1 && hands[0].offers(f) {
			offered++
		}
	}
	if offered != len(fingers)-len(enrolled) {
		t.Fatalf("the users page offers %d fingers, want %d", offered, len(fingers)-len(enrolled))
	}
	if got := len(treetest.WithClass(w.page, "fingerprint-enrolled")); got != len(enrolled) {
		t.Fatalf("the users page lists %d enrolled fingers, want %d", got, len(enrolled))
	}
	assertPageFits(t, w)
}

type fakeUsersService struct {
	requests dbustest.Var[[]accounts.Request]
}

func (f *fakeUsersService) Change(request []byte) *dbus.Error {
	r, err := accounts.Decode(request)
	if err != nil {
		return accounts.Errorf(accounts.ErrInvalid, "%v", err).BusError()
	}
	f.requests.Update(func(rs []accounts.Request) []accounts.Request { return append(rs, r) })
	if d, ok := r.(accounts.Delete); ok && d.Login == "zoe" {
		return accounts.Errorf(accounts.ErrTaken, "zoe is already an account").BusError()
	}
	return nil
}

func TestAChangeIsTheServicesChangeAndItsRefusalIsTheError(t *testing.T) {
	bus := dbustest.Start(t)
	dial := func() (*dbus.Conn, error) { return dbus.Connect(bus.Address) }
	change := changeAccountOver(dial)
	if err := change(t.Context(), accounts.Delete{Login: "bob"}); err == nil || errors.As(err, new(*accounts.Error)) {
		t.Fatalf("a change with no service on the bus: %v", err)
	}
	fake := &fakeUsersService{}
	release, err := dbusx.Serve(bus.Conn(t), dbusx.Service{Name: accounts.BusName, Path: accounts.BusPath, Interface: accounts.BusInterface, Methods: fake})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if err := change(t.Context(), accounts.Delete{Login: "bob"}); err != nil {
		t.Fatalf("a change the service made: %v", err)
	}
	if got := fake.requests.Load(); len(got) != 1 || got[0] != (accounts.Delete{Login: "bob"}) {
		t.Fatalf("the service got %#v", got)
	}
	err = change(t.Context(), accounts.Delete{Login: "zoe"})
	if e, ok := errors.AsType[*accounts.Error](err); !ok || e.Kind != accounts.ErrTaken || e.Detail != "zoe is already an account" {
		t.Fatalf("a refusal: %v", err)
	}
	unreachable := errors.New("no system bus")
	if err := changeAccountOver(func() (*dbus.Conn, error) { return nil, unreachable })(t.Context(), accounts.Delete{Login: "bob"}); !errors.Is(err, unreachable) {
		t.Fatalf("no bus: %v", err)
	}
}

type fakeLogin struct {
	mu         sync.Mutex
	old, next  string
	rekeyCalls int
}

func (f *fakeLogin) UnlockLogin([]byte) error { return nil }

func (f *fakeLogin) RekeyLogin(old, next []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.old, f.next = string(old), string(next)
	f.rekeyCalls++
	return nil
}

func pwqualityStack(t *testing.T) (passwordChanger, *fakeLogin) {
	t.Helper()
	module := os.Getenv("LYS_SETTINGS_TEST_PWQUALITY")
	if module == "" {
		t.Fatal("LYS_SETTINGS_TEST_PWQUALITY names pam_pwquality.so; the dev shell sets it")
	}
	dir := t.TempDir()
	put(t, filepath.Join(dir, "lys-settings-passwd"), "password requisite "+module+" retry=1 minlen=12 dictcheck=0\npassword required pam_permit.so\n")
	sock, err := os.MkdirTemp("", "lys-settings-pw-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sock) })
	path := filepath.Join(sock, "control")
	ln, err := secrets.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	login := &fakeLogin{}
	ctx, cancel := context.WithCancel(context.Background())
	go secrets.Serve(ctx, ln, login)
	t.Cleanup(func() { cancel(); _ = ln.Close() })
	pc := passwordChanger{service: "lys-settings-passwd", confdir: dir, rekey: func(old, next []byte) error { return secrets.Rekey(path, old, next) }}
	return pc, login
}

func passwordDialog(t *testing.T, p *usersPanel) (*fakePickers, *passwordForm) {
	t.Helper()
	pickers := &fakePickers{}
	p.k.pickers = pickers
	p.openPasswordDialog()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("changing the password opened %d dialogs, want one", len(pickers.dialogs))
	}
	return pickers, pickers.dialogs[0].Content.(*passwordForm)
}

func TestAPasswordChangeShowsPwqualitysRefusalAndRekeysOnSuccess(t *testing.T) {
	pc, login := pwqualityStack(t)
	logged := logtest.Capture(t)
	k := testKit(t, "")
	p := &usersPanel{Box: gapBox(widget.Column), k: k, changer: pc, user: "nobody"}
	pickers, f := passwordDialog(t, p)
	entries := treetest.All[*widget.Entry](f)
	if len(entries) != 3 {
		t.Fatalf("%d entries", len(entries))
	}
	set := func(old, next, confirm string) bool {
		entries[0].SetText(old)
		entries[1].SetText(next)
		entries[2].SetText(confirm)
		return pickers.respond(responseConfirm)
	}
	if set("old-secret-1", "short", "short") {
		t.Fatal("a refused change closed the dialog")
	}
	if !f.messages.Visible() || !strings.Contains(f.messages.Text(), "BAD PASSWORD") {
		t.Fatalf("the dialog shows %q, want pwquality's refusal", f.messages.Text())
	}
	if login.rekeyCalls != 0 || pickers.dialogsClosed != 0 {
		t.Fatalf("a refused change rekeyed %d times and closed the dialog %d times", login.rekeyCalls, pickers.dialogsClosed)
	}
	set("old-secret-1", "a-much-Longer-pass9", "a-much-Longer-pass9")
	if login.rekeyCalls != 1 || login.old != "old-secret-1" || login.next != "a-much-Longer-pass9" {
		t.Fatalf("rekey %d with %q -> %q", login.rekeyCalls, login.old, login.next)
	}
	if pickers.dialogsClosed != 1 {
		t.Fatalf("the changed password closed the dialog %d times, want once", pickers.dialogsClosed)
	}
	for _, e := range entries {
		if e.Text() != "" {
			t.Fatal("a changed password is left in its entry")
		}
	}
	for _, pw := range []string{"old-secret-1", "a-much-Longer-pass9", "short"} {
		if strings.Contains(logged.String(), pw) || strings.Contains(strings.Join(os.Args, " "), pw) || strings.Contains(strings.Join(os.Environ(), " "), pw) {
			t.Fatalf("a password reached the log, argv or the environment")
		}
	}
}

func TestThePasswordDialogSaysHowStrongInWordsAndRefusesAMismatch(t *testing.T) {
	p := usersFooter(testKit(t, "")).(*usersPanel)
	t.Cleanup(p.close)
	pickers, f := passwordDialog(t, p)
	cfg := pickers.dialogs[0]
	tr := locale.T()
	if !cfg.Modal || cfg.CancelResponse != responseCancel || cfg.DefaultResponse != responseConfirm || cfg.Focus != widget.Widget(f.current) {
		t.Fatalf("dialog modal %v cancel %q default %q focus %v", cfg.Modal, cfg.CancelResponse, cfg.DefaultResponse, cfg.Focus)
	}
	if len(treetest.WithClass(p, "password-change")) != 0 {
		t.Fatal("the password form is drawn on the page, not in the dialog")
	}
	entries := treetest.All[*widget.Entry](f)
	next, word := entries[1], treetest.WithClass(f, "password-strength")[0].(*widget.Label)
	for pw, want := range map[string]string{"": "", "abc": tr.Get("settings-users-password-weak"), "correct-Horse-battery-9": tr.Get("settings-users-password-good"), "correct-Horse-battery-staple-9": tr.Get("settings-users-password-strong")} {
		next.SetText(pw)
		next.OnChanged(pw)
		if word.Text() != want {
			t.Errorf("%q reads %q, want %q", pw, word.Text(), want)
		}
	}
	reveal := slices.DeleteFunc(treetest.WithClass(f, css.Icon), func(w widget.Widget) bool { return treetest.Contains(f.messages, w) })
	if len(reveal) != 2 {
		t.Fatalf("%d reveal buttons, want one for the current and one for the new password", len(reveal))
	}
	reveal[1].(*widget.Button).OnClick()
	if entries[0].Echo() != widget.EchoPassword || entries[1].Echo() != widget.EchoNormal || entries[2].Echo() != widget.EchoNormal {
		t.Errorf("revealing the new password shows echoes %v %v %v", entries[0].Echo(), entries[1].Echo(), entries[2].Echo())
	}
	reveal[1].(*widget.Button).OnClick()
	if entries[1].Echo() != widget.EchoPassword || entries[2].Echo() != widget.EchoPassword {
		t.Error("hiding the new password again left it shown")
	}
	entries[0].SetText("old")
	next.SetText("one-Password-9")
	entries[2].SetText("another-Password-9")
	if pickers.respond(responseConfirm) || f.messages.Text() != tr.Get("settings-users-password-mismatch") {
		t.Fatalf("a mismatch closed the dialog or reads %q", f.messages.Text())
	}
	if !pickers.respond(responseCancel) {
		t.Fatal("cancel left the dialog open")
	}
	for _, e := range entries {
		if e.Text() != "" {
			t.Error("cancelling left a password in its entry")
		}
	}
}

func TestWithoutAWindowChangingThePasswordIsRefused(t *testing.T) {
	p := usersFooter(testKit(t, "")).(*usersPanel)
	t.Cleanup(p.close)
	p.k.pickers = nil
	p.openPasswordDialog()
	if !p.problem.Visible() || p.problem.Text() != locale.T().Get("settings-users-failed") || !strings.Contains(p.problem.detail.Text(), "no window") {
		t.Fatalf("without a window the page says %q over %q", p.problem.Text(), p.problem.detail.Text())
	}
}

// The identity header is a row of an item group on its card, as every
// row of a card is, not an inset row placed in the card on its own.
func TestTheIdentityHeaderIsInAnItemGroupOnItsCard(t *testing.T) {
	p := usersFooter(testKit(t, "")).(*usersPanel)
	found := false
	walkAncestry(p, nil, func(x widget.Widget, above []widget.Widget) {
		row, ok := x.(*insetRow)
		if !ok || !treetest.Contains(row, p.face) {
			return
		}
		found = true
		if !onCard(above) {
			t.Error("the identity header is in no item group on a card")
		}
	})
	if !found {
		t.Fatal("no inset row holds the identity header's picture")
	}
}

func TestTheAccountTileSaysWhoIsSignedInFromThePasswdDatabase(t *testing.T) {
	k := testKit(t, "")
	if _, err := accountStatus(context.Background(), k); err == nil {
		t.Fatal("a status without the accounts")
	}
	serveAccounts(k, alice)
	got, err := accountStatus(context.Background(), k)
	if want := locale.T().Get("settings-overview-signed-in", i18n.Str("name", "Alice")); err != nil || got != want {
		t.Fatalf("status %q, %v; want %q", got, err, want)
	}
	serveAccounts(k, fakeAccount{name: "nameless", uid: alice.uid})
	if got, _ := accountStatus(context.Background(), k); got != locale.T().Get("settings-overview-signed-in", i18n.Str("name", "nameless")) {
		t.Fatalf("an account without a full name reads %q", got)
	}
}
