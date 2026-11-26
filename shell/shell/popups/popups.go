// Package popups renders the notification popup window: stacks of
// cards on one output, following the notification service's feed,
// auto-dismissing after the configured duration, hidden while DND.
// A stack shows its newest card in front of the older ones and fans
// out into a list while the pointer is over it.
package popups

import (
	"context"
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/notifyview"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/notifications"
	"github.com/stubbedev/lys/shell/shell/layering"
	"github.com/stubbedev/lys/shell/shell/notifyui"
)

// Popup geometry.
const (
	// popupGapBasePx and popupMarginBasePx are the bases the popup-gap
	// and popup-margin-x/y multipliers scale (POPUP_GAP_BASE_REM 0.5,
	// POPUP_MARGIN_BASE_REM 0.75, at 16px a rem).
	popupGapBasePx    = 8
	popupMarginBasePx = 12
)

// Popups owns the popup surface for one output. It belongs to the
// goroutine that owns its widget tree, the application's loop (the one
// calling it, headless), and every method runs there.
type Popups struct {
	app   *app.Application
	svc   *notifications.Service
	cfg   *config.Config
	font  render.Font
	sheet *widget.Stylesheet
	// outputs lists the connected outputs, the primary first; the
	// surface maps on popup-monitor among them.
	outputs func() []*app.Output
	now     func() time.Time
	// minter mints the activation tokens action presses carry.
	minter notifyui.TokenMinter
	// launch opens a link with an activation token.
	launch func(uri, token string) error
	// look previews a notification's attachment in Quick Look, running
	// closed once it no longer shows it; nil offers no look button.
	look func(path string, closed func()) error
	// open maps a layer surface; nil maps none (headless).
	open func(app.LayerConfig) (window, error)
	// post runs a function on the loop from any goroutine.
	post func(func())
	win  window
	// winOut is the output win is on.
	winOut *app.Output
	root   *widget.Box
	// host and list are the surface's tree, kept while no surface shows
	// it; idle are stacks whose last card went, kept for the next.
	host, list *widget.Box
	idle       []*stack
	// cards are the live popups by id; stacks are every stack on
	// screen top to bottom, those whose cards are all still playing
	// their exit included.
	cards  map[uint32]*card
	stacks []*stack
}

// window is the part of a layer window the popups drive
// (app.LayerWindow).
type window interface{ Close() }

// stack is the cards one popup-stack key groups: the newest in front,
// the older ones behind it, fanned out into a list while hovered.
type stack struct {
	key string
	// cards are newest first, those still playing their exit included.
	cards []*card
	// hovered is whether the pointer is over the stack, which expands it.
	hovered bool
	root    *widget.Box
	pile    *widget.Pile
	// clear dismisses the whole stack once expanded.
	clear *widget.Button
}

// card is one popup: notifyview's card, inside the revealer that plays
// its enter and exit and the swipe that takes it away.
type card struct {
	*notifyview.Card
	id uint32
	// leaving marks a card playing its exit.
	leaving bool
	// swipe takes the card away on a sideways swipe; rev inside it
	// plays the card's enter and exit (the LysShellRevealer around it).
	swipe *widget.SwipeDismiss
	rev   *widget.Revealer
	shown *notifications.Notification
}

// New builds the popup host; it maps on popup-monitor among outputs
// when the first popup arrives. The service's popup countdowns take the
// configured popup-duration; sheet is the shell's stylesheet, which
// styles the cards (nil leaves them unstyled); look previews a card's
// attachment in Quick Look (nil offers no look button).
func New(application *app.Application, svc *notifications.Service, cfg *config.Config, font render.Font, sheet *widget.Stylesheet, outputs func() []*app.Output, look func(path string, closed func()) error) *Popups {
	svc.SetPopupDuration(time.Duration(cfg.Notification.PopupDuration) * time.Millisecond)
	p := &Popups{
		app:     application,
		svc:     svc,
		cfg:     cfg,
		font:    font,
		sheet:   sheet,
		outputs: outputs,
		now:     time.Now,
		minter:  notifyui.Minter(application),
		launch:  notifyui.OpenLink,
		look:    look,
		cards:   make(map[uint32]*card),
	}
	if application != nil {
		p.open = func(c app.LayerConfig) (window, error) { return application.NewLayer(c) }
		p.post = application.Invoke
	}
	return p
}

// DetachOutput closes the surface when its output went away; the
// stacks remap on the configured monitor (the primary fallback) at
// once, collapsed, as the pointer left with the surface.
func (p *Popups) DetachOutput(name string) {
	if p.win == nil || p.winOut == nil || p.winOut.Name != name {
		return
	}
	win := p.win
	p.win, p.winOut, p.root = nil, nil, nil
	win.Close()
	p.unhover()
	p.layout()
}

// unhover collapses every hovered stack, as when the pointer leaves it,
// resuming the countdowns the hover paused.
func (p *Popups) unhover() {
	for _, st := range p.stacks {
		if st.hovered {
			p.setHovered(st, false)
		}
	}
}

// SetConfig applies a reloaded config on the loop goroutine: the
// surface closes and the stacks rebuild at the new position, layer,
// gap, limits, grouping, and card behavior. The countdowns a hovered
// stack paused resume, as the pointer leaves with the surface.
func (p *Popups) SetConfig(cfg *config.Config) {
	p.svc.SetPopupDuration(time.Duration(cfg.Notification.PopupDuration) * time.Millisecond)
	p.unhover()
	p.cfg = cfg
	win := p.win
	p.win, p.winOut, p.root = nil, nil, nil
	p.host, p.list, p.idle = nil, nil, nil
	p.cards = make(map[uint32]*card)
	p.stacks = nil
	if win != nil {
		win.Close()
	}
	p.sync()
}

// Run follows the service's feed until life ends; each change
// reconciles on the loop goroutine, which owns the widget tree, or,
// headless, on Run's own.
func (p *Popups) Run(life context.Context) {
	events, stop := p.svc.Subscribe()
	defer stop()
	for {
		select {
		case <-life.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
		}
		if p.app == nil {
			p.sync()
			continue
		}
		p.app.Invoke(p.sync)
	}
}

// stackKey is the popup-stack grouping: by app, everything in one
// stack, or every popup on its own.
func stackKey(mode config.PopupStack) func(*notifications.Notification) string {
	switch mode {
	case config.PopupStackAll:
		return func(*notifications.Notification) string { return "" }
	case config.PopupStackOff:
		return func(n *notifications.Notification) string { return strconv.FormatUint(uint64(n.ID), 10) }
	}
	return notifyui.AppKey
}

// sync reconciles the stacks with the service's popup set (reconcile),
// the newest popup-max-visible grouped by popup-stack: a card no longer
// listed, or now grouped elsewhere, plays its exit and leaves its stack
// once it has; a new one enters at the front of its stack. The stacks
// order by their newest card, newest first or oldest first, and a stack
// whose cards are all leaving keeps its place until they are gone.
// Headless construction tracks the cards; only the window needs the
// application.
func (p *Popups) sync() {
	visible := p.svc.Popups()
	cfg := p.cfg.Notification
	if limit := int(cfg.PopupMaxVisible); len(visible) > limit {
		visible = visible[:limit]
	}
	groups := notifyui.GroupBy(visible, stackKey(cfg.PopupStack))
	live := make(map[uint32]string, len(visible))
	for _, g := range groups {
		for _, n := range g.Notifs {
			live[n.ID] = g.Key
		}
	}
	var leaving, entering []*card
	byKey := make(map[string]*stack, len(p.stacks))
	for _, st := range p.stacks {
		byKey[st.key] = st
		for _, c := range st.cards {
			if c.leaving {
				continue
			}
			if key, ok := live[c.id]; ok && key == st.key {
				continue
			}
			c.leaving = true
			delete(p.cards, c.id)
			leaving = append(leaving, c)
		}
	}
	order := make([]*stack, 0, len(groups)+len(p.stacks))
	for _, g := range groups {
		st, ok := byKey[g.Key]
		if !ok {
			st = p.newStack(g.Key)
		}
		for _, n := range slices.Backward(g.Notifs) {
			if c, ok := p.cards[n.ID]; ok {
				p.updateCard(c, n)
				continue
			}
			c := p.newCard(n)
			p.cards[n.ID] = c
			st.cards = slices.Insert(st.cards, 0, c)
			entering = append(entering, c)
			if st.hovered && cfg.PopupHoverPause {
				p.svc.InhibitPopup(n.ID)
			}
		}
		order = append(order, st)
	}
	if cfg.PopupStackingOrder == config.StackingOrderOldestFirst {
		slices.Reverse(order)
	}
	for i, st := range p.stacks {
		if !slices.Contains(order, st) {
			order = slices.Insert(order, min(i, len(order)), st)
		}
	}
	p.stacks = order
	for _, st := range p.stacks {
		p.refreshStack(st)
	}
	p.layout()
	anims := p.cfg.Animations
	// An exit that lands at once (animations off) drops its card through
	// dropCard right away.
	for _, c := range leaving {
		reveal.Hide(c.rev, anims, config.AnimNotifications, func() { p.dropCard(c) })
	}
	for _, c := range entering {
		reveal.Show(c.rev, anims, config.AnimNotifications)
	}
}

// dropCard takes a card whose exit has played off its stack, the stack
// off the screen with its last card, and the window down with the last
// stack.
func (p *Popups) dropCard(c *card) {
	for i, st := range p.stacks {
		j := slices.Index(st.cards, c)
		if j < 0 {
			continue
		}
		st.cards = slices.Delete(st.cards, j, j+1)
		if len(st.cards) == 0 {
			p.stacks = slices.Delete(p.stacks, i, i+1)
			st.hovered = false
			p.refreshStack(st)
			p.idle = append(p.idle, st)
		} else {
			p.refreshStack(st)
		}
		p.layout()
		return
	}
}

// newStack builds an empty stack for key: the clear-all button over
// the pile of cards.
func (p *Popups) newStack(key string) *stack {
	if n := len(p.idle); n > 0 {
		st := p.idle[n-1]
		p.idle = p.idle[:n-1]
		st.key = key
		return st
	}
	th := widget.Current()
	st := &stack{key: key}
	st.root = widget.NewBox(widget.Column, th.Spacing, 0)
	st.root.AddClass("notification-popup-stack")
	// The stack is the card width exactly, lys-ui's explicit width for
	// content that fills it: its cards fill the pile's rects, the
	// peeking ones narrower, and their Items would otherwise fill the
	// output.
	st.root.SetInlineStyle(styling.WidthStyle(notifyview.Width(float64(p.cfg.Styling.Scale))))
	label := widget.NewLabel(p.font, widgets.TextPx(styling.Body, float64(p.cfg.Styling.Scale)), i18n.T("notification-popup-clear-all"), 0)
	st.clear = widget.NewButton(label, 0, 0)
	st.clear.AddClass("notification-popup-clear-all", css.Glass, css.Small)
	st.clear.SetHAlign(widget.AlignEnd)
	st.clear.OnClick = func() { p.clearStack(st) }
	st.root.Append(st.clear, false)
	st.pile = widget.NewPile()
	st.pile.AddClass("notification-popup-pile")
	st.pile.SetSpacing(th.Spacing)
	st.pile.SetPeekOffset(th.Spacing)
	st.pile.SetPeekEdge(peekEdge(p.cfg.Notification.PopupPosition))
	st.root.Append(st.pile, false)
	st.root.SetOnHoverWithin(func(on bool) { p.hoverStack(st, on) })
	return st
}

// refreshStack lays the stack's cards into its pile, front first, and
// shows what its state calls for: collapsed, the front card over the
// edges of the ones behind it, counting them in its top row; expanded,
// every card in full and the clear-all button.
func (p *Popups) refreshStack(st *stack) {
	expanded := st.hovered
	want := make([]widget.Widget, len(st.cards))
	ids := st.liveIDs()
	for i, c := range st.cards {
		want[i] = c.swipe
		c.SetFull(expanded)
		front := len(ids) > 0 && c.id == ids[0] && !c.leaving
		c.Count.SetText("+" + strconv.Itoa(len(ids)-1))
		c.Count.SetVisible(front && !expanded && len(ids) > 1)
	}
	reconcileChildren(pileChildren(st.pile), want)
	st.pile.SetExpanded(expanded)
	st.clear.SetVisible(expanded && len(ids) > 1)
}

// children is a container's child list as reconcileChildren edits it.
type children struct {
	list   func() []widget.Widget
	remove func(widget.Widget)
	insert func(i int, w widget.Widget)
	// move takes the child at from to to, keeping it attached.
	move func(from, to int)
}

// reconcileChildren makes the children want, in order, detaching none
// that stays: each keeps its layout motion and the pointer's hover.
func reconcileChildren(c children, want []widget.Widget) {
	for _, w := range c.list() {
		if !slices.Contains(want, w) {
			c.remove(w)
		}
	}
	for i, w := range want {
		have := c.list()
		switch at := slices.Index(have, w); {
		case at == i:
		case at >= 0:
			c.move(at, i)
		default:
			c.insert(i, w)
		}
	}
}

// boxChildren edits a box's children.
func boxChildren(b *widget.Box) children {
	return children{
		list:   b.Children,
		remove: func(w widget.Widget) { b.Remove(w) },
		insert: func(i int, w widget.Widget) { b.InsertAt(i, w, false) },
		move:   b.Move,
	}
}

// pileChildren edits a pile's cards. A pile has no move; its cards
// never reorder (a card enters at the front and leaves), so a move
// detaches and reinserts.
func pileChildren(p *widget.Pile) children {
	return children{
		list:   p.Children,
		remove: p.Remove,
		insert: p.Insert,
		move: func(from, to int) {
			w := p.Children()[from]
			p.Remove(w)
			p.Insert(to, w)
		},
	}
}

// hoverStack expands the stack while the pointer is over it and
// collapses it when the pointer leaves; under popup-hover-pause the
// countdowns of its cards pause meanwhile and resume with the time
// they had left.
func (p *Popups) hoverStack(st *stack, on bool) {
	if st.hovered != on {
		p.setHovered(st, on)
	}
}

// setHovered moves the stack to hovered or not, its countdowns with it.
func (p *Popups) setHovered(st *stack, on bool) {
	st.hovered = on
	p.refreshStack(st)
	if !p.cfg.Notification.PopupHoverPause {
		return
	}
	for _, id := range st.liveIDs() {
		if on {
			p.svc.InhibitPopup(id)
		} else {
			p.svc.ReleasePopup(id)
		}
	}
}

// clearStack is the clear-all button: every popup of the stack's
// group leaves the history as the dropdown's clear all does, those
// popup-max-visible keeps off screen included, so none moves up into
// the cleared stack. The service runs it off the loop, where its flush
// waits for it.
func (p *Popups) clearStack(st *stack) {
	key := stackKey(p.cfg.Notification.PopupStack)
	var ids []uint32
	for _, n := range p.svc.Popups() {
		if key(n) == st.key {
			ids = append(ids, n.ID)
		}
	}
	p.svc.Detach(func() { p.svc.Dismiss(ids) })
}

// liveIDs are the ids of the stack's cards not leaving, front first.
func (st *stack) liveIDs() []uint32 {
	ids := make([]uint32, 0, len(st.cards))
	for _, c := range st.cards {
		if !c.leaving {
			ids = append(ids, c.id)
		}
	}
	return ids
}

// newCard builds one card: notifyview's card for n, its parts acting
// on the popup.
func (p *Popups) newCard(n *notifications.Notification) *card {
	c := &card{id: n.ID, shown: n}
	id := n.ID
	h := notifyview.Handlers{
		Close:  func() { p.closeCard(id) },
		Action: func(i int) { p.invokeAction(id, notifyui.VisibleActions(c.shown)[i].ID) },
		Reply:  func(text string) { p.change(func() { p.svc.Reply(id, text) }) },
		ReplyFocus: func(focused bool) {
			if focused {
				p.svc.InhibitPopup(id)
				return
			}
			p.svc.ReleasePopup(id)
		},
		Link: p.openLink,
		Post: p.post,
		Look: notifyview.Looker("notification popups", p.look),
	}
	if _, ok := n.DefaultAction(); ok {
		h.Open = func() { p.invokeDefault(id) }
	}
	c.Card = notifyview.NewCard(p.cfg, p.font, p.now(), p.content(n), h)
	c.rev = widget.NewRevealer(c.Root)
	c.rev.SetGenieEdge(reveal.GenieEdge(popupAtTop(p.cfg.Notification.PopupPosition)))
	c.swipe = widget.NewSwipeDismiss(c.rev)
	c.swipe.OnDismiss = func(int) { p.closeCard(id) }
	return c
}

// content is what n's card shows: its icon by icon-source, and its
// buttons and reply field from its actions.
func (p *Popups) content(n *notifications.Notification) notifyview.Content {
	actions := notifyui.VisibleActions(n)
	buttons := make([]notifyview.Action, len(actions))
	for i, a := range actions {
		buttons[i] = notifyview.Action{Label: a.Label, Purpose: a.Purpose}
	}
	var reply *notifyview.Reply
	if _, ok := n.InlineReply(); ok {
		reply = &notifyview.Reply{Placeholder: n.ReplyPlaceholder}
	}
	icon := notifyui.ResolveIcon(p.cfg.Notification.IconSource, n)
	return notifyview.Content{
		App:        n.AppName,
		Icon:       icon,
		Urgency:    n.Urgency,
		Added:      n.Added,
		Summary:    n.Summary,
		Body:       n.Body,
		Progress:   n.Progress,
		Actions:    buttons,
		Reply:      reply,
		Attachment: notifyview.Attachment(icon, n.ImagePath),
	}
}

// popupAtTop is whether the popups sit along the top edge, the edge a
// genie collapses toward.
func popupAtTop(position config.ScreenPosition) bool {
	_, v := widgets.Placement(position)
	return v == widget.AlignStart
}

// peekEdge is the edge a stack's older cards peek past: away from the
// screen edge the popups hang from.
func peekEdge(position config.ScreenPosition) widget.Edge {
	if popupAtTop(position) {
		return widget.EdgeBottom
	}
	return widget.EdgeTop
}

// closeCard is the close button under popup-close-behavior: dismiss
// hides the popup and keeps the notification in the history, remove
// deletes it.
func (p *Popups) closeCard(id uint32) {
	if p.cfg.Notification.PopupCloseBehavior == config.PopupCloseBehaviorRemove {
		p.svc.Close(id, notifications.Dismissed)
		return
	}
	p.svc.DismissPopup(id)
}

// invokeAction is an action button: the popup goes, the action is
// invoked with a token from the press, and the notification is
// dismissed.
func (p *Popups) invokeAction(id uint32, key string) {
	notifyui.WithActivationToken(p.minter, func(token string) {
		p.svc.DismissPopup(id)
		p.change(func() {
			p.svc.InvokeAction(id, key, token)
			p.svc.Close(id, notifications.Dismissed)
		})
	})
}

// invokeDefault is the card's click: the default action with a token
// from the click, then the close behavior. Invoked first, while the
// notification still exists for the service to emit ActionInvoked on.
func (p *Popups) invokeDefault(id uint32) {
	notifyui.WithActivationToken(p.minter, func(token string) {
		remove := p.cfg.Notification.PopupCloseBehavior == config.PopupCloseBehaviorRemove
		p.change(func() {
			p.svc.InvokeAction(id, notifications.DefaultActionID, token)
			if remove {
				p.svc.Close(id, notifications.Dismissed)
				return
			}
			p.svc.DismissPopup(id)
		})
	})
}

// layout puts the stacks into the root, mapping the window for the
// first card and closing it after the last has gone.
func (p *Popups) layout() {
	if len(p.stacks) == 0 {
		if p.win != nil {
			win := p.win
			reconcileChildren(boxChildren(p.root), nil)
			p.win, p.winOut, p.root = nil, nil, nil
			win.Close()
		}
		return
	}
	if p.win == nil && p.open != nil {
		out := layering.Output(p.outputs(), p.cfg.Notification.PopupMonitor)
		if out == nil {
			return
		}
		win, root, err := p.ensureWindow(out)
		if err != nil {
			log.Printf("notification popups: cannot map on %s: %v", out.Name, err)
			return
		}
		p.win, p.winOut, p.root = win, out, root
	}
	if p.root == nil {
		// Headless (or window creation failed): the card set tracks the
		// popup state without a surface.
		return
	}
	want := make([]widget.Widget, len(p.stacks))
	for i, st := range p.stacks {
		want[i] = st.root
	}
	reconcileChildren(boxChildren(p.root), want)
}

// ensureWindow maps the popup surface on out at the configured
// position, margins, and layer, sized by its stacks. The host window is
// transparent (.notification-popup-host) and asks the compositor to
// blur behind its frosted cards; the list spaces the stacks by
// popup-gap and each card draws its own plate from the stylesheet. It
// returns the list the stacks go into. The tree is the one the last
// surface showed, when there was one.
func (p *Popups) ensureWindow(out *app.Output) (window, *widget.Box, error) {
	cfg := p.cfg.Notification
	scale := float64(p.cfg.Styling.Scale)
	if p.host == nil {
		p.host, p.list = p.newHost()
	}
	win, err := p.open(app.LayerConfig{
		Output:         out,
		Layer:          layering.For(p.cfg.General, cfg.PopupLayer),
		Anchor:         layering.ScreenAnchors(cfg.PopupPosition),
		Margin:         popupMargins(cfg, scale),
		Namespace:      layers.NotificationPopup.Name(),
		Keyboard:       app.KeyboardOnDemand,
		Root:           p.host,
		BackgroundBlur: p.cfg.Styling.Translucent(),
	})
	return win, p.list, err
}

// newHost builds the surface's widget tree: the transparent host
// carrying the stylesheet, around the list that spaces the stacks by
// popup-gap and keeps that same gap from every edge of the surface,
// room the cards' shadows paint into.
func (p *Popups) newHost() (host, list *widget.Box) {
	gap := int(p.cfg.Notification.PopupGap.ResolvePx(popupGapBasePx, float64(p.cfg.Styling.Scale)))
	list = widget.NewBox(widget.Column, gap, gap)
	list.AddClass("notification-popup-list")
	host = widget.NewBox(widget.Column, 0, 0)
	host.SetElement("window")
	host.AddClass("background", "notification-popup-host")
	if p.sheet != nil {
		host.AttachStylesheet(p.sheet)
	}
	host.Append(list, false)
	return host, list
}

// popupMargins are the surface's insets: popup-margin-y on the
// anchored top or bottom edge, popup-margin-x on the anchored side,
// truncated toward zero.
func popupMargins(cfg config.NotificationConfig, scale float64) app.Margins {
	mx := int32(cfg.PopupMarginX.ResolvePx(popupMarginBasePx, scale))
	my := int32(cfg.PopupMarginY.ResolvePx(popupMarginBasePx, scale))
	var m app.Margins
	anchors := layering.ScreenAnchors(cfg.PopupPosition)
	if anchors&app.AnchorTop != 0 {
		m.Top = my
	}
	if anchors&app.AnchorBottom != 0 {
		m.Bottom = my
	}
	if anchors&app.AnchorLeft != 0 {
		m.Left = mx
	}
	if anchors&app.AnchorRight != 0 {
		m.Right = mx
	}
	return m
}

// Visible reports the card count (tests).
func (p *Popups) Visible() int {
	return len(p.cards)
}

// stackOrder is the stacks on screen top to bottom, each its card ids
// front first, cards still playing their exit included (tests).
func (p *Popups) stackOrder() [][]uint32 {
	out := make([][]uint32, len(p.stacks))
	for i, st := range p.stacks {
		for _, c := range st.cards {
			out[i] = append(out[i], c.id)
		}
	}
	return out
}

func (p *Popups) offLoop(fn func()) {
	if p.app == nil {
		fn()
		return
	}
	go fn()
}

// change runs fn, which changes the service, off the loop as the
// service detaches it: its flush waits for fn and the write it queues.
// Headless it runs in place.
func (p *Popups) change(fn func()) {
	if p.app == nil {
		fn()
		return
	}
	p.svc.Detach(fn)
}

// updateCard shows a replacement in the card it replaces, touching only
// what changed.
func (p *Popups) updateCard(c *card, n *notifications.Notification) {
	if c.shown == n {
		return
	}
	c.shown = n
	c.Update(p.content(n))
}

// openLink opens a body link with the system's handler, raised by a token
// from the click; only web and mail links open.
func (p *Popups) openLink(href string) {
	if !notifyui.OpenableLink(href) {
		log.Printf("notification popups: not opening %q: only http, https and mailto links open", href)
		return
	}
	notifyui.WithActivationToken(p.minter, func(token string) {
		p.offLoop(func() {
			if err := p.launch(href, token); err != nil {
				log.Printf("notification popups: open %s: %v", href, err)
			}
		})
	})
}
