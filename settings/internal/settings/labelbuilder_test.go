package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"

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
