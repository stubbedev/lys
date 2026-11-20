// Package notifyui is what the notification popups and the bar's
// notification dropdown share: icon resolution, urgency classes, relative times, and body text.
package notifyui

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/internal/appicons"
	"github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/notifications"
)

// FallbackIcon is the bell every unresolved notification shows.
const FallbackIcon = "ld-bell-symbolic"

// Icon is a resolved notification icon: a theme name or an image file
// (ResolvedIcon). Exactly one is set.
type Icon struct {
	Name string
	File string
}

// ResolveIcon picks a notification's icon: Mapped is the curated icon for the app
// name; Automatic prefers the image; Application prefers the image,
// then the app icon, then the desktop entry; each falls back to Mapped,
// and Mapped to the bell.
func ResolveIcon(source config.IconSource, n *notifications.Notification) Icon {
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
			return Icon{Name: n.DesktopEntry}
		}
	}
	return mappedIcon(n.AppName)
}

// iconString reads an icon hint: an absolute path (file:// stripped) is
// a file, unless it is a -symbolic.svg, which resolves to its theme
// name so it recolors; anything else is a theme name.
func iconString(value string) (Icon, bool) {
	if value == "" {
		return Icon{}, false
	}
	path := strings.TrimPrefix(value, "file://")
	if !strings.HasPrefix(path, "/") {
		return Icon{Name: value}, true
	}
	if stem, ok := strings.CutSuffix(filepath.Base(path), ".svg"); ok && strings.HasSuffix(stem, "-symbolic") {
		return Icon{Name: stem}, true
	}
	return Icon{File: path}, true
}

func mappedIcon(appName string) Icon {
	if appName != "" {
		if name, ok := appicons.Lookup(appName); ok {
			return Icon{Name: name}
		}
	}
	return Icon{Name: FallbackIcon}
}

// UrgencyClass is the CSS class of an urgency: low, normal, or critical.
func UrgencyClass(u notifymodel.Urgency) string {
	switch u {
	case notifymodel.UrgencyLow:
		return "low"
	case notifymodel.UrgencyCritical:
		return "critical"
	}
	return "normal"
}

// UrgencyBarVisible reports whether an urgency reaches the bar's threshold.
func UrgencyBarVisible(u notifymodel.Urgency, threshold config.UrgencyBarThreshold) bool {
	switch threshold {
	case config.UrgencyBarThresholdLow:
		return true
	case config.UrgencyBarThresholdNormal:
		return u >= notifymodel.UrgencyNormal
	case config.UrgencyBarThresholdCritical:
		return u >= notifymodel.UrgencyCritical
	}
	return false
}

// AgeUnit is the unit an Age counts in.
type AgeUnit uint8

// The age units, finest first.
const (
	AgeJustNow AgeUnit = iota
	AgeMinutes
	AgeHours
	AgeDays
)

// Age is RelativeTime: under a minute is just now, under an hour is
// whole minutes, under a day whole hours, else whole days.
type Age struct {
	Unit  AgeUnit
	Count int64
}

// RelativeTime is the age of ts at now.
func RelativeTime(now, ts time.Time) Age {
	d := now.Sub(ts)
	switch {
	case d < time.Minute:
		return Age{Unit: AgeJustNow}
	case d < time.Hour:
		return Age{Unit: AgeMinutes, Count: int64(d / time.Minute)}
	case d < 24*time.Hour:
		return Age{Unit: AgeHours, Count: int64(d / time.Hour)}
	}
	return Age{Unit: AgeDays, Count: int64(d / (24 * time.Hour))}
}

// BodyMarkup is the sanitized body: markup
// lys-ui's parser accepts passes through with its tags intact, anything
// else escapes to the literal characters.
func BodyMarkup(body string) string {
	if _, ok := widget.ParseMarkup(body); ok {
		return body
	}
	repl := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return repl.Replace(body)
}

// NewIcon builds the widget for a resolved icon at size pixels: a file
// image, decoded off the loop with the generic glyph until post delivers it, or
// a theme icon tinted when its name is symbolic. ready runs once a file shows.
func NewIcon(icon Icon, size int, tint render.Color, post func(func()), ready func()) *widget.Icon {
	if icon.File != "" {
		return widgets.NewAsyncFileIcon(icon.File, FallbackIcon, size, post, ready)
	}
	w := widget.NewThemeIconFallback(icon.Name, size)
	if strings.HasSuffix(icon.Name, "-symbolic") {
		w.SetTint(tint)
	}
	return w
}

// IsFileIcon reports whether an icon draws as a picture rather than a
// recolored glyph (an image, or a theme name that is not -symbolic):
// the card's icon box takes the file-icon class for it.
func IsFileIcon(icon Icon) bool {
	return icon.File != "" || !strings.HasSuffix(icon.Name, "-symbolic")
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

// notifySendName is what notify-send calls itself without -a.
const notifySendName = "notify-send"

// AppLabel is the name a notification goes by on screen. notify-send and
// the shell's own name say who delivered it, not what it is about, so
// those show as the system. Everything else is the app's own name.
func AppLabel(appName string) string {
	if appName == notifySendName || appName == identity.Name {
		return i18n.T("notification-source-system")
	}
	return appName
}

// AppKey groups notifications by the name they show under.
func AppKey(n *notifications.Notification) string { return AppLabel(n.AppName) }

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

// ActionsPerRow is how many action buttons share a row.
const ActionsPerRow = 3

// TimeLabel renders an age in a surface's own
// strings: domain is "notification-popup" or "notification-dropdown",
// whose -time-just-now, -time-minutes-ago, -time-hours-ago and
// -time-days-ago messages it reads.
func TimeLabel(age Age, domain string) string {
	count := strconv.FormatInt(age.Count, 10)
	switch age.Unit {
	case AgeJustNow:
		return i18n.T(domain + "-time-just-now")
	case AgeMinutes:
		return i18n.T(domain+"-time-minutes-ago", i18n.Str("minutes", count))
	case AgeHours:
		return i18n.T(domain+"-time-hours-ago", i18n.Str("hours", count))
	case AgeDays:
		return i18n.T(domain+"-time-days-ago", i18n.Str("days", count))
	}
	panic(fmt.Sprintf("notifyui: age unit %d has no label", age.Unit))
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

// PurposeClass is the style class a button purpose gives an action
// button: accepting a call is the suggested action, declining or hanging
// up the destructive one; "" for any other.
func PurposeClass(purpose string) string {
	switch purpose {
	case "call.accept":
		return css.SuggestedAction
	case "call.decline", "call.hang-up":
		return css.DestructiveAction
	}
	return ""
}
