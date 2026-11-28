package settings

import (
	"cmp"
	"context"
	"log"
	"math"
	"os"
	"slices"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/shell/service/privacy"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	soundID      = "sound"
	soundTimeout = 5 * time.Second
	balanceRange = 100
)

var openPulse = func(ctx context.Context) (*pulse.Service, error) { return pulse.Connect(ctx, "") }

func soundPage(*config.Config) pageSpec {
	return pageSpec{
		id: soundID, navKey: "settings-nav-sound", icon: "ld-audio-lines-symbolic",
		header: "settings-page-sound",
		lead:   soundLead,
		panel: []panelSetting{
			{"sound.output-device", "settings-sound-output-device", "settings-section-sound-output"},
			{"sound.output-volume", "settings-sound-volume", "settings-section-sound-output"},
			{"sound.balance", "settings-sound-balance", "settings-section-sound-output"},
			{"sound.test", "settings-sound-test", "settings-section-sound-output"},
			{"sound.input-device", "settings-sound-input-device", "settings-section-sound-input"},
			{"sound.input-level", "settings-sound-level", "settings-section-sound-input"},
			{"sound.apps", "settings-section-sound-apps", "settings-section-sound-apps"},
			{"sound.recording", "settings-section-sound-recording", "settings-section-sound-recording"},
		},
		sections: []sectionSpec{{title: "settings-section-sound-alerts", rows: []rowSpec{
			field("modules.notifications.sounds"),
			field("modules.notifications.sound-volume"),
			field("modules.notifications.sound-default", soundNameRow),
		}}},
	}
}

type soundConnection struct {
	svc *pulse.Service
	err error
}

type soundPanel struct {
	*widget.Box
	k       *kit
	svc     *pulse.Service
	stop    func()
	capture *pulse.Capture
	// failure is why the last change failed, nil once one did not.
	failure error
	// connected are the sections that show only once the sound server
	// answered.
	connected []*widget.Box
	output    *deviceControls
	input     *deviceControls
	meter     *levelMeter
	playing   *streamList
	recording *streamList
	ticked    atomic.Bool
	closed    bool
	syncing   bool
}

func soundLead(k *kit) widget.Widget {
	t := locale.T()
	p := &soundPanel{Box: sectionStack(), k: k}
	p.output = newDeviceControls(p, pulse.DeviceOutput)
	p.input = newDeviceControls(p, pulse.DeviceInput)
	p.meter = newLevelMeter(k)
	p.meter.SetAccessibleName(t.Get("settings-sound-level"))
	p.input.rows.Append(k.row(t.Get("settings-sound-level"), "").withControl(p.meter))
	p.playing = newStreamList(p, playbackStreams)
	p.recording = newStreamList(p, recordingStreams)
	p.Append(newSection(k, "settings-section-sound-output", p.output.body), false)
	p.connected = []*widget.Box{
		newSection(k, "settings-section-sound-input", p.input.body),
		newSection(k, "settings-section-sound-apps", p.playing.body),
		newSection(k, "settings-section-sound-recording", p.recording.body),
	}
	for _, s := range p.connected {
		s.SetVisible(false)
		p.Append(s, false)
	}
	p.output.state.Loading(t.Get("settings-sound-connecting"))
	p.connect()
	return p
}

func (p *soundPanel) connect() {
	async(p.k, func() soundConnection {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		svc, err := openPulse(ctx)
		return soundConnection{svc, err}
	}, func(c soundConnection) {
		if c.err != nil {
			log.Printf("settings: sound: %v", c.err)
			p.output.state.Unavailable("ld-volume-x-symbolic", locale.T().Get("settings-sound-unreachable"), c.err)
			return
		}
		if p.closed {
			_ = c.svc.Close()
			return
		}
		p.svc = c.svc
		ticks, stop, err := c.svc.Subscribe(context.Background())
		if err != nil {
			p.fail(err)
			return
		}
		p.stop = stop
		go func() {
			for range ticks {
				if p.ticked.CompareAndSwap(false, true) {
					p.k.invoke(p.tick)
				}
			}
		}()
		capture, err := c.svc.Capture(pulse.CaptureDefaultInput(), meterSpec, p.meter.feed)
		if err != nil {
			p.fail(err)
		} else {
			p.capture = capture
		}
		p.render()
	})
}

func (p *soundPanel) tick() {
	p.ticked.Store(false)
	p.render()
}

func (p *soundPanel) close() {
	p.closed = true
	if p.capture != nil {
		p.capture.Close()
		p.capture = nil
	}
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
	if p.svc != nil {
		_ = p.svc.Close()
		p.svc = nil
	}
}

// fail shows why a change or the following of the sound server failed,
// or clears what failed before for nil.
func (p *soundPanel) fail(err error) {
	if err != nil {
		log.Printf("settings: sound: %v", err)
	}
	if err == nil && p.failure == nil {
		return
	}
	p.failure = err
	p.render()
}

func (p *soundPanel) act(change func(context.Context, *pulse.Service) error) {
	svc := p.svc
	if svc == nil {
		return
	}
	async(p.k, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		return change(ctx, svc)
	}, p.fail)
}

func (p *soundPanel) write(change func(context.Context, *pulse.Service) error) func() {
	svc := p.svc
	if svc == nil {
		return nil
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		if err := change(ctx, svc); err != nil {
			p.k.invoke(func() { p.fail(err) })
		}
	}
}

func (p *soundPanel) render() {
	if p.svc == nil {
		return
	}
	p.syncing = true
	defer func() { p.syncing = false }()
	for _, s := range p.connected {
		s.SetVisible(true)
	}
	cards := p.svc.Cards()
	outputs := p.svc.OutputDevices()
	outDevices := make([]pulse.Device, len(outputs))
	for i, d := range outputs {
		outDevices[i] = d.Device
	}
	out, hasOut := p.svc.DefaultOutput()
	p.output.show(outDevices, out.Device, hasOut, cards)
	in, hasIn := p.svc.DefaultInput()
	inputs := p.svc.InputDevices()
	var inDevices, microphones []pulse.Device
	for _, d := range inputs {
		if !d.IsMonitor() {
			microphones = append(microphones, d.Device)
		}
		if !d.IsMonitor() || (hasIn && d.Key == in.Key) {
			inDevices = append(inDevices, d.Device)
		}
	}
	p.input.show(inDevices, in.Device, hasIn, cards)
	p.playing.show(p.svc.PlaybackStreams(), outDevices)
	p.recording.show(microphoneStreams(p.svc.RecordingStreams(), inputs), microphones)
	if p.failure != nil {
		p.output.state.Error(locale.T().Get("settings-sound-failed"), p.failure, nil)
	}
}

func microphoneStreams(streams []pulse.AudioStream, inputs []pulse.InputDevice) []pulse.AudioStream {
	users := privacy.MicUsers(streams, inputs, intconv.Clamp[uint32](os.Getpid()))
	return slices.DeleteFunc(slices.Clone(streams), func(s pulse.AudioStream) bool {
		return !slices.ContainsFunc(users, func(u privacy.MicUser) bool { return u.Stream == s.Key })
	})
}

type streamKind struct {
	device              pulse.DeviceType
	glyphs              muteGlyphs
	emptyIcon, emptyKey string
	routeKey            string
}

var (
	playbackStreams  = streamKind{device: pulse.DeviceOutput, glyphs: speakerGlyphs, emptyIcon: "ld-volume-x-symbolic", emptyKey: "settings-sound-no-apps", routeKey: "settings-sound-app-output"}
	recordingStreams = streamKind{device: pulse.DeviceInput, glyphs: micGlyphs, emptyIcon: "ld-mic-off-symbolic", emptyKey: "settings-sound-no-recording", routeKey: "settings-sound-app-input"}
)

type streamList struct {
	p     *soundPanel
	kind  streamKind
	body  *widget.Box
	group *widget.ItemGroup
	state *cardState
	rows  map[pulse.StreamKey]*appRow
	keys  []pulse.StreamKey
}

func newStreamList(p *soundPanel, kind streamKind) *streamList {
	l := &streamList{p: p, kind: kind, body: gapBox(widget.Column), group: widget.NewItemGroup(), rows: map[pulse.StreamKey]*appRow{}}
	l.state = p.k.cardState()
	l.body.Append(l.state.lone(), false)
	l.body.Append(l.group, false)
	return l
}

func (l *streamList) show(streams []pulse.AudioStream, devices []pulse.Device) {
	keys := make([]pulse.StreamKey, len(streams))
	for i, s := range streams {
		keys[i] = s.Key
	}
	if !slices.Equal(keys, l.keys) {
		l.group.Clear()
		clear(l.rows)
		for _, s := range streams {
			row := newAppRow(l.p, l.kind, s.Key)
			l.rows[s.Key] = row
			l.group.Append(row)
		}
		l.keys = keys
	}
	if len(streams) == 0 {
		l.state.Empty(l.kind.emptyIcon, locale.T().Get(l.kind.emptyKey))
	} else {
		l.state.Clear()
	}
	l.group.SetVisible(len(streams) > 0)
	for _, s := range streams {
		l.rows[s.Key].show(s, devices)
	}
}

type deviceControls struct {
	p          *soundPanel
	kind       pulse.DeviceType
	body       *widget.Box
	state      *cardState
	noneIcon   string
	noneKey    string
	rows       *widget.ItemGroup
	devices    *widget.Dropdown
	keys       []pulse.DeviceKey
	more       *widget.Button
	ports      []deviceOption
	port       string
	profiles   []deviceOption
	profile    string
	card       native.CardIndex
	volume     *widgets.DebouncedSlider
	mute       *muteToggle
	balance    *widgets.DebouncedSlider
	balanceRow *settingRow
	tests      *widget.Grid
	testsRow   *settingRow
	testMap    native.ChannelMap
	current    pulse.Device
}

// balanceLabel reads a balance as the side it leans to and how far,
// or 0 at the centre.
func balanceLabel(b float64) string {
	n := int64(math.Round(math.Abs(b)))
	switch {
	case n == 0:
		return "0"
	case b < 0:
		return locale.T().Get("settings-sound-balance-left", i18n.Int("n", n))
	}
	return locale.T().Get("settings-sound-balance-right", i18n.Int("n", n))
}

func newDeviceControls(p *soundPanel, kind pulse.DeviceType) *deviceControls {
	t := locale.T()
	k := p.k
	c := &deviceControls{p: p, kind: kind, body: gapBox(widget.Column), state: k.cardState(), rows: widget.NewItemGroup()}
	deviceKey, muteKey, glyphs := "settings-sound-input-device", "settings-sound-mute-input", micGlyphs
	c.noneIcon, c.noneKey = recordingStreams.emptyIcon, "settings-sound-no-input"
	if kind == pulse.DeviceOutput {
		deviceKey, muteKey, glyphs = "settings-sound-output-device", "settings-sound-mute-output", speakerGlyphs
		c.noneIcon, c.noneKey = playbackStreams.emptyIcon, "settings-sound-no-output"
	}
	c.rows.SetVisible(false)
	c.body.Append(c.state.lone(), false)
	c.body.Append(c.rows, false)
	c.devices = widget.NewDropdown(k.face, k.textPx(), nil, 0)
	c.devices.SetAccessibleName(t.Get(deviceKey))
	c.devices.OnSelect = func(i int) {
		if p.syncing || i >= len(c.keys) {
			return
		}
		key := c.keys[i]
		p.act(func(ctx context.Context, svc *pulse.Service) error { return svc.SetDefault(ctx, key) })
	}
	c.more = k.iconButton("ld-ellipsis-symbolic", "settings-sound-device-options", c.openMenu)
	c.more.SetAccessibleName(t.Get("settings-sound-device-options"))
	c.rows.Append(k.row(t.Get("settings-sound-device"), "").withControl(c.devices).withTrailing(c.more))
	c.volume = k.valueSlider(0, 100, unitPercent.format(0))
	c.volume.Knob.SetAccessibleName(t.Get("settings-sound-volume"))
	c.volume.Write = func(v float64) func() {
		key, levels := c.current.Key, scaledTo(c.current.Volume.Levels(), v/100)
		return p.write(func(ctx context.Context, svc *pulse.Service) error {
			return svc.SetDeviceVolume(ctx, key, pulse.NewVolume(levels))
		})
	}
	c.mute = newMuteToggle(k, glyphs, func(on bool) {
		if p.syncing {
			return
		}
		key := c.current.Key
		p.act(func(ctx context.Context, svc *pulse.Service) error { return svc.SetDeviceMute(ctx, key, on) })
	})
	c.mute.name(t.Get(muteKey))
	c.rows.Append(k.row(t.Get("settings-sound-volume"), "").withActions(c.mute).withControl(c.volume))
	c.balance = k.valueSlider(-balanceRange, balanceRange, balanceLabel)
	c.balance.Knob.SetAccessibleName(t.Get("settings-sound-balance"))
	c.balance.Write = func(b float64) func() {
		key, levels := c.current.Key, withBalance(c.current.Volume.Levels(), c.current.ChannelMap, b/balanceRange)
		return p.write(func(ctx context.Context, svc *pulse.Service) error {
			return svc.SetDeviceVolume(ctx, key, pulse.NewVolume(levels))
		})
	}
	c.balanceRow = k.row(t.Get("settings-sound-balance"), "").withControl(c.balance)
	c.rows.Append(c.balanceRow)
	if kind == pulse.DeviceOutput {
		c.tests = widget.NewGrid(widget.Current().Spacing, widget.Current().Spacing).SetColumnHomogeneous(true)
		c.tests.AddClass("sound-speakers")
		c.testsRow = k.keyRow("settings-sound-test").withControl(c.tests)
		c.rows.Append(c.testsRow)
	}
	return c
}

func (c *deviceControls) show(devices []pulse.Device, current pulse.Device, has bool, cards []pulse.Card) {
	t := locale.T()
	c.rows.SetVisible(has)
	if !has {
		c.state.Empty(c.noneIcon, t.Get(c.noneKey))
		c.keys = nil
		return
	}
	c.state.Clear()
	c.current = current
	c.keys = showSoundDevices(c.devices, devices, c.kind, current.Key)
	c.port = current.ActivePort
	c.ports = c.ports[:0]
	for _, port := range current.Ports {
		label := cmp.Or(port.Description, port.Name)
		if !port.Available {
			label = t.Get("settings-sound-unplugged", i18n.Str("name", label))
		}
		c.ports = append(c.ports, deviceOption{name: port.Name, label: label})
	}
	c.showProfiles(current, cards)
	c.more.SetVisible(len(c.ports) > 1 || len(c.profiles) > 1)
	c.volume.Set(peakLevel(current.Volume.Levels()) * 100)
	c.mute.show(current.Muted)
	b, stereo := balanceOf(current.Volume.Levels(), current.ChannelMap)
	c.balanceRow.SetVisible(stereo)
	c.balance.Set(b * balanceRange)
	if c.tests != nil {
		c.showTests(current)
	}
}

func (c *deviceControls) showProfiles(current pulse.Device, cards []pulse.Card) {
	t := locale.T()
	c.profiles, c.profile = c.profiles[:0], ""
	index, ok := current.Card.Get()
	if !ok {
		return
	}
	i := slices.IndexFunc(cards, func(card pulse.Card) bool { return uint32(card.Index) == index })
	if i < 0 {
		return
	}
	card := cards[i]
	c.card, c.profile = card.Index, card.ActiveProfile
	for _, profile := range card.Profiles {
		label := cmp.Or(profile.Description, profile.Name)
		if !profile.Available {
			label = t.Get("settings-sound-unavailable", i18n.Str("name", label))
		}
		c.profiles = append(c.profiles, deviceOption{name: profile.Name, label: label})
	}
}

func (c *deviceControls) openMenu() {
	key, card := c.current.Key, c.card
	items := deviceMenu(c.ports, c.port, c.profiles, c.profile,
		func(port string) {
			c.p.act(func(ctx context.Context, svc *pulse.Service) error { return svc.SetDevicePort(ctx, key, port) })
		},
		func(profile string) {
			c.p.act(func(ctx context.Context, svc *pulse.Service) error { return svc.SetCardProfile(ctx, card, profile) })
		})
	widget.OpenMenu(c.more, c.more, c.p.k.face, c.p.k.textPx(), items, widget.MenuOptions{Gap: widget.PopoverGap})
}

type deviceOption struct{ name, label string }

func deviceMenu(ports []deviceOption, port string, profiles []deviceOption, profile string, onPort, onProfile func(string)) []widget.MenuItem {
	t := locale.T()
	var items []widget.MenuItem
	group := func(titleKey string, options []deviceOption, active string, choose func(string)) {
		if len(options) < 2 {
			return
		}
		if len(items) > 0 {
			items = append(items, widget.MenuSeparator())
		}
		items = append(items, widget.MenuItem{Label: t.Get(titleKey), Kind: widget.ItemHeader})
		for _, o := range options {
			name := o.name
			items = append(items, widget.MenuItem{Label: o.label, Kind: widget.ItemRadio, Group: titleKey, Checked: name == active, OnClick: func() { choose(name) }})
		}
	}
	group("settings-sound-port", ports, port, onPort)
	group("settings-sound-profile", profiles, profile, onProfile)
	return items
}

func (c *deviceControls) showTests(current pulse.Device) {
	c.testsRow.SetVisible(len(current.ChannelMap) > 0)
	if slices.Equal(c.testMap, current.ChannelMap) {
		return
	}
	c.testMap = slices.Clone(current.ChannelMap)
	c.showSpeakers(c.testMap)
}

func setChoices(d *widget.Dropdown, items []string, selected int) {
	if slices.Equal(d.Items(), items) && d.Selected() == selected {
		return
	}
	d.SetItems(items, selected)
}

type appRow struct {
	*settingRow
	kind   streamKind
	icon   *widget.Icon
	route  *widget.Dropdown
	routes []pulse.DeviceKey
	volume *widgets.DebouncedSlider
	mute   *muteToggle
	levels []float64
}

func newAppRow(p *soundPanel, kind streamKind, key pulse.StreamKey) *appRow {
	k := p.k
	r := &appRow{kind: kind, icon: widget.NewThemeIconFallback(appFallbackIcon, appIconPx)}
	r.icon.AddClass("sound-app-icon")
	r.route = widget.NewDropdown(k.face, k.textPx(), nil, 0)
	r.route.OnSelect = func(i int) {
		if p.syncing || i >= len(r.routes) {
			return
		}
		device := r.routes[i]
		p.act(func(ctx context.Context, svc *pulse.Service) error { return svc.MoveStream(ctx, key, device) })
	}
	r.volume = k.valueSlider(0, 100, unitPercent.format(0))
	r.volume.Write = func(v float64) func() {
		levels := scaledTo(r.levels, v/100)
		return p.write(func(ctx context.Context, svc *pulse.Service) error {
			return svc.SetStreamVolume(ctx, key, pulse.NewVolume(levels))
		})
	}
	r.mute = newMuteToggle(k, kind.glyphs, func(on bool) {
		if p.syncing {
			return
		}
		p.act(func(ctx context.Context, svc *pulse.Service) error { return svc.SetStreamMute(ctx, key, on) })
	})
	r.settingRow = k.row("", "").withActions(r.route, r.mute).withControl(r.volume)
	r.AddClass("sound-app")
	r.SetMedia(r.icon, true)
	return r
}

func (r *appRow) show(s pulse.AudioStream, devices []pulse.Device) {
	t := locale.T()
	name := cmp.Or(s.ApplicationName, s.Binary, s.Name)
	r.SetTitle(name)
	r.icon.SetThemeName(streamIcon(s))
	r.SetDescription(cmp.Or(s.Media.Title, s.Name))
	r.showRoutes(s, devices, name)
	r.volume.Knob.SetAccessibleName(t.Get("settings-sound-app-volume", i18n.Str("app", name)))
	r.mute.name(t.Get("settings-sound-app-mute", i18n.Str("app", name)))
	r.levels = s.Volume.Levels()
	r.volume.Knob.SetEnabled(s.HasVolume && s.VolumeWritable)
	r.volume.Set(peakLevel(r.levels) * 100)
	r.mute.show(s.Muted)
}

func (r *appRow) showRoutes(s pulse.AudioStream, devices []pulse.Device, name string) {
	r.route.SetAccessibleName(locale.T().Get(r.kind.routeKey, i18n.Str("app", name)))
	r.route.SetVisible(len(devices) > 1)
	r.routes = showSoundDevices(r.route, devices, r.kind.device, pulse.DeviceKey{Index: s.DeviceIndex, Type: r.kind.device})
}

func streamIcon(s pulse.AudioStream) string {
	return cmp.Or(s.Media.IconName, appFallbackIcon)
}

type muteGlyphs struct{ sounding, muted string }

var (
	speakerGlyphs = muteGlyphs{sounding: "ld-volume-2-symbolic", muted: "ld-volume-x-symbolic"}
	micGlyphs     = muteGlyphs{sounding: "ld-mic-symbolic", muted: "ld-mic-off-symbolic"}
)

type muteToggle struct {
	*widget.ToggleButton
	glyph  *widget.Icon
	glyphs muteGlyphs
}

func newMuteToggle(k *kit, glyphs muteGlyphs, onMuted func(bool)) *muteToggle {
	m := &muteToggle{glyph: k.icon(glyphs.sounding), glyphs: glyphs}
	m.ToggleButton = widget.NewToggleButton(m.glyph, 0, 0)
	m.AddClass(roleIcon.classes()...)
	m.AddClass("sound-mute")
	m.OnToggled = func(muted bool) {
		m.glyph.SetThemeName(m.glyphFor(muted))
		onMuted(muted)
	}
	return m
}

func (m *muteToggle) glyphFor(muted bool) string {
	if muted {
		return m.glyphs.muted
	}
	return m.glyphs.sounding
}

func (m *muteToggle) name(text string) {
	m.SetAccessibleName(text)
	m.SetTooltip(text)
}

func (m *muteToggle) show(muted bool) {
	m.SetActive(muted)
	m.glyph.SetThemeName(m.glyphFor(muted))
}
