package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func previewedPopup(t *testing.T, p *settingsPage) (*widget.Box, render.Rect) {
	t.Helper()
	return previewedSurface(t, p, "notification-popup-card", notificationPreviewScreenH)
}

func TestTheNotificationsPagePreviewsTheTestNotificationsPopup(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, notificationsPage(nil))
	card, at := previewedPopup(t, p)
	sample := testNotificationContent()
	if labels := treetest.Labels(card); !slices.Contains(labels, sample.Summary) || !slices.Contains(labels, sample.App) {
		t.Errorf("the popup previews %v, want the test notification", labels)
	}
	if at.Y > notificationPreviewScreenH/2 || at.X < previewScreenW/2 {
		t.Errorf("the default popup previews at %+v, want the top right", at)
	}
	shadowed := card.HasClass("shadow")

	set(t, k, p, "modules.notifications.popup-shadow", !shadowed)
	set(t, k, p, "modules.notifications.popup-position", "bottom-left")
	card, at = previewedPopup(t, p)
	if card.HasClass("shadow") == shadowed {
		t.Errorf("flipping popup-shadow previews the card as %v", card.Classes())
	}
	if at.Y+at.H < notificationPreviewScreenH/2 || at.X+at.W > previewScreenW/2 {
		t.Errorf("popup-position bottom-left previews the popup at %+v", at)
	}
}

func TestDisabledNotificationsPreviewThatNoPopupShows(t *testing.T) {
	k := testKit(t, "modules:\n  notifications:\n    enabled: false\n")
	p := buildPage(k, notificationsPage(nil))
	preview := onlyPreview(t, p)
	if cards := treetest.WithClass(preview, "notification-popup-card"); len(cards) != 0 {
		t.Error("disabled notifications still preview a popup")
	}
	if want := locale.T().Get("settings-notification-preview-off"); !slices.Contains(treetest.Labels(preview), want) {
		t.Errorf("the preview says %v, want %q", treetest.Labels(preview), want)
	}
	set(t, k, p, "modules.notifications.enabled", true)
	previewedPopup(t, p)
}

func TestThePreviewedNotificationIsTheOneSendTestNotificationSends(t *testing.T) {
	k := testKit(t, "")
	var sent []string
	k.notify = func(summary, body string) error {
		sent = append(sent, summary, body)
		return nil
	}
	if err := testNotification(k); err != nil {
		t.Fatal(err)
	}
	sample := testNotificationContent()
	if !slices.Equal(sent, []string{sample.Summary, sample.Body}) {
		t.Errorf("Send test notification sent %q, the preview shows %q / %q", sent, sample.Summary, sample.Body)
	}
	if !slices.Contains(listedIcons(t), sample.Icon.Name) {
		t.Errorf("the previewed notification shows %s, which resources/icons.txt does not list", sample.Icon.Name)
	}
}
