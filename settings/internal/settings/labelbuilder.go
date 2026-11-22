package settings

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
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

type labelBuilder struct {
	*widget.Box
	k       *kit
	slot    slot
	kind    formatKind
	names   map[string]string
	segs    []segment
	chips   *widget.ChipBox
	add     *widget.Button
	preview *widget.Label
	raw     *formatEditor
	toggle  *widget.Button
	toggled *widget.Label
	picker  *searchPicker
	editing bool
}

func newLabelBuilder(k *kit, s slot, optional bool, kind formatKind) *labelBuilder {
	t := locale.T()
	b := &labelBuilder{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s, kind: kind, names: map[string]string{}}
	b.AddClass("label-builder")
	for _, c := range kind.insert {
		b.names[c.value] = c.label
	}
	b.chips = widget.NewChipBox()
	b.chips.AddClass("label-builder-chips")
	b.chips.OnMoved = func(_ *widget.ChipBox, from, to int) {
		seg := b.segs[from]
		b.segs = slices.Insert(slices.Delete(b.segs, from, from+1), to, seg)
		b.write()
	}
	b.add = k.addButton("settings-format-insert", b.pickPart)
	line := gapBox(widget.Row)
	line.Append(b.chips, true)
	line.AppendAligned(b.add, false, widget.AlignStart)
	b.preview = k.label("", "format-preview", css.Caption, css.DimLabel)
	b.preview.SetEllipsize(widget.EllipsizeEnd)
	b.raw = newFormatEditor(k, s, optional, kind)
	b.raw.SetVisible(false)
	b.toggled = k.label(t.Get("settings-format-edit-text"))
	b.toggle = k.roleButton(roleQuiet, b.toggled, func() {
		b.editing = !b.editing
		b.raw.SetVisible(b.editing)
		b.refresh()
	})
	b.Append(line, false)
	b.Append(b.preview, false)
	b.AppendAligned(b.toggle, false, widget.AlignStart)
	b.Append(b.raw, false)
	b.refresh()
	return b
}

func (b *labelBuilder) stacked() {}

func (b *labelBuilder) dirtyBadge() *widget.Badge { return b.raw.dirtyBadge() }

func (b *labelBuilder) tokens() []string {
	out := make([]string, 0, len(b.kind.insert))
	for _, c := range b.kind.insert {
		out = append(out, c.value)
	}
	return out
}

func (b *labelBuilder) chipText(s segment) string {
	if s.token != "" {
		return b.names[s.token]
	}
	return "“" + s.text + "”"
}

func (b *labelBuilder) refresh() {
	b.segs = parseTemplate(b.slot.text(), b.tokens(), b.kind.escapes)
	b.chips.Clear()
	for i, s := range b.segs {
		chip := widget.NewChip(b.k.face, b.chipText(s))
		if s.token == "" {
			chip.AddClass("label-builder-text")
		}
		chip.SetDismiss(chipRemoveName(locale.T()), func() {
			b.segs = slices.Delete(b.segs, i, i+1)
			b.write()
		})
		b.chips.Append(chip)
	}
	b.preview.SetText(locale.T().Get("settings-format-preview", i18n.Str("text", b.kind.render(b.slot.text()))))
	key := "settings-format-edit-text"
	if b.editing {
		key = "settings-format-edit-parts"
	}
	b.toggled.SetText(locale.T().Get(key))
	b.raw.refresh()
}

func (b *labelBuilder) write() {
	_ = b.slot.set(joinSegments(b.segs))
	b.refresh()
}

func (b *labelBuilder) options() []choice {
	t := locale.T()
	out := slices.Clone(b.kind.insert)
	for _, sep := range b.kind.separators {
		out = append(out, choice{value: sep, label: "“" + sep + "”", group: t.Get("settings-format-separators")})
	}
	for _, p := range b.kind.presets {
		out = append(out, choice{value: p, label: b.kind.render(p), group: t.Get("settings-format-whole")})
	}
	return out
}

func (b *labelBuilder) pickPart() {
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
