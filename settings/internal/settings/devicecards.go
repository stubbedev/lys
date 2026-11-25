package settings

import (
	"errors"
	"log"
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

const devicesPath = "input.devices"

// deviceKind is what a page lists in input.devices: the connected devices
// it takes, the class an unconnected one is shown in, and the rows of a
// device of a class with the section and the key of the device they
// override.
type deviceKind struct {
	key string
	// icon is the glyph of the list's state.
	icon   string
	lists  func(d wmipc.Device) bool
	absent wmipc.DeviceClass
	rows   func(class wmipc.DeviceClass) (section string, within []string, rows []rowSpec)
	// classed captions a connected device's card with its class.
	classed bool
}

var pointerDevices = deviceKind{
	key:    "wm-input-devices",
	icon:   "ld-mouse-symbolic",
	lists:  func(d wmipc.Device) bool { return d.Class != wmipc.ClassNone },
	absent: wmipc.ClassPointer,
	rows: func(class wmipc.DeviceClass) (string, []string, []rowSpec) {
		return "input." + string(class), nil, wmPointerRows(class)
	},
	classed: true,
}

var keyboardDevices = deviceKind{
	key:    "wm-input-devices-keyboard",
	icon:   "ld-keyboard-symbolic",
	lists:  func(d wmipc.Device) bool { return slices.Contains(d.Capabilities, wmipc.CapKeyboard) },
	absent: wmipc.ClassNone,
	rows: func(wmipc.DeviceClass) (string, []string, []rowSpec) {
		return "input.keyboard", []string{"keyboard"}, append(wmKeyboardLayoutRows(), wmKeyboardModelRows()...)
	},
}

// wmDevicesRow is input.devices as cards of kind's devices: each one
// connected, and each one not connected that sets a key of kind's rows.
func wmDevicesRow(kind deviceKind) rowSpec {
	return wmField(devicesPath, withKey(kind.key), func(r *rowSpec) {
		r.fullWidth, r.action = true, true
		r.editor = func(k *kit, _ slot, _ config.FieldMeta) control { return newDeviceCards(k, kind) }
	})
}

// deviceCards lists a page's devices, each a card of its overrides that
// opens to the rows of its class.
type deviceCards struct {
	*widget.Box
	k         *kit
	kind      deviceKind
	state     *cardState
	list      *widget.Box
	reload    *widget.Button
	read      bool
	connected []wmipc.Device
	err       error
	cards     []*deviceCard
}

type deviceCard struct {
	name      string
	class     wmipc.DeviceClass
	connected bool
	scope     deviceScope
	specs     []rowSpec
	root      *widget.Box
	body      *widget.Box
	rows      []*fieldRow
	disclose  *disclosure
}

type devicesRead struct {
	devices []wmipc.Device
	err     error
}

func newDeviceCards(k *kit, kind deviceKind) *deviceCards {
	c := &deviceCards{Box: gapBox(widget.Column), k: k, kind: kind}
	c.AddClass(cardFormClasses.editor)
	c.state = k.cardState()
	c.list = gapBox(widget.Column)
	c.Append(c.state.lone(), false)
	c.Append(c.list, false)
	c.reload = k.reloadButton("wm-devices-reload", c.load)
	c.AppendAligned(c.reload, false, widget.AlignStart)
	c.rebuild()
	c.load()
	return c
}

// load asks lys-wm which devices are connected and lists them.
func (c *deviceCards) load() {
	c.reload.SetEnabled(false)
	run := c.k.wm.run
	async(c.k, func() devicesRead {
		devices, err := wmstate.Devices(run)
		return devicesRead{devices, err}
	}, func(r devicesRead) {
		c.read, c.connected, c.err = true, slices.DeleteFunc(r.devices, func(d wmipc.Device) bool { return !c.kind.lists(d) }), r.err
		c.reload.SetEnabled(true)
		c.rebuild()
	})
}

// shown are the cards the list holds: the connected devices in lys-wm's
// order, each name once, then the devices not connected that set a key
// of their rows, by name.
func (c *deviceCards) shown() []*deviceCard {
	var out []*deviceCard
	for _, d := range c.connected {
		if !slices.ContainsFunc(out, func(cd *deviceCard) bool { return cd.name == d.Name }) {
			out = append(out, c.card(d.Name, d.Class, true))
		}
	}
	configured := asMap(c.k.wm.valueAt(wmPath(devicesPath)))
	for _, name := range slices.Sorted(maps.Keys(configured)) {
		if slices.ContainsFunc(out, func(cd *deviceCard) bool { return cd.name == name }) {
			continue
		}
		if cd := c.card(name, c.kind.absent, false); cd.scope.sets(cd.specs) {
			out = append(out, cd)
		}
	}
	return out
}

func (c *deviceCards) card(name string, class wmipc.DeviceClass, connected bool) *deviceCard {
	section, within, rows := c.kind.rows(class)
	scope := newDeviceScope(c.k.wm, section, name, within...)
	allowed := slices.DeleteFunc(rows, func(r rowSpec) bool {
		_, ok := c.k.wm.schema.Field(scope.keys(r.path))
		return !ok
	})
	return &deviceCard{name: name, class: class, connected: connected, scope: scope, specs: allowed}
}

func (c *deviceCards) rebuild() {
	c.list.Clear()
	c.cards = c.shown()
	for _, cd := range c.cards {
		c.list.Append(c.cardWidget(cd), false)
	}
	c.showState()
}

// showState says why the list holds only the configured devices, or
// that it holds none; a failure to read them carries the reload as its
// retry, in place of the reload button.
func (c *deviceCards) showState() {
	t := locale.T()
	c.reload.SetVisible(true)
	switch {
	case !c.read:
		c.state.Loading(t.Get("wm-devices-reading"))
	case errors.Is(c.err, wmstate.ErrNotRunning):
		c.state.Unavailable(c.kind.icon, t.Get("wm-devices-not-running"), nil)
	case errors.Is(c.err, wmstate.ErrOutdated):
		c.state.Unavailable(c.kind.icon, t.Get("wm-devices-outdated"), nil)
	case c.err != nil:
		c.reload.SetVisible(false)
		c.state.Error(t.Get("wm-devices-unreadable"), c.err, c.load)
	case len(c.cards) == 0:
		c.state.Empty(c.kind.icon, t.Get("wm-devices-none"))
	default:
		c.state.Clear()
	}
}

func (c *deviceCards) cardWidget(cd *deviceCard) *widget.Box {
	t := locale.T()
	header := gapBox(widget.Row)
	header.AddClass(cardFormClasses.header)
	title := c.k.label(cd.name, cardFormClasses.title, itemTitle)
	title.SetEllipsize(widget.EllipsizeEnd)
	header.AppendAligned(title, true, widget.AlignCenter)
	if !cd.connected {
		absent := c.k.badge(t.Get("wm-device-absent"), "warning")
		absent.SetTooltip(t.Attr("wm-device-absent", "description"))
		header.AppendAligned(absent, false, widget.AlignCenter)
	} else if c.kind.classed {
		header.AppendAligned(c.k.label(t.Get("wm-pointers-class-"+string(cd.class)), css.Caption, css.DimLabel), false, widget.AlignCenter)
	}
	cd.body = gapBox(widget.Column)
	cd.body.AddClass(cardFormClasses.body)
	cd.root, cd.disclose = c.k.foldedCard(header, cd.body, cardFormClasses.card, func() { c.fill(cd) })
	if !cd.connected {
		header.AppendAligned(c.k.removeButton("wm-device-forget", func() { c.forget(cd) }), false, widget.AlignCenter)
	}
	return cd.root
}

// fill builds the card's rows, each a row of its class's section over
// the device's key.
func (c *deviceCards) fill(cd *deviceCard) {
	cd.rows = make([]*fieldRow, 0, len(cd.specs))
	for _, spec := range cd.specs {
		row := newSettingRow(c.k, cd.scope, spec, c.control(cd.scope, spec))
		cd.rows = append(cd.rows, row)
		cd.body.Append(row, false)
	}
	c.follow(cd)
}

func (c *deviceCards) control(scope deviceScope, spec rowSpec) control {
	if spec.scoped == nil {
		return fieldControl(c.k, scope, spec)
	}
	meta, ok := scope.field(spec.path)
	if !ok {
		log.Panicf("settings: lys-wm's devices have no %s", spec.path)
	}
	return spec.scoped(c.k, scope, scope.slot(spec.path), meta)
}

// follow shows and enables each row of the card as its conditions over
// the device's keys say.
func (c *deviceCards) follow(cd *deviceCard) {
	for i, row := range cd.rows {
		spec := cd.specs[i]
		if spec.shownWhen.set() {
			row.SetVisible(spec.shownWhen.holds(cd.scope.slot(spec.shownWhen.path).get()))
		}
		if spec.enabledWhen.set() {
			row.SetEnabled(spec.enabledWhen.holds(cd.scope.slot(spec.enabledWhen.path).get()))
		}
	}
}

// forget takes every key of the card's rows the device sets out of what
// lys-wm reads, its Undo setting each back to what it held.
func (c *deviceCards) forget(cd *deviceCard) {
	var edits, undo []wmconfig.Edit
	for _, spec := range cd.specs {
		for _, path := range append([]string{spec.path}, spec.also...) {
			if v := cd.scope.value(path); v != nil {
				edits = append(edits, cd.scope.clearing(path))
				undo = append(undo, cd.scope.editing(path, v))
			}
		}
	}
	c.edit("forget", cd.name, edits)
	c.k.offerUndo(cd.name, func() { c.edit("restore", cd.name, undo) })
}

// edit applies the edits to device name's keys, what saying why in the
// error, and rebuilds the cards.
func (c *deviceCards) edit(what, name string, edits []wmconfig.Edit) {
	if err := c.k.wm.edit(edits...); err != nil {
		log.Printf("settings: %s the device %s: %v", what, name, err)
	}
	c.rebuild()
}

// refresh follows the config: the same devices keep their cards and
// reread their rows, a device that gained or lost its last key rebuilds
// the list.
func (c *deviceCards) refresh() {
	shown := c.shown()
	if !slices.EqualFunc(shown, c.cards, func(a, b *deviceCard) bool { return a.name == b.name && a.connected == b.connected }) {
		c.rebuild()
		return
	}
	for _, cd := range c.cards {
		for _, row := range cd.rows {
			row.refresh()
		}
		c.follow(cd)
	}
}
