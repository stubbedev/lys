package settings

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/identity"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const shellCommandTimeout = 5 * time.Second

func runShellCommand(args ...string) error { return runCommand(identity.Name, args...) }

const loginProgram = "lys-login"

func checkPAMService(service string) error { return runCommand(loginProgram, "check-pam", service) }

func runCommand(program string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), shellCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, program, args...).CombinedOutput() //nolint:gosec // a lys program with argv the pages build
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", program, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

const (
	notificationsName      = "org.freedesktop.Notifications"
	notificationsPath      = "/org/freedesktop/Notifications"
	notificationsNotify    = notificationsName + ".Notify"
	testNotificationIcon   = "ld-bell-symbolic"
	serverDefaultExpiresIn = int32(-1)
)

func sendNotification(summary, body string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect to the session bus: %w", err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), shellCommandTimeout)
	defer cancel()
	call := conn.Object(notificationsName, notificationsPath).CallWithContext(ctx, notificationsNotify, 0,
		notificationSender(), uint32(0), testNotificationIcon, summary, body, []string{}, map[string]dbus.Variant{}, serverDefaultExpiresIn)
	if call.Err != nil {
		return fmt.Errorf("send a notification: %w", call.Err)
	}
	return nil
}

type shellToast struct {
	label, icon, preset string
	percentage          int
}

func (t shellToast) args() []string {
	if t.preset != "" {
		return []string{"toast", "--preset", t.preset}
	}
	return []string{"toast", t.label, "--icon", t.icon, "--percentage", strconv.Itoa(t.percentage)}
}

func (k *kit) showToast(t shellToast) error { return k.shellCommand(t.args()...) }

type tryButton struct {
	*widget.Box
	button  *widget.Button
	outcome *widget.Label
}

func newTryButton(k *kit, labelKey string, try func() error) *tryButton {
	t := locale.T()
	b := &tryButton{Box: gapBox(widget.Row), outcome: wrapped(k.label("", "setting-problem", css.Caption, css.Error))}
	b.outcome.SetVisible(false)
	run := func() {
		async(k, try, func(err error) {
			b.outcome.SetVisible(err != nil)
			if err != nil {
				b.outcome.SetText(t.Get("settings-try-failed", i18n.Str("reason", err.Error())))
			}
		})
	}
	b.button, _ = k.contentButton("ld-play-symbolic", t.Get(labelKey), run, css.Pill)
	b.AppendAligned(b.button, false, widget.AlignCenter)
	b.Append(b.outcome, true)
	return b
}

func tryLead(labelKey string, try func(k *kit) error) func(k *kit) widget.Widget {
	return func(k *kit) widget.Widget {
		return newSection(k, "settings-section-try", insetRow(newTryButton(k, labelKey, func() error { return try(k) })))
	}
}

type presetTry struct {
	*widget.Box
	id      slot
	command *widget.Label
}

func newPresetTry(k *kit, id slot) *presetTry {
	p := &presetTry{Box: gapBox(widget.Column), id: id, command: k.label("", css.Caption, css.DimLabel, css.Monospace)}
	p.command.SetWrap(true)
	p.Append(newTryButton(k, "settings-show-toast", func() error { return k.showToast(shellToast{preset: p.id.text()}) }), false)
	p.Append(p.command, false)
	p.refresh()
	return p
}

func (p *presetTry) refresh() {
	p.command.SetText(strings.Join(append([]string{identity.Name}, shellToast{preset: p.id.text()}.args()...), " "))
}

// notificationSender is the app name lys-settings' notifications carry.
func notificationSender() string { return locale.T().Get("settings-title") }

func testNotification(k *kit) error {
	c := testNotificationContent()
	return k.notify(c.Summary, c.Body)
}

func showOsd(k *kit) error { return k.showToast(osdSampleToast()) }
