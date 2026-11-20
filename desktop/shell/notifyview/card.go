// Package notifyview draws lys-shell's notification popup card from a
// config and what a notification shows: its top row, summary, body,
// progress, reply field and action rows, with the classes the
// notification stylesheet keys on, and the helpers the popups and the
// bar's notification dropdown share. It holds no service, bus or
// surface, so lys-shell's popups stack it on a layer surface and
// lys-settings previews it from a draft config with the same code.
package notifyview

import (
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
)

const (
	cardMaxWidthChars = 40
	// BodyLines clamps a resting card's body; a full card shows all of
	// it.
	BodyLines   = 2
	cardWidthPx = 340
	cardIconPx  = 32
	cardGlyphPx = 16
)

// Width is a card's width at the styling scale.
func Width(scale float64) int { return int(cardWidthPx * scale) }

// Action is a button a card draws: its label, and the purpose that
// styles it (PurposeClass).
type Action struct {
	Label   string
	Purpose string
}

// Reply is a card's inline-reply field; an empty placeholder shows the
// card's own.
type Reply struct{ Placeholder string }

// Content is what a card shows of a notification.
type Content struct {
	// App is the sender's app name, shown through AppLabel.
	App      string
	Icon     Icon
	Urgency  notifymodel.Urgency
	Added    time.Time
	Summary  string
	Body     string
	Progress notifymodel.Progress
	// Actions are the buttons, the default action and the inline reply
	// left out.
	Actions []Action
	// Reply is the inline-reply field; nil draws none.
	Reply *Reply
}

// Handlers are what a card's parts do.
type Handlers struct {
	// Close is the close button.
	Close func()
	// Action is the press of Content.Actions[i].
	Action func(i int)
	// Open is a click anywhere the card's buttons do not take; nil
	// leaves the card unclickable.
	Open func()
	// Reply sends a non-empty reply; ReplyFocus follows the field's
	// focus.
	Reply      func(text string)
	ReplyFocus func(focused bool)
	// Link opens a body link.
	Link func(href string)
	// Post delivers a decoded file icon on the UI loop.
	Post func(func())
}

// Card is one popup on lys-ui's card: the top row as an Item (the app
// icon, the app name, Count, the age and the close button), the summary,
// the body, the progress, the reply field and the action rows.
type Card struct {
	Root     *widget.Box
	Head     *widget.Item
	Count    *widget.Badge
	Time     *widget.Label
	Summary  *widget.Label
	Body     *widget.RichLabel
	Progress *widget.ProgressBar
	Reply    *widget.Entry
	Close    *widget.Button
	Actions  *widget.Box

	content  *widget.Box
	font     render.Font
	px       float64
	handlers Handlers
	shown    Content
}

// NewCard builds the card showing c under cfg, its age read at now.
func NewCard(cfg *config.Config, font render.Font, now time.Time, c Content, h Handlers) *Card {
	notif := cfg.Notification
	th := widget.Current()
	k := &Card{font: font, px: widgets.TextPx(styling.Body, float64(cfg.Styling.Scale)), handlers: h, shown: c}
	k.Root = widget.NewBox(widget.Column, 0, 0)
	k.Root.AddClass(css.Card, css.Glass, "notification-popup-card", UrgencyClass(c.Urgency))
	if notif.PopupShadow {
		k.Root.AddClass("shadow")
	}
	if UrgencyBarVisible(c.Urgency, notif.PopupUrgencyBar) {
		k.Root.AddClass("urgency-bar")
	}

	appName := AppLabel(c.App)
	if appName == "" {
		appName = i18n.T("notification-popup-unknown-app")
	}
	k.Head = widget.NewItem(font, appName, "")
	k.Head.SetSmall(true)
	k.Head.SetEllipsize(true)
	k.Head.AddClass("notification-popup-content")
	size := cardGlyphPx
	if IsFileIcon(c.Icon) {
		size = cardIconPx
	}
	glyph := NewIcon(c.Icon, size, 0, h.Post, nil)
	glyph.AddClass("notification-popup-icon")
	if IsFileIcon(c.Icon) {
		glyph.AddClass("file-icon")
	}
	k.Head.SetMedia(glyph, IsFileIcon(c.Icon))
	k.Count = widget.NewBadge(font, "")
	k.Count.AddClass("notification-popup-count")
	k.Count.SetVisible(false)
	k.Head.AddAction(k.Count)
	k.Time = widget.NewLabel(font, k.px, TimeLabel(RelativeTime(now, c.Added), "notification-popup"), 0)
	k.Time.AddClass("notification-popup-time", css.Caption, css.DimLabel)
	k.Head.AddAction(k.Time)
	k.Close = widget.NewButton(widget.NewThemeIcon("ld-x-symbolic", cardGlyphPx), 0, 0)
	k.Close.AddClass("notification-popup-close", css.Ghost, css.Icon, css.Small)
	k.Close.OnClick = h.Close
	k.Head.AddAction(k.Close)
	k.Root.Append(k.Head, false)

	k.content = widget.NewBox(widget.Column, th.Spacing, 0)
	k.content.SetPadding(render.Insets{Left: 2 * th.Padding, Right: 2 * th.Padding, Bottom: 2 * th.Padding})
	k.Summary = widget.NewLabel(font, k.px, c.Summary, 0)
	k.Summary.AddClass("notification-popup-summary", css.Heading)
	k.Summary.SetWrap(true)
	k.Summary.SetEllipsize(widget.EllipsizeEnd)
	k.Summary.SetVisible(c.Summary != "")
	k.content.Append(k.Summary, false)
	k.Body = widget.NewRichLabel(font, k.px, BodyMarkup(c.Body), 0)
	k.Body.SetVariants(app.FontVariantsOf(font))
	k.Body.AddClass("notification-popup-body", css.DimLabel)
	k.Body.SetWrap(true)
	k.Body.SetMaxWidthChars(cardMaxWidthChars)
	k.Body.SetEllipsize(widget.EllipsizeEnd)
	k.Body.SetVisible(c.Body != "")
	k.Body.OnLinkClick = h.Link
	k.content.Append(k.Body, false)
	k.Progress = widget.NewProgressBar(0)
	k.Progress.AddClass("notification-popup-progress")
	setProgress(k.Progress, c.Progress)
	k.content.Append(k.Progress, false)
	if c.Reply != nil {
		k.Reply = k.replyEntry(*c.Reply)
		k.content.Append(k.Reply, false)
	}
	k.Actions = k.actionRows(c.Actions)
	k.content.Append(k.Actions, false)
	k.syncContentVisible()
	k.Root.Append(k.content, false)
	k.SetFull(false)

	if h.Open != nil {
		k.Root.SetCursorName("pointer")
		k.Head.SetOnActivate(h.Open)
		k.Root.SetOnClickWithin(h.Open)
	}
	return k
}

// Preview is the card showing c under cfg, its age read at now, at the
// card's width; its buttons do nothing.
func Preview(cfg *config.Config, font render.Font, now time.Time, c Content) *widget.Box {
	none := func() {}
	k := NewCard(cfg, font, now, c, Handlers{
		Close:      none,
		Action:     func(int) {},
		Reply:      func(string) {},
		ReplyFocus: func(bool) {},
		Link:       func(string) {},
		Post:       func(fn func()) { fn() },
	})
	sized := widget.NewBox(widget.Column, 0, 0)
	sized.SetInlineStyle(styling.WidthStyle(Width(float64(cfg.Styling.Scale))))
	sized.Append(k.Root, false)
	return sized
}

// SetFull shows the card whole, as its expanded stack does: the summary
// and the body unclamped and the action rows; at rest the summary keeps
// to a line, the body to BodyLines, and the actions stay hidden.
func (k *Card) SetFull(full bool) {
	if full {
		k.Summary.SetMaxLines(0)
		k.Body.SetMaxLines(0)
	} else {
		k.Summary.SetMaxLines(1)
		k.Body.SetMaxLines(BodyLines)
	}
	k.Actions.SetVisible(full && len(k.Actions.Children()) > 0)
}

// Update shows c in the card, touching only what changed: a new
// progress value repaints the bar and lays nothing out again.
func (k *Card) Update(c Content) {
	old := k.shown
	k.shown = c
	k.Summary.SetText(c.Summary)
	k.Summary.SetVisible(c.Summary != "")
	k.Body.SetMarkup(BodyMarkup(c.Body))
	k.Body.SetVisible(c.Body != "")
	setProgress(k.Progress, c.Progress)
	if old.Urgency != c.Urgency {
		k.Root.RemoveClass(UrgencyClass(old.Urgency))
		k.Root.AddClass(UrgencyClass(c.Urgency))
	}
	if !slices.Equal(old.Actions, c.Actions) {
		rows := k.actionRows(c.Actions)
		k.content.Remove(k.Actions)
		k.content.Append(rows, false)
		k.Actions = rows
		k.SetFull(k.Summary.MaxLines() == 0)
	}
	k.syncContentVisible()
}

func setProgress(bar *widget.ProgressBar, p notifymodel.Progress) {
	percent, ok := p.Percent()
	bar.SetVisible(ok)
	if ok {
		bar.SetValue(float64(percent) / 100)
	}
}

func (k *Card) syncContentVisible() {
	k.content.SetVisible(k.Summary.Visible() || k.Body.Visible() || k.Progress.Visible() || k.Reply != nil || len(k.Actions.Children()) > 0)
}

// actionRows lays out the actions as lys-ui's small outline buttons,
// ActionsPerRow to a row, each row homogeneous so its buttons split its
// width evenly.
func (k *Card) actionRows(actions []Action) *widget.Box {
	gap := widget.Current().Spacing / 2
	box := widget.NewBox(widget.Column, gap, 0)
	box.AddClass("notification-popup-actions")
	for start := 0; start < len(actions); start += ActionsPerRow {
		row := widget.NewBox(widget.Row, gap, 0)
		row.AddClass("notification-popup-action-row")
		row.SetHomogeneous(true)
		for i := start; i < min(start+ActionsPerRow, len(actions)); i++ {
			label := widget.NewLabel(k.font, k.px, actions[i].Label, 0)
			label.SetAlignment(render.AlignCenter)
			b := widget.NewButton(label, 0, 0)
			b.AddClass("notification-popup-action-btn", css.Outline, css.Small)
			if class := PurposeClass(actions[i].Purpose); class != "" {
				b.AddClass(class)
			}
			b.OnClick = func() { k.handlers.Action(i) }
			row.Append(b, true)
		}
		box.Append(row, false)
	}
	return box
}

// replyEntry is the inline-reply field: Enter sends a non-empty reply,
// Escape clears the field and sends nothing.
func (k *Card) replyEntry(r Reply) *widget.Entry {
	e := widget.NewEntry(k.font, k.px, 0)
	e.AddClass("notification-popup-reply")
	placeholder := r.Placeholder
	if placeholder == "" {
		placeholder = i18n.T("notification-popup-reply-placeholder")
	}
	e.SetPlaceholder(placeholder)
	e.OnActivate = func(text string) {
		if text == "" {
			return
		}
		k.handlers.Reply(text)
	}
	e.OnEscape = func() bool {
		e.SetText("")
		return true
	}
	e.SetOnFocusChanged(k.handlers.ReplyFocus)
	return e
}

// Placement is where popup-position puts the popups on their output
// along each axis: a corner at both ends, an edge's middle centered
// along it.
func Placement(p config.PopupPosition) (h, v widget.Align) {
	switch p {
	case config.PopupPositionTopLeft:
		return widget.AlignStart, widget.AlignStart
	case config.PopupPositionTopCenter:
		return widget.AlignCenter, widget.AlignStart
	case config.PopupPositionBottomLeft:
		return widget.AlignStart, widget.AlignEnd
	case config.PopupPositionBottomCenter:
		return widget.AlignCenter, widget.AlignEnd
	case config.PopupPositionBottomRight:
		return widget.AlignEnd, widget.AlignEnd
	case config.PopupPositionCenterLeft:
		return widget.AlignStart, widget.AlignCenter
	case config.PopupPositionCenterRight:
		return widget.AlignEnd, widget.AlignCenter
	}
	return widget.AlignEnd, widget.AlignStart
}
