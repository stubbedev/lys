package bar

import (
	"context"
	"errors"
	"strconv"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/mail"
)

// mailView is the mail dropdown: an item card of the accounts (each
// one's icon, name, and unread count as a badge, secondary at zero), or
// the single query's total as one row, and the error page when no count
// could be read, rebuilt as the service changes while open.
type mailView struct {
	ctx  ModuleContext
	font render.Font
	px   float64

	*widget.Box
	pages     *widget.Stack
	list      *widget.ItemGroup
	errorText *widget.Label

	*openFollow
}

func mailDropdown(ctx ModuleContext) widget.Widget {
	font, px := dropdownFont(ctx)
	v := &mailView{ctx: ctx, font: font, px: px}
	v.openFollow = newOpenFollow(ctx, v.follow)
	// The popover frame: the header strip full width over the content.
	v.Box = widget.NewBox(widget.Column, 0, 0)
	v.AddClass("dropdown", "mail-dropdown")
	v.Append(dropdownHeader(font, "ld-mail-symbolic", i18n.T("dropdown-mail-title")), false)
	card, list := itemCard()
	card.AddClass("mail-dropdown-list")
	v.list = list
	v.pages = pageStack()
	v.pages.Add("accounts", growingScroll(scrolledCard(card, "")))
	v.pages.Add("error", v.errorPage())
	content := dropdownContent()
	content.Append(v.pages, true)
	v.Append(content, true)
	return v
}

func (v *mailView) errorPage() widget.Widget {
	page := widgets.EmptyState(v.font, v.px, "ld-mail-symbolic", i18n.T("dropdown-mail-error-title"), "")
	page.AddClass("mail-dropdown-error")
	v.errorText = widget.NewLabel(v.font, v.px, "", 0)
	v.errorText.AddClass("error-text", css.DimLabel)
	v.errorText.SetAlignment(render.AlignCenter)
	v.errorText.SetWrap(true)
	page.SetChild(v.errorText)
	return page
}

func mailDropdownRows(perAccount bool, st mail.State, cfg config.MailConfig) []mail.AccountUnread {
	if perAccount {
		return st.Accounts
	}
	return []mail.AccountUnread{{Name: i18n.T("dropdown-mail-unread"), Icon: cfg.IconName, Count: st.Total, Err: st.Err}}
}

func mailFailure(rows []mail.AccountUnread) error {
	errs := make([]error, 0, len(rows))
	for _, a := range rows {
		if a.Err == nil {
			return nil
		}
		errs = append(errs, a.Err)
	}
	return errors.Join(errs...)
}

func (v *mailView) rebuild() {
	if v.ctx.Mail == nil {
		return
	}
	v.list.Clear()
	rows := mailDropdownRows(v.ctx.Mail.PerAccount(), v.ctx.Mail.State(), v.ctx.Config.Mail)
	if err := mailFailure(rows); err != nil {
		showFailureDetail(v.errorText, err)
		v.pages.Show("error")
		return
	}
	for _, a := range rows {
		v.list.Append(v.row(a))
	}
	v.pages.Show("accounts")
}

// row is an account on lys-ui's Item: the provider icon as the
// media, the account's name, and the unread count as a badge, the
// secondary one at zero; an account whose count failed says so instead
// of a count.
func (v *mailView) row(a mail.AccountUnread) *widget.Item {
	row := widget.NewItem(v.font, a.Name, "")
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("mail-dropdown-row")
	row.SetMedia(widget.NewThemeIcon(a.Icon, itemIconPx), false)
	if a.Err != nil {
		row.AddClass("failed")
		row.SetDescription(i18n.T("dropdown-mail-account-error"))
		return row
	}
	count := widget.NewBadge(v.font, strconv.FormatUint(uint64(a.Count), 10))
	count.AddClass("mail-account-count")
	if a.Count == 0 {
		count.SetVariant(widget.BadgeSecondary)
	}
	row.AddAction(count)
	return row
}

func (v *mailView) follow(life context.Context) {
	v.rebuild()
	if v.ctx.App == nil || v.ctx.Mail == nil {
		return
	}
	ticks, stop := v.ctx.Mail.Subscribe()
	v.ctx.Go(func() {
		defer stop()
		for {
			select {
			case <-life.Done():
				return
			case _, ok := <-ticks:
				if !ok {
					return
				}
				v.ctx.Invoke(v.rebuild)
			}
		}
	})
}
