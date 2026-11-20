package settings

import (
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/notifyview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const notificationPreviewScreenH = 240

// notificationPreview is lys-shell's notification popup on a small
// wallpaper, where popup-position puts it, showing the notification Send
// test notification sends.
func notificationPreview(k *kit) widget.Widget {
	return newLivePreview(k, "settings-notification-preview", drawNotificationOnScreen)
}

func drawNotificationOnScreen(k *kit, cfg *config.Config, font render.Font) widget.Widget {
	shown := wallpaperScreen(notificationPreviewScreenH)
	if !cfg.Notification.Enabled {
		shown.AppendAligned(k.label(locale.T().Get("settings-notification-preview-off"), "settings-preview-state", css.Caption), widget.AlignCenter, widget.AlignCenter)
		return shown
	}
	inset := widget.NewBox(widget.Column, 0, widget.Current().Padding)
	inset.Append(notifyview.Preview(cfg, font, time.Time{}, testNotificationContent()), false)
	h, v := widgets.Placement(cfg.Notification.PopupPosition)
	shown.AppendAligned(inset, h, v)
	return overflowing(shown)
}

// testNotificationContent is the notification Send test notification
// sends, as sent at the zero time.
func testNotificationContent() notifyview.Content {
	t := locale.T()
	return notifyview.Content{
		App:     notificationSender(),
		Icon:    notifyview.Icon{Name: testNotificationIcon},
		Urgency: notifymodel.UrgencyNormal,
		Summary: t.Get("settings-test-notification-summary"),
		Body:    t.Get("settings-test-notification-body"),
	}
}
