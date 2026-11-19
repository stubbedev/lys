package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestATemplateSplitsIntoPlaceholdersAndTextAndJoinsBack(t *testing.T) {
	tokens := []string{"{percent}", "{time}", "{p}"}
	for _, tmpl := range []string{"{percent}% · {time}", "Bat {percent}", "", "{percent}{time}", "{p}{percent}", "plain text"} {
		if got := joinSegments(parseTemplate(tmpl, tokens)); got != tmpl {
			t.Errorf("%q came back as %q", tmpl, got)
		}
	}
	segs := parseTemplate("{percent}% · {time}", tokens)
	want := []segment{{token: "{percent}"}, {text: "% · "}, {token: "{time}"}}
	if !slices.Equal(segs, want) {
		t.Errorf("segments %+v, want %+v", segs, want)
	}
	if segs := parseTemplate("{percent}", []string{"{p}", "{percent}"}); len(segs) != 1 || segs[0].token != "{percent}" {
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
