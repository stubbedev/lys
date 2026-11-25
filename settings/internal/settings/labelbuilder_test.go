package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestAChipsRemoveButtonIsNamedInEveryLanguage(t *testing.T) {
	available, err := locale.Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, lang := range available {
		tr, err := i18n.Select(locale.Assets(), []i18n.LangID{lang})
		if err != nil {
			t.Fatal(err)
		}
		if !tr.Has("settings-chip-remove") {
			t.Errorf("%s: no settings-chip-remove", lang)
		}
		name := chipRemoveName(tr)("Uhr")
		if !strings.Contains(name, "Uhr") || strings.Contains(name, "No localization") {
			t.Errorf("%s: the remove button reads %q, which does not name the chip", lang, name)
		}
		names[lang.String()] = name
	}
	if got, want := unisolated(names["de"]), "Uhr entfernen"; got != want {
		t.Errorf("de: the remove button reads %q, want %q", got, want)
	}
	for lang, name := range names {
		if lang != "en-US" && name == names["en-US"] {
			t.Errorf("%s: the remove button reads the English %q", lang, name)
		}
	}
}

func TestATemplateSplitsIntoPlaceholdersAndTextAndJoinsBack(t *testing.T) {
	tokens := []string{"{percent}", "{time}", "{p}"}
	for _, tmpl := range []string{"{percent}% · {time}", "Bat {percent}", "", "{percent}{time}", "{p}{percent}", "plain text"} {
		if got := joinSegments(parseTemplate(tmpl, tokens, nil)); got != tmpl {
			t.Errorf("%q came back as %q", tmpl, got)
		}
	}
	segs := parseTemplate("{percent}% · {time}", tokens, nil)
	want := []segment{{token: "{percent}"}, {text: "% · "}, {token: "{time}"}}
	if !slices.Equal(segs, want) {
		t.Errorf("segments %+v, want %+v", segs, want)
	}
	if segs := parseTemplate("{percent}", []string{"{p}", "{percent}"}, nil); len(segs) != 1 || segs[0].token != "{percent}" {
		t.Errorf("a longer token was cut by a shorter one: %+v", segs)
	}
}

func TestTheLabelBuilderShowsChipsAndBuildsTheTemplate(t *testing.T) {
	const path = "modules.battery.format"
	ps := config.Placeholders(path)
	k := testKit(t, "modules:\n  battery:\n    format: \""+ps[0].Token+" x\"\n")
	b := shellControl(t, k, path).(*labelBuilder)
	if b.chips.Len() != 2 {
		t.Fatalf("%d chips for a placeholder and a text", b.chips.Len())
	}
	fp := &fakePickers{}
	k.pickers = fp
	b.pickPart()
	opts := b.options()
	sep := opts[len(opts)-1].value
	b.segs = append(b.segs, segment{text: sep})
	b.write()
	if v := k.store.value(path); v != ps[0].Token+" x"+sep {
		t.Fatalf("adding a separator stored %v", v)
	}
	if b.chips.Len() != 2 {
		t.Errorf("text next to text reads back as %d chips, want one text chip", b.chips.Len())
	}
	b.segs = slices.Delete(b.segs, 1, 2)
	b.write()
	if v := k.store.value(path); v != ps[0].Token {
		t.Errorf("removing the text stored %v", v)
	}
	if b.raw.Visible() {
		t.Error("the raw template shows before Edit as text was chosen")
	}
	b.toggle.OnClick()
	if !b.raw.Visible() {
		t.Error("Edit as text did not show the raw template")
	}
}

func TestATimeFormatIsBuiltFromNamedParts(t *testing.T) {
	const path = "modules.clock.format"
	k := testKit(t, "modules:\n  clock:\n    format: \"%H:%M\"\n")
	b := shellControl(t, k, path).(*labelBuilder)
	want := []segment{{token: "%H"}, {text: ":"}, {token: "%M"}}
	if !slices.Equal(b.segs, want) {
		t.Fatalf("%%H:%%M reads as %+v, want %+v", b.segs, want)
	}
	if got := b.chipText(b.segs[0]); got == "%H" || got == "" {
		t.Errorf("the hour chip reads %q, not a name", got)
	}
	var whole, parts int
	for _, o := range b.options() {
		switch {
		case slices.Contains(strftimePresets, o.value):
			whole++
		case o.value == "%-d":
			parts++
		}
	}
	if whole != len(strftimePresets) || parts != 1 {
		t.Errorf("the picker offers %d whole formats and %d day-without-zero parts", whole, parts)
	}
	b.segs = slices.Insert(b.segs, 0, segment{token: "%-d"}, segment{text: " "})
	b.write()
	if v := k.store.value(path); v != "%-d %H:%M" {
		t.Errorf("adding a day stored %v", v)
	}
	for _, s := range parseTemplate("%%H", b.tokens(), b.kind.escapes) {
		if s.token == "%H" {
			t.Error("an escaped percent was read as the hour")
		}
	}
}

func TestATemplatedCommandIsBuiltFromItsPlaceholders(t *testing.T) {
	const path = "launcher.ssh.command"
	k := testKit(t, "")
	b := shellControl(t, k, path).(*labelBuilder)
	tokens := b.tokens()
	for _, want := range []string{"{terminal}", "{ssh-client}", "{host}"} {
		if !slices.Contains(tokens, want) {
			t.Errorf("the connect command offers %q, not %s", tokens, want)
		}
	}
	if !strings.Contains(b.preview.Text(), "example.org") {
		t.Errorf("preview %q", b.preview.Text())
	}
	if _, ok := shellControl(t, k, "modules.power.lock-command").(*choiceEditor); !ok {
		t.Error("a command without placeholders left the command picker")
	}
}

// styledLabelBuilder is path's label builder over configYAML, laid out
// by relayOut, which it has run once.
func styledLabelBuilder(t *testing.T, path, configYAML string) (k *kit, b *labelBuilder, relayOut func()) {
	t.Helper()
	k = testKit(t, configYAML)
	b = shellControl(t, k, path).(*labelBuilder)
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(b, false)
	relayOut = func() { styledLayOut(k, root, 700) }
	relayOut()
	return k, b, relayOut
}

func centreY(r render.Rect) int { return r.Y + r.H/2 }

// The add button is centred on the chip row it ends, not on the chips,
// preview and toggle below it.
func TestAFormatsAddButtonCentresOnItsChipRow(t *testing.T) {
	const path = "modules.ram.format"
	ps := config.Placeholders(path)
	_, b, _ := styledLabelBuilder(t, path, "modules:\n  ram:\n    format: \""+ps[0].Token+"%\"\n")
	if b.chips.Len() == 0 {
		t.Fatal("no chips")
	}
	if chips, add := centreY(b.chips.ChipAt(0).Bounds()), centreY(b.add.Bounds()); chips != add {
		t.Errorf("the chips centre at y %d, the add button at %d", chips, add)
	}
}

// A format emptied of its parts says it is empty in the chips' place,
// in a row no taller than its add button, and shows no bare preview.
func TestAnEmptyFormatReadsAsEmptyWithoutABarePreview(t *testing.T) {
	const path = "modules.ram.format"
	ps := config.Placeholders(path)
	k, b, relayOut := styledLabelBuilder(t, path, "modules:\n  ram:\n    format: \""+ps[0].Token+"\"\n")
	if !b.chips.Visible() || b.empty.Visible() || !b.preview.Visible() {
		t.Fatalf("with a part: chips %v, empty %v, preview %v", b.chips.Visible(), b.empty.Visible(), b.preview.Visible())
	}
	b.removeSegment(0)
	if v := k.store.value(path); v != "" {
		t.Fatalf("removing the only part stored %v", v)
	}
	if b.chips.Visible() || !b.empty.Visible() || b.preview.Visible() {
		t.Fatalf("emptied: chips %v, empty %v, preview %v", b.chips.Visible(), b.empty.Visible(), b.preview.Visible())
	}
	if got, want := b.empty.Text(), locale.T().Get("settings-format-empty"); got != want {
		t.Errorf("the empty format reads %q, want %q", got, want)
	}
	relayOut()
	if row, add := b.formatParts.Bounds().H, b.add.Bounds().H; row != add {
		t.Errorf("the emptied parts row is %dpx tall, its add button %dpx", row, add)
	}
	if centreY(b.empty.Bounds()) != centreY(b.add.Bounds()) {
		t.Errorf("the empty line centres at y %d, the add button at %d", centreY(b.empty.Bounds()), centreY(b.add.Bounds()))
	}
}

// A format's text reads plainly on its chip, each whitespace character
// a dimmed mark, so a space never reads as an empty pair of quotes; the
// picker offers separators the same way.
func TestAFormatsTextChipsReadPlainWithTheirSpacesMarked(t *testing.T) {
	k := testKit(t, "modules:\n  clock:\n    format: \"%H:%M - %S\"\n")
	b := shellControl(t, k, "modules.clock.format").(*labelBuilder)
	var texts []string
	for _, chip := range b.chips.Chips() {
		if chip.HasClass("label-builder-text") {
			texts = append(texts, chip.Text())
		}
	}
	if want := []string{":", spaceMark + "-" + spaceMark}; !slices.Equal(texts, want) {
		t.Fatalf("the text chips read %q, want %q", texts, want)
	}
	dash := b.chips.ChipAt(3)
	marks := treetest.WithClass(dash, "label-builder-space")
	if len(marks) != 2 {
		t.Fatalf("the dash's chip marks %d spaces", len(marks))
	}
	for _, m := range marks {
		if !widget.HasClass(m, css.DimLabel) {
			t.Error("a space mark is not dimmed")
		}
	}
	if marks := treetest.WithClass(b.chips.ChipAt(1), "label-builder-space"); len(marks) != 0 {
		t.Errorf("the colon's chip marks %d spaces", len(marks))
	}
	for _, o := range b.options() {
		if strings.ContainsAny(o.label, "“”") {
			t.Errorf("the picker offers %q in quotes", o.label)
		}
	}
}

// A format row's reset sits on its parts' line just ahead of their add
// button, not on the title line above it; hidden, it takes no room
// there, so the chips wrap only where the line runs out.
func TestAFormatRowsResetSitsBesideItsAdd(t *testing.T) {
	k := testKit(t, "")
	r := newFieldRow(k, k.store, field("modules.clock.format"))
	if err := k.store.set("modules.clock.format", "%H"); err != nil {
		t.Fatal(err)
	}
	r.refresh()
	if !r.reset.shown() {
		t.Fatal("an overridden format shows no reset")
	}
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(r, false)
	styledLayOut(k, root, 900)
	b := r.ctl.(*labelBuilder)
	kids := b.formatParts.Children()
	at := slices.Index(kids, widget.Widget(b.add))
	if at < 1 || kids[at-1] != widget.Widget(r.reset.Button) {
		t.Fatal("the reset is not just ahead of the parts' add button")
	}
	reset, add := r.reset.Bounds(), b.add.Bounds()
	if reset.Y+reset.H/2 != add.Y+add.H/2 {
		t.Errorf("the reset's middle is at y=%d, the add's at %d", reset.Y+reset.H/2, add.Y+add.H/2)
	}
	k.store.unset("modules.clock.format")
	r.refresh()
	styledLayOut(k, root, 900)
	if body := b.body.(widget.Boundser).Bounds(); r.reset.shown() || body.X+body.W+widget.Current().Spacing != b.add.Bounds().X {
		t.Errorf("with the reset hidden the parts end at %d, the add starts at %d", body.X+body.W, b.add.Bounds().X)
	}
}

// The chips stood 17px over the preview on one line but 10px once they
// wrapped, the add button's height pushing the one-line row taller. The
// preview stands one gap under the chips' last line however many lines
// they take, the first line still centred on the add button.
func TestAFormatsPreviewStandsOneGapUnderItsChips(t *testing.T) {
	const path = "modules.ram.format"
	ps := config.Placeholders(path)
	gap := func(format string) (int, int) {
		_, b, _ := styledLabelBuilder(t, path, "modules:\n  ram:\n    format: \""+format+"\"\n")
		chips, preview := b.chips.Bounds(), b.preview.Bounds()
		return preview.Y - (chips.Y + chips.H), len(chipLines(b))
	}
	one, lines := gap(ps[0].Token)
	if lines != 1 {
		t.Fatalf("one part lies on %d lines", lines)
	}
	long := strings.Repeat(ps[0].Token+" — ", 12)
	wrapped, lines := gap(long)
	if lines < 2 {
		t.Fatalf("twelve parts lie on %d line", lines)
	}
	if one != wrapped {
		t.Errorf("the preview stands %dpx under one line of chips and %dpx under %d", one, wrapped, lines)
	}
}

// chipLines are the distinct tops of b's chips, one a line.
func chipLines(b *labelBuilder) []int {
	var tops []int
	for _, c := range b.chips.Chips() {
		if y := c.Bounds().Y; !slices.Contains(tops, y) {
			tops = append(tops, y)
		}
	}
	return tops
}
