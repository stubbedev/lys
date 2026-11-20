// Package notifyui is what the notification popups and the bar's
// notification dropdown share over the service's notifications: icon
// resolution, the actions a card draws, grouping, activation tokens and
// opening links.
package notifyui

import (
	"errors"
	"log"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/notifyview"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/shell/internal/appicons"
	"github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/notifications"
)

// ResolveIcon picks a notification's icon: Mapped is the curated icon for the app
// name; Automatic prefers the image; Application prefers the image,
// then the app icon, then the desktop entry; each falls back to Mapped,
// and Mapped to the bell.
func ResolveIcon(source config.IconSource, n *notifications.Notification) notifyview.Icon {
	switch source {
	case config.IconSourceAutomatic:
		if icon, ok := iconString(n.ImagePath); ok {
			return icon
		}
	case config.IconSourceApplication:
		if icon, ok := iconString(n.ImagePath); ok {
			return icon
		}
		if icon, ok := iconString(n.AppIcon); ok {
			return icon
		}
		if n.DesktopEntry != "" {
			return notifyview.Icon{Name: n.DesktopEntry}
		}
	}
	return mappedIcon(n.AppName)
}

// iconString reads an icon hint: an absolute path (file:// stripped) is
// a file, unless it is a -symbolic.svg, which resolves to its theme
// name so it recolors; anything else is a theme name.
func iconString(value string) (notifyview.Icon, bool) {
	if value == "" {
		return notifyview.Icon{}, false
	}
	path := strings.TrimPrefix(value, "file://")
	if !strings.HasPrefix(path, "/") {
		return notifyview.Icon{Name: value}, true
	}
	if stem, ok := strings.CutSuffix(filepath.Base(path), ".svg"); ok && strings.HasSuffix(stem, "-symbolic") {
		return notifyview.Icon{Name: stem}, true
	}
	return notifyview.Icon{File: path}, true
}

func mappedIcon(appName string) notifyview.Icon {
	if appName != "" {
		if name, ok := appicons.Lookup(appName); ok {
			return notifyview.Icon{Name: name}
		}
	}
	return notifyview.Icon{Name: notifyview.FallbackIcon}
}

// VisibleActions are the actions a card draws as buttons: all but the
// default one, which the card itself invokes on a click, and the inline
// reply, which it draws as a reply field.
func VisibleActions(n *notifications.Notification) []notifications.Action {
	var out []notifications.Action
	for _, a := range n.Actions {
		if a.ID != notifications.DefaultActionID && a.ID != notifications.InlineReplyActionID {
			out = append(out, a)
		}
	}
	return out
}

// Group is notifications sharing a key, newest first.
type Group struct {
	Key    string
	Notifs []*notifications.Notification
}

// AppKey groups notifications by the name they show under.
func AppKey(n *notifications.Notification) string { return notifyview.AppLabel(n.AppName) }

// GroupBy groups by any key: one group per key, each newest
// first, the groups ordered by their newest notification; ties keep the
// order of notifs.
func GroupBy(notifs []*notifications.Notification, key func(*notifications.Notification) string) []Group {
	index := map[string]int{}
	var groups []Group
	for _, n := range notifs {
		k := key(n)
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, Group{Key: k})
		}
		groups[i].Notifs = append(groups[i].Notifs, n)
	}
	newestFirst := func(a, b *notifications.Notification) int { return b.Added.Compare(a.Added) }
	for i := range groups {
		slices.SortStableFunc(groups[i].Notifs, newestFirst)
	}
	slices.SortStableFunc(groups, func(a, b Group) int {
		return newestFirst(a.Notifs[0], b.Notifs[0])
	})
	return groups
}

// TokenMinter mints xdg-activation tokens from the latest input:
// *app.Application.
type TokenMinter interface {
	RequestActivationToken(done func(token string, err error))
}

// Minter is the application as a TokenMinter; nil (no minting) for a
// nil application.
func Minter(a *app.Application) TokenMinter {
	if a == nil {
		return nil
	}
	return a
}

// WithActivationToken mints a token for an action press and hands it
// to then, so the app the action reaches can raise its window: call it
// from the press's handler, whose input the token vouches for. then
// gets "" without a minter or xdg-activation on the compositor, and
// after a failed request, which logs.
func WithActivationToken(m TokenMinter, then func(token string)) {
	if m == nil {
		then("")
		return
	}
	m.RequestActivationToken(func(token string, err error) {
		if err != nil && !errors.Is(err, app.ErrActivationUnavailable) {
			log.Printf("notifications: activation token for an action: %v", err)
		}
		then(token)
	})
}

// OpenableLink reports whether a notification body's link may open: web
// and mail links only, never a file or a custom scheme an app slipped in.
func OpenableLink(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	}
	return false
}

// OpenLink opens uri with the user's default app for its scheme,
// handing it token to raise its window ("" hands none).
func OpenLink(uri, token string) error { return launcher.Open(uri, "", token) }
