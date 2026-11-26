package settings

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/tzdb"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/land"
	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	mapDegreesWide   = 360
	mapDegreesHigh   = 180
	bandDegrees      = 15
	secondsPerDegree = 3600 / bandDegrees
	bandAlpha        = 0.25
	landAlpha        = 0.2
	pinsAcross       = 240
	minPinPx         = 2
	zoneDetailSep    = " · "
)

type tzZone struct {
	name      string
	city      string
	countries []string
	abbr      string
	offset    string
	seconds   int
}

type zonePin struct {
	zone string
	at   geo.Coordinates
}

type zoneCatalog struct {
	zones []tzZone
	pins  []zonePin
	noMap error
}

func utcOffset(t time.Time) string { return "UTC" + t.Format("-07:00") }

func (h host) zoneCatalog(listed []string, now time.Time) (zoneCatalog, error) {
	db, err := h.tz().Database()
	var countries map[string]string
	if err == nil {
		countries, err = db.CountryNames()
	}
	if err != nil {
		cat, cerr := newZoneCatalog(listed, nil, nil, now)
		cat.noMap = err
		return cat, cerr
	}
	return newZoneCatalog(listed, db.Zones(), countries, now)
}

func newZoneCatalog(listed []string, zones []tzdb.Zone, countries map[string]string, now time.Time) (zoneCatalog, error) {
	byZone := make(map[string]tzdb.Zone, len(zones))
	for _, z := range zones {
		byZone[z.Name()] = z
	}
	cat := zoneCatalog{zones: make([]tzZone, 0, len(listed))}
	for _, name := range listed {
		loc, err := time.LoadLocation(name)
		if err != nil {
			return zoneCatalog{}, fmt.Errorf("read the time zone %q: %w", name, err)
		}
		at := now.In(loc)
		abbr, seconds := at.Zone()
		z := tzZone{name: name, city: name, offset: utcOffset(at), seconds: seconds}
		if abbr != name && !strings.HasPrefix(abbr, "+") && !strings.HasPrefix(abbr, "-") {
			z.abbr = abbr
		}
		if r, ok := byZone[name]; ok {
			z.city = zoneCity(name)
			for _, code := range r.Countries() {
				country, ok := countries[code]
				if !ok {
					return zoneCatalog{}, fmt.Errorf("the time zone %s is in the country %q, which %s does not name", name, code, tzdb.CountryTableName)
				}
				z.countries = append(z.countries, country)
			}
			cat.pins = append(cat.pins, zonePin{zone: name, at: r.Coordinates()})
		}
		cat.zones = append(cat.zones, z)
	}
	slices.SortFunc(cat.zones, func(a, b tzZone) int { return cmp.Or(strings.Compare(a.city, b.city), strings.Compare(a.name, b.name)) })
	slices.SortFunc(cat.pins, func(a, b zonePin) int {
		return cmp.Or(cmp.Compare(a.at.Longitude(), b.at.Longitude()), strings.Compare(a.zone, b.zone))
	})
	return cat, nil
}

func (c zoneCatalog) zone(name string) (tzZone, bool) {
	i := slices.IndexFunc(c.zones, func(z tzZone) bool { return z.name == name })
	if i < 0 {
		return tzZone{}, false
	}
	return c.zones[i], true
}

func (z tzZone) detail() string {
	parts := slices.Clone(z.countries)
	if z.abbr != "" {
		parts = append(parts, z.abbr)
	}
	return strings.Join(append(parts, z.offset), zoneDetailSep)
}

func (z tzZone) searchText() string {
	return strings.Join([]string{z.city, z.name, z.detail()}, " ")
}

func (c zoneCatalog) label(name string) string {
	z, ok := c.zone(name)
	if !ok {
		return name
	}
	return z.city + zoneDetailSep + z.detail()
}

func degreeDistance(a geo.Coordinates, latitude, longitude float64) float64 {
	across := math.Abs(a.Longitude() - longitude)
	if across > mapDegreesWide/2 {
		across = mapDegreesWide - across
	}
	down := a.Latitude() - latitude
	return float64(across*across) + float64(down*down)
}

func nearestPin(pins []zonePin, latitude, longitude float64) (zonePin, bool) {
	if len(pins) == 0 {
		return zonePin{}, false
	}
	return slices.MinFunc(pins, func(a, b zonePin) int {
		return cmp.Compare(degreeDistance(a.at, latitude, longitude), degreeDistance(b.at, latitude, longitude))
	}), true
}

func mapOffset(latitude, longitude float64, r render.Rect) (x, y float64) {
	return float64((longitude + mapDegreesWide/2) / mapDegreesWide * float64(r.W)), float64((mapDegreesHigh/2 - latitude) / mapDegreesHigh * float64(r.H))
}

func mapPoint(at geo.Coordinates, r render.Rect) widget.Point {
	x, y := mapOffset(at.Latitude(), at.Longitude(), r)
	return widget.Point{X: r.X + int(x), Y: r.Y + int(y)}
}

func landPath(r render.Rect) *render.Path {
	return land.Path(func(latitude, longitude float64) (x, y float64) {
		x, y = mapOffset(latitude, longitude, r)
		return float64(r.X) + x, float64(r.Y) + y
	})
}

func mapDegrees(p widget.Point, r render.Rect) (latitude, longitude float64) {
	across := float64(float64(p.X-r.X) / float64(r.W) * mapDegreesWide)
	down := float64(float64(p.Y-r.Y) / float64(r.H) * mapDegreesHigh)
	return mapDegreesHigh/2 - down, across - mapDegreesWide/2
}

func bandRects(seconds int, r render.Rect) []render.Rect {
	center := float64(seconds) / secondsPerDegree
	west := center - bandDegrees/2
	out := make([]render.Rect, 0, 3)
	for _, shift := range []float64{-mapDegreesWide, 0, mapDegreesWide} {
		x := (west + shift + mapDegreesWide/2) / mapDegreesWide * float64(r.W)
		w := float64(bandDegrees) / mapDegreesWide * float64(r.W)
		out = append(out, render.Rect{X: r.X + int(math.Round(x)), Y: r.Y, W: max(int(math.Round(w)), 1), H: r.H})
	}
	return out
}

type zoneMap struct {
	widget.Base
	cat      zoneCatalog
	current  string
	pick     func(string)
	land     *render.Path
	landArea render.Rect
}

func newZoneMap(cat zoneCatalog, pick func(string)) *zoneMap {
	m := &zoneMap{Base: widget.NewBase(), cat: cat, pick: pick}
	m.SetAccessibleName(locale.T().Get("settings-date-zone-map"))
	return m
}

func (m *zoneMap) show(zone string) {
	m.current = zone
	m.Invalidate()
}

func (m *zoneMap) Measure(axis widget.Axis, forSize int) widget.Sizes {
	if axis == widget.Row || forSize == widget.Unknown {
		return widget.Sizes{}
	}
	return widget.FixedSizes(forSize * mapDegreesHigh / mapDegreesWide)
}

func (m *zoneMap) area() render.Rect {
	b := m.Bounds()
	return render.Rect{X: b.X, Y: b.Y, W: b.W, H: b.W * mapDegreesHigh / mapDegreesWide}
}

func (m *zoneMap) Paint(cv *render.Canvas) {
	th := widget.Current()
	r := m.area()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	cv.RoundedRect(r, tileRadius, th.MutedColor())
	region := cv.MapRect(r).Intersect(cv.Rect())
	if !region.Empty() {
		face := cv.ScratchLayer(region)
		m.paintFace(face.Canvas(), r, th)
		tile := &render.Path{}
		render.RoundedRectPath(tile, r, render.UniformCorners(tileRadius))
		face.ApplyPathMask(tile, region)
		cv.DrawLayer(face, region, render.BlendNormal)
		face.Release()
	}
	ring, width := th.Border, 1
	if widget.FocusVisible(m) {
		ring, width = th.Text, focusRing
	}
	cv.RoundedBorder(r, render.UniformCorners(tileRadius), render.UniformInsets(width), ring)
}

func (m *zoneMap) paintFace(cv *render.Canvas, r render.Rect, th *widget.Theme) {
	if m.land == nil || m.landArea != r {
		m.land, m.landArea = landPath(r), r
	}
	alpha := cv.PushAlpha(landAlpha)
	cv.FillPath(m.land, th.TextMuted)
	cv.PopAlpha(alpha)
	current, known := m.cat.zone(m.current)
	if known {
		alpha := cv.PushAlpha(bandAlpha)
		for _, band := range bandRects(current.seconds, r) {
			cv.FillRect(band, th.Accent)
		}
		cv.PopAlpha(alpha)
	}
	dot := max(r.W/pinsAcross, minPinPx)
	var chosen *zonePin
	for i, pin := range m.cat.pins {
		if pin.zone == m.current {
			chosen = &m.cat.pins[i]
			continue
		}
		fill := th.TextMuted
		if z, ok := m.cat.zone(pin.zone); known && ok && z.seconds == current.seconds {
			fill = th.Accent
		}
		at := mapPoint(pin.at, r)
		cv.RoundedRect(render.Rect{X: at.X - dot/2, Y: at.Y - dot/2, W: dot, H: dot}, dot/2, fill)
	}
	if chosen != nil {
		at, side := mapPoint(chosen.at, r), 3*dot
		mark := render.Rect{X: at.X - side/2, Y: at.Y - side/2, W: side, H: side}
		cv.RoundedRect(mark, side/2, th.Accent)
		cv.RoundedBorder(mark, render.UniformCorners(side/2), render.UniformInsets(1), th.Text)
	}
}

func (m *zoneMap) HitTest(pt widget.Point) widget.Widget { return m.HitLeaf(m, pt) }

func (m *zoneMap) ClickAt(pt widget.Point) {
	r := m.area()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	latitude, longitude := mapDegrees(pt, r)
	if pin, ok := nearestPin(m.cat.pins, latitude, longitude); ok {
		m.pick(pin.zone)
	}
}

func (m *zoneMap) FocusMoved(_, _ widget.Widget) { m.Invalidate() }

func (m *zoneMap) Role() widget.Role { return widget.RoleList }

func (m *zoneMap) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if len(m.cat.pins) == 0 {
		return
	}
	i := slices.IndexFunc(m.cat.pins, func(p zonePin) bool { return p.zone == m.current })
	switch {
	case a == widget.KeyRight && i < 0:
		i = 0
	case a == widget.KeyRight:
		i = (i + 1) % len(m.cat.pins)
	case a == widget.KeyLeft && i <= 0:
		i = len(m.cat.pins) - 1
	case a == widget.KeyLeft:
		i--
	default:
		return
	}
	m.pick(m.cat.pins[i].zone)
}

type zoneChooser struct {
	k        *kit
	cat      zoneCatalog
	choose   func(string)
	button   *widget.Button
	face     *widget.ButtonContent
	worldMap *zoneMap
	notice   *inlineState
	mapRow   *insetRow
	noteRow  *insetRow
	picker   *searchPicker
}

func newZoneChooser(k *kit, cat zoneCatalog, choose func(string)) *zoneChooser {
	c := &zoneChooser{k: k, cat: cat}
	c.choose = func(zone string) {
		c.show(zone)
		choose(zone)
	}
	c.button, c.face = k.contentButton(rolePicker, "ld-globe-symbolic", "", c.open)
	c.button.SetAccessibleName(locale.T().Get("settings-date-zone"))
	c.worldMap = newZoneMap(cat, c.choose)
	c.notice = k.inlineState()
	c.mapRow, c.noteRow = newInsetRow(c.worldMap), newInsetRow(c.notice)
	c.mapRow.SetVisible(len(cat.pins) > 0)
	c.noteRow.SetVisible(cat.noMap != nil)
	if cat.noMap != nil {
		c.notice.Note(locale.T().Get("settings-date-zone-no-map"), cat.noMap)
	}
	return c
}

func (c *zoneChooser) show(zone string) {
	c.face.SetLabel(c.cat.label(zone))
	c.worldMap.show(zone)
}

func (c *zoneChooser) open() {
	if c.k.pickers == nil {
		return
	}
	names := make([]string, len(c.cat.zones))
	for i, z := range c.cat.zones {
		names[i] = z.name
	}
	c.picker = openSearchPicker(c.k, c.button, names, pickerSpec{
		class: "choice-popover", placeholder: "settings-date-zone-search", maxH: choiceListPx,
		row: func(k *kit, name string) widget.Widget {
			z, _ := c.cat.zone(name)
			return choiceRow(k, choice{value: z.name, label: z.city, group: z.detail()})
		},
		text: func(name string) string { z, _ := c.cat.zone(name); return z.searchText() },
	}, c.choose)
}
