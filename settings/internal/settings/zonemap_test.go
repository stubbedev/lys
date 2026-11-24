package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/tzdb"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	mapZoneTable    = "DK\t+5540+01235\tEurope/Copenhagen\nCH,DE,LI\t+4723+00832\tEurope/Zurich\nJP\t+353916+1394441\tAsia/Tokyo\nUS\t+404251-0740023\tAmerica/New_York\tEastern\n"
	mapCountryTable = "# ISO 3166\nCH\tSwitzerland\nDE\tGermany\nDK\tDenmark\nJP\tJapan\nLI\tLiechtenstein\nUS\tUnited States\n"
)

var (
	mapListed = []string{"Europe/Copenhagen", "Europe/Zurich", "Asia/Tokyo", "America/New_York", "UTC"}
	winter    = time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
)

func zoneinfoHost(t *testing.T, zones, countries string) host {
	t.Helper()
	h := fakeHost(t, nil)
	put(t, h.path("usr/share/zoneinfo", tzdb.ZoneTableName), zones)
	if countries != "" {
		put(t, h.path("usr/share/zoneinfo", tzdb.CountryTableName), countries)
	}
	return h
}

func testCatalog(t *testing.T, at time.Time) zoneCatalog {
	t.Helper()
	cat, err := zoneinfoHost(t, mapZoneTable, mapCountryTable).zoneCatalog(mapListed, at)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestAZoneIsNamedByItsCityCountriesAbbreviationAndOffset(t *testing.T) {
	cat := testCatalog(t, winter)
	for zone, want := range map[string]string{
		"Europe/Copenhagen": "Copenhagen · Denmark · CET · UTC+01:00",
		"Europe/Zurich":     "Zurich · Switzerland · Germany · Liechtenstein · CET · UTC+01:00",
		"America/New_York":  "New York · United States · EST · UTC-05:00",
		"UTC":               "UTC · UTC+00:00",
		"Mars/Olympus":      "Mars/Olympus",
	} {
		if got := cat.label(zone); got != want {
			t.Errorf("%s reads %q, want %q", zone, got, want)
		}
	}
	summer := testCatalog(t, time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC))
	if got := summer.label("Europe/Copenhagen"); got != "Copenhagen · Denmark · CEST · UTC+02:00" {
		t.Errorf("in July Copenhagen reads %q", got)
	}
	var pinned []string
	for _, p := range cat.pins {
		pinned = append(pinned, p.zone)
	}
	if !slices.Equal(pinned, []string{"America/New_York", "Europe/Zurich", "Europe/Copenhagen", "Asia/Tokyo"}) || cat.noMap != nil {
		t.Errorf("pins west to east %q, no map %v", pinned, cat.noMap)
	}
}

func TestAZoneTimedatedDoesNotListIsNeitherOfferedNorPinned(t *testing.T) {
	cat, err := zoneinfoHost(t, mapZoneTable, mapCountryTable).zoneCatalog([]string{"Asia/Tokyo"}, winter)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.zones) != 1 || len(cat.pins) != 1 || cat.pins[0].zone != "Asia/Tokyo" {
		t.Errorf("offered %+v, pinned %+v", cat.zones, cat.pins)
	}
}

func TestWithoutTheTzTablesTheZonesAreListedAndTheMapSaysWhy(t *testing.T) {
	cat, err := fakeHost(t, nil).zoneCatalog(mapListed, winter)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(cat.noMap, tzdb.ErrNoZoneTable) || len(cat.pins) != 0 || len(cat.zones) != len(mapListed) {
		t.Errorf("no map %v, %d pins, %d zones", cat.noMap, len(cat.pins), len(cat.zones))
	}
	if got := cat.label("Asia/Tokyo"); got != "Asia/Tokyo · JST · UTC+09:00" {
		t.Errorf("a zone without the table reads %q", got)
	}
	cat, err = zoneinfoHost(t, mapZoneTable, "").zoneCatalog(mapListed, winter)
	if err != nil {
		t.Fatal(err)
	}
	if cat.noMap == nil || !strings.Contains(cat.noMap.Error(), tzdb.CountryTableName) || len(cat.pins) != 0 {
		t.Errorf("without %s: no map %v, %d pins", tzdb.CountryTableName, cat.noMap, len(cat.pins))
	}
	c := newZoneChooser(testKit(t, ""), cat, func(string) {})
	wantClosedFailure(t, "the chooser without a map", c.notice.stateParts, c.notice.Text(), locale.T().Get("settings-date-zone-no-map"), tzdb.CountryTableName)
	if !c.noteRow.Visible() {
		t.Error("the note's row is hidden")
	}
}

func TestAZoneInACountryTheTableLacksOrAnUnknownZoneIsAnError(t *testing.T) {
	_, err := zoneinfoHost(t, mapZoneTable, "DK\tDenmark\n").zoneCatalog(mapListed, winter)
	if err == nil || !strings.Contains(err.Error(), "Europe/Zurich") || !strings.Contains(err.Error(), `"CH"`) {
		t.Errorf("a missing country: %v", err)
	}
	if _, err := zoneinfoHost(t, mapZoneTable, mapCountryTable).zoneCatalog([]string{"Mars/Olympus"}, winter); err == nil || !strings.Contains(err.Error(), "Mars/Olympus") {
		t.Errorf("an unknown zone: %v", err)
	}
}

func arrangedMap(t *testing.T, cat zoneCatalog) (*zoneMap, *[]string) {
	t.Helper()
	var picked []string
	m := newZoneMap(cat, func(zone string) { picked = append(picked, zone) })
	m.Arrange(render.Rect{W: mapDegreesWide, H: mapDegreesHigh})
	return m, &picked
}

func at(t *testing.T, latitude, longitude float64) geo.Coordinates {
	t.Helper()
	c, err := geo.NewCoordinates(latitude, longitude)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAClickOnTheMapPicksTheNearestCitysZoneAcrossTheDateLine(t *testing.T) {
	m, picked := arrangedMap(t, testCatalog(t, winter))
	r := m.area()
	m.ClickAt(mapPoint(at(t, 35, 140), r))
	m.ClickAt(mapPoint(at(t, 55, 13), r))
	m.ClickAt(mapPoint(at(t, 35, -175), r))
	m.ClickAt(mapPoint(at(t, 40, -80), r))
	if !slices.Equal(*picked, []string{"Asia/Tokyo", "Europe/Copenhagen", "Asia/Tokyo", "America/New_York"}) {
		t.Errorf("picked %q", *picked)
	}
}

func TestAMapWithoutPinsPicksNothing(t *testing.T) {
	cat, err := fakeHost(t, nil).zoneCatalog(mapListed, winter)
	if err != nil {
		t.Fatal(err)
	}
	m, picked := arrangedMap(t, cat)
	m.ClickAt(widget.Point{X: 10, Y: 10})
	m.KeyAction(widget.KeyRight, 0)
	if len(*picked) != 0 {
		t.Errorf("a map without cities picked %q", *picked)
	}
}

func TestTheArrowKeysStepThroughTheZonesWestToEast(t *testing.T) {
	m, picked := arrangedMap(t, testCatalog(t, winter))
	m.KeyAction(widget.KeyRight, 0)
	m.show("Asia/Tokyo")
	m.KeyAction(widget.KeyRight, 0)
	m.show("America/New_York")
	m.KeyAction(widget.KeyLeft, 0)
	m.show("Europe/Copenhagen")
	m.KeyAction(widget.KeyLeft, 0)
	m.KeyAction(widget.KeyUp, 0)
	if !slices.Equal(*picked, []string{"America/New_York", "America/New_York", "Asia/Tokyo", "Europe/Zurich"}) {
		t.Errorf("picked %q", *picked)
	}
}

func TestTheZoneBandSpansFifteenDegreesAndWrapsAtTheDateLine(t *testing.T) {
	r := render.Rect{W: mapDegreesWide, H: mapDegreesHigh}
	bands := bandRects(int(time.Hour.Seconds()), r)
	if bands[1] != (render.Rect{X: 188, W: 15, H: 180}) {
		t.Errorf("UTC+1's band %+v", bands[1])
	}
	east := bandRects(int((13 * time.Hour).Seconds()), r)
	if east[0] != (render.Rect{X: 8, W: 15, H: 180}) {
		t.Errorf("UTC+13's band does not wrap to the west edge: %+v", east)
	}
}

func shownZones(p *searchPicker) []string {
	var out []string
	for i, name := range p.all {
		if p.rows.ChildAt(i).Visible() {
			out = append(out, name)
		}
	}
	return out
}

func TestTheZoneSearchFindsACityCountryOrAbbreviationAndPicksIt(t *testing.T) {
	k := testKit(t, "")
	f := &fakePickers{}
	k.pickers = f
	var chosen []string
	c := newZoneChooser(k, testCatalog(t, winter), func(zone string) { chosen = append(chosen, zone) })
	c.show("UTC")
	c.button.ClickAt(widget.Point{})
	if c.picker == nil || f.anchor != widget.Widget(c.button) {
		t.Fatal("the zone list did not open beside its button")
	}
	for query, want := range map[string][]string{
		"liechtenstein": {"Europe/Zurich"},
		"cet":           {"Europe/Copenhagen", "Europe/Zurich"},
		"tokyo":         {"Asia/Tokyo"},
		"est":           {"America/New_York"},
		"atlantis":      nil,
	} {
		c.picker.filter(query)
		if got := shownZones(c.picker); !slices.Equal(got, want) {
			t.Errorf("%q shows %q, want %q", query, got, want)
		}
	}
	pickName(t, c.picker, "Asia/Tokyo")
	if !slices.Equal(chosen, []string{"Asia/Tokyo"}) || !slices.Contains(treetest.Labels(c.button), "Tokyo · Japan · JST · UTC+09:00") || c.worldMap.current != "Asia/Tokyo" {
		t.Errorf("chose %q, the button reads %q, the map marks %q", chosen, treetest.Labels(c.button), c.worldMap.current)
	}
}
