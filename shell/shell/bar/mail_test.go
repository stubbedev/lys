package bar

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/service/mail"
)

// staticNotmuch answers every query with one count, or fails the
// queries in fails, and has no database, so Run publishes the seed and
// then idles.
type staticNotmuch struct {
	count uint32
	fails map[string]error
}

func (n staticNotmuch) Count(_ context.Context, query string) (uint32, error) {
	if err := n.fails[query]; err != nil {
		return 0, err
	}
	return n.count, nil
}

func (staticNotmuch) Newest(context.Context, string, int) ([]mail.Message, error) { return nil, nil }

func (staticNotmuch) DatabasePath(context.Context) (string, bool) { return "", false }

func seededMail(t *testing.T, cfg config.MailConfig, count uint32) *mail.Service {
	t.Helper()
	return seededMailWith(t, cfg, staticNotmuch{count: count})
}

func seededMailWith(t *testing.T, cfg config.MailConfig, nm staticNotmuch) *mail.Service {
	t.Helper()
	svc := mail.New(cfg, nm, nil)
	ticks, stop := svc.Subscribe()
	defer stop()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go svc.Run(ctx)
	waittest.For(t, ticks, "the mail service's seed")
	return svc
}

func TestMailModuleShowsTheServiceTotal(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Mail = seededMail(t, cfg.Mail, 7)
	m, err := newMail(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mm := m.(*mailModule)
	defer mm.Stop()
	if got := mm.btn.Label().Text(); got != "7" {
		t.Errorf("label = %q, want the service's 7", got)
	}
	// A bar button with the mail icon.
	if _, ok := mm.Root().(*barButton); !ok || mm.icon == nil || mm.icon.Name() != cfg.Mail.IconName {
		t.Errorf("root %T icon %v, want a bar button with %q", mm.Root(), mm.icon, cfg.Mail.IconName)
	}
}

// hide-when-zero collapses the module root at zero and only then; the
// label keeps the count.
func TestMailHidesWhenZero(t *testing.T) {
	cfg := config.Defaults()
	cfg.Mail.HideWhenZero = true
	ctx := newTestContext(t, cfg)
	ctx.Mail = seededMail(t, cfg.Mail, 0)
	m, err := newMail(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mm := m.(*mailModule)
	defer mm.Stop()
	chrome := widget.NewBox(widget.Row, 0, 0)
	onHeadlessLoop(func() bool { mm.setChrome(chrome); return true })
	if onHeadlessLoop(func() bool { return chrome.Visible() }) {
		t.Error("zero unread under hide-when-zero is shown")
	}
	onHeadlessLoop(func() bool { mm.apply(mail.State{Total: 2}); return true })
	if !onHeadlessLoop(func() bool { return chrome.Visible() }) || mm.btn.Label().Text() != "2" {
		t.Errorf("two unread: visible %v label %q", chrome.Visible(), mm.btn.Label().Text())
	}
	cfg.Mail.HideWhenZero = false
	onHeadlessLoop(func() bool { mm.apply(mail.State{}); return true })
	if !onHeadlessLoop(func() bool { return chrome.Visible() }) || mm.btn.Label().Text() != "0" {
		t.Errorf("zero without hide-when-zero: visible %v label %q", chrome.Visible(), mm.btn.Label().Text())
	}
}

func TestMailModuleWithoutTheServiceErrors(t *testing.T) {
	if _, err := newMail(newTestContext(t, config.Defaults())); !errors.Is(err, errMailNoService) {
		t.Errorf("err = %v, want errMailNoService", err)
	}
}

func TestMailLabel(t *testing.T) {
	cfg := config.DefaultsMail()
	if got := mailLabel(cfg.Format, 3); got != "3" {
		t.Errorf("= %q, want 3", got)
	}
	if got := mailLabel(cfg.Format, 0); got != "0" {
		t.Errorf("zero = %q, want 0", got)
	}
	if got := mailLabel("Unread: {{ count }}", 12); got != "Unread: 12" {
		t.Errorf("format = %q", got)
	}
}

func TestLoadFileAppliesMailAccounts(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	good := "modules:\n  mail:\n    accounts:\n      - name: Work\n        query: \"folder:work and tag:unread\"\n        provider: gmail\n    format: \"{{ count }} new\"\n    hide-when-zero: false\n"
	if err := osWrite(path, good); err != nil {
		t.Fatal(err)
	}
	c, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(c.Mail.Accounts) != 1 {
		t.Fatalf("accounts = %+v", c.Mail.Accounts)
	}
	account := c.Mail.Accounts[0]
	if account.Name != "Work" || account.Query != "folder:work and tag:unread" || account.Provider != "gmail" {
		t.Errorf("account = %+v", account)
	}
	if c.Mail.Format != "{{ count }} new" || c.Mail.HideWhenZero {
		t.Errorf("mail = %+v", c.Mail)
	}

	for _, bad := range []string{
		"modules:\n  mail:\n    accounts:\n      - query: \"tag:unread\"\n",
		"modules:\n  mail:\n    accounts:\n      - name: Work\n",
	} {
		if err := osWrite(path, bad); err != nil {
			t.Fatal(err)
		}
		if _, err := config.LoadFile(path); err == nil {
			t.Errorf("%q: want a load error", bad)
		}
	}
}

func TestMailDropdownRows(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	if v := mailDropdown(ctx).(*mailView); len(mailRows(v)) != 0 || v.pages.Visible() == "error" {
		t.Fatalf("no service: page %q, %d rows; want nothing read and no error", v.pages.Visible(), len(mailRows(v)))
	}
	mc := cfg.Mail
	mc.Accounts = []config.MailAccount{{Name: "Work", Query: "tag:work"}, {Name: "Home", Query: "tag:home"}}
	ctx.Mail = seededMail(t, mc, 0)
	v := mailDropdown(ctx).(*mailView)
	rows := mailRows(v)
	if v.pages.Visible() != "accounts" || len(rows) != 2 {
		t.Fatalf("page %q, rows = %d, want one per account", v.pages.Visible(), len(rows))
	}
	count := treetest.First[*widget.Badge](t, rows[0])
	if count.Text() != "0" || count.Variant() != widget.BadgeSecondary {
		t.Errorf("zero count = %q variant %v, want the secondary badge", count.Text(), count.Variant())
	}
	ctx.Mail = seededMail(t, mc, 3)
	v = mailDropdown(ctx).(*mailView)
	if count := treetest.First[*widget.Badge](t, mailRows(v)[0]); count.Text() != "3" || count.Variant() == widget.BadgeSecondary {
		t.Errorf("unread count = %q variant %v, want the default badge", count.Text(), count.Variant())
	}
}

func TestMailDropdownShowsAQueryOnlyTotalAsOneRow(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	for _, n := range []uint32{0, 4} {
		ctx.Mail = seededMail(t, cfg.Mail, n)
		v := mailDropdown(ctx).(*mailView)
		rows := mailRows(v)
		if v.pages.Visible() != "accounts" || len(rows) != 1 {
			t.Fatalf("%d unread: page %q, %d rows, want the query's one row", n, v.pages.Visible(), len(rows))
		}
		if got := itemText(rows[0], "title"); got != i18n.T("dropdown-mail-unread") {
			t.Errorf("the query row reads %q", got)
		}
		if got := treetest.First[*widget.Badge](t, rows[0]).Text(); got != strconv.FormatUint(uint64(n), 10) {
			t.Errorf("the query row counts %q, want %d", got, n)
		}
	}
}

func TestMailDropdownShowsTheErrorPage(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	broken := errors.New("notmuch count tag:unread: exit status 1: Error: no database")
	ctx.Mail = seededMailWith(t, cfg.Mail, staticNotmuch{fails: map[string]error{cfg.Mail.Query: broken}})
	v := mailDropdown(ctx).(*mailView)
	if got := v.pages.Visible(); got != "error" {
		t.Fatalf("a failed query shows page %q, want error", got)
	}
	if !strings.Contains(v.errorText.Text(), broken.Error()) {
		t.Errorf("the error page says %q, want the failure", v.errorText.Text())
	}
	if got := itemText(v.pages, "title"); got != i18n.T("dropdown-mail-error-title") {
		t.Errorf("the error page title reads %q", got)
	}
}

func TestMailDropdownMarksOnlyTheFailedAccount(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	mc := cfg.Mail
	mc.Accounts = []config.MailAccount{{Name: "Work", Query: "tag:work"}, {Name: "Home", Query: "tag:home"}}
	broken := errors.New("exit status 1")
	ctx.Mail = seededMailWith(t, mc, staticNotmuch{count: 2, fails: map[string]error{"tag:work": broken}})
	v := mailDropdown(ctx).(*mailView)
	rows := mailRows(v)
	if v.pages.Visible() != "accounts" || len(rows) != 2 {
		t.Fatalf("one failed account: page %q, %d rows, want both accounts", v.pages.Visible(), len(rows))
	}
	if !rows[0].HasClass("failed") || itemText(rows[0], "description") != i18n.T("dropdown-mail-account-error") || len(treetest.All[*widget.Badge](rows[0])) != 0 {
		t.Error("the failed account is not marked, or still shows a count")
	}
	if rows[1].HasClass("failed") || treetest.First[*widget.Badge](t, rows[1]).Text() != "2" {
		t.Error("the healthy account is marked or lost its count")
	}

	ctx.Mail = seededMailWith(t, mc, staticNotmuch{fails: map[string]error{"tag:work": broken, "tag:home": broken}})
	if got := mailDropdown(ctx).(*mailView).pages.Visible(); got != "error" {
		t.Errorf("every account failed: page %q, want error", got)
	}
}

func TestFailedMailStaysShownUnderHideWhenZero(t *testing.T) {
	cfg := config.Defaults()
	cfg.Mail.HideWhenZero = true
	ctx := newTestContext(t, cfg)
	broken := errors.New("exit status 1")
	ctx.Mail = seededMailWith(t, cfg.Mail, staticNotmuch{fails: map[string]error{cfg.Mail.Query: broken}})
	m, err := newMail(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mm := m.(*mailModule)
	defer mm.Stop()
	chrome := widget.NewBox(widget.Row, 0, 0)
	onHeadlessLoop(func() bool { mm.setChrome(chrome); return true })
	if !onHeadlessLoop(func() bool { return chrome.Visible() && widget.HasClass(chrome, mailFailedClass) }) {
		t.Error("a failed count is hidden by hide-when-zero or unmarked")
	}
	if got := mm.btn.Label().Text(); got != mailUnknownCount {
		t.Errorf("a failed count reads %q, want %q", got, mailUnknownCount)
	}
	if mm.btn.detail != i18n.T("dropdown-mail-error-title") {
		t.Errorf("detail = %q, want the failure", mm.btn.detail)
	}

	onHeadlessLoop(func() bool { mm.apply(mail.State{}); return true })
	if onHeadlessLoop(func() bool {
		return chrome.Visible() || widget.HasClass(chrome, mailFailedClass) || mm.btn.detail != ""
	}) {
		t.Error("a read zero after the failure stays shown or marked")
	}
}

// mailRows is the dropdown's account rows in order.
func mailRows(v *mailView) []*widget.Item {
	return treetest.All[*widget.Item](v.list)
}
