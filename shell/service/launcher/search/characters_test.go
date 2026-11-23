package search

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/launchermatch"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/launcher/charnames"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
)

func countingTable(n *atomic.Int32) func() (*charnames.Table, error) {
	return func() (*charnames.Table, error) {
		n.Add(1)
		return charnames.Shared()
	}
}

func TestCharactersReadTheTableOnTheFirstSearchOnly(t *testing.T) {
	var reads atomic.Int32
	c := NewCharacters(countingTable(&reads))
	c.Load(ctx, func([]Entry) { t.Error("characters published rows") })
	if reads.Load() != 0 {
		t.Fatal("opening the panel read the table")
	}
	for _, q := range []string{"right arrow", "degree"} {
		if got := c.Search(launchermatch.NewFuzzyQuery(q), q, 4); len(got) == 0 {
			t.Errorf("%q found nothing", q)
		}
	}
	if reads.Load() != 1 {
		t.Errorf("table read %d times, want once", reads.Load())
	}
}

func TestCharactersInTheMainListFromThreeLetters(t *testing.T) {
	s := &sink{}
	e := start(t, s, nil, NewCharacters(charnames.Shared), &fakeProvider{spec: Spec{Section: SectionApps}, entries: []Entry{entry("Firefox")}})
	e.SetQuery("right arrow", 1)
	waittest.Until(t, func() bool { return slices.Contains(titles(s.last()), "Rightwards Arrow") }, "the arrow")
	got := s.last()
	if got.Rows[0].Header != SectionCharacters.Title() || len(got.Rows) > 1+SectionCharacters.rowCap() {
		t.Errorf("rows %q", titles(got))
	}
	for _, r := range got.Rows[1:] {
		if r.Entry.Title == "Rightwards Arrow" && (r.Entry.Glyph != "→" || r.Entry.Subtitle != "U+2192") {
			t.Errorf("row %+v", *r.Entry)
		}
	}
	e.SetQuery("degree", 2)
	waittest.Until(t, func() bool {
		rows := s.last().Rows
		return len(rows) > 1 && rows[1].Entry != nil && rows[1].Entry.Glyph == "°"
	}, "the degree sign")

	e.SetQuery("de", 3)
	waittest.Until(t, func() bool { return s.last().Query == "de" }, "two letters")
	if got := titles(s.last()); slices.Contains(got, "#"+SectionCharacters.Title()) {
		t.Errorf("two letters listed characters: %q", got)
	}
}

func TestTheCharacterLevelListsWithAnEmptyQuery(t *testing.T) {
	s := &sink{}
	e := startLevel(t, s, CharactersLevel(charnames.Shared), nil)
	e.SetQuery("x", 1)
	e.SetQuery("", 2)
	waittest.Until(t, func() bool { return len(s.last().Rows) == flatRowCap && s.last().Query == "" }, "every character")
	if got := s.last().Rows[0].Entry; got.Subtitle != "U+0000" {
		t.Errorf("first row %+v", *got)
	}
	e.SetQuery("x", 3)
	waittest.Until(t, func() bool { return s.last().Query == "x" && len(s.last().Rows) > 0 }, "one letter")
}

func TestCharacterActionsCopyTheCodePointAndEntity(t *testing.T) {
	c := NewCharacters(charnames.Shared)
	found := c.Search(launchermatch.NewFuzzyQuery("U+2192"), "U+2192", 4)
	if len(found) != 1 {
		t.Fatalf("found %d", len(found))
	}
	var got []Outcome
	for _, a := range c.Actions(found[0].Entry) {
		got = append(got, a.Run(ctx))
	}
	want := []Outcome{Type{Text: "→"}, Copy{Text: "→"}, Copy{Text: "U+2192"}, Copy{Text: "&#x2192;"}, Copy{Text: "Rightwards Arrow"}}
	if !slices.Equal(got, want) {
		t.Errorf("actions %v, want %v", got, want)
	}
	if c.Actions(Entry{Data: "x"}) != nil {
		t.Error("a foreign row has actions")
	}
}

func TestACharacterSearchBuildsOnlyTheRowsItShows(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector allocates")
	}
	c := NewCharacters(charnames.Shared)
	cost := func(query string) float64 {
		q := launchermatch.NewFuzzyQuery(query)
		if got := c.Search(q, query, 1); len(got) != 1 {
			t.Fatalf("%q found %d", query, len(got))
		}
		return testing.AllocsPerRun(10, func() { c.Search(q, query, 1) })
	}
	many, one := cost("letter"), cost("interrobang")
	if many != one {
		t.Errorf("one row costs %v allocations over thousands of matches, %v over one: the search allocates per character", many, one)
	}
}

func TestToneableEmojiOfferEveryToneAndRememberThePick(t *testing.T) {
	table := func() []modes.Emoji { return modes.AvailableEmoji() }
	var kept []modes.SkinTone
	p := NewEmoji(table, modes.ToneMedium, func(tone modes.SkinTone, saved func(error)) {
		kept = append(kept, tone)
		saved(nil)
	})
	var thumbs, heart Entry
	for _, e := range loaded(t, p) {
		switch e.Title {
		case "thumbs up":
			thumbs = e
		case "red heart":
			heart = e
		}
	}
	if thumbs.Glyph != "👍🏽" {
		t.Fatalf("thumbs up shows %q with skin-tone medium", thumbs.Glyph)
	}
	actions := p.Actions(thumbs)
	if len(actions) != 3+len(modes.SkinTones) {
		t.Fatalf("%d actions", len(actions))
	}
	if got := actions[0].Run(ctx); got != (Type{Text: "👍🏽"}) {
		t.Errorf("the default action %v", got)
	}
	var variants []string
	for _, a := range actions[3:] {
		out, _ := a.Run(ctx).(Type)
		variants = append(variants, out.Text)
	}
	if !slices.Equal(variants, []string{"👍🏻", "👍🏼", "👍🏽", "👍🏾", "👍🏿"}) {
		t.Errorf("variants %q", variants)
	}
	if !slices.Equal(kept, modes.SkinTones[:]) {
		t.Errorf("kept tones %v", kept)
	}
	if len(p.Actions(heart)) != 3 || heart.Glyph != "❤️" {
		t.Errorf("red heart: %q, %d actions", heart.Glyph, len(p.Actions(heart)))
	}
}

func thumbsUp(t *testing.T, p *Emoji) Entry {
	t.Helper()
	for _, e := range loaded(t, p) {
		if e.Title == "thumbs up" {
			return e
		}
	}
	t.Fatal("no thumbs up")
	return Entry{}
}

func TestAToneThatIsNotSavedTypesNothingAndSaysWhy(t *testing.T) {
	refused := errors.New("config: save: read-only file system")
	p := NewEmoji(modes.AvailableEmoji, modes.ToneNone, func(_ modes.SkinTone, saved func(error)) { saved(refused) })
	actions := p.Actions(thumbsUp(t, p))
	if f, ok := actions[3].Run(ctx).(Failed); !ok || f.Summary != i18n.T("launcher-skin-tone-not-saved") || !errors.Is(f.Err, refused) {
		t.Errorf("a refused tone ran %#v, want it said instead of typed, the error apart", actions[3].Run(ctx))
	}
	if got := actions[0].Run(ctx); got != (Type{Text: "👍"}) {
		t.Errorf("the default action %v after a refused tone", got)
	}
}

func TestAToneSavingWhenTheSessionEndsTypesNothing(t *testing.T) {
	p := NewEmoji(modes.AvailableEmoji, modes.ToneNone, func(modes.SkinTone, func(error)) {})
	actions := p.Actions(thumbsUp(t, p))
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if got := actions[3].Run(ended); got != nil {
		t.Errorf("a tone still saving as the session ended ran %#v", got)
	}
}

func TestEverySkinToneOfTheConfigMapsBothWays(t *testing.T) {
	for _, name := range []config.LauncherSkinTone{
		config.SkinToneNone, config.SkinToneLight, config.SkinToneMediumLight,
		config.SkinToneMedium, config.SkinToneMediumDark, config.SkinToneDark,
	} {
		if got := SkinToneName(SkinToneOf(name)); got != name {
			t.Errorf("%s came back as %s", name, got)
		}
	}
	if SkinToneOf(config.SkinToneMedium) != modes.ToneMedium || SkinToneOf(config.SkinToneNone) != modes.ToneNone {
		t.Error("tones map out of order")
	}
	defer func() {
		if recover() == nil {
			t.Error("an unknown tone mapped")
		}
	}()
	SkinToneOf("purple")
}
