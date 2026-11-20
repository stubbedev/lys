package mail

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/waittest"
)

// fakeNotmuch serves counts by query and records calls.
type fakeNotmuch struct {
	mu      sync.Mutex
	counts  map[string]uint32
	fails   map[string]error
	newest  map[string][]Message
	dbPath  string
	counted int
}

func (f *fakeNotmuch) fail(query string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fails == nil {
		f.fails = map[string]error{}
	}
	if err == nil {
		delete(f.fails, query)
		return
	}
	f.fails[query] = err
}

func (f *fakeNotmuch) set(query string, n uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[query] = n
}

func (f *fakeNotmuch) setNewest(query string, messages []Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newest[query] = messages
}

func (f *fakeNotmuch) Count(_ context.Context, query string) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counted++
	if err := f.fails[query]; err != nil {
		return 0, err
	}
	return f.counts[query], nil
}

func (f *fakeNotmuch) Newest(_ context.Context, query string, limit int) ([]Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	all := f.newest[query]
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (f *fakeNotmuch) DatabasePath(context.Context) (string, bool) {
	return f.dbPath, f.dbPath != ""
}

type sent struct{ summary, body, icon, click string }

type fakeNotifier struct {
	mu   sync.Mutex
	sent []sent
}

func (f *fakeNotifier) Notify(summary, body, icon, click string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{summary, body, icon, click})
}

func (f *fakeNotifier) all() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sent(nil), f.sent...)
}

func startService(t *testing.T, cfg config.MailConfig, nm *fakeNotmuch, notifier Notifier) (*Service, <-chan struct{}) {
	t.Helper()
	// Keep the host's installed icons out of the notification icons.
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	svc := New(cfg, nm, notifier)
	svc.debounce = 20 * time.Millisecond
	ticks, stop := svc.Subscribe()
	t.Cleanup(stop)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waittest.For(t, ticks, "the seed publish")
	return svc, ticks
}

const quietWindow = 100 * time.Millisecond

func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func notifyConfig() config.MailConfig {
	cfg := config.DefaultsMail()
	cfg.Notify = true
	return cfg
}

func TestSeedPublishesWithoutNotifying(t *testing.T) {
	nm := &fakeNotmuch{counts: map[string]uint32{"tag:unread": 4}, newest: map[string][]Message{
		"tag:unread": {{Sender: "Alice", Subject: "Hi"}},
	}}
	notifier := &fakeNotifier{}
	svc, _ := startService(t, notifyConfig(), nm, notifier)
	if got := svc.State(); got.Total != 4 || len(got.Accounts) != 0 {
		t.Errorf("state = %+v, want total 4 and no accounts", got)
	}
	if n := notifier.all(); len(n) != 0 {
		t.Errorf("startup notified %v; unread mail at startup is not new", n)
	}
}

func TestMaildirChangeRequeriesAndNotifiesArrivals(t *testing.T) {
	maildir := t.TempDir()
	cur := filepath.Join(maildir, "INBOX", "cur")
	if err := os.MkdirAll(cur, 0o700); err != nil {
		t.Fatal(err)
	}
	nm := &fakeNotmuch{dbPath: maildir, counts: map[string]uint32{"tag:unread": 1}, newest: map[string][]Message{}}
	notifier := &fakeNotifier{}
	cfg := notifyConfig()
	cfg.NotifySummary = "{{ new }} from {{ sender }}"
	cfg.NotifyBody = "{{ subject }} ({{ count }})"
	svc, ticks := startService(t, cfg, nm, notifier)

	nm.set("tag:unread", 3)
	nm.setNewest("tag:unread", []Message{{Sender: "Alice", Subject: "Lunch"}, {Sender: "Bob", Subject: "Build"}})
	touch(t, cur, "2:2,")
	waittest.For(t, ticks, "the publish after a maildir write")

	if got := svc.State().Total; got != 3 {
		t.Errorf("total = %d, want 3", got)
	}
	want := []sent{
		{"2 from Alice", "Lunch (3)", "ld-mail-symbolic", ""},
		{"2 from Bob", "Build (3)", "ld-mail-symbolic", ""},
	}
	if got := notifier.all(); !reflect.DeepEqual(got, want) {
		t.Errorf("notifications = %+v, want %+v", got, want)
	}

	// A drop (mail read) re-queries but notifies nothing.
	nm.set("tag:unread", 0)
	touch(t, cur, "3:2,S")
	waittest.For(t, ticks, "the publish after a read")
	if got := svc.State().Total; got != 0 {
		t.Errorf("total = %d, want 0", got)
	}
	if got := notifier.all(); len(got) != 2 {
		t.Errorf("a falling count notified: %+v", got)
	}
}

func TestNotifyOffStaysQuiet(t *testing.T) {
	maildir := t.TempDir()
	nm := &fakeNotmuch{dbPath: maildir, counts: map[string]uint32{"tag:unread": 0}, newest: map[string][]Message{}}
	notifier := &fakeNotifier{}
	svc, ticks := startService(t, config.DefaultsMail(), nm, notifier)
	nm.set("tag:unread", 9)
	touch(t, maildir, "new")
	waittest.For(t, ticks, "the publish after a write")
	if svc.State().Total != 9 {
		t.Errorf("total = %d, want 9", svc.State().Total)
	}
	if got := notifier.all(); len(got) != 0 {
		t.Errorf("notify=false still notified: %+v", got)
	}
}

func TestAccountsCountSeparatelyAndFallBackToACountOnlyNotification(t *testing.T) {
	maildir := t.TempDir()
	cfg := notifyConfig()
	house := "house"
	cfg.Accounts = []config.MailAccount{
		{Name: "Work", Query: "folder:work", Provider: config.MailProviderGmail},
		{Name: "Home", Query: "folder:home", Provider: config.MailProviderGeneric, Icon: &house},
	}
	nm := &fakeNotmuch{dbPath: maildir, counts: map[string]uint32{"folder:work": 2, "folder:home": 1}, newest: map[string][]Message{}}
	notifier := &fakeNotifier{}
	svc, ticks := startService(t, cfg, nm, notifier)

	state := svc.State()
	wantAccounts := []AccountUnread{{Name: "Work", Icon: "si-gmail-symbolic", Count: 2}, {Name: "Home", Icon: "house", Count: 1}}
	if state.Total != 3 || !reflect.DeepEqual(state.Accounts, wantAccounts) {
		t.Fatalf("state = %+v", state)
	}

	// Work gains 8 with no readable details; Home is unchanged.
	nm.set("folder:work", 10)
	touch(t, maildir, "new")
	waittest.For(t, ticks, "the publish after a write")
	want := []sent{{"Work", i18n.T("mail-notification-arrived", i18n.Int("new", 8), i18n.Int("count", 10)), "si-gmail-symbolic", ""}}
	if got := notifier.all(); !reflect.DeepEqual(got, want) {
		t.Errorf("notifications = %+v, want %+v", got, want)
	}
}

func TestBurstIsCappedAtNotifyMax(t *testing.T) {
	maildir := t.TempDir()
	nm := &fakeNotmuch{dbPath: maildir, counts: map[string]uint32{"tag:unread": 0}, newest: map[string][]Message{}}
	notifier := &fakeNotifier{}
	_, ticks := startService(t, notifyConfig(), nm, notifier)
	var many []Message
	for range 20 {
		many = append(many, Message{Sender: "s", Subject: "x"})
	}
	nm.setNewest("tag:unread", many)
	nm.set("tag:unread", 20)
	touch(t, maildir, "burst")
	waittest.For(t, ticks, "the publish after a burst")
	if got := len(notifier.all()); got != NotifyMax {
		t.Errorf("notifications = %d, want the %d cap", got, NotifyMax)
	}
}

func TestNoDatabaseMeansNoRequery(t *testing.T) {
	nm := &fakeNotmuch{counts: map[string]uint32{"tag:unread": 1}, newest: map[string][]Message{}}
	_, ticks := startService(t, config.DefaultsMail(), nm, nil)
	select {
	case <-ticks:
		t.Fatal("re-queried without a database to watch")
	case <-time.After(quietWindow):
	}
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if nm.counted != 1 {
		t.Errorf("count calls = %d, want the seed only", nm.counted)
	}
}

func TestParseSearchTakesTheFirstMatchedAuthor(t *testing.T) {
	out := []byte(`[
		{"authors": "Alice Smith, Bob| Carol", "subject": " Lunch "},
		{"authors": "", "subject": ""},
		{"authors": "Dave", "subject": "Build"}
	]`)
	want := []Message{{Sender: "Alice Smith", Subject: "Lunch"}, {Sender: "Dave", Subject: "Build"}}
	if got, err := parseSearch(out); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("parse = %+v, %v, want %+v", got, err, want)
	}
	if got, err := parseSearch([]byte("not json")); err == nil || got != nil {
		t.Errorf("bad json = %+v, %v, want an error", got, err)
	}
}

func fakeNotmuchBinary(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	fdtest.WriteExecutable(t, filepath.Join(dir, "notmuch"), []byte("#!/bin/sh\n"+script))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCLIParsesAndReportsFailures(t *testing.T) {
	fakeNotmuchBinary(t, `case "$1" in
count) case "$2" in
  tag:unread) echo 12 ;;
  garbled) echo many ;;
  *) echo "Error: bad query" >&2; exit 1 ;;
  esac ;;
config) echo "/home/me/Mail" ;;
search) [ "$5" = "broken" ] && { echo "Error: no database" >&2; exit 1; }; echo '[{"authors":"Eve","subject":"'"$4"'"}]' ;;
esac`)
	ctx := context.Background()
	if got, err := (CLI{}).Count(ctx, "tag:unread"); err != nil || got != 12 {
		t.Errorf("count = %d, %v, want 12", got, err)
	}
	if _, err := (CLI{}).Count(ctx, "bad query"); err == nil || !strings.Contains(err.Error(), "bad query") || !strings.Contains(err.Error(), "Error: bad query") {
		t.Errorf("failing count err = %v, want the query and notmuch's message", err)
	}
	if _, err := (CLI{}).Count(ctx, "garbled"); err == nil || !strings.Contains(err.Error(), `"many"`) {
		t.Errorf("unparsable count err = %v, want the output named", err)
	}
	if path, ok := (CLI{}).DatabasePath(ctx); !ok || path != "/home/me/Mail" {
		t.Errorf("database path = %q, %v", path, ok)
	}
	if got, err := (CLI{}).Newest(ctx, "tag:unread", 3); err != nil || len(got) != 1 || got[0].Subject != "--limit=3" {
		t.Errorf("newest = %+v, %v, want the limit passed through", got, err)
	}
	if _, err := (CLI{}).Newest(ctx, "broken", 3); err == nil || !strings.Contains(err.Error(), "no database") {
		t.Errorf("failing search err = %v, want notmuch's message", err)
	}
	if got, err := (CLI{}).Newest(ctx, "tag:unread", 0); err != nil || got != nil {
		t.Errorf("limit 0 = %+v, %v, want no search", got, err)
	}
}

func TestAFailedQueryIsAnErrorNotAnEmptyInbox(t *testing.T) {
	broken := errors.New("notmuch count tag:unread: exit status 1")
	nm := &fakeNotmuch{counts: map[string]uint32{"tag:unread": 4}, newest: map[string][]Message{}}
	nm.fail("tag:unread", broken)
	svc, _ := startService(t, config.DefaultsMail(), nm, nil)
	if st := svc.State(); !errors.Is(st.Err, broken) || st.Total != 0 {
		t.Errorf("state = %+v, want the count's error", st)
	}
	if svc.PerAccount() {
		t.Error("a query-only setup counts per account")
	}

	healthy := &fakeNotmuch{counts: map[string]uint32{"tag:unread": 0}, newest: map[string][]Message{}}
	empty, _ := startService(t, config.DefaultsMail(), healthy, nil)
	if st := empty.State(); st.Err != nil || st.Total != 0 {
		t.Errorf("an empty inbox = %+v, want zero and no error", st)
	}
}

func TestAFailedAccountIsMarkedAndTheOthersStillCount(t *testing.T) {
	cfg := config.DefaultsMail()
	cfg.Accounts = []config.MailAccount{
		{Name: "Work", Query: "folder:work", Provider: config.MailProviderGmail},
		{Name: "Home", Query: "folder:home"},
	}
	broken := errors.New("exit status 1")
	nm := &fakeNotmuch{counts: map[string]uint32{"folder:work": 2, "folder:home": 5}, newest: map[string][]Message{}}
	nm.fail("folder:work", broken)
	svc, _ := startService(t, cfg, nm, nil)
	st := svc.State()
	if !svc.PerAccount() || len(st.Accounts) != 2 {
		t.Fatalf("state = %+v, want both accounts", st)
	}
	work, home := st.Accounts[0], st.Accounts[1]
	if !errors.Is(work.Err, broken) || !strings.Contains(work.Err.Error(), "Work") || work.Count != 0 {
		t.Errorf("work = %+v, want its error naming it", work)
	}
	if home.Err != nil || home.Count != 5 || st.Total != 5 {
		t.Errorf("home = %+v total %d, want 5 read", home, st.Total)
	}
	if !errors.Is(st.Err, broken) {
		t.Errorf("state err = %v, want the failed account's", st.Err)
	}
}

func TestARecoveredQueryDoesNotNotifyItsBacklog(t *testing.T) {
	maildir := t.TempDir()
	nm := &fakeNotmuch{dbPath: maildir, counts: map[string]uint32{"tag:unread": 3}, newest: map[string][]Message{
		"tag:unread": {{Sender: "Alice", Subject: "Hi"}},
	}}
	notifier := &fakeNotifier{}
	svc, ticks := startService(t, notifyConfig(), nm, notifier)

	nm.fail("tag:unread", errors.New("locked"))
	touch(t, maildir, "a")
	waittest.For(t, ticks, "the publish of the failed read")
	if svc.State().Err == nil {
		t.Fatal("the failed read published no error")
	}
	nm.fail("tag:unread", nil)
	touch(t, maildir, "b")
	waittest.For(t, ticks, "the publish of the recovered read")
	if st := svc.State(); st.Err != nil || st.Total != 3 {
		t.Errorf("recovered state = %+v", st)
	}
	if got := notifier.all(); len(got) != 0 {
		t.Errorf("a recovery after a failure notified %+v as new mail", got)
	}

	nm.set("tag:unread", 4)
	touch(t, maildir, "c")
	waittest.For(t, ticks, "the publish of an arrival")
	if got := notifier.all(); len(got) != 1 {
		t.Errorf("an arrival after the recovery notified %d, want 1", len(got))
	}
}

func TestCLIWithoutADatabase(t *testing.T) {
	fakeNotmuchBinary(t, "exit 1\n")
	if _, ok := (CLI{}).DatabasePath(context.Background()); ok {
		t.Error("failing config get: want no database")
	}
}

func TestNotifyIconArgPrefersTheRegistrySVG(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	dir := filepath.Join(data, "lys-shell", "icons", "hicolor", "scalable", "actions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	touch(t, dir, "si-gmail-symbolic.svg")
	if got := notifyIconArg("si-gmail-symbolic"); got != filepath.Join(dir, "si-gmail-symbolic.svg") {
		t.Errorf("installed icon = %q, want its path", got)
	}
	if got := notifyIconArg("mail-unread"); got != "mail-unread" {
		t.Errorf("themed icon = %q, want the bare name", got)
	}
}

func TestNotificationsRenderExactlyTheirDeclaredPlaceholders(t *testing.T) {
	rendered := slices.Sorted(maps.Keys(notificationVars(newMail{})))
	for _, path := range []string{"modules.mail.notify-summary", "modules.mail.notify-body"} {
		var declared []string
		for _, p := range config.Placeholders(path) {
			declared = append(declared, p.Name)
		}
		slices.Sort(declared)
		if len(declared) == 0 || !slices.Equal(rendered, declared) {
			t.Errorf("%s: the notification renders %v, declares %v", path, rendered, declared)
		}
	}
	m := newMail{sender: "Ada", subject: "{{ count }}", count: 3, new: 1}
	if got := renderNotification("{{ sender }}: {{ subject }} ({{ new }}/{{ count }}) {{ pending }}", m); got != "Ada: {{ count }} (1/3) {{ pending }}" {
		t.Errorf("rendered %q: a value is substituted once and an undeclared placeholder stays literal", got)
	}
}

func TestACountOnlyNotificationIsTranslatedWithItsPluralCategories(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	unisolated := strings.NewReplacer("\u2068", "", "\u2069", "")
	cases := []struct {
		lang            string
		previous, total uint32
		summary, body   string
	}{
		{"en-US", 0, 1, "New mail", "1 new message (1 unread)"},
		{"en-US", 3, 5, "New mail", "2 new messages (5 unread)"},
		{"fr", 0, 1, "Nouveau courrier", "1 nouveau message (1 non lu)"},
		{"fr", 1, 3, "Nouveau courrier", "2 nouveaux messages (3 non lus)"},
		{"fr", 0, 1_000_000, "Nouveau courrier", "1000000 de nouveaux messages (1000000 non lus)"},
		{"pt", 4, 5, "Correio novo", "1 mensagem nova (5 por ler)"},
		{"pt", 0, 2, "Correio novo", "2 mensagens novas (2 por ler)"},
		{"pt", 1, 1_000_001, "Correio novo", "1000000 de mensagens novas (1000001 por ler)"},
		{"it", 0, 1, "Nuova posta", "1 nuovo messaggio (1 non letto)"},
		{"it", 0, 1_000_000, "Nuova posta", "1000000 di nuovi messaggi (1000000 non letti)"},
		{"es", 0, 1_000_000, "Correo nuevo", "1000000 de mensajes nuevos (1000000 sin leer)"},
		{"de", 0, 1, "Neue E-Mail", "1 neue Nachricht (1 ungelesen)"},
		{"nl", 0, 2, "Nieuwe e-mail", "2 nieuwe berichten (2 ongelezen)"},
	}
	for _, c := range cases {
		messages, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID(c.lang)})
		if err != nil {
			t.Fatal(err)
		}
		nm := &fakeNotmuch{counts: map[string]uint32{"tag:unread": c.previous}, newest: map[string][]Message{}}
		notifier := &fakeNotifier{}
		cfg := notifyConfig()
		cfg.NotifySummary = "{{ sender }}"
		cfg.NotifyBody = "{{ subject }}"
		svc := New(cfg, nm, notifier)
		svc.messages = messages
		svc.recompute(t.Context(), false)
		nm.set("tag:unread", c.total)
		svc.recompute(t.Context(), true)
		got := notifier.all()
		if len(got) != 1 {
			t.Errorf("%s %d->%d: notifications = %+v, want one count-only notification", c.lang, c.previous, c.total, got)
			continue
		}
		if summary, body := unisolated.Replace(got[0].summary), unisolated.Replace(got[0].body); summary != c.summary || body != c.body {
			t.Errorf("%s %d->%d: notified %q / %q, want %q / %q", c.lang, c.previous, c.total, summary, body, c.summary, c.body)
		}
	}
}

func TestANotificationWithReadableMessagesUsesThemNotTheCountText(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	nm := &fakeNotmuch{counts: map[string]uint32{"tag:unread": 0}, newest: map[string][]Message{
		"tag:unread": {{Sender: "Alice", Subject: "Lunch"}},
	}}
	notifier := &fakeNotifier{}
	cfg := notifyConfig()
	cfg.NotifySummary = "{{ sender }}"
	cfg.NotifyBody = "{{ subject }}"
	svc := New(cfg, nm, notifier)
	svc.recompute(t.Context(), false)
	nm.set("tag:unread", 1)
	svc.recompute(t.Context(), true)
	if got, want := notifier.all(), []sent{{"Alice", "Lunch", "ld-mail-symbolic", ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("notifications = %+v, want %+v", got, want)
	}
}

func TestParseSearchKeepsTheThread(t *testing.T) {
	got, err := parseSearch([]byte(`[{"thread": "0000000000003285", "authors": "Ann", "subject": "Hi"}]`))
	want := []Message{{Sender: "Ann", Subject: "Hi", Thread: "0000000000003285"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("parse = %+v, %v, want %+v", got, err, want)
	}
}

func TestANotificationClickRunsTheCommandForItsThread(t *testing.T) {
	maildir := t.TempDir()
	cur := filepath.Join(maildir, "INBOX", "cur")
	if err := os.MkdirAll(cur, 0o700); err != nil {
		t.Fatal(err)
	}
	nm := &fakeNotmuch{dbPath: maildir, counts: map[string]uint32{"tag:unread": 0}, newest: map[string][]Message{}}
	notifier := &fakeNotifier{}
	cfg := notifyConfig()
	cfg.NotifyClick = "mail-open thread:{{ thread }}"
	_, ticks := startService(t, cfg, nm, notifier)

	nm.set("tag:unread", 1)
	nm.setNewest("tag:unread", []Message{{Sender: "Alice", Subject: "Lunch", Thread: "00ab"}})
	touch(t, cur, "1:2,")
	waittest.For(t, ticks, "the publish after a maildir write")
	if got := notifier.all(); len(got) != 1 || got[0].click != "mail-open thread:00ab" {
		t.Fatalf("notifications = %+v, want one whose click opens thread 00ab", got)
	}
}

func TestNoClickCommandWithoutTheSettingOrAThread(t *testing.T) {
	if got := clickCommand("", newMail{thread: "00ab"}); got != "" {
		t.Errorf("no notify-click gave the click %q", got)
	}
	if got := clickCommand("mail-open thread:{{ thread }}", newMail{}); got != "" {
		t.Errorf("a count-only notification without a thread gave the click %q", got)
	}
}
