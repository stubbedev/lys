package settings

import (
	"fmt"
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	locationDigits = 4
	locationStep   = 0.0001
)

type locationChoice int

const (
	pinnedOnly locationChoice = iota
	automaticOrPinned
)

func locationRow(choice locationChoice) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newLocationPicker(k, s, choice, systemHost.zonePlaces)
		}
	}
}

type locationPicker struct {
	*widget.Box
	k         *kit
	slot      slot
	choice    locationChoice
	automatic *widget.Switch
	syncing   bool
	choosing  bool
	manual    *widget.Box
	city      *widget.Button
	cityText  *widget.ButtonContent
	cityLabel string
	latitude  *number
	longitude *number
	notice    *widget.Label
	problem   *widget.Label
	places    []zonePlace
	picker    *searchPicker
}

func newLocationPicker(k *kit, s slot, choice locationChoice, places func() ([]zonePlace, error)) *locationPicker {
	t := locale.T()
	p := &locationPicker{Box: gapBox(widget.Column), k: k, slot: s, choice: choice}
	p.AddClass("location-picker")
	if choice == automaticOrPinned {
		p.automatic = widget.NewSwitch(false)
		p.automatic.SetAccessibleName(t.Get("settings-location-automatic"))
		p.automatic.OnChanged = p.switched
		row := gapBox(widget.Row)
		row.AppendAligned(k.label(t.Get("settings-location-automatic")), true, widget.AlignCenter)
		row.AppendAligned(p.automatic, false, widget.AlignCenter)
		p.Append(row, false)
	}
	p.city, p.cityText = k.contentButton("ld-globe-symbolic", "", p.openCities, css.Outline)
	p.city.SetAccessibleName(t.Get("settings-location-city"))
	p.city.SetEnabled(false)
	p.latitude = newNumber(k, p.coordinate(true), -90, 90, locationStep, locationDigits)
	p.latitude.SetAccessibleName(t.Get("settings-location-latitude"))
	p.longitude = newNumber(k, p.coordinate(false), -180, 180, locationStep, locationDigits)
	p.longitude.SetAccessibleName(t.Get("settings-location-longitude"))
	coords := gapBox(widget.Row)
	for _, part := range []struct {
		key string
		n   *number
	}{{"settings-location-latitude", p.latitude}, {"settings-location-longitude", p.longitude}} {
		coords.AppendAligned(k.label(t.Get(part.key), css.DimLabel), false, widget.AlignCenter)
		coords.AppendAligned(part.n, false, widget.AlignCenter)
	}
	p.manual = gapBox(widget.Column)
	p.manual.Append(p.city, false)
	p.manual.Append(coords, false)
	p.Append(p.manual, false)
	p.notice = k.label("", "setting-description", css.Caption, css.DimLabel)
	p.notice.SetVisible(false)
	p.Append(p.notice, false)
	p.problem = k.label("", "setting-description", css.Caption, "error")
	p.problem.SetVisible(false)
	p.Append(p.problem, false)
	async(k, func() placesResult { list, err := places(); return placesResult{list, err} }, p.listed)
	p.refresh()
	return p
}

type placesResult struct {
	list []zonePlace
	err  error
}

func (p *locationPicker) listed(r placesResult) {
	p.places = r.list
	p.city.SetEnabled(len(r.list) > 0)
	if r.err != nil {
		p.notice.SetText(locale.T().Get("settings-location-no-cities", i18n.Str("error", r.err.Error())))
	}
	p.notice.SetVisible(r.err != nil)
}

func (p *locationPicker) current() (geo.Place, bool, error) {
	var a config.AutoPlace
	if err := a.UnmarshalConfig(p.slot.get()); err != nil {
		return geo.Place{}, false, err
	}
	place, pinned := a.Pinned()
	return place, pinned, nil
}

func (p *locationPicker) refresh() {
	place, pinned, err := p.current()
	if err != nil {
		p.fail(err)
		return
	}
	p.problem.SetVisible(false)
	if pinned {
		p.choosing = false
	}
	if p.automatic != nil {
		p.syncing = true
		p.automatic.SetOn(!pinned)
		p.syncing = false
	}
	p.manual.SetVisible(pinned || p.choosing)
	p.showCity(placeLabel(place, pinned))
	p.latitude.refresh()
	p.longitude.refresh()
}

func (p *locationPicker) showCity(text string) {
	p.cityLabel = text
	p.cityText.SetLabel(text)
}

func placeLabel(place geo.Place, pinned bool) string {
	t := locale.T()
	switch {
	case !pinned:
		return t.Get("settings-location-choose")
	case place.Name() != "":
		return place.Name()
	}
	at := place.Coordinates()
	return t.Get("settings-location-unnamed", i18n.Float("latitude", at.Latitude()), i18n.Float("longitude", at.Longitude()))
}

func (p *locationPicker) switched(on bool) {
	if p.syncing {
		return
	}
	if on {
		p.choosing = false
		if err := p.slot.set(config.AutoPlace{}.MarshalConfig()); err != nil {
			p.fail(err)
		}
		p.refresh()
		return
	}
	p.choosing = true
	p.refresh()
	p.openCities()
}

func (p *locationPicker) openCities() {
	if p.k.pickers == nil || len(p.places) == 0 {
		return
	}
	opts := make([]choice, len(p.places))
	for i, z := range p.places {
		opts[i] = choice{value: z.zone, label: z.place.Name()}
	}
	p.picker = openChoicePicker(p.k, p.city, opts, p.pickZone)
}

func (p *locationPicker) pickZone(zone string) {
	for _, z := range p.places {
		if z.zone == zone {
			p.pin(z.place)
			return
		}
	}
	p.fail(fmt.Errorf("no city for the zone %q", zone))
}

func (p *locationPicker) pin(place geo.Place) {
	at := place.Coordinates()
	rounded, err := geo.NewCoordinates(roundDegrees(at.Latitude()), roundDegrees(at.Longitude()))
	if err != nil {
		p.fail(err)
		return
	}
	if err := p.slot.set(config.PinPlace(geo.NewPlace(place.Name(), rounded)).MarshalConfig()); err != nil {
		p.fail(err)
		return
	}
	p.refresh()
}

func roundDegrees(v float64) float64 {
	scale := math.Pow10(locationDigits)
	return math.Round(v*scale) / scale
}

func (p *locationPicker) fail(err error) {
	p.problem.SetText(err.Error())
	p.problem.SetVisible(true)
}

func (p *locationPicker) coordinate(latitude bool) slot {
	pick := func(at geo.Coordinates) float64 {
		if latitude {
			return at.Latitude()
		}
		return at.Longitude()
	}
	return slot{
		get: func() any {
			place, pinned, err := p.current()
			if err != nil || !pinned {
				return 0.0
			}
			return pick(place.Coordinates())
		},
		set: func(v any) error {
			degrees, _ := numeric(v)
			place, _, err := p.current()
			if err != nil {
				return err
			}
			lat, lon := place.Coordinates().Latitude(), place.Coordinates().Longitude()
			if latitude {
				lat = degrees
			} else {
				lon = degrees
			}
			at, err := geo.NewCoordinates(lat, lon)
			if err != nil {
				p.fail(err)
				return err
			}
			if err := p.slot.set(config.PinPlace(geo.NewPlace("", at)).MarshalConfig()); err != nil {
				p.fail(err)
				return err
			}
			p.problem.SetVisible(false)
			p.showCity(placeLabel(geo.NewPlace("", at), true))
			return nil
		},
		unset: func() {},
	}
}
