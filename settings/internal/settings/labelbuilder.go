package settings

import (
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/ui/widget"

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
	*widget.Box
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
	p := &formatParts{Box: gapBox(widget.Row), k: k, slot: s, kind: kind, names: map[string]string{}}
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
	b.Append(b.formatParts, false)
	b.Append(b.preview, false)
	b.AppendAligned(b.toggle, false, widget.AlignStart)
	b.Append(b.raw, false)
	b.after = b.showFormat
	b.refresh()
	return b
}

func (b *labelBuilder) stacked() {}

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
	return "“" + s.text + "”"
}

func (b *formatParts) refresh() {
	b.segs = parseTemplate(b.slot.text(), b.tokens(), b.kind.escapes)
	b.chips.Clear()
	for i, s := range b.segs {
		chip := widget.NewChip(b.k.face, b.chipText(s))
		if s.token == "" {
			chip.AddClass("label-builder-text")
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
		out = append(out, choice{value: sep, label: "“" + sep + "”", group: t.Get("settings-format-separators")})
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
