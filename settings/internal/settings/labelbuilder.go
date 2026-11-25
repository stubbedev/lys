package settings

import (
	"log"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var labelSeparators = []string{" ", " · ", " | ", " – ", ": ", " / "}

type segment struct {
	token string
	text  string
}

func parseTemplate(s string, tokens, escapes []string) []segment {
	byLength := slices.Clone(tokens)
	slices.SortFunc(byLength, func(a, b string) int { return len(b) - len(a) })
	var out []segment
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			out = append(out, segment{text: literal.String()})
			literal.Reset()
		}
	}
	for i := 0; i < len(s); {
		if j := slices.IndexFunc(escapes, func(e string) bool { return strings.HasPrefix(s[i:], e) }); j >= 0 {
			literal.WriteString(escapes[j])
			i += len(escapes[j])
			continue
		}
		matched := ""
		for _, tok := range byLength {
			if tok != "" && strings.HasPrefix(s[i:], tok) {
				matched = tok
				break
			}
		}
		if matched != "" {
			flush()
			out = append(out, segment{token: matched})
			i += len(matched)
			continue
		}
		literal.WriteByte(s[i])
		i++
	}
	flush()
	return out
}

func joinSegments(segs []segment) string {
	var b strings.Builder
	for _, s := range segs {
		if s.token != "" {
			b.WriteString(s.token)
		} else {
			b.WriteString(s.text)
		}
	}
	return b.String()
}

// formatParts is a format as a row of chips, one per placeholder or
// text, with an add button at the row's end; an empty format reads as
// empty in the chips' place. after, when set, runs once the chips show
// the format anew.
type formatParts struct {
	*firstLineRow
	k      *kit
	slot   slot
	kind   formatKind
	names  map[string]string
	segs   []segment
	chips  *widget.ChipBox
	empty  *widget.Label
	add    *widget.Button
	picker *searchPicker
	after  func()
}

func newFormatParts(k *kit, s slot, kind formatKind) *formatParts {
	p := &formatParts{firstLineRow: &firstLineRow{Box: gapBox(widget.Row)}, k: k, slot: s, kind: kind, names: map[string]string{}}
	p.AddClass("label-builder-parts")
	for _, c := range kind.insert {
		p.names[c.value] = c.label
	}
	p.chips = widget.NewChipBox()
	p.chips.AddClass("label-builder-chips")
	p.chips.OnMoved = func(_ *widget.ChipBox, from, to int) {
		seg := p.segs[from]
		p.segs = slices.Insert(slices.Delete(p.segs, from, from+1), to, seg)
		p.write()
	}
	p.empty = k.note(locale.T().Get("settings-format-empty"))
	p.add = k.addButton("settings-format-insert", p.pickPart)
	p.AppendAligned(p.chips, true, widget.AlignCenter)
	p.AppendAligned(p.empty, true, widget.AlignCenter)
	p.AppendAligned(p.add, false, widget.AlignCenter)
	p.refresh()
	return p
}

// labelBuilder is a format's parts, its preview and its raw text behind
// Edit as text.
type labelBuilder struct {
	*widget.Box
	*formatParts
	// reset is the row's reset button on the parts' line, nil outside a
	// row.
	reset   *widget.Button
	preview *formatPreview
	raw     *formatEditor
	toggle  *modeToggle
}

func newLabelBuilder(k *kit, s slot, optional bool, kind formatKind) *labelBuilder {
	b := &labelBuilder{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), formatParts: newFormatParts(k, s, kind)}
	b.AddClass("label-builder")
	b.preview = k.formatPreview()
	b.raw = newFormatEditor(k, s, optional, kind)
	b.raw.SetVisible(false)
	b.toggle = k.textModeToggle(b.raw.SetVisible)
	b.formatParts.Remove(b.chips)
	b.formatParts.Remove(b.empty)
	body := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
	body.Append(b.chips, false)
	body.Append(b.empty, false)
	body.Append(b.preview, false)
	b.body, b.button, b.line = body, b.add, b.firstLine
	b.formatParts.InsertAligned(0, body, true, widget.AlignStart)
	b.formatParts.Remove(b.add)
	b.formatParts.AppendAligned(b.add, false, widget.AlignStart)
	b.Append(b.formatParts, false)
	b.AppendAligned(b.toggle, false, widget.AlignStart)
	b.Append(b.raw, false)
	b.after = b.showFormat
	b.refresh()
	return b
}

func (b *labelBuilder) stacked() {}

// firstLine is the chips' first line, the empty line while there are
// none.
func (b *labelBuilder) firstLine() widget.Widget {
	if b.chips.Visible() && b.chips.Len() > 0 {
		return b.chips.ChipAt(0)
	}
	return b.empty
}

// Measure and Arrange are the builder's column's; its parts' row lays
// itself out (firstLineRow).
func (b *labelBuilder) Measure(axis widget.Axis, forSize int) widget.Sizes {
	return b.Box.Measure(axis, forSize)
}

func (b *labelBuilder) Arrange(r render.Rect) { b.Box.Arrange(r) }

// holdReset puts the row's reset button on the parts' line, just ahead
// of their add button, where the row's control would have it, not on
// the title line above.
func (b *labelBuilder) holdReset(reset *widget.Button) {
	b.reset = reset
	b.formatParts.InsertAligned(slices.Index(b.formatParts.Children(), widget.Widget(b.add)), reset, false, widget.AlignCenter)
}

// showReset gives the hidden reset no room on the parts' line, so the
// chips wrap only where the line runs out, not where an invisible button
// would stand.
func (b *labelBuilder) showReset(on bool) {
	if b.reset != nil {
		b.reset.SetVisible(on)
	}
}

func (b *labelBuilder) dirtyBadge() *widget.Badge { return b.raw.dirtyBadge() }

func (b *formatParts) tokens() []string {
	out := make([]string, 0, len(b.kind.insert))
	for _, c := range b.kind.insert {
		out = append(out, c.value)
	}
	return out
}

func (b *formatParts) chipText(s segment) string {
	if s.token != "" {
		return b.names[s.token]
	}
	return visibleLiteral(s.text)
}

// spaceMark stands for one whitespace character of a format's text, so
// a space reads as one instead of as nothing.
const spaceMark = "␣"

// literalRun is a run of a format's text: whitespace, or the rest.
type literalRun struct {
	text  string
	space bool
}

// literalRuns splits text into its runs of whitespace and of the rest.
func literalRuns(text string) []literalRun {
	var out []literalRun
	for _, r := range text {
		space := unicode.IsSpace(r)
		if n := len(out); n > 0 && out[n-1].space == space {
			out[n-1].text += string(r)
			continue
		}
		out = append(out, literalRun{text: string(r), space: space})
	}
	return out
}

// visibleLiteral is text as its chip reads it: plain, a mark for each
// whitespace character.
func visibleLiteral(text string) string {
	var b strings.Builder
	for _, run := range literalRuns(text) {
		if run.space {
			b.WriteString(strings.Repeat(spaceMark, utf8.RuneCountInString(run.text)))
			continue
		}
		b.WriteString(run.text)
	}
	return b.String()
}

// literalChip is the chip of a format's text: the text plain, its
// whitespace marks dimmed.
func (b *formatParts) literalChip(text string) *widget.Chip {
	chip := widget.NewChip(b.k.face, visibleLiteral(text))
	chip.AddClass("label-builder-text")
	runs := literalRuns(text)
	if !slices.ContainsFunc(runs, func(r literalRun) bool { return r.space }) {
		return chip
	}
	content := widget.NewBox(widget.Row, 0, 0)
	for _, run := range runs {
		if run.space {
			content.AppendAligned(b.k.label(visibleLiteral(run.text), "label-builder-space", css.DimLabel), false, widget.AlignCenter)
			continue
		}
		content.AppendAligned(b.k.label(run.text), false, widget.AlignCenter)
	}
	chip.SetContent(content)
	return chip
}

func (b *formatParts) refresh() {
	b.segs = parseTemplate(b.slot.text(), b.tokens(), b.kind.escapes)
	b.chips.Clear()
	for i, s := range b.segs {
		chip := widget.NewChip(b.k.face, b.chipText(s))
		if s.token == "" {
			chip = b.literalChip(s.text)
		}
		chip.SetDismiss(chipRemoveName(locale.T()), func() { b.removeSegment(i) })
		b.chips.Append(chip)
	}
	b.chips.SetVisible(len(b.segs) > 0)
	b.empty.SetVisible(len(b.segs) == 0)
	if b.after != nil {
		b.after()
	}
}

// showFormat shows the format the parts hold: its preview, unless it is
// empty, as the parts then say, and its raw text.
func (b *labelBuilder) showFormat() {
	b.preview.SetVisible(len(b.segs) > 0)
	b.preview.show(b.kind.render(b.slot.text()))
	b.raw.refresh()
}

// removeSegment takes segment i out of the label, its Undo putting the
// label back as it was while nothing changed it since: the text around a
// removed part merges, so only the whole label says where it stood.
func (b *formatParts) removeSegment(i int) {
	before, name := b.slot.text(), b.chipText(b.segs[i])
	b.segs = slices.Delete(b.segs, i, i+1)
	b.write()
	after := b.slot.text()
	b.k.offerUndo(name, func() {
		if now := b.slot.text(); now != after {
			log.Printf("settings: undo removing %s from the label: it changed since, to %q", name, now)
			return
		}
		_ = b.slot.set(before)
		b.refresh()
	})
}

func (b *formatParts) write() {
	_ = b.slot.set(joinSegments(b.segs))
	b.refresh()
}

func (b *formatParts) options() []choice {
	t := locale.T()
	out := slices.Clone(b.kind.insert)
	for _, sep := range b.kind.separators {
		out = append(out, choice{value: sep, label: visibleLiteral(sep), group: t.Get("settings-format-separators")})
	}
	for _, p := range b.kind.presets {
		out = append(out, choice{value: p, label: b.kind.label(p), group: t.Get("settings-format-whole")})
	}
	return out
}

func (b *formatParts) pickPart() {
	if b.k.pickers == nil {
		return
	}
	b.picker = openChoicePicker(b.k, b.add, b.options(), func(v string) {
		if slices.Contains(b.kind.presets, v) {
			_ = b.slot.set(v)
			b.refresh()
			return
		}
		if _, isToken := b.names[v]; isToken {
			b.segs = append(b.segs, segment{token: v})
		} else {
			b.segs = append(b.segs, segment{text: v})
		}
		b.write()
	})
}

// firstLineRow is a row of chips beside a button (the add, a reset
// before it): the body holding the chips, any line under them included,
// sits lowered so its first line centres on the button, every other
// child at the row's top, however many lines the chips take; the line
// under them so stands one gap under their last line. Without a body it
// is a plain row.
type firstLineRow struct {
	*widget.Box
	body   widget.Widget
	line   func() widget.Widget
	button widget.Widget
}

// lift is how far the body sits under the row's top.
func (r *firstLineRow) lift() int {
	if r.body == nil {
		return 0
	}
	return max(0, (r.button.Measure(widget.Column, -1).Nat-r.line().Measure(widget.Column, -1).Nat)/2)
}

// Measure is the row's request, its body lowered by lift.
func (r *firstLineRow) Measure(axis widget.Axis, forSize int) widget.Sizes {
	s := r.Box.Measure(axis, forSize)
	if axis != widget.Column || r.body == nil {
		return s
	}
	lowered := r.body.Measure(widget.Column, forSize).Nat + r.lift()
	s.Nat, s.Min = max(s.Nat, lowered), max(s.Min, lowered)
	return s
}

// Arrange lays the row out in rect, every child at its top but the
// body, lowered by lift.
func (r *firstLineRow) Arrange(rect render.Rect) {
	r.Box.Arrange(rect)
	if r.body == nil {
		return
	}
	lift := r.lift()
	for _, w := range r.Children() {
		placed, ok := w.(widget.Boundser)
		if !ok {
			log.Panicf("settings: a %T on a chips' row has no bounds to place", w)
		}
		b := placed.Bounds()
		y := rect.Y
		if w == r.body {
			y += lift
		}
		w.Arrange(render.Rect{X: b.X, Y: y, W: b.W, H: b.H})
	}
}
