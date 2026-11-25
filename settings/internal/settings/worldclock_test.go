package settings

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestAWorldClockFormatReadsAsClocksAndWritesBack(t *testing.T) {
	for _, s := range []string{
		"{{ tz('UTC', '%H:%M %Z') }}",
		"NYC {{ tz('America/New_York', '%H:%M') }}  TYO {{ tz('Asia/Tokyo', '%H:%M') }}",
		`{{ tz("Europe/London", "%H o'clock") }} !`,
		"",
	} {
		w, ok := parseWorldClocks(s)
		if !ok {
			t.Fatalf("%q does not read as clocks", s)
		}
		back, err := w.String()
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		again, _ := parseWorldClocks(back)
		if !slices.Equal(again.clocks, w.clocks) || again.after != w.after {
			t.Errorf("%q came back as %q", s, back)
		}
	}
	w, _ := parseWorldClocks("NYC {{ tz('America/New_York', '%H:%M') }}  TYO {{tz(\"Asia/Tokyo\",'%H')}}")
	want := []zoneClock{{"NYC ", "America/New_York", "%H:%M"}, {"  TYO ", "Asia/Tokyo", "%H"}}
	if !slices.Equal(w.clocks, want) {
		t.Errorf("clocks %+v, want %+v", w.clocks, want)
	}
	for _, s := range []string{"{{ tz('UTC', '%H') }} {{ now }}", "{% if x %}{{ tz('UTC', '%H') }}{% endif %}"} {
		if _, ok := parseWorldClocks(s); ok {
			t.Errorf("%q, a template with more than clocks, read as clocks", s)
		}
	}
	if _, err := (worldClocks{clocks: []zoneClock{{zone: "UTC", layout: `it's "now"`}}}).String(); err == nil {
		t.Error("a format holding both quotes was written")
	}
}

func TestAWorldClockRendersEachZone(t *testing.T) {
	at := time.Date(2026, time.March, 14, 15, 9, 0, 0, time.UTC)
	w, _ := parseWorldClocks("UTC {{ tz('UTC', '%H:%M') }} TYO {{ tz('Asia/Tokyo', '%H:%M') }}")
	if got, err := w.render(at); err != nil || got != "UTC 15:09 TYO 00:09" {
		t.Fatalf("%q, %v", got, err)
	}
	w, _ = parseWorldClocks("{{ tz('Nowhere/Atlantis', '%H') }}")
	if _, err := w.render(at); err == nil || !strings.Contains(err.Error(), "Nowhere/Atlantis") {
		t.Errorf("an unknown zone rendered: %v", err)
	}
}

func TestTheTimeZonesAreTheSystemTable(t *testing.T) {
	h := fakeHost(t, nil)
	put(t, h.path("usr/share/zoneinfo/zone1970.tab"), "# comment\nJP\t+353916+1394441\tAsia/Tokyo\nUS\t+404251-0740023\tAmerica/New_York\tEastern\n")
	if got := values(h.timeZones()); !slices.Equal(got, []string{"Local", "UTC", "America/New_York", "Asia/Tokyo"}) {
		t.Fatalf("%q", got)
	}
	if got := values(fakeHost(t, nil).timeZones()); !slices.Equal(got, []string{"Local", "UTC"}) {
		t.Fatalf("without a table: %q", got)
	}
}

func TestTheWorldClockIsEditedAsCards(t *testing.T) {
	k := testKit(t, "modules:\n  world-clock:\n    format: \"NYC {{ tz('America/New_York', '%H:%M') }}\"\n")
	e := fieldControl(k, k.store, field(worldClockPath, worldClockRow)).(*worldClockEditor)
	if len(e.cards.cards) != 1 || e.notice.Visible() || e.raw.Visible() {
		t.Fatalf("%d cards, notice %v, raw %v", len(e.cards.cards), e.notice.Visible(), e.raw.Visible())
	}
	e.cards.local = append(e.cards.local, e.cards.spec.blank())
	e.cards.commit()
	if v := k.store.value(worldClockPath); v != "NYC {{ tz('America/New_York', '%H:%M') }} {{ tz('UTC', '%H:%M') }}" {
		t.Fatalf("adding a clock stored %v", v)
	}
	if want := formatSampleTime.UTC().Format("15:04"); !strings.Contains(e.preview.Text(), "NYC ") || !strings.Contains(e.preview.Text(), want) {
		t.Errorf("preview %q", e.preview.Text())
	}
	if err := k.store.set(worldClockPath, "{{ tz('UTC', '%H') }}{% if true %}!{% endif %}"); err != nil {
		t.Fatal(err)
	}
	e.refresh()
	if !e.notice.Visible() || e.cards.Visible() || !e.raw.Visible() {
		t.Errorf("a template with more than clocks shows notice %v, cards %v, raw %v", e.notice.Visible(), e.cards.Visible(), e.raw.Visible())
	}
}

// A clock's card holds its time as parts alone, the zone's abbreviation
// among them by name: the one preview and Edit as text are the whole
// format's, under the cards, and the add button ends the cards in the
// control column, as a format editor's ends its parts.
func TestAWorldClockCardHoldsItsTimeAsNamedPartsAlone(t *testing.T) {
	k := testKit(t, "modules:\n  world-clock:\n    format: \"{{ tz('UTC', '%H:%M %Z') }}\"\n")
	e := fieldControl(k, k.store, field(worldClockPath, worldClockRow)).(*worldClockEditor)
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(e, false)
	styledLayOut(k, root, 900)
	var previews, toggles int
	walkOwn(e, func(w widget.Widget) {
		if !shown(w) {
			return
		}
		switch w.(type) {
		case *formatPreview:
			previews++
		case *modeToggle:
			toggles++
		}
	})
	if previews != 1 || toggles != 1 {
		t.Errorf("the world clock shows %d previews and %d Edit as text toggles, want one of each", previews, toggles)
	}
	parts, ok := e.cards.cards[0].controls[2].(*formatParts)
	if !ok {
		t.Fatalf("the time format is a %T, not parts", e.cards.cards[0].controls[2])
	}
	want := []segment{{token: "%H"}, {text: ":"}, {token: "%M"}, {text: " "}, {token: "%Z"}}
	if !slices.Equal(parts.segs, want) {
		t.Fatalf("%%H:%%M %%Z reads as %+v, want %+v", parts.segs, want)
	}
	if got := parts.chipText(parts.segs[4]); got != locale.T().Get("settings-world-clock-zone-name") {
		t.Errorf("the zone abbreviation reads %q", got)
	}
	add := e.cards.end.add
	if card := e.cards.cards[0].root.Bounds(); add.Bounds().X != card.X {
		t.Errorf("the add button starts at x %d, the card at %d", add.Bounds().X, card.X)
	}
}

// An emptied World Clock says its list is empty, as an emptied format
// says its parts are, and shows no preview of nothing.
func TestAnEmptiedWorldClockSaysSoAndPreviewsNothing(t *testing.T) {
	k := testKit(t, "modules:\n  world-clock:\n    format: \"{{ tz('UTC', '%H:%M') }}\"\n")
	e := fieldControl(k, k.store, field(worldClockPath, worldClockRow)).(*worldClockEditor)
	if !e.preview.Visible() || e.cards.end.empty.Visible() {
		t.Fatalf("with a clock the preview shows %v and the empty line %v", e.preview.Visible(), e.cards.end.empty.Visible())
	}
	e.cards.removeItem(0)
	if e.preview.Visible() || !e.cards.end.empty.Visible() {
		t.Errorf("emptied, the preview shows %v and the empty line %v", e.preview.Visible(), e.cards.end.empty.Visible())
	}
}
