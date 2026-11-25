package settings

import (
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/jinja"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/notifyview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	mailSummaryPath = "modules.mail.notify-summary"
	mailBodyPath    = "modules.mail.notify-body"
)

// mailNotificationPreview is lys-shell's notification card as a new
// message's notification shows it, its wording filled with the
// placeholders' samples.
func mailNotificationPreview(k *kit) widget.Widget {
	return newLivePreview(k, "settings-mail-notification-preview", drawMailNotification)
}

func drawMailNotification(k *kit, cfg *config.Config, font render.Font) widget.Widget {
	if !cfg.Mail.Notify {
		return k.note(locale.T().Get("settings-mail-notification-preview-off"), "settings-preview-state")
	}
	box := widget.NewBox(widget.Column, 0, widget.Current().Padding)
	box.AppendAligned(notifyview.Preview(cfg, font, time.Time{}, mailNotificationContent(cfg.Mail)), false, widget.AlignCenter)
	return box
}

// mailNotificationContent is the notification lys-shell's mail service
// sends for a new message, the placeholders holding their samples: as
// the first account when there are accounts, the module otherwise.
func mailNotificationContent(m config.MailConfig) notifyview.Content {
	icon := m.IconName
	if len(m.Accounts) > 0 {
		icon = m.Accounts[0].ResolvedIcon()
	}
	return notifyview.Content{
		App:     i18n.Shell().Get("notification-source-mail"),
		Icon:    notifyview.Icon{Name: icon},
		Urgency: notifymodel.UrgencyNormal,
		Summary: jinja.ReplaceVars(m.NotifySummary, config.PlaceholderSamples(mailSummaryPath)),
		Body:    jinja.ReplaceVars(m.NotifyBody, config.PlaceholderSamples(mailBodyPath)),
	}
}
