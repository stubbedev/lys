package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func mailPreview(t *testing.T, p *settingsPage) *livePreview {
	t.Helper()
	name := locale.T().Get("settings-mail-notification-preview")
	for _, lp := range treetest.All[*livePreview](p) {
		if lp.AccessibleName() == name {
			return lp
		}
	}
	t.Fatal("the mail page previews no new mail notification")
	return nil
}

func previewTexts(w widget.Widget) []string {
	out := treetest.Labels(w)
	for _, r := range treetest.All[*widget.RichLabel](w) {
		out = append(out, r.Text())
	}
	return out
}

func TestTheMailNotificationIsPreviewedWithSampleWording(t *testing.T) {
	k := testKit(t, "modules:\n  mail:\n    notify: true\n    notify-summary: \"{{ new }} from {{ sender }}\"\n")
	p := buildPage(k, mailPage(nil))
	labels := previewTexts(mailPreview(t, p))
	for _, want := range []string{"1 from Ada Lovelace", "Lunch on Friday?", i18n.Shell().Get("notification-source-mail")} {
		if !slices.Contains(labels, want) {
			t.Errorf("the preview shows %q, want %q", labels, want)
		}
	}
	set(t, k, p, "modules.mail.notify-body", "{{ count }} unread")
	if labels := previewTexts(mailPreview(t, p)); !slices.Contains(labels, "3 unread") {
		t.Errorf("after a new body the preview shows %q", labels)
	}
}

func TestMailNotificationsOffPreviewNoCard(t *testing.T) {
	k := testKit(t, "modules:\n  mail:\n    notify: false\n")
	p := buildPage(k, mailPage(nil))
	preview := mailPreview(t, p)
	if cards := treetest.WithClass(preview, "notification-popup-card"); len(cards) != 0 {
		t.Error("with notifications off the preview still draws a card")
	}
	if want := locale.T().Get("settings-mail-notification-preview-off"); !slices.Contains(treetest.Labels(preview), want) {
		t.Errorf("the preview says %q, want %q", treetest.Labels(preview), want)
	}
}

func TestTheMailNotificationWearsTheFirstAccountsIcon(t *testing.T) {
	m := config.DefaultsMail()
	if got := mailNotificationContent(m).Icon.Name; got != m.IconName {
		t.Errorf("without accounts the notification wears %q, want the module's %q", got, m.IconName)
	}
	icon := "ld-briefcase-symbolic"
	m.Accounts = []config.MailAccount{{Name: "Work", Query: "tag:work", Icon: &icon}, {Name: "Home", Query: "tag:home"}}
	if got := mailNotificationContent(m).Icon.Name; got != "ld-briefcase-symbolic" {
		t.Errorf("with accounts the notification wears %q, want the first account's", got)
	}
}
