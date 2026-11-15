package bar

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/bluetooth"
)

const (
	btScanDuration  = 30 * time.Second
	btActionTimeout = 30 * time.Second
	// btDetailSeparator sits between the device type and the battery.
	btDetailSeparator = "•"
)

// btPending is a device action in flight.
type btPending uint8

const (
	pendingConnecting btPending = iota + 1
	pendingDisconnecting
	pendingForgetting
)

// btDropdown is the live dropdown: it follows the service until its
// popover closes.
type btDropdown struct {
	ctx  ModuleContext
	src  bluetooth.Source
	font render.Font
	px   float64

	root       *btDropdownRoot
	headerIcon *widget.Icon
	scanBtn    *widget.Button
	scanIcon   *widget.Icon
	toggle     *widget.Switch
	card       *btPairingCard
	errLabel   *widget.Label

	mySection, availSection *widget.Box
	myCard, availCard       *widget.Box
	myList, availList       *widget.ItemGroup
	scanningHint            *widget.Label
	emptyNoDevices          widget.Widget
	emptyOff                widget.Widget
	emptyNoAdapter          widget.Widget

	// enabled and available are the dropdown's view, set optimistically
	// by the switch and overwritten when the service's value changes.
	enabled, available bool
	seen               bool
	svcEnabled         bool
	svcAvailable       bool

	scanning bool
	scanGen  int
	pending  map[dbus.ObjectPath]btPending
	// shownPairing is the request the card was last given.
	shownPairing bluetooth.PairingRequest
	myKeys       []string
	availKeys    []string
	*openFollow
}

// btDropdownRoot is the popover content; it releases the subscription
// when the popover closes, and takes the popover handle so the passkey
// boxes can move the keyboard focus.
type btDropdownRoot struct {
	*widget.Box
	d *btDropdown
	popoverHook
}

// dropdownClosed implements dropdownCloser; the no-adapter shell has
// no dropdown behind it and holds nothing.
func (r *btDropdownRoot) dropdownClosed() {
	if r.d != nil {
		r.d.dropdownClosed()
	}
}

func (r *btDropdownRoot) dropdownOpened() {
	if r.d != nil {
		r.d.dropdownOpened()
	}
}

func (r *btDropdownRoot) dropdownHidden() {
	if r.d != nil {
		r.d.dropdownHidden()
	}
}

// btNoAdapterDropdown is the dropdown shell for a machine without an
// adapter: the header stays, the content shows the no-adapter empty
// state alone, and nothing follows a service.
func btNoAdapterDropdown(ctx ModuleContext) widget.Widget {
	font, px := dropdownFont(ctx)
	col := dropdownContent("bluetooth-content")
	empty := widgets.EmptyState(font, px, "ld-bluetooth-off-symbolic",
		btText("dropdown-bluetooth-no-adapter-title"), btText("dropdown-bluetooth-no-adapter-description"))
	col.AppendAligned(empty, true, widget.AlignCenter)
	return btShell(font, "ld-bluetooth-off-symbolic", dropdownScroll(col, "bluetooth-scroll"))
}

// btShell is the frame the two entries share: the header over the
// scrolled content; the registry gives it the popover's
// size, as for every dropdown.
func btShell(font render.Font, icon string, scroll widget.Widget) widget.Widget {
	header, _, _ := dropdownHeaderParts(font, icon, btText("dropdown-bluetooth-title"))
	frame := widget.NewBox(widget.Column, 0, 0)
	frame.AddClass("dropdown", "bluetooth-dropdown")
	frame.Append(header, false)
	frame.Append(scroll, true)
	return &btDropdownRoot{Box: frame}
}

// bluetoothDropdown builds the dropdown content; the registry's
// "bluetooth" builder.
func bluetoothDropdown(ctx ModuleContext) widget.Widget {
	if ctx.Bluetooth == nil {
		// No adapter: the same shell — header above the
		// no-adapter empty state — without a service to follow.
		return btNoAdapterDropdown(ctx)
	}
	d := newBtDropdown(ctx, ctx.Bluetooth)
	d.openFollow = newOpenFollow(ctx, d.follow)
	d.stopped = func() { btDropdownClosed(d.src) }
	d.startFollowing()
	return d.root
}

func newBtDropdown(ctx ModuleContext, src bluetooth.Source) *btDropdown {
	font, px := dropdownFont(ctx)
	d := &btDropdown{
		ctx: ctx, src: src, font: font, px: px,
		pending: make(map[dbus.ObjectPath]btPending),
	}
	// The header: icon, title, scan button, switch. The
	// template's `.dropdown-title` box lets the stylesheet ink the icon
	// and title.
	d.scanIcon = widget.NewThemeIcon("ld-refresh-cw-symbolic", int(px))
	// lys-ui's ghost icon square.
	d.scanBtn = dropdownButton(d.scanIcon, nil, css.Ghost, css.Icon)
	d.scanBtn.AddClass("bluetooth-scan-btn")
	d.scanBtn.OnClick = d.onScan
	d.toggle = widget.NewSwitch(false)
	d.toggle.OnChanged = d.onToggle
	header, glyph, _ := dropdownHeaderParts(font, "ld-bluetooth-symbolic", btText("dropdown-bluetooth-title"), d.scanBtn, d.toggle)
	d.headerIcon = glyph

	// The content column, scrolled.
	col := dropdownContent("bluetooth-content")
	d.errLabel = widget.NewLabel(font, px, "", 0)
	d.errLabel.AddClass("bluetooth-error", css.Error)
	d.errLabel.SetWrap(true)
	d.errLabel.SetVisible(false)
	col.Append(d.errLabel, false)
	d.card = newBtPairingCard(ctx)
	d.card.onOutput = d.onPairingOutput
	col.Append(d.card.root, false)

	d.myCard, d.myList = itemCard()
	d.mySection = dropdownSection(sectionLabel(font, btText("dropdown-bluetooth-my-devices")), d.myCard)
	col.Append(d.mySection, false)
	d.availCard, d.availList = itemCard()
	d.scanningHint = widget.NewLabel(font, px, btText("dropdown-bluetooth-no-new"), 0)
	d.scanningHint.AddClass(css.DimLabel)
	d.availSection = dropdownSection(sectionLabel(font, btText("dropdown-bluetooth-available-devices")), d.availCard, d.scanningHint)
	col.Append(d.availSection, false)
	d.emptyNoDevices = widgets.EmptyState(font, px, "ld-bluetooth-searching-symbolic",
		btText("dropdown-bluetooth-no-devices-title"), btText("dropdown-bluetooth-no-devices-description"))
	d.emptyOff = widgets.EmptyState(font, px, "ld-bluetooth-off-symbolic",
		btText("dropdown-bluetooth-off-title"), btText("dropdown-bluetooth-off-description"))
	d.emptyNoAdapter = widgets.EmptyState(font, px, "ld-bluetooth-off-symbolic",
		btText("dropdown-bluetooth-no-adapter-title"), btText("dropdown-bluetooth-no-adapter-description"))
	// The empty state expands and centers itself.
	col.AppendAligned(d.emptyNoDevices, true, widget.AlignCenter)
	col.AppendAligned(d.emptyOff, true, widget.AlignCenter)
	col.AppendAligned(d.emptyNoAdapter, true, widget.AlignCenter)

	scroll := dropdownScroll(col, "bluetooth-scroll")
	frame := widget.NewBox(widget.Column, 0, 0)
	frame.AddClass("dropdown", "bluetooth-dropdown")
	frame.Append(header, false)
	frame.Append(scroll, true)
	d.root = &btDropdownRoot{Box: frame, d: d}
	d.sync()
	return d
}

func (d *btDropdown) follow(life context.Context) {
	d.sync()
	ticks, stop := d.src.Subscribe()
	btDropdownOpened(d.src)
	d.ctx.Go(func() {
		defer stop()
		for {
			select {
			case <-life.Done():
				return
			case _, ok := <-ticks:
				if !ok {
					return
				}
				d.ctx.Invoke(d.sync)
			}
		}
	})
}

// async runs work off the loop and then back on it; headless (no
// application) it runs inline so tests are deterministic.
func (d *btDropdown) async(work func(), then func()) {
	if d.ctx.App == nil {
		work()
		if then != nil {
			then()
		}
		return
	}
	d.ctx.Go(func() {
		work()
		if then != nil {
			d.ctx.Invoke(then)
		}
	})
}

// sync folds the service state into the view.
func (d *btDropdown) sync() {
	st := d.src.State()
	if !d.seen || st.Enabled != d.svcEnabled {
		d.enabled = st.Enabled
	}
	if !d.seen || st.Available != d.svcAvailable {
		d.available = st.Available
	}
	d.svcEnabled, d.svcAvailable, d.seen = st.Enabled, st.Available, true

	if st.Pairing != d.shownPairing {
		d.showPairing(st)
	}
	mine, avail := splitDeviceLists(st.Devices)
	d.settlePending(mine, avail)
	d.myKeys = d.rebuildList(d.myList, mine, d.myKeys)
	d.availKeys = d.rebuildList(d.availList, avail, d.availKeys)
	d.render()
}

// showPairing shows the request in the card, for the device it names.
func (d *btDropdown) showPairing(st bluetooth.State) {
	d.shownPairing = st.Pairing
	if st.Pairing == nil {
		d.card.clear()
		return
	}
	display := unknownDisplay
	if dev, ok := st.Device(st.Pairing.DevicePath()); ok {
		display = resolveDeviceDisplay(dev)
	}
	d.card.set(st.Pairing, display)
}

// settlePending drops the pending marks the new state completed;
// a forget completes by the row vanishing.
func (d *btDropdown) settlePending(lists ...[]deviceSnapshot) {
	present := map[dbus.ObjectPath]deviceSnapshot{}
	for _, list := range lists {
		for _, s := range list {
			present[s.path] = s
		}
	}
	for path, action := range d.pending {
		s, ok := present[path]
		switch {
		case !ok:
			delete(d.pending, path)
		case action == pendingConnecting && s.connected, action == pendingDisconnecting && !s.connected:
			delete(d.pending, path)
		}
	}
}

// rebuildList re-renders a list when a row's view changed; unchanged
// lists keep their rows, so RSSI churn does not reset a hovered row.
func (d *btDropdown) rebuildList(list *widget.ItemGroup, snaps []deviceSnapshot, old []string) []string {
	keys := make([]string, len(snaps))
	for i, s := range snaps {
		keys[i] = fmt.Sprintf("%s|%s|%s|%s|%v|%v|%v|%d|%d", s.path, s.name, s.icon, s.typeKey,
			batteryText(s.battery), s.connected, s.paired, s.category, d.pending[s.path])
	}
	if slices.Equal(keys, old) {
		return old
	}
	list.Clear()
	for _, s := range snaps {
		list.Append(d.buildRow(s))
	}
	list.InvalidateLayout()
	return keys
}

func batteryText(battery *uint8) string {
	if battery == nil {
		return ""
	}
	return btText("dropdown-bluetooth-battery", "percent", *battery)
}

// buildRow is DeviceItem's view for one snapshot, on lys-ui's Item:
// the device icon as the media (ringed by its battery while connected,
// when it reports one), the name over the type and battery,
// and the status or actions slot as the row's action. The text
// ellipsizes so a long name never holds the row past the dropdown;
// activating the row is the device click.
func (d *btDropdown) buildRow(s deviceSnapshot) *btDeviceRow {
	font, px := d.font, d.px
	pending := d.pending[s.path]
	detail := btText(s.typeKey)
	if s.battery != nil {
		detail += " " + btDetailSeparator + " " + batteryText(s.battery)
	}
	row := &btDeviceRow{hoverSwaps: s.category != categoryAvailable, pending: pending != 0}
	row.revealItem = newRevealItem(widget.NewItem(font, s.name, detail), row.syncSlot)
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("bluetooth-device")
	switch s.category {
	case categoryAvailable:
		row.AddClass("available")
	case categoryConnected:
		row.AddClass("connected")
	case categoryPaired:
		row.AddClass("paired")
	}
	if pending != 0 {
		row.AddClass("pending")
	}
	row.onClick = func() { d.onDeviceClick(s) }
	row.SetOnActivate(func() { row.onClick() })
	if s.connected && s.battery != nil {
		ring := newGlyphRing(s.icon, *s.battery)
		ring.glyph.AddClass("bluetooth-icon")
		row.SetMedia(ring, true)
	} else {
		icon := widget.NewThemeIcon(s.icon, itemIconPx)
		icon.AddClass("bluetooth-icon")
		row.SetMedia(icon, false)
	}

	if s.category != categoryAvailable || pending != 0 {
		status := widget.NewLabel(font, px*styling.Caption.Ratio(), statusLabel(s, pending), 0)
		status.AddClass("bluetooth-device-status", css.Caption, css.DimLabel)
		row.statusShown = s.connected || s.paired || pending != 0
		row.status = status
		// The status sits at the slot's end and middle: the slot is as
		// wide as the actions, so the status hugs the row's edge.
		statusPage := atEnd(status)

		actions := widget.NewBox(widget.Row, widget.Current().Spacing/2, 0)
		actions.AddClass("bluetooth-device-actions")
		// Icon buttons named by their tooltips: the slot reserves the
		// actions' width beside the status, so text buttons would cost
		// the device's name and detail that width on every row.
		toggleKey, toggleIcon := "dropdown-bluetooth-connect", "ld-plug-zap-symbolic"
		if s.connected {
			toggleKey, toggleIcon = "dropdown-bluetooth-disconnect", "ld-unplug-symbolic"
		}
		toggle := btActionButton(px, toggleIcon, btText(toggleKey), func() { d.onDeviceClick(s) }, "bluetooth-action-toggle")
		forget := btActionButton(px, "ld-trash-2-symbolic", btText("dropdown-bluetooth-forget"), func() { d.onForget(s) }, "bluetooth-forget")
		toggle.SetEnabled(pending == 0)
		forget.SetEnabled(pending == 0)
		actions.Append(toggle, false)
		actions.Append(forget, false)
		row.actions = actions
		// The hover stack crossfades between them,
		// as wide as the wider page so hovering never shifts the row.
		row.slot = widget.NewStack()
		row.slot.SetTransition(widget.StackCrossfade, hoverTransition)
		row.slot.Add(btSlotStatus, statusPage)
		row.slot.Add(btSlotActions, actions)
		row.AddAction(row.slot)
		row.syncSlot()
	}
	return row
}

// btActionButton is a row action: a small ghost icon button whose
// tooltip and accessible name are its label.
func btActionButton(px float64, icon, label string, onClick func(), class string) *widget.Button {
	b := dropdownButton(widget.NewThemeIcon(icon, int(px)), onClick, css.Ghost, css.Icon, css.Small, class)
	b.SetTooltip(label)
	b.SetAccessibleName(label)
	return b
}

// statusLabel is the pending action, else connected or
// paired.
func statusLabel(s deviceSnapshot, pending btPending) string {
	switch pending {
	case pendingConnecting:
		return btText("dropdown-bluetooth-status-connecting")
	case pendingDisconnecting:
		return btText("dropdown-bluetooth-status-disconnecting")
	case pendingForgetting:
		return btText("dropdown-bluetooth-status-forgetting")
	}
	switch {
	case s.connected:
		return btText("dropdown-bluetooth-connected")
	case s.paired:
		return btText("dropdown-bluetooth-paired")
	}
	return ""
}

// render applies the view's visibility rules.
func (d *btDropdown) render() {
	icon := "ld-bluetooth-off-symbolic"
	if d.enabled {
		icon = "ld-bluetooth-symbolic"
	}
	d.headerIcon.SetThemeName(icon)
	d.scanBtn.SetVisible(d.available && d.enabled)
	d.scanBtn.SetEnabled(!d.scanning)
	if d.scanning {
		d.scanBtn.AddClass("scanning")
	} else {
		d.scanBtn.RemoveClass("scanning")
	}
	// A state sync is not a user toggle.
	onChanged := d.toggle.OnChanged
	d.toggle.OnChanged = nil
	d.toggle.SetOn(d.enabled)
	d.toggle.OnChanged = onChanged
	d.toggle.SetVisible(d.available)

	haveMine, haveAvail := len(d.myKeys) > 0, len(d.availKeys) > 0
	d.mySection.SetVisible(d.enabled && haveMine)
	d.availSection.SetVisible(d.enabled && (haveAvail || d.scanning))
	d.availCard.SetVisible(d.enabled && haveAvail)
	none := !haveMine && !haveAvail
	d.scanningHint.SetVisible(d.enabled && d.scanning && none)
	setVisible(d.emptyNoDevices, d.enabled && !d.scanning && none)
	setVisible(d.emptyOff, !d.enabled && d.available)
	setVisible(d.emptyNoAdapter, !d.available)
}

func setVisible(w widget.Widget, on bool) {
	if v, ok := w.(interface{ SetVisible(bool) }); ok {
		v.SetVisible(on)
	}
}

func (d *btDropdown) onToggle(active bool) {
	d.enabled = active
	d.clearError()
	if !active {
		d.scanGen++
		d.scanning = false
	}
	d.render()
	var err error
	d.async(func() {
		if active {
			err = d.src.Enable(context.Background())
		} else {
			err = d.src.Disable(context.Background())
		}
	}, func() { d.toggleDone(active, err) })
}

func (d *btDropdown) toggleDone(active bool, err error) {
	if err == nil {
		return
	}
	log.Printf("bluetooth: set powered %v: %v", active, err)
	key := "dropdown-bluetooth-error-power-off"
	if active {
		key = "dropdown-bluetooth-error-power-on"
	}
	d.enabled = d.src.State().Enabled
	d.showError(btText(key, "error", err.Error()))
	d.render()
}

func (d *btDropdown) showError(text string) {
	d.errLabel.SetText(text)
	d.errLabel.SetVisible(true)
}

func (d *btDropdown) clearError() {
	d.errLabel.SetText("")
	d.errLabel.SetVisible(false)
}

// onScan is a timed discovery, with the button held in its scanning
// state for the scan's length, or until the discovery is refused.
func (d *btDropdown) onScan() {
	if d.scanning {
		return
	}
	d.scanning = true
	d.scanGen++
	gen := d.scanGen
	d.clearError()
	d.render()
	var err error
	d.async(func() {
		err = d.src.StartTimedDiscovery(context.Background(), btScanDuration)
	}, func() { d.scanStarted(gen, err) })
	if d.ctx.App != nil {
		time.AfterFunc(btScanDuration, func() { d.ctx.Invoke(func() { d.finishScan(gen) }) })
	}
}

func (d *btDropdown) scanStarted(gen int, err error) {
	if err == nil || gen != d.scanGen {
		return
	}
	log.Printf("bluetooth: start discovery: %v", err)
	d.showError(btText("dropdown-bluetooth-error-scan", "error", err.Error()))
	d.finishScan(gen)
}

// finishScan is ScanComplete for the scan gen started; a toggle-off in
// between already ended it.
func (d *btDropdown) finishScan(gen int) {
	if gen != d.scanGen {
		return
	}
	d.scanning = false
	d.render()
}

// onDeviceClick disconnects a connected device and connects (and so
// pairs) any other.
func (d *btDropdown) onDeviceClick(s deviceSnapshot) {
	if d.pending[s.path] != 0 {
		return
	}
	if s.connected {
		d.startAction(s.path, pendingDisconnecting, func(ctx context.Context) error {
			return d.src.Disconnect(ctx, s.path)
		})
		return
	}
	d.startAction(s.path, pendingConnecting, func(ctx context.Context) error {
		if err := d.src.Connect(ctx, s.path); err != nil {
			return err
		}
		// BlueZ re-asks the agent to authorize every service on each
		// reconnect of an untrusted device; connecting is consent, so
		// persist it as trust.
		trustPaired(ctx, d.src, s.path)
		return nil
	})
}

func (d *btDropdown) onForget(s deviceSnapshot) {
	if d.pending[s.path] != 0 {
		return
	}
	d.ctx.showAlert(alertSpec{
		message: btText("dropdown-bluetooth-forget-confirm", "name", s.name),
		detail:  btText("dropdown-bluetooth-forget-confirm-detail"),
		cancel:  btText("dropdown-bluetooth-cancel"),
		accept:  btText("dropdown-bluetooth-forget"),
	}, func(accepted bool) {
		if !accepted || d.pending[s.path] != 0 {
			return
		}
		d.startAction(s.path, pendingForgetting, func(ctx context.Context) error {
			return d.src.Forget(ctx, s.path)
		})
	})
}

// startAction: mark the row pending, run the
// action under btActionTimeout, and clear the mark on failure.
func (d *btDropdown) startAction(path dbus.ObjectPath, action btPending, run func(context.Context) error) {
	d.pending[path] = action
	d.sync()
	var err error
	d.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), btActionTimeout)
		defer cancel()
		err = run(ctx)
	}, func() {
		if err != nil {
			log.Printf("bluetooth device action failed: %v", err)
			delete(d.pending, path)
			d.sync()
		}
	})
}

func trustPaired(ctx context.Context, src bluetooth.Source, path dbus.ObjectPath) {
	if err := src.TrustPaired(ctx, path); err != nil {
		log.Printf("bluetooth device trust failed: %v", err)
	}
}

// onPairingOutput sends the card's answer. The answers go to the
// service on the loop (they only hand the value to the waiting agent
// call), so the next sync sees the request gone; a refused answer
// leaves the request pending and the card shows it again.
func (d *btDropdown) onPairingOutput(out pairingOutput) {
	d.card.clear()
	d.shownPairing = nil
	if out.kind == outputCancelled {
		d.src.CancelPendingRequest()
		d.sync()
		return
	}
	// Captured before answering: the answer clears the request.
	var serviceAuthPath dbus.ObjectPath
	if out.kind == outputServiceAuthorizationAccepted {
		if req := d.src.State().Pairing; req != nil {
			serviceAuthPath = req.DevicePath()
		}
	}
	if err := provide(d.src, out); err != nil {
		log.Printf("pairing response failed: %v", err)
		d.sync()
		return
	}
	d.sync()
	// Allowing a service is consent for the device: persist it as trust
	// so BlueZ stops re-prompting on every reconnect.
	if serviceAuthPath != "" {
		d.async(func() { trustPaired(context.Background(), d.src, serviceAuthPath) }, nil)
	}
}

// provide routes one card answer to its service provider.
func provide(src bluetooth.Source, out pairingOutput) error {
	switch out.kind {
	case outputLegacyPinSubmitted:
		return src.ProvidePin(out.pin)
	case outputPasskeySubmitted:
		return src.ProvidePasskey(out.passkey)
	case outputPasskeyConfirmed, outputPasskeyRejected:
		return src.ProvideConfirmation(out.kind == outputPasskeyConfirmed)
	case outputAuthorizationAccepted, outputAuthorizationRejected:
		return src.ProvideAuthorization(out.kind == outputAuthorizationAccepted)
	case outputServiceAuthorizationAccepted, outputServiceAuthorizationRejected:
		return src.ProvideServiceAuthorization(out.kind == outputServiceAuthorizationAccepted)
	}
	return nil
}
