package widget

import (
	"slices"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/widget/css"
)

// ItemVariant picks an Item's look.
type ItemVariant uint8

// The item variants.
const (
	// ItemDefault is a plain row.
	ItemDefault ItemVariant = iota
	// ItemOutline draws a border around it.
	ItemOutline
	// ItemMuted gives it the muted fill.
	ItemMuted
)

// itemClasses are the variants' classes on the item element.
var itemClasses = [...]string{ItemDefault: "", ItemOutline: css.Outline, ItemMuted: css.Muted}

// Item is a content row: media - an icon on a muted
// tile, an avatar - then a title over a description (wrapped pretty, no
// lone word ending either), then actions and
// the row's control, for lists and cards. Its text and media update in
// place, so a live list reconciles rows instead of rebuilding them, and
// SetOnActivate makes the whole row a control (click, Enter, Space).
// The controls of the items in one ItemGroup share a column
// (ItemControlColumn), so a card's fields start and end at one x, and
// their readings (ItemValue) share a width, so the tracks before them
// end at one x. Trailing actions sit outside the column: every column
// of a group leaves room for the widest of them, so a column control
// lines up whether or not its item has one. It
// styles as the `item` element with the variant's class (.outline,
// .muted), .small for the small size, .activatable while it activates,
// its .media tile, .title and .description.
type Item struct {
	composite
	stack   *Box
	line    *itemLine
	details *Box
	media   *Box
	title   *Label
	desc    *Label
	actions *Box
	// titleLine holds the title with its suffixes, once one is added.
	titleLine *Box
	// leading counts the actions ahead of the control; the trailing
	// ones follow it.
	leading int
	// control is the placed control as the tree holds it: the control
	// itself, or the column slot holding it.
	control Widget
	fit     ItemControlFit
	column  *itemSlot
	// trailing hold the actions after the control.
	trailing []*itemSlot
	// values are the readings inside the control.
	values []*ItemValue
	// titleLines and descLines cap the labels' rows, 0 for no cap.
	titleLines, descLines int
	group                 *ItemGroup
	variant               ItemVariant
	activate              func()
}

// ItemControlFit is how an Item's control takes its place in the row.
type ItemControlFit uint8

// The control fits.
const (
	// ItemControlNatural keeps the control its natural width at the
	// row's end: a switch, a button.
	ItemControlNatural ItemControlFit = iota
	// ItemControlColumn stretches the control over the column every
	// item of its ItemGroup shares, at least ItemColumnWidth wide: the
	// group's dropdowns, spins and entries start and end at one x
	// whatever they hold.
	ItemControlColumn
	// ItemControlWide gives the control a line of its own under the
	// text, the row's full width: a list, a picker too wide for the
	// column.
	ItemControlWide
)

// ItemColumnWidth is the narrowest control column, an entry's text
// width: a lone or empty dropdown keeps it.
const ItemColumnWidth = GTKTextWidth

// itemGap is the gap between an item's line and its wide control.
const itemGap = 8

// itemLineGap is the gap between an item's media, text and actions, and
// itemTextGap the one between its title, description and details.
const (
	itemLineGap = 16
	itemTextGap = 4
)

// itemLine lays out an item's line: the media, the title over the
// description and the details, then the actions. The media and the
// actions stand centred on the head, the title and the description as
// they wrap beside the actions, and the text centres on them when they
// are the taller. The description runs around the actions: its rows
// beside them break short of them, and the rows below them, and the
// details once they start below them, take the line's width from the
// text's start, under the actions column.
type itemLine struct {
	node
	it *Item
}

// itemLineBoxes are the parts' rects in an itemLine of some width, from
// its top left, the description's flow, and the line's height.
type itemLineBoxes struct {
	media, title, desc, details, actions render.Rect
	flow                                 labelFlow
	h                                    int
}

// titleWidget is the title, or the title's line once it has suffixes.
func (it *Item) titleWidget() Widget {
	if it.titleLine != nil {
		return it.titleLine
	}
	return it.title
}

// shownWidth is w's natural width when it is shown and takes any, else 0.
func (l *itemLine) shownWidth(w Widget) int {
	if w == nil || !IsVisible(w) {
		return 0
	}
	return measureChild(l, w, Row, Unknown).Nat
}

// lay computes the parts' boxes at width w.
func (l *itemLine) lay(w int) itemLineBoxes {
	it := l.it
	var b itemLineBoxes
	var media Widget
	if it.media != nil {
		media = it.media
	}
	x, end := 0, w
	mediaH := 0
	if mw := l.shownWidth(media); mw > 0 {
		mediaH = measureChild(l, media, Column, mw).Nat
		b.media = render.Rect{W: mw, H: mediaH}
		x = mw + itemLineGap
	}
	actH := 0
	if aw := l.actionsWidth(w - x); aw > 0 {
		actH = measureChild(l, it.actions, Column, aw).Nat
		b.actions = render.Rect{X: w - aw, W: aw, H: actH}
		end = w - aw - itemLineGap
	}
	textW, fullW := max(0, end-x), max(0, w-x)
	title := it.titleWidget()
	titleH := measureChild(l, title, Column, textW).Nat
	descOn := IsVisible(it.desc)
	head := titleH
	if descOn {
		it.desc.setFlow(labelFlow{})
		head += itemTextGap + measureChild(l, it.desc, Column, textW).Nat
	}
	rowH := max(head, actH, mediaH)
	headY := (rowH - head) / 2
	b.media.Y = (rowH - mediaH) / 2
	b.actions.Y = (rowH - actH) / 2
	actBottom := b.actions.Y + actH
	b.title = render.Rect{X: x, Y: headY, W: textW, H: titleH}
	y := headY + titleH
	if descOn {
		y += itemTextGap
		if beside := actBottom - y; actH > 0 && beside > 0 {
			lh := it.desc.lineBox(it.desc.style(it.desc))
			b.flow = labelFlow{cut: fullW - textW, rows: (beside + lh - 1) / max(lh, 1)}
		}
		it.desc.setFlow(b.flow)
		dh := measureChild(l, it.desc, Column, fullW).Nat
		b.desc = render.Rect{X: x, Y: y, W: fullW, H: dh}
		y += dh
	}
	if l.shownWidth(it.details) > 0 {
		y += itemTextGap
		dw := textW
		if y >= actBottom {
			dw = fullW
		}
		dh := measureChild(l, it.details, Column, dw).Nat
		b.details = render.Rect{X: x, Y: y, W: dw, H: dh}
		y += dh
	}
	b.h = max(y, actBottom, b.media.Y+mediaH)
	return b
}

// actionsWidth is the width the actions take out of avail, the room
// after the media: their natural width, given up down to their minimum
// where the text's minimum and the gap would not fit beside it, as a
// box shrinks its children; 0 without shown actions.
func (l *itemLine) actionsWidth(avail int) int {
	if !IsVisible(l.it.actions) {
		return 0
	}
	s := measureChild(l, l.it.actions, Row, Unknown)
	if s.Nat <= 0 {
		return 0
	}
	return max(s.Min, min(s.Nat, avail-itemLineGap-l.textSizes().Min))
}

// textSizes is the widest request across of the shown text parts.
func (l *itemLine) textSizes() Sizes {
	var text Sizes
	for _, w := range [...]Widget{l.it.titleWidget(), l.it.desc, l.it.details} {
		if IsVisible(w) {
			text = text.widest(measureChild(l, w, Row, Unknown))
		}
	}
	return text
}

// kids are the line's parts, in paint and walk order.
func (l *itemLine) kids() []Widget {
	out := make([]Widget, 0, 5)
	if l.it.media != nil {
		out = append(out, l.it.media)
	}
	return append(out, l.it.titleWidget(), l.it.desc, l.it.details, l.it.actions)
}

// Children are the line's parts.
func (l *itemLine) Children() []Widget { return l.kids() }

// appendChildren appends the line's parts, matching Children.
func (l *itemLine) appendChildren(buf []Widget) []Widget { return append(buf, l.kids()...) }

// appendStyleKids is the line's parts (styleKids).
func (l *itemLine) appendStyleKids(buf []Widget) []Widget { return append(buf, l.kids()...) }

// Measure is the parts side by side across, the media, the widest of
// the text's parts and the actions with the gaps between, and the laid
// out line's height down.
func (l *itemLine) Measure(axis Axis, forSize int) Sizes {
	if sz, ok := l.measureHit(axis, forSize); ok {
		return sz
	}
	if axis == Column {
		w := forSize
		if w == Unknown {
			w = l.Measure(Row, Unknown).Nat
		}
		return l.measureStore(axis, forSize, FixedSizes(l.lay(w).h))
	}
	req := l.textSizes()
	var media Widget
	if l.it.media != nil {
		media = l.it.media
	}
	for _, w := range [...]Widget{media, l.it.actions} {
		if w == nil || !IsVisible(w) {
			continue
		}
		if s := measureChild(l, w, Row, Unknown); s.Nat > 0 {
			req = Sizes{Min: req.Min + s.Min + itemLineGap, Nat: req.Nat + s.Nat + itemLineGap}
		}
	}
	return l.measureStore(axis, forSize, req)
}

// Arrange places the parts as lay computes them.
func (l *itemLine) Arrange(r render.Rect) {
	l.node.Arrange(r)
	kids := l.kids()
	setParents(l, kids...)
	b := l.lay(r.W)
	at := func(w Widget, box render.Rect) {
		arrangeChild(w, render.Rect{X: r.X + box.X, Y: r.Y + box.Y, W: box.W, H: box.H})
	}
	if l.it.media != nil {
		at(l.it.media, b.media)
	}
	at(l.it.titleWidget(), b.title)
	at(l.it.desc, b.desc)
	at(l.it.details, b.details)
	at(l.it.actions, b.actions)
}

// Paint draws the shown parts.
func (l *itemLine) Paint(cv *render.Canvas) {
	for _, w := range l.kids() {
		if IsVisible(w) {
			PaintChild(cv, w)
		}
	}
}

// HitTest returns the topmost part under p, or the line inside its
// bounds.
func (l *itemLine) HitTest(p Point) Widget {
	kids := l.kids()
	for i := range slices.Backward(kids) {
		if IsVisible(kids[i]) {
			if hit := HitChild(kids[i], p); hit != nil {
				return hit
			}
		}
	}
	return l.HitLeaf(l, p)
}

// NewItem returns a default item; an empty description shows the
// title alone.
func NewItem(face render.Font, title, description string) *Item {
	face = requireFace("widget.NewItem", face)
	it := &Item{stack: NewBox(Column, itemGap, 0), details: NewBox(Column, itemTextGap, 0), actions: NewBox(Row, 8, 0)}
	it.SetElement("item")
	it.title = NewLabel(face, 14, title, 0)
	it.title.AddClass("title")
	it.title.SetWrap(true)
	it.title.SetPrettyWrap(true)
	it.desc = NewLabel(face, 14, description, 0)
	it.desc.AddClass("description")
	it.desc.SetWrap(true)
	it.desc.SetPrettyWrap(true)
	it.desc.SetVisible(description != "")
	it.line = &itemLine{it: it}
	it.stack.Append(it.line, false)
	it.initComposite(it, it.stack)
	it.SetHExpand(true)
	it.boxed = true
	return it
}

// SetMedia leads the row with w on a muted 32px tile (an icon), or as
// it is when bare (an avatar, an image); nil removes it.
func (it *Item) SetMedia(w Widget, bare bool) {
	if it.media != nil {
		clearParents(it.media)
		it.media = nil
		it.line.InvalidateLayout()
	}
	if w == nil {
		return
	}
	it.media = NewBox(Column, 0, 0)
	if !bare {
		it.media.AddClass("media")
		it.media.SetSizeRequest(32, 32)
	}
	it.media.Append(NewSpacer(0, 0), true).AppendAligned(w, false, AlignCenter).Append(NewSpacer(0, 0), true)
	it.line.InvalidateLayout()
}

// SetTitle replaces the title.
func (it *Item) SetTitle(title string) {
	it.title.SetText(title)
	tipCapped(it.title, it.titleLines)
}

// Title returns the title's text.
func (it *Item) Title() string { return it.title.Text() }

// Description returns the description's text.
func (it *Item) Description() string { return it.desc.Text() }

// SetDescription replaces the description; empty hides its line.
func (it *Item) SetDescription(description string) {
	it.desc.SetText(description)
	it.desc.SetVisible(description != "")
	tipCapped(it.desc, it.descLines)
}

// SetLines caps the title at title rows and the description at
// description rows: a capped label wraps up to its cap, ends its last
// row in an ellipsis and carries its whole text as its tooltip. 0
// lets the label wrap freely again.
func (it *Item) SetLines(title, description int) {
	it.titleLines, it.descLines = max(title, 0), max(description, 0)
	capLines(it.title, it.titleLines)
	capLines(it.desc, it.descLines)
}

func capLines(l *Label, n int) {
	l.SetWrap(true)
	l.SetMaxLines(n)
	l.SetEllipsize(EllipsizeNone)
	if n > 0 {
		l.SetEllipsize(EllipsizeEnd)
	}
	tipCapped(l, n)
}

// tipCapped sets a label's tooltip to its text while it is capped at n
// rows, and clears it otherwise.
func tipCapped(l *Label, n int) {
	if n == 0 {
		l.SetTooltip("")
		return
	}
	l.SetTooltip(l.Text())
}

// SetTitleClass adds or drops a class on the title alone (a highlight).
func (it *Item) SetTitleClass(name string, on bool) { it.title.SetClass(name, on) }

// AddTitleSuffix sets w (a badge, a unit) on the title's line, after
// the title and what was added before it.
func (it *Item) AddTitleSuffix(w Widget) {
	if it.titleLine == nil {
		it.titleLine = NewBox(Row, 8, 0)
		it.titleLine.AppendAligned(it.title, false, AlignCenter)
		it.line.InvalidateLayout()
	}
	it.titleLine.AppendAligned(w, false, AlignCenter)
}

// AddDetail adds w (a problem line, a note) under the description,
// outside the head the media and the actions centre on.
func (it *Item) AddDetail(w Widget) { it.details.Append(w, false) }

// SetControl places the row's control by fit, after the actions and
// before the trailing actions; nil removes it. A column control joins
// the column of the ItemGroup the item is in, and the readings inside
// any control join the group's reading width.
func (it *Item) SetControl(w Widget, fit ItemControlFit) {
	if it.control != nil {
		if !it.actions.Remove(it.control) {
			it.stack.Remove(it.control)
		}
		for _, v := range it.values {
			v.item = nil
		}
		it.control, it.column, it.values = nil, nil, nil
		it.group.relaySlots()
	}
	if w == nil {
		return
	}
	it.fit = fit
	it.values = valuesIn(w)
	for _, v := range it.values {
		v.item = it
	}
	defer it.group.relaySlots()
	switch fit {
	case ItemControlWide:
		it.control = w
		it.stack.Append(w, false)
		return
	case ItemControlColumn:
		it.column = &itemSlot{item: it, kind: slotColumn}
		it.column.initComposite(it.column, w)
		it.control = it.column
	default:
		it.control = w
	}
	it.actions.InsertAligned(it.leading, it.control, false, AlignCenter)
}

// valuesIn are the readings under w, not looking inside one.
func valuesIn(w Widget) []*ItemValue {
	var out []*ItemValue
	walkWidgets(w, func(x Widget) bool {
		if v, ok := x.(*ItemValue); ok {
			out = append(out, v)
			return false
		}
		return true
	})
	return out
}

// ControlFit is how the control was placed.
func (it *Item) ControlFit() ItemControlFit { return it.fit }

// AddTrailing appends w (a reveal, an overflow button) after the
// control, at the row's end, outside the control column.
func (it *Item) AddTrailing(w Widget) {
	s := &itemSlot{item: it, kind: slotTrailing}
	s.initComposite(s, w)
	it.trailing = append(it.trailing, s)
	it.actions.AppendAligned(s, false, AlignCenter)
	it.eachPeer(slotColumn, func(c *itemSlot) { c.InvalidateLayout() })
}

// gutter is what the item's shown trailing actions take after its
// control, each with the gap before it.
func (it *Item) gutter() int {
	gap := it.actions.gap(it.actions.style(it.actions))
	g := 0
	for _, s := range it.trailing {
		if ownVisible(s) {
			g += gap + s.natural().Nat
		}
	}
	return g
}

// item is the Item an embedding row is built on, so an ItemGroup finds
// the column its controls share.
func (it *Item) item() *Item { return it }

// itemOf is the Item w is or is built on, nil for another widget.
func itemOf(w Widget) *Item {
	if r, ok := w.(interface{ item() *Item }); ok {
		return r.item()
	}
	return nil
}

// slotKind is what an item's slots size alike across its ItemGroup.
type slotKind uint8

const (
	// slotColumn is the control column.
	slotColumn slotKind = iota
	// slotValue is a control's reading.
	slotValue
	// slotTrailing is a trailing action, its own width; the columns of
	// its item's group make room for the widest.
	slotTrailing
)

// itemSlot holds a widget as wide as the widest slot of its kind in its
// item's group, or in its item outside one: the control column (at
// least ItemColumnWidth), a reading.
type itemSlot struct {
	composite
	item *Item
	kind slotKind
}

// ItemValue is a control's reading - a slider's value, the field
// typing it - that the controls of one ItemGroup show in one width: as
// wide as the widest reading of the group, so the tracks before them
// end at one x whatever each reads. Outside a group it is as wide as
// the widest reading of its item, and outside an item its own width.
type ItemValue struct{ itemSlot }

// NewItemValue holds w, a reading, in the width its item's group
// shares.
func NewItemValue(w Widget) *ItemValue {
	v := &ItemValue{itemSlot{kind: slotValue}}
	v.initComposite(v, w)
	return v
}

// width is the slot's width across: the widest slot of its kind in its
// group, or its item's outside one, a column at least ItemColumnWidth.
func (s *itemSlot) width() Sizes {
	var req Sizes
	if s.kind == slotColumn {
		req.Nat = ItemColumnWidth
	}
	s.item.eachPeer(s.kind, func(o *itemSlot) { req = req.widest(o.natural()) })
	return req.grow(s.pad())
}

// pad is the room a column slot leaves after its control for the
// trailing actions of the widest gutter among its peers, less its own
// item's, which follow it: 0 for another slot.
func (s *itemSlot) pad() int {
	if s.kind != slotColumn || s.item == nil {
		return 0
	}
	widest := 0
	s.item.eachPeer(slotColumn, func(o *itemSlot) { widest = max(widest, o.item.gutter()) })
	return widest - s.item.gutter()
}

// natural is the held widget's own request across.
func (s *itemSlot) natural() Sizes { return measureChild(s, s.root, Row, Unknown) }

// Measure is the slot's shared width across, and the held widget's
// height for the width it gets; outside an item, the widget's own
// request.
func (s *itemSlot) Measure(axis Axis, forSize int) Sizes {
	if sz, ok := s.measureHit(axis, forSize); ok {
		return sz
	}
	if axis == Column && forSize != Unknown {
		return s.measureStore(axis, forSize, measureChild(s, s.root, axis, max(0, forSize-s.pad())))
	}
	if axis == Column || s.item == nil || s.kind == slotTrailing {
		return s.measureStore(axis, forSize, measureChild(s, s.root, axis, forSize))
	}
	return s.measureStore(axis, forSize, s.width())
}

// Arrange lays the held widget over the slot but its pad, at its start.
func (s *itemSlot) Arrange(r render.Rect) {
	s.node.Arrange(r)
	setParents(s.self, s.root)
	inner := s.insets().outer().Shrink(r)
	inner.W = max(0, inner.W-s.pad())
	arrangeChild(s.root, inner)
}

// Visible is the slot's own flag, and for a trailing action its
// widget's too: a hidden action takes no room and no gap.
func (s *itemSlot) Visible() bool {
	if s.kind == slotTrailing && !ownVisible(s.root) {
		return false
	}
	return s.node.Visible()
}

// markMeasureDirty relayouts the other slots of its kind with this one,
// and the columns with a trailing action: their width depends on its
// widget's.
func (s *itemSlot) markMeasureDirty() bool {
	was := s.node.markMeasureDirty()
	if s.item == nil {
		return was
	}
	kind := s.kind
	if kind == slotTrailing {
		kind = slotColumn
	}
	s.item.eachPeer(kind, func(o *itemSlot) {
		if o != s && !o.measureDirty {
			o.InvalidateLayout()
		}
	})
	return was
}

// eachPeer visits the slots of kind sharing a width with the item's:
// its group's and those of the groups sharing its widths, or its own
// outside one.
func (it *Item) eachPeer(kind slotKind, visit func(*itemSlot)) {
	if it.group != nil {
		it.group.eachShared(kind, visit)
		return
	}
	it.eachSlot(kind, visit)
}

// eachSlot visits the item's slots of kind.
func (it *Item) eachSlot(kind slotKind, visit func(*itemSlot)) {
	switch kind {
	case slotColumn:
		if it.column != nil {
			visit(it.column)
		}
	case slotValue:
		for _, v := range it.values {
			visit(&v.itemSlot)
		}
	}
}

// SetEllipsize keeps the title and the description on one line each,
// ending in an ellipsis when the row is too narrow, instead of
// wrapping: a one-token name (a device's address) then no longer sets
// the row's minimum width and pushes its actions out.
func (it *Item) SetEllipsize(on bool) {
	mode := EllipsizeNone
	if on {
		mode = EllipsizeEnd
	}
	for _, l := range [...]*Label{it.title, it.desc} {
		l.SetWrap(!on)
		l.SetEllipsize(mode)
	}
}

// SetOnActivate makes the whole row a control running fn on a click
// outside its actions, Enter or Space, with the hover fill and the
// focus ring; nil makes it a plain row again.
func (it *Item) SetOnActivate(fn func()) {
	it.activate = fn
	if fn == nil {
		it.RemoveClass(css.Activatable)
		it.SetCursorName("")
		return
	}
	it.AddClass(css.Activatable)
	it.SetCursorName(cursorPointer)
}

// focusable implements focusOptOut: only an activatable item takes the
// focus.
func (it *Item) focusable() bool { return it.activate != nil }

// HitTest resolves an activatable item to itself, but for its action
// buttons; a plain item hits its parts.
func (it *Item) HitTest(p Point) Widget {
	if it.activate == nil {
		return it.composite.HitTest(p)
	}
	if hit := HitChild(it.actions, p); hit != nil && hit != Widget(it.actions) {
		return hit
	}
	if it.control != nil && it.column == nil {
		if hit := HitChild(it.control, p); hit != nil {
			return hit
		}
	}
	return it.HitLeaf(it, p)
}

// ClickAt runs the activation.
func (it *Item) ClickAt(Point) {
	if it.activate != nil {
		it.activate()
	}
}

// KeyAction runs the activation on Enter or Space.
func (it *Item) KeyAction(a KeyAction, _ Mods) {
	if it.activate != nil && (a == KeyEnter || a == KeySpace) {
		it.activate()
	}
}

// AddAction appends w (a button) to the actions, which sit after the
// text and before the control.
func (it *Item) AddAction(w Widget) {
	it.actions.InsertAligned(it.leading, w, false, AlignCenter)
	it.leading++
}

// SetVariant switches the item's look.
func (it *Item) SetVariant(v ItemVariant) {
	if c := itemClasses[it.variant]; c != "" {
		it.RemoveClass(c)
	}
	if c := itemClasses[v]; c != "" {
		it.AddClass(c)
	}
	it.variant = v
}

// SetSmall switches to the small size.
func (it *Item) SetSmall(on bool) {
	it.SetClass(css.Small, on)
}

// ButtonGroup joins buttons into one control: side by side or stacked,
// only the outer corners rounded and the inner borders shared, with
// SetSeparator lines between and groups nesting in groups. It styles as
// the `buttongroup` element, .vertical when stacked.
type ButtonGroup struct {
	composite
	box *Box
}

// NewButtonGroup joins buttons side by side.
func NewButtonGroup(buttons ...Widget) *ButtonGroup {
	g := &ButtonGroup{box: NewBox(Row, 0, 0)}
	g.SetElement("buttongroup")
	g.initComposite(g, g.box)
	for _, b := range buttons {
		g.Append(b)
	}
	g.boxed = true
	return g
}

// SetVertical stacks the buttons.
func (g *ButtonGroup) SetVertical(on bool) {
	if on {
		g.box.SetAxis(Column)
		g.AddClass("vertical")
		return
	}
	g.box.SetAxis(Row)
	g.RemoveClass("vertical")
}

// Append adds w (a button, a separator, a nested group) at the end.
// Nested groups sit apart.
func (g *ButtonGroup) Append(w Widget) {
	if _, ok := w.(*ButtonGroup); ok {
		g.box.SetSpacing(8)
	}
	g.box.Append(w, false)
}

// ItemGroup stacks Items into one list, a
// separator between each item and the next. It styles as the
// `itemgroup` element.
type ItemGroup struct {
	composite
	box    *Box
	items  int
	widths *ItemWidths
}

// ItemWidths joins ItemGroups, the cards of one page, so their items
// share widths as the items of one group do: one control column, one
// reading width, so the fields and the slider tracks of every joined
// group start and end at one x.
type ItemWidths struct{ groups []*ItemGroup }

// NewItemWidths returns widths no group shares yet.
func NewItemWidths() *ItemWidths { return &ItemWidths{} }

// SetWidths makes the group share w's widths with the other groups
// sharing them; nil gives it widths of its own again. A group leaving a
// page leaves its widths too, or it keeps holding them.
func (g *ItemGroup) SetWidths(w *ItemWidths) {
	if g.widths == w {
		return
	}
	if old := g.widths; old != nil {
		old.groups = slices.DeleteFunc(old.groups, func(o *ItemGroup) bool { return o == g })
		if len(old.groups) > 0 {
			old.groups[0].relaySlots()
		}
	}
	g.widths = w
	if w != nil {
		w.groups = append(w.groups, g)
	}
	g.relaySlots()
}

// NewItemGroup returns a group of items, in order.
func NewItemGroup(items ...Widget) *ItemGroup {
	g := &ItemGroup{box: NewBox(Column, 0, 0)}
	g.SetElement("itemgroup")
	g.initComposite(g, g.box)
	for _, it := range items {
		g.Append(it)
	}
	g.boxed = true
	return g
}

// Append adds w at the end, after a separator when the group already
// holds an item.
func (g *ItemGroup) Append(w Widget) {
	if g.items > 0 {
		g.box.Append(NewSeparator(Horizontal), false)
	}
	g.box.Append(w, false)
	g.items++
	if it := itemOf(w); it != nil {
		it.group = g
		g.relaySlots()
	}
}

// Remove drops item w with the separator that set it apart (the one
// before it, or after it for the first), reporting whether w was in
// the group.
func (g *ItemGroup) Remove(w Widget) bool {
	kids := g.box.Children()
	i := slices.Index(kids, w)
	if i < 0 {
		return false
	}
	g.box.Remove(w)
	switch {
	case i > 0:
		g.box.Remove(kids[i-1])
	case len(kids) > 1:
		g.box.Remove(kids[1])
	}
	g.items--
	if it := itemOf(w); it != nil {
		it.group = nil
		g.relaySlots()
	}
	return true
}

// Measure shows a separator only between two shown items, as a GTK
// boxed list does, then measures the stack: a hidden item takes the
// separators beside it with it.
func (g *ItemGroup) Measure(axis Axis, forSize int) Sizes {
	g.syncSeparators()
	return g.composite.Measure(axis, forSize)
}

// syncSeparators shows the separator before an item when the item and
// an item above it are shown. The stack alternates item, separator,
// item: the separator before item k sits just before it.
func (g *ItemGroup) syncSeparators() {
	kids := g.box.Children()
	above := false
	for i := 0; i < len(kids); i += 2 {
		shown := ownVisible(kids[i])
		if i > 0 {
			if sep, ok := kids[i-1].(*Separator); ok {
				sep.SetVisible(shown && above)
			}
		}
		above = above || shown
	}
}

// Clear drops every item.
func (g *ItemGroup) Clear() {
	for _, w := range g.box.Children() {
		if it := itemOf(w); it != nil {
			it.group = nil
		}
	}
	g.box.Clear()
	g.items = 0
}

// eachSlot visits the slots of kind of every item in the group.
func (g *ItemGroup) eachSlot(kind slotKind, visit func(*itemSlot)) {
	for _, w := range g.box.Children() {
		if it := itemOf(w); it != nil {
			it.eachSlot(kind, visit)
		}
	}
}

// eachShared visits the slots of kind sharing a width with the group's:
// those of every group sharing its ItemWidths, or its own.
func (g *ItemGroup) eachShared(kind slotKind, visit func(*itemSlot)) {
	if g.widths == nil {
		g.eachSlot(kind, visit)
		return
	}
	for _, o := range g.widths.groups {
		o.eachSlot(kind, visit)
	}
}

// relaySlots remeasures every slot sharing a width with the group's
// after its slots changed; nil has none.
func (g *ItemGroup) relaySlots() {
	if g == nil {
		return
	}
	for _, kind := range [...]slotKind{slotColumn, slotValue} {
		g.eachShared(kind, func(s *itemSlot) { s.InvalidateLayout() })
	}
}

// CursorName asks for no shape of its own: only part of it is clickable.
func (*Item) CursorName() string { return "" }
