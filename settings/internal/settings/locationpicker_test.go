package settings

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/ui/widget"
)

const testZoneTable = "# comment\nJP\t+353916+1394441\tAsia/Tokyo\nUS\t+404251-0740023\tAmerica/New_York\tEastern\nAR\t-3436-05827\tAmerica/Argentina/Buenos_Aires\n"

func testZonePlaces(t *testing.T) []zonePlace {
	t.Helper()
	h := fakeHost(t, nil)
	put(t, h.path("usr/share/zoneinfo/zone1970.tab"), testZoneTable)
	places, err := h.zonePlaces()
	if err != nil {
		t.Fatal(err)
	}
	return places
}

func placeNames(places []zonePlace) []string {
	out := make([]string, len(places))
	for i, p := range places {
		out[i] = p.place.Name()
	}
	return out
}

func TestTheCitiesAreTheZoneTablesPlaces(t *testing.T) {
	places := testZonePlaces(t)
	if got := placeNames(places); !slices.Equal(got, []string{"Buenos Aires", "New York", "Tokyo"}) {
		t.Fatalf("cities = %q", got)
	}
	tokyo := places[2]
	if tokyo.zone != "Asia/Tokyo" || math.Abs(tokyo.place.Coordinates().Latitude()-(35+39.0/60+16.0/3600)) > 1e-9 {
		t.Errorf("Tokyo = %s at %v", tokyo.zone, tokyo.place.Coordinates())
	}
}

func TestTheCitiesFailWithoutAUsableZoneTable(t *testing.T) {
	if _, err := fakeHost(t, nil).zonePlaces(); !errors.Is(err, errNoZoneTable) {
		t.Errorf("without a table: %v", err)
	}
	h := fakeHost(t, nil)
	put(t, h.path("etc/zoneinfo/zone1970.tab"), "XX\t+9999+99999\tNowhere/Atlantis\n")
	if _, err := h.zonePlaces(); err == nil || !strings.Contains(err.Error(), "Nowhere/Atlantis") {
		t.Errorf("a bad position: %v, want an error naming its zone", err)
	}
	h = fakeHost(t, nil)
	put(t, h.path("etc/zoneinfo/zone1970.tab"), "XX\t+0000+00000\n")
	if _, err := h.zonePlaces(); err == nil || !strings.Contains(err.Error(), "no zone column") {
		t.Errorf("a line without a zone: %v", err)
	}
}

func listedPlaces(places []zonePlace, err error) func() ([]zonePlace, error) {
	return func() ([]zonePlace, error) { return places, err }
}

func TestTheWeatherLocationIsPickedFromTheCities(t *testing.T) {
	k := testKit(t, "")
	f := &fakePickers{}
	k.pickers = f
	p := newLocationPicker(k, k.store.slot("modules.weather.location"), pinnedOnly, listedPlaces(testZonePlaces(t), nil))
	if p.automatic != nil {
		t.Error("the weather offers an automatic location it cannot follow")
	}
	if got := p.cityLabel; got != "San Francisco" {
		t.Errorf("the button reads %q, want the default place", got)
	}
	if !p.manual.Visible() || p.notice.Visible() {
		t.Errorf("manual shown %v, notice shown %v", p.manual.Visible(), p.notice.Visible())
	}
	p.city.ClickAt(widget.Point{})
	if p.picker == nil || f.anchor != widget.Widget(p.city) {
		t.Fatal("the city list did not open beside its button")
	}
	if !slices.Equal(p.picker.all, []string{"America/Argentina/Buenos_Aires", "America/New_York", "Asia/Tokyo"}) {
		t.Fatalf("offered %q", p.picker.all)
	}
	pickName(t, p.picker, "Asia/Tokyo")
	want := map[string]any{"name": "Tokyo", "latitude": 35.6544, "longitude": 139.7447}
	if got := k.store.value("modules.weather.location"); !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
	if p.cityLabel != "Tokyo" || p.latitude.Value() != 35.6544 {
		t.Errorf("shows %q at %v", p.cityLabel, p.latitude.Value())
	}
}

func TestAHandSetPositionDropsTheCityName(t *testing.T) {
	k := testKit(t, "")
	p := newLocationPicker(k, k.store.slot("modules.weather.location"), pinnedOnly, listedPlaces(nil, nil))
	p.latitude.SetValue(40.5)
	p.latitude.OnValueChanged(p.latitude.Value())
	want := map[string]any{"latitude": 40.5, "longitude": -122.4194}
	if got := k.store.value("modules.weather.location"); !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
	if strings.Contains(p.cityLabel, "San Francisco") || !strings.Contains(p.cityLabel, "40.5") {
		t.Errorf("a hand-set position reads %q", p.cityLabel)
	}
}

func TestAPositionOffTheGlobeIsRefusedAndShown(t *testing.T) {
	k := testKit(t, "")
	p := newLocationPicker(k, k.store.slot("modules.weather.location"), pinnedOnly, listedPlaces(nil, nil))
	before := k.store.value("modules.weather.location")
	if err := p.coordinate(false).set(181.0); err == nil {
		t.Fatal("a longitude of 181 was accepted")
	}
	if got := k.store.value("modules.weather.location"); !reflect.DeepEqual(got, before) {
		t.Errorf("a refused position stored %v", got)
	}
	if !p.problem.Visible() || !strings.Contains(p.problem.Text(), "181") {
		t.Errorf("the refusal shows %v %q", p.problem.Visible(), p.problem.Text())
	}
}

func TestTheNightLightLocationIsAutomaticOrAPickedCity(t *testing.T) {
	k := testKit(t, "")
	f := &fakePickers{}
	k.pickers = f
	const path = "modules.nightlight.location"
	p := newLocationPicker(k, k.store.slot(path), automaticOrPinned, listedPlaces(testZonePlaces(t), nil))
	if p.automatic == nil || !p.automatic.On() || p.manual.Visible() {
		t.Fatal("the default location is not shown as automatic")
	}
	p.automatic.SetOn(false)
	if !p.manual.Visible() || p.picker == nil {
		t.Fatal("turning automatic off did not offer the cities")
	}
	if got := k.store.value(path); got != "auto" {
		t.Errorf("turning automatic off before a pick stored %v", got)
	}
	pickName(t, p.picker, "America/New_York")
	place, pinned, err := p.current()
	if err != nil || !pinned || place.Name() != "New York" {
		t.Fatalf("picked %q pinned %v: %v", place.Name(), pinned, err)
	}
	if p.automatic.On() {
		t.Error("the switch stayed on over a picked city")
	}
	p.automatic.SetOn(true)
	if got := k.store.value(path); got != "auto" {
		t.Errorf("turning automatic back on stored %v", got)
	}
	if p.manual.Visible() {
		t.Error("the city stays shown under an automatic location")
	}
}

func TestWithoutACityListTheCoordinatesStillPinTheLocation(t *testing.T) {
	k := testKit(t, "")
	p := newLocationPicker(k, k.store.slot("modules.nightlight.location"), automaticOrPinned, listedPlaces(nil, errNoZoneTable))
	if !p.notice.Visible() || !strings.Contains(p.notice.Text(), "zone1970.tab") {
		t.Errorf("the missing list reads %v %q", p.notice.Visible(), p.notice.Text())
	}
	if p.city.Enabled() {
		t.Error("an empty city list can be opened")
	}
	p.automatic.SetOn(false)
	if err := p.coordinate(true).set(55.68); err != nil {
		t.Fatal(err)
	}
	if err := p.coordinate(false).set(12.57); err != nil {
		t.Fatal(err)
	}
	place, pinned, _ := p.current()
	if !pinned || place.Coordinates() != mustTestCoordinates(t, 55.68, 12.57) {
		t.Errorf("pinned %v at %v", pinned, place.Coordinates())
	}
}

func mustTestCoordinates(t *testing.T, latitude, longitude float64) geo.Coordinates {
	t.Helper()
	at, err := geo.NewCoordinates(latitude, longitude)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func TestThePagesPickTheirLocations(t *testing.T) {
	k := testKit(t, "")
	for page, path := range map[string]string{"weather": "modules.weather.location", "nightlight": "modules.nightlight.location"} {
		if _, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*locationPicker); !ok {
			t.Errorf("%s is not picked as a location", path)
		}
	}
}
