package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/strftime"
	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/glibclocale"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	dateTimeID      = "date-time"
	regionID        = "region"
	timedateName    = "org.freedesktop.timedate1"
	timedatePath    = dbus.ObjectPath("/org/freedesktop/timedate1")
	localeName      = "org.freedesktop.locale1"
	localePath      = dbus.ObjectPath("/org/freedesktop/locale1")
	clockFormatPath = "modules.clock.format"
	localeConfName  = "locale.conf"
	interactive     = true
	previewNumber   = 123456789
	previewPrice    = 123450
	previewScale    = 2
)

var userFormats = []string{"LC_TIME", "LC_NUMERIC", "LC_MONETARY", "LC_MEASUREMENT", "LC_PAPER"}

func dateTimePage(*config.Config) pageSpec {
	return pageSpec{
		id: dateTimeID, navKey: "settings-nav-date-time", icon: "ld-clock-symbolic",
		header: "settings-page-date-time",
		lead:   dateTimeLead,
		sections: []sectionSpec{
			{title: "settings-section-date-calendar", rows: []rowSpec{field("modules.clock.calendar-weekday-start", weekStartRow, weekFollowsRegion)}},
		},
		links: dateTimeLinks,
		panel: []panelSetting{
			{"date-time.automatic", "settings-date-ntp", "settings-section-date-system"},
			{"date-time.manual", "settings-date-manual", "settings-section-date-system"},
			{"date-time.time-zone", "settings-date-zone", "settings-section-date-system"},
			{"date-time.24-hour", "settings-date-24h", "settings-section-date-clock"},
		},
	}
}

var commonWeekStarts = []string{string(config.WeekStartAuto), string(config.WeekStartMonday), string(config.WeekStartSunday), string(config.WeekStartSaturday)}

const (
	weekDays      = 7
	weekRows      = 3
	weekCellGap   = 2
	weekendDayMin = 5
)

var weekStartRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
	values := slices.Clone(commonWeekStarts)
	if current, _ := s.get().(string); !slices.Contains(values, current) && slices.Contains(meta.Variants, current) {
		values = append(values, current)
	}
	all := enumLabels(meta)
	locale := sessionTimeLocale().FirstWeekday()
	labels := make([]string, len(values))
	pictures := make(map[string]picture, len(values))
	for i, v := range values {
		labels[i] = all[slices.Index(meta.Variants, v)]
		day := mondayFirst(config.WeekStart(v).Weekday(locale))
		pictures[v] = drawn(func(cv *render.Canvas, r render.Rect) { weekGrid(cv, r, day) })
	}
	return newCards(k, s, values, labels, pictures)
})

func mondayFirst(d time.Weekday) int { return (int(d) + weekDays - 1) % weekDays }

func weekGrid(cv *render.Canvas, r render.Rect, firstDay int) {
	th := widget.Current()
	cellW := (r.W - (weekDays-1)*weekCellGap) / weekDays
	cellH := (r.H - (weekRows-1)*weekCellGap) / weekRows
	for col := range weekDays {
		fill := th.Border
		switch {
		case col == 0:
			fill = th.Accent
		case (firstDay+col)%weekDays >= weekendDayMin:
			fill = th.TextMuted
		}
		for row := range weekRows {
			cell := render.Rect{X: r.X + col*(cellW+weekCellGap), Y: r.Y + row*(cellH+weekCellGap), W: cellW, H: cellH}
			cv.RoundedRect(cell, weekCellGap, fill)
		}
	}
}

var (
	dateTimeLinks = []pageLink{{"world-clock", "settings-date-link-world-clock-detail"}}
	regionLinks   = []pageLink{{"wm-keyboard", "settings-region-link-keyboard-detail"}}
)

func regionPage(*config.Config) pageSpec {
	return pageSpec{
		id: regionID, navKey: "settings-nav-region", icon: "ld-globe-symbolic",
		header: "settings-page-region",
		footer: regionFooter,
		panel: []panelSetting{
			{"region.lc-time", "settings-region-lc-time", "settings-section-region-formats"},
			{"region.lc-numeric", "settings-region-lc-numeric", "settings-section-region-formats"},
			{"region.lc-monetary", "settings-region-lc-monetary", "settings-section-region-formats"},
			{"region.lc-measurement", "settings-region-lc-measurement", "settings-section-region-formats"},
			{"region.lc-paper", "settings-region-lc-paper", "settings-section-region-formats"},
			{"region.language", "settings-region-language", "settings-section-region-system"},
			{"region.keymap", "settings-region-keymap", "settings-section-region-system"},
			{"region.keyboard-layout", "settings-region-layout", "settings-section-region-system"},
		},
	}
}

type sysPanel struct {
	*widget.Box
	k       *kit
	svc     sysService
	conn    *dbus.Conn
	props   sysProps
	problem *widget.Label
	body    *widget.Box
	watch   *dbusx.Watch
	refresh func()
	closed  bool
}

func newSysPanel(k *kit, name string, path dbus.ObjectPath) *sysPanel {
	p := &sysPanel{Box: gapBox(widget.Column), k: k, svc: sysService{name: name, iface: name, path: path}}
	p.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	p.problem.SetVisible(false)
	p.body = gapBox(widget.Column)
	p.Append(p.problem, false)
	p.Append(p.body, false)
	return p
}

type sysOpened struct {
	conn  *dbus.Conn
	props sysProps
	extra any
	err   error
}

func (p *sysPanel) open(extra func(context.Context, sysService) (any, error), show func(any)) {
	async(p.k, func() sysOpened {
		conn, err := p.k.systemBus()
		if err != nil {
			return sysOpened{err: err}
		}
		svc := p.svc
		svc.conn = conn
		props, err := withTimeout(svc.props)
		if err != nil {
			_ = conn.Close()
			return sysOpened{err: err}
		}
		var more any
		if extra != nil {
			if more, err = withTimeout(func(ctx context.Context) (any, error) { return extra(ctx, svc) }); err != nil {
				_ = conn.Close()
				return sysOpened{err: err}
			}
		}
		return sysOpened{conn: conn, props: props, extra: more}
	}, func(o sysOpened) {
		if o.err != nil {
			p.body.Clear()
			p.body.Append(wrapped(p.k.label(locale.T().Get("settings-sys-unavailable", i18n.Str("service", p.svc.name))+": "+o.err.Error(), css.Error, css.Caption)), false)
			return
		}
		if p.closed {
			_ = o.conn.Close()
			return
		}
		p.conn, p.svc.conn, p.props = o.conn, o.conn, o.props
		watch, err := p.svc.watch(func(changed sysProps) {
			p.k.invoke(func() { p.changed(changed) })
		}, func(err error) {
			p.k.invoke(func() { p.fail(err) })
		})
		if err != nil {
			p.fail(err)
		}
		p.watch = watch
		show(o.extra)
	})
}

func (p *sysPanel) changed(changed sysProps) {
	if p.props == nil {
		return
	}
	maps.Copy(p.props, changed)
	if p.refresh != nil {
		p.refresh()
	}
}

func (p *sysPanel) fail(err error) {
	p.problem.SetVisible(err != nil)
	if err != nil {
		p.problem.SetText(err.Error())
	}
}

func (p *sysPanel) do(method string, args ...any) {
	svc := p.svc
	async(p.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, svc.call(ctx, method, args...) })
		return err
	}, func(err error) {
		p.fail(err)
		if err != nil && p.refresh != nil {
			p.refresh()
		}
	})
}

func (p *sysPanel) close() {
	p.closed = true
	if p.watch != nil {
		if err := p.watch.Stop(); err != nil {
			p.fail(err)
		}
		p.watch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

func panelRow(k *kit, title, detail string, controls ...widget.Widget) *widget.Box {
	if detail == "" {
		return panelRowWith(k, title, nil, controls...)
	}
	return panelRowWith(k, title, detailLabel(k, detail), controls...)
}

func detailLabel(k *kit, text string) *widget.Label {
	return wrapped(k.label(text, "setting-description", css.Caption, css.DimLabel))
}

func panelRowWith(k *kit, title string, detail *widget.Label, controls ...widget.Widget) *widget.Box {
	row := gapBox(widget.Row)
	row.SetElement("item")
	row.AddClass(css.Small, "setting-row")
	text := widget.NewBox(widget.Column, widget.Current().Spacing/4, 0)
	text.Append(k.label(title, "setting-label"), false)
	if detail != nil {
		text.Append(detail, false)
	}
	row.AppendAligned(text, true, widget.AlignCenter)
	for _, c := range controls {
		row.AppendAligned(c, false, widget.AlignCenter)
	}
	return row
}

func hourCycleRow(k *kit, fail func(error), changed func()) widget.Widget {
	t := locale.T()
	preview := k.label("", "setting-description", css.Caption, css.DimLabel)
	h24 := widget.NewSwitch(false)
	h24.SetAccessibleName(t.Get("settings-date-24h"))
	showing := false
	layout := func() (*strftime.Layout, error) { return clockLayout(k.store) }
	show := func() {
		l, err := layout()
		if err != nil {
			h24.SetEnabled(false)
			preview.SetText("")
			return
		}
		c := l.HourCycle()
		showing = true
		h24.SetOn(c == strftime.Hour24)
		showing = false
		h24.SetEnabled(c == strftime.Hour12 || c == strftime.Hour24)
		preview.SetText(l.Format(time.Now()))
	}
	h24.OnChanged = func(on bool) {
		if showing {
			return
		}
		l, err := layout()
		if err != nil {
			fail(err)
			return
		}
		to := strftime.Hour12
		if on {
			to = strftime.Hour24
		}
		if err := k.store.set(clockFormatPath, l.WithHourCycle(to).String()); err != nil {
			fail(err)
		}
		show()
		changed()
	}
	show()
	return panelRow(k, t.Get("settings-date-24h"), t.Attr("settings-date-24h", "description"), preview, h24)
}

func timezoneChoices(zones []string) []choice {
	now := time.Now()
	out := make([]choice, 0, len(zones))
	for _, z := range zones {
		label := z
		if loc, err := time.LoadLocation(z); err == nil {
			label = z + " (" + utcOffset(now.In(loc)) + ")"
		}
		out = append(out, choice{value: z, label: label})
	}
	return out
}

func dateTimeLead(k *kit) widget.Widget {
	t := locale.T()
	p := newSysPanel(k, timedateName, timedatePath)
	card := newClockCard(k)
	clock := gapBox(widget.Column)
	clockRows := widget.NewItemGroup()
	clockRows.Append(hourCycleRow(k, p.fail, card.retick))
	clock.Append(clockRows, false)
	now := k.clock.Now()
	p.open(func(ctx context.Context, svc sysService) (any, error) {
		var zones []string
		if err := svc.store(ctx, "ListTimezones", &zones); err != nil {
			return nil, err
		}
		return systemHost.zoneCatalog(zones, now)
	}, func(extra any) {
		catalog, _ := extra.(zoneCatalog)
		ntp := widget.NewSwitch(p.props.flag("NTP"))
		ntp.SetAccessibleName(t.Get("settings-date-ntp"))
		syncing := false
		ntp.OnChanged = func(on bool) {
			if !syncing {
				p.do("SetNTP", on, interactive)
			}
		}
		manual := newManualTime(k, k.clock.Now())
		manual.set.OnClick = func() {
			at, err := manual.at(p.props.str("Timezone"))
			if err != nil {
				p.fail(err)
				return
			}
			p.do("SetTime", at.UnixMicro(), false, interactive)
		}
		zone := newZoneChooser(k, catalog, func(name string) { p.do("SetTimezone", name, interactive) })
		rows := widget.NewItemGroup()
		rows.Append(panelRow(k, t.Get("settings-date-ntp"), t.Attr("settings-date-ntp", "description"), ntp))
		rows.Append(panelRow(k, t.Get("settings-date-manual"), t.Attr("settings-date-manual", "description"), manual.clock, manual.set))
		rows.Append(insetRow(manual.calendar))
		rows.Append(panelRow(k, t.Get("settings-date-zone"), t.Attr("settings-date-zone", "description"), zone.button))
		rows.Append(zone.mapRow)
		rows.Append(zone.noteRow)
		refresh := func() {
			syncing = true
			defer func() { syncing = false }()
			ntp.SetEnabled(p.props.flag("CanNTP"))
			if ntp.On() != p.props.flag("NTP") {
				ntp.SetOn(p.props.flag("NTP"))
			}
			manual.setEnabled(!p.props.flag("NTP"))
			zone.show(p.props.str("Timezone"))
			card.showSystem(p.props.str("Timezone"), p.props.flag("NTP"), p.props.flag("NTPSynchronized"))
		}
		p.refresh = refresh
		refresh()
		p.body.Append(rows, false)
	})
	content := sectionStack()
	content.Append(card, false)
	content.Append(newSection(k, "settings-section-date-system", p), false)
	content.Append(newSection(k, "settings-section-date-clock", clock), false)
	return &closingBox{content, func() { card.close(); p.close() }}
}

type clockCard struct {
	*widget.Box
	k      *kit
	date   *widget.Label
	clock  *widget.Label
	zone   *widget.Label
	sync   *widget.Label
	names  *glibclocale.Time
	stop   func() bool
	closed bool
}

func newClockCard(k *kit) *clockCard {
	c := &clockCard{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k}
	c.AddClass(css.Card, "clock-card")
	c.SetPadding(cardInsets())
	c.clock = k.label("", "clock-card-time", css.Title1, css.Numeric)
	c.date = k.label("", "clock-card-date", css.H4)
	c.zone = k.label("", css.Caption, css.DimLabel)
	c.sync = k.label("", css.Caption, css.DimLabel)
	c.zone.SetVisible(false)
	c.sync.SetVisible(false)
	for _, l := range []*widget.Label{c.clock, c.date, c.zone, c.sync} {
		c.Append(l, false)
	}
	c.names = sessionTimeLocale()
	c.tick()
	return c
}

// sessionTimeLocale is the LC_TIME of the session's time locale, C's as
// glibc falls back to when it cannot be read, which is logged.
func sessionTimeLocale() *glibclocale.Time {
	name := glibclocale.CategoryLocale(systemHost.env, glibclocale.CategoryTime)
	names, err := localeSource.Time(name)
	if err != nil {
		log.Printf("settings: the time locale %s, using C's as glibc does: %v", name, err)
		names, _ = localeSource.Time("C")
	}
	return names
}

const clockDateFormat = "%A, %x"

func clockTimeFormat(cycle strftime.HourCycle) string {
	if cycle == strftime.Hour12 {
		return "%I:%M %p"
	}
	return "%H:%M"
}

func clockLayout(s store) (*strftime.Layout, error) {
	f, _ := s.value(clockFormatPath).(string)
	l, err := strftime.Compile(f)
	if err != nil {
		return nil, fmt.Errorf("read the clock format %q: %w", f, err)
	}
	return l, nil
}

func formatClock(names *glibclocale.Time, s store, now time.Time) (date, clock string, err error) {
	l, err := clockLayout(s)
	if err != nil {
		return "", "", err
	}
	if date, err = names.Format(clockDateFormat, now); err != nil {
		return "", "", err
	}
	if clock, err = names.Format(clockTimeFormat(l.HourCycle()), now); err != nil {
		return "", "", err
	}
	return date, clock, nil
}

func timeOfDayText(names *glibclocale.Time, s store, at, now time.Time) (string, error) {
	l, err := clockLayout(s)
	if err != nil {
		return "", err
	}
	format := clockTimeFormat(l.HourCycle())
	if !dayOf(at).Equal(dayOf(now.In(at.Location()))) {
		format = "%a " + format
	}
	return names.Format(format, at)
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func (c *clockCard) tick() {
	if c.closed {
		return
	}
	now := c.k.clock.Now()
	date, clock, err := formatClock(c.names, c.k.store, now)
	if err != nil {
		date = err.Error()
	}
	c.date.SetText(date)
	c.clock.SetText(clock)
	next := now.Truncate(time.Minute).Add(time.Minute).Sub(now)
	c.stop = c.k.clock.AfterFunc(next, func() { c.k.invoke(c.tick) })
}

func zoneLine(zone string, now time.Time) (string, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", fmt.Errorf("read the time zone %q: %w", zone, err)
	}
	return zone + zoneDetailSep + utcOffset(now.In(loc)), nil
}

func syncKey(ntp, synchronized bool) string {
	switch {
	case !ntp:
		return "settings-date-sync-manual"
	case synchronized:
		return "settings-date-sync-done"
	default:
		return "settings-date-sync-waiting"
	}
}

func (c *clockCard) showSystem(zone string, ntp, synchronized bool) {
	line, err := zoneLine(zone, c.k.clock.Now())
	if err != nil {
		line = err.Error()
	}
	c.zone.SetText(line)
	c.zone.SetVisible(zone != "")
	c.sync.SetText(locale.T().Get(syncKey(ntp, synchronized)))
	c.sync.SetVisible(true)
}

func (c *clockCard) retick() {
	if c.stop != nil {
		c.stop()
	}
	c.tick()
}

func (c *clockCard) close() {
	c.closed = true
	if c.stop != nil {
		c.stop()
	}
}

type closingBox struct {
	*widget.Box
	release func()
}

func (c *closingBox) close() { c.release() }

func fixedChoices(items []choice) lister {
	return func() []choice { return slices.Clone(items) }
}

var localeSource = glibclocale.System(systemHost.env)

func listLocales() ([]string, error) {
	names, err := localeSource.List()
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(names, func(name string) bool {
		return name == "C" || name == "POSIX" || strings.HasPrefix(name, "C.")
	}), nil
}

func localeIndex(names []string, name string) int {
	want := glibclocale.Normalize(name)
	return slices.IndexFunc(names, func(n string) bool { return glibclocale.Normalize(n) == want })
}

type formatsPreview struct{ date, number, price, units string }

type formatLocales struct{ time, numeric, monetary, measurement, paper string }

type paperSize struct {
	width, height uint32
	name          string
}

var paperSizes = []paperSize{
	{210, 297, "A4"},
	{297, 420, "A3"},
	{148, 210, "A5"},
	{216, 279, "settings-region-paper-letter"},
	{216, 356, "settings-region-paper-legal"},
}

func paperName(w, h uint32) string {
	t := locale.T()
	i := slices.IndexFunc(paperSizes, func(s paperSize) bool { return s.width == w && s.height == h })
	switch {
	case i < 0:
		return t.Get("settings-region-paper-millimetres", i18n.Str("width", strconv.FormatUint(uint64(w), 10)), i18n.Str("height", strconv.FormatUint(uint64(h), 10)))
	case strings.HasPrefix(paperSizes[i].name, "settings-"):
		return t.Get(paperSizes[i].name)
	}
	return paperSizes[i].name
}

func measurementName(m glibclocale.Measurement) string {
	if m == glibclocale.USCustomary {
		return locale.T().Get("settings-region-measurement-us")
	}
	return locale.T().Get("settings-region-measurement-metric")
}

func previewUnits(measurementLocale, paperLocale string) (string, error) {
	m, err := localeSource.Measurement(measurementLocale)
	if err != nil {
		return "", err
	}
	p, err := localeSource.Paper(paperLocale)
	if err != nil {
		return "", err
	}
	return locale.T().Get("settings-region-preview-units", i18n.Str("measurement", measurementName(m)), i18n.Str("paper", paperName(p.Millimetres()))), nil
}

func previewFormats(l formatLocales, at time.Time) (formatsPreview, error) {
	lt, err := localeSource.Time(l.time)
	if err != nil {
		return formatsPreview{}, err
	}
	date, err := lt.Format("%c", at)
	if err != nil {
		return formatsPreview{}, err
	}
	numeric, err := localeSource.Numeric(l.numeric)
	if err != nil {
		return formatsPreview{}, err
	}
	monetary, err := localeSource.Monetary(l.monetary)
	if err != nil {
		return formatsPreview{}, err
	}
	units, err := previewUnits(l.measurement, l.paper)
	if err != nil {
		return formatsPreview{}, err
	}
	return formatsPreview{date: date, number: numeric.Format(previewNumber, previewScale), price: monetary.Format(previewPrice, previewScale, numeric), units: units}, nil
}

func localeConfPath(h host) (string, error) {
	config, err := h.configHome()
	if err != nil {
		return "", fmt.Errorf("find the user's %s: %w", localeConfName, err)
	}
	return filepath.Join(config, localeConfName), nil
}

func readLocaleConf(path string) (map[string]string, []string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the user's locale.conf
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	values := map[string]string{}
	for _, l := range lines {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok && !strings.HasPrefix(k, "#") {
			values[k] = strings.Trim(v, `"`)
		}
	}
	return values, lines, nil
}

func writeLocaleConf(path, key, value string) error {
	_, lines, err := readLocaleConf(path)
	if err != nil {
		return err
	}
	at := slices.IndexFunc(lines, func(l string) bool {
		k, _, ok := strings.Cut(strings.TrimSpace(l), "=")
		return ok && k == key
	})
	switch {
	case value == "" && at >= 0:
		lines = slices.Delete(lines, at, at+1)
	case value == "":
	case at >= 0:
		lines[at] = key + "=" + value
	default:
		lines = append(lines, key+"="+value)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // ~/.config
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	text := strings.Join(lines, "\n")
	if text != "" {
		text += "\n"
	}
	if err := atomicfile.WriteFile(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func regionFormats(k *kit, lang func() string) (widget.Widget, func()) {
	t := locale.T()
	conf, err := localeConfPath(systemHost)
	if err != nil {
		return wrapped(k.label(err.Error(), "setting-problem", css.Caption, css.Error)), func() {}
	}
	preview := wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	units := wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	formatsProblem := wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	formatsProblem.SetVisible(false)
	head := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
	head.SetElement("item")
	head.Append(preview, false)
	head.Append(units, false)
	head.Append(formatsProblem, false)
	formats := widget.NewBox(widget.Column, 0, 0)
	formats.Append(head, false)
	formats.Append(widget.NewSeparator(widget.Horizontal), false)
	showPreview := func() {
		values, _, err := readLocaleConf(conf)
		if err != nil {
			formatsProblem.SetText(err.Error())
			formatsProblem.SetVisible(true)
			return
		}
		pick := func(key string) string { return cmp.Or(values[key], lang(), "C") }
		pv, err := previewFormats(formatLocales{pick("LC_TIME"), pick("LC_NUMERIC"), pick("LC_MONETARY"), pick("LC_MEASUREMENT"), pick("LC_PAPER")}, time.Now())
		units.SetVisible(err == nil)
		if err != nil {
			preview.SetText(err.Error())
			return
		}
		preview.SetText(t.Get("settings-region-preview", i18n.Str("date", pv.date), i18n.Str("number", pv.number), i18n.Str("price", pv.price)))
		units.SetText(pv.units)
	}
	async(k, func() localesListing {
		names, err := listLocales()
		return localesListing{names, err}
	}, func(l localesListing) {
		if l.err != nil {
			head.Append(wrapped(k.label(l.err.Error(), css.Error, css.Caption)), false)
			return
		}
		values, _, err := readLocaleConf(conf)
		if err != nil {
			formatsProblem.SetText(err.Error())
			formatsProblem.SetVisible(true)
		}
		rows := widget.NewItemGroup()
		names, shown := localeLabels(l.names)
		l.names = names
		labels := append([]string{t.Get("settings-region-same")}, shown...)
		for _, key := range userFormats {
			dd := widget.NewDropdown(k.face, k.textPx(), labels, max(localeIndex(l.names, values[key])+1, 0))
			dd.SetAccessibleName(key)
			dd.OnSelect = func(i int) {
				value := ""
				if i > 0 {
					value = l.names[i-1]
				}
				err := writeLocaleConf(conf, key, value)
				formatsProblem.SetVisible(err != nil)
				if err != nil {
					formatsProblem.SetText(err.Error())
				}
				showPreview()
			}
			rows.Append(panelRow(k, t.Get("settings-region-"+strings.ToLower(strings.ReplaceAll(key, "_", "-"))), "", dd))
		}
		formats.Append(rows, false)
		showPreview()
	})
	return formats, showPreview
}

func regionFooter(k *kit) widget.Widget {
	t := locale.T()
	p := newSysPanel(k, localeName, localePath)
	lang := func() string {
		for _, kv := range p.props.strs("Locale") {
			if v, ok := strings.CutPrefix(kv, "LANG="); ok {
				return v
			}
		}
		return ""
	}
	formats, showPreview := regionFormats(k, lang)
	p.open(func(context.Context, sysService) (any, error) {
		names, err := listLocales()
		return regionLists{names: names, keymaps: systemHost.consoleKeymaps()}, err
	}, func(extra any) {
		lists, _ := extra.(regionLists)
		names, shown := localeLabels(lists.names)
		keymaps := lists.keymaps
		language := widget.NewDropdown(k.face, k.textPx(), shown, max(localeIndex(names, lang()), 0))
		language.SetAccessibleName(t.Get("settings-region-language"))
		syncing := false
		language.OnSelect = func(i int) {
			if !syncing && i >= 0 && i < len(names) {
				p.do("SetLocale", []string{"LANG=" + names[i]}, interactive)
			}
		}
		keymap := newChoiceEditor(k, slot{
			get: func() any { return p.props.str("VConsoleKeymap") },
			set: func(v any) error {
				s, _ := v.(string)
				p.do("SetVConsoleKeyboard", s, "", false, interactive)
				return nil
			},
			unset: func() {},
		}, choiceSpec{list: fixedChoices(keymaps), known: func(current string, items []choice) bool { return hasValue(items, current) }})
		layout := k.label("", "setting-description", css.Caption, css.DimLabel)
		copyLayout := k.button(k.label(t.Get("settings-region-copy-layout")), func() {
			kb := wmKeyboard(k)
			p.do("SetX11Keyboard", kb.layout, kb.model, kb.variant, kb.options, false, interactive)
		}, css.Secondary)
		copyLayout.SetEnabled(k.parts.wm)
		rows := widget.NewItemGroup()
		rows.Append(panelRow(k, t.Get("settings-region-language"), t.Attr("settings-region-language", "description"), language))
		rows.Append(panelRow(k, t.Get("settings-region-keymap"), t.Attr("settings-region-keymap", "description"), keymap))
		rows.Append(panelRow(k, t.Get("settings-region-layout"), t.Attr("settings-region-layout", "description"), layout, copyLayout))
		refresh := func() {
			syncing = true
			defer func() { syncing = false }()
			if i := localeIndex(names, lang()); i >= 0 && language.Selected() != i {
				language.SetSelected(i)
			}
			keymap.refresh()
			layout.SetText(strings.Join(slices.DeleteFunc([]string{p.props.str("X11Layout"), p.props.str("X11Variant"), p.props.str("X11Options")}, func(s string) bool { return s == "" }), " · "))
			showPreview()
		}
		p.refresh = refresh
		refresh()
		p.body.Append(rows, false)
	})
	content := sectionStack()
	content.Append(newSection(k, "settings-section-region-formats", formats), false)
	content.Append(newSection(k, "settings-section-region-system", p), false)
	content.Append(linksSection(k, "settings-section-related", regionLinks), false)
	return &closingBox{content, p.close}
}

type regionLists struct {
	names   []string
	keymaps []choice
}

type localesListing struct {
	names []string
	err   error
}

type keyboardNames struct{ layout, model, variant, options string }

func wmKeyboard(k *kit) keyboardNames {
	w, err := k.wmBackend()
	if err != nil {
		return keyboardNames{}
	}
	get := func(key string) string { s, _ := w.value("input.keyboard." + key).(string); return s }
	return keyboardNames{layout: get("layout"), model: get("model"), variant: get("variant"), options: get("options")}
}
