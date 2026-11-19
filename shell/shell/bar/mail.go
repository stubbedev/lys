package bar

import (
	"context"
	"errors"
	"strconv"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/internal/jinja"
	"github.com/stubbedev/lys/shell/service/mail"
)

// startMail builds and runs the shared mail service, whether or not a
// mail module is placed. Without a
// session bus it still counts; it only cannot notify. The returned function
// stops it.
func startMail(ctx *ModuleContext, conn *dbus.Conn) func() {
	var notifier mail.Notifier
	if conn != nil {
		notifier = mail.DesktopNotifier{Sender: desktopnotify.NewSender(conn)}
	}
	svc := mail.New(ctx.Config.Mail, mail.CLI{}, notifier)
	ctx.Mail = svc
	runCtx, cancel := context.WithCancel(context.Background())
	go svc.Run(runCtx)
	return cancel
}

// mailLabel: {{ count }} replaced, not a template.
func mailLabel(format string, count uint32) string {
	return jinja.ReplaceVars(format, mailVars(count))
}

func mailVars(count uint32) map[string]string {
	return mailCountVars(strconv.FormatUint(uint64(count), 10))
}

func mailCountVars(count string) map[string]string {
	return map[string]string{"count": count}
}

const (
	mailUnknownCount = "--"
	mailFailedClass  = "mail-failed"
)

func mailStateLabel(format string, st mail.State) string {
	if st.Err != nil {
		return jinja.ReplaceVars(format, mailCountVars(mailUnknownCount))
	}
	return mailLabel(format, st.Total)
}

// mailModule is the module: the shared service's unread total on a bar
// button, the module root hidden at zero under hide-when-zero. A count
// that could not be read shows as unknown, marks the module mail-failed
// and keeps it shown.
type mailModule struct {
	ctx    ModuleContext
	btn    *barButton
	icon   *widget.Icon
	chrome moduleChrome
	state  mail.State
	stop   func()
}

var errMailNoService = errors.New("mail: requires the shared mail service")

func newMail(ctx ModuleContext) (Module, error) {
	if ctx.Mail == nil {
		return nil, errMailNoService
	}
	m := &mailModule{ctx: ctx}
	m.icon = barview.Icon(ctx.Config.Mail.Icon())
	m.btn = newBarButton(ctx, m.icon, "")
	m.apply(ctx.Mail.State())
	ticks, unsubscribe := ctx.Mail.Subscribe()
	done := make(chan struct{})
	m.stop = func() {
		unsubscribe()
		close(done)
	}
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticks:
			}
			st := ctx.Mail.State()
			m.ctx.Invoke(func() { m.apply(st) })
		}
	}()
	return m, nil
}

func (m *mailModule) apply(st mail.State) {
	m.state = st
	m.btn.SetLabel(mailStateLabel(m.ctx.Config.Mail.Format, st))
	detail := ""
	if st.Err != nil {
		detail = i18n.T("dropdown-mail-error-title")
	}
	m.btn.SetDescription(detail)
	m.showChrome()
}

// showChrome hides the module root at zero under hide-when-zero, never
// while a count failed.
func (m *mailModule) showChrome() {
	if m.chrome == nil {
		return
	}
	failed := m.state.Err != nil
	setClass(m.chrome, mailFailedClass, failed)
	m.chrome.SetVisible(failed || m.state.Total > 0 || !m.ctx.Config.Mail.HideWhenZero)
}

func (m *mailModule) setChrome(c moduleChrome) {
	m.chrome = c
	m.showChrome()
}

func (m *mailModule) Root() widget.Widget { return m.btn }

// Stop unsubscribes from the service.
func (m *mailModule) Stop() {
	if m.stop != nil {
		m.stop()
		m.stop = nil
	}
}
