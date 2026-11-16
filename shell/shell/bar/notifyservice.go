package bar

import (
	"log"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/shell/service/notifications"
)

// startNotifications builds the notification service only when [modules.notification] is enabled, so
// another daemon can own the bus name otherwise (and the module, its
// dropdown, and the popups are absent), seeded with the blocklist.
func startNotifications(cfg config.NotificationConfig, clk notifications.Clock) *notifications.Service {
	if !cfg.Enabled {
		return nil
	}
	svc := notifications.NewService(clk)
	applyNotificationConfig(svc, cfg)
	attachNotificationStore(svc, notifications.StorePath)
	return svc
}

// applyNotificationConfig hands a running service the reloadable keys
// (the blocklist and the history limits); a disabled service stays off
// until the next start.
func applyNotificationConfig(svc *notifications.Service, cfg config.NotificationConfig) {
	if svc == nil {
		return
	}
	svc.SetBlocklist(cfg.Blocklist)
	svc.SetFocusRules(cfg.Dnd.Rules())
	svc.SetHistoryLimits(cfg.HistoryMemory.Limit(), cfg.HistoryStored.Limit())
}

// attachNotificationStore persists the history in the store's file: a
// store that cannot open leaves the history in memory, without
// persistence.
func attachNotificationStore(svc *notifications.Service, path func() (string, error)) {
	p, err := path()
	if err == nil {
		var st *notifications.Store
		if st, err = notifications.OpenStore(p); err == nil {
			err = svc.AttachStore(st)
		}
	}
	if err != nil {
		log.Printf("notifications: %v; the history will not persist across restarts", err)
	}
}
