package settings

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/region"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/hibernate"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// rowDescriptionLines is how many rows a row's description wraps to
// before it ends in an ellipsis, its whole text in its tooltip.
const rowDescriptionLines = 2

// kit builds the window's widgets: the text face every label starts
// from (the stylesheet then sizes and weighs it) and the store the
// controls write to.
type kit struct {
	face render.Font
	// mono and monoVariants are the code views' monospace face and its
	// bold and italic faces.
	mono         render.Font
	monoVariants widget.VariantFunc
	store        store
	parts        parts
	nav          []navSection
	openPage     func(id string)
	// invoke runs a function on the loop (the slider's trailing commit).
	invoke func(func())
	// clock times every delayed callback, whose work goes through invoke.
	clock widgets.Clock
	// pickers opens popovers and dialogs; nil without a window.
	pickers pickers
	// toast shows text in a toast over the window for life; undoing is
	// the toast whose Undo puts the latest removal back (offerUndo).
	toast   func(text string, life time.Duration) (*widget.Toast, error)
	undoing *widget.Toast
	// wm is lys-wm's config once a page of it was opened; onWMChange
	// refreshes the window when a change to it settled.
	wm         *wmBackend
	onWMChange func()
	// background runs blocking work off the loop (a D-Bus query);
	// nil runs it in place, the tests' synchronous loop.
	background func(func())
	systemBus  func() (*dbus.Conn, error)
	network    func() (netControl, error)
	statuses   map[string]tileStatus
	power      func() (powerControl, error)
	pausing    func() (pauseControl, error)
	battery    *batteryPresence
	bluetooth  func() (btControl, error)
	// mailCount is how many messages a notmuch query matches.
	mailCount  func(query string) (uint32, error)
	dnd        func() (dndControl, error)
	nightlight func() (nightlightControl, error)
	// nightLocation is where lys-shell last found itself for the night
	// light, fs.ErrNotExist before it ever did.
	nightLocation func() (geo.Coordinates, error)
	// region is the session's region, which the fields following it
	// resolve against.
	region       func() (region.Prefs, error)
	brightness   func() (brightnessControl, error)
	vault        func() (vaultControl, error)
	cameras      func() (cameraSource, func(), error)
	screenShares func() (shareSource, func(), error)
	// i2c says which I2C buses there are and which this user may open.
	i2c func() i2cAccess
	// hibernation names what keeps the system from hibernating.
	hibernation func() ([]hibernate.Blocker, error)
	copySecret  func(text string) error
	copyText    func(text string) error
	// notify sends a desktop notification, shellCommand runs lys-shell
	// with the arguments given; the tests stub both.
	notify       func(summary, body string) error
	shellCommand func(args ...string) error
	checkPAM     func(ctx context.Context, service string) error
	// openURI opens a URI with the user's default app for it.
	openURI func(uri string) error
	// overlays shows content on the output named on top of everything,
	// taking the keys when interactive; nil without a session.
	overlays func(output string, content widget.Widget, interactive bool) (close func(), err error)
	// barFont loads the face lys-shell's bar labels in, for the previews.
	barFont func(cfg *config.Config) (render.Font, error)
	// familyFont loads a font family, for the samples of the font rows.
	familyFont func(family string) (render.Font, error)
	fontFile   func(family string) (string, error)
	fileFamily func(path string) (string, error)
	// forgetImage drops a raster's GPU copy before its pixels are freed.
	forgetImage func(render.ImageID)
	lockBg      *lockBackdrop
}

// async runs work off the loop and hands its result to done on it.
func async[T any](k *kit, work func() T, done func(T)) {
	if k.background == nil {
		done(work())
		return
	}
	k.background(func() {
		v := work()
		k.invoke(func() { done(v) })
	})
}

// palette is the render palette the stored styling resolves to, the
// colors the token swatches show; a provider that fails is logged and
// leaves the configured palette.
func (k *kit) palette() styling.Palette {
	p, err := styling.ConfigPalette(k.store.svc.Config().Styling)
	if err != nil {
		log.Printf("settings: palette: %v", err)
	}
	return *p
}

// textPx is the body size at the styling scale, which every label and
// control of the window takes.
func (k *kit) textPx() float64 {
	return widgets.TextPx(styling.Body, float64(k.store.svc.Config().Styling.Scale))
}

// label is a styled label; color, size and weight come from the
// stylesheet.
func (k *kit) label(text string, classes ...string) *widget.Label {
	l := widget.NewLabel(k.face, k.textPx(), text, 0)
	l.AddClass(classes...)
	return l
}

// captionTile is body under its caption, as close as a tile sets them.
func captionTile(caption, body widget.Widget) *widget.Box {
	tile := widget.NewBox(widget.Column, choiceCardGap/2, 0)
	tile.Append(caption, false)
	tile.Append(body, false)
	return tile
}

// captioned is body under text, a dim caption.
func (k *kit) captioned(text string, body widget.Widget) *widget.Box {
	return captionTile(k.label(text, css.Caption, css.DimLabel), body)
}

// entry is a text field sized the way a GtkEntry is: a fixed width
// the text pans inside, whatever it holds.
func (k *kit) entry() *widget.Entry {
	e := widget.NewEntry(k.face, k.textPx(), 0)
	e.SetTextWidth(widget.GTKTextWidth)
	return e
}

func chipRemoveName(tr *i18n.Loader) func(text string) string {
	return func(text string) string { return tr.Get("settings-chip-remove", i18n.Str("name", text)) }
}

// searchField is a picker's search entry: an entry showing the
// placeholder key's text that hands every edit to onChange.
func (k *kit) searchField(placeholder string, onChange func(query string)) *widget.Entry {
	e := k.entry()
	e.SetPlaceholder(locale.T().Get(placeholder))
	e.OnChanged = onChange
	return e
}

// icon is a theme icon the stylesheet sizes and tints (an image node).
func (k *kit) icon(name string, classes ...string) *widget.Icon {
	ic := widget.NewThemeIcon(name, 16)
	ic.AddClass(classes...)
	return ic
}

// gapBox lines widgets up along axis a theme's spacing apart, the one
// gap between the controls of a group, the fields of a form and the
// rows of a list.
func gapBox(axis widget.Axis) *widget.Box {
	return widget.NewBox(axis, widget.Current().Spacing, 0)
}

func cardInsets() render.Insets {
	th := widget.Current()
	return render.Insets{Top: th.Padding, Right: 2 * th.Padding, Bottom: th.Padding, Left: 2 * th.Padding}
}

// insetRow is a row of its card that holds content rather than a
// setting, inset like the rows around it: a list's search field, a
// note, a preview.
type insetRow struct{ *widget.Box }

func newInsetRow(w widget.Widget) *insetRow {
	row := &insetRow{widget.NewBox(widget.Row, 0, 0)}
	row.SetElement("item")
	row.Append(w, true)
	return row
}

// badge is lys-ui's quiet badge reading text; a status class
// (success, warning, error, info) tints it.
func (k *kit) badge(text string, classes ...string) *widget.Badge {
	b := widget.NewBadge(k.face, text)
	b.SetVariant(widget.BadgeSecondary)
	b.AddClass(classes...)
	return b
}

// badgeStatuses are the status classes a badge's tint comes from.
var badgeStatuses = []string{css.Success, css.Warning, css.Error, "info"}

// control is a row's editor: its widget, and refresh re-reading the
// value after the config changed.
type control interface {
	widget.Widget
	refresh()
}

// settingRow is the window's one row: a title, a description wrapping
// to rowDescriptionLines rows, what sits on the title's line (a unit, a
// badge), a problem line, the actions and the control. It is lys-ui's
// Item, so its media and trailing actions are the Item's, and the
// controls of the rows of one card share the card's control column.
type settingRow struct {
	*widget.Item
	k *kit
	// problem says why the row did not do what it was last asked, nil
	// until it first has to.
	problem *inlineState
}

// row is a row titled title, described by description ("" for none).
func (k *kit) row(title, description string) *settingRow {
	r := &settingRow{Item: widget.NewItem(k.face, title, description), k: k}
	r.SetSmall(true)
	r.SetLines(1, rowDescriptionLines)
	return r
}

// rowWidget is a widget that is a row: a settingRow or one built on it.
type rowWidget interface {
	widget.Widget
	asRow() *settingRow
}

func (r *settingRow) asRow() *settingRow { return r }

// keyRow is the row the message key titles, described by its
// description attribute.
func (k *kit) keyRow(key string) *settingRow { return k.row(locale.T().Get(key), description(key)) }

// withControl places ctl as the row's control where controlFit holds it.
func (r *settingRow) withControl(ctl widget.Widget) *settingRow {
	r.SetControl(ctl, controlFit(ctl))
	if d, ok := ctl.(interface{ detail() widget.Widget }); ok {
		r.AddDetail(d.detail())
	}
	return r
}

// withActions adds ws ahead of the control.
func (r *settingRow) withActions(ws ...widget.Widget) *settingRow {
	for _, w := range ws {
		r.AddAction(w)
	}
	return r
}

// withControls places the last of ws as the control and the rest
// ahead of it; no ws leaves the row without either.
func (r *settingRow) withControls(ws ...widget.Widget) *settingRow {
	if len(ws) == 0 {
		return r
	}
	return r.withActions(ws[:len(ws)-1]...).withControl(ws[len(ws)-1])
}

// withTrailing adds ws after the control, at the row's end.
func (r *settingRow) withTrailing(ws ...widget.Widget) *settingRow {
	for _, w := range ws {
		r.AddTrailing(w)
	}
	return r
}

// controlFit is where a row holds ctl: where a control that decides its
// own fit says, under its text when it stacks, in the card's control
// column when it is a field (a dropdown, a spin, an entry, or an editor
// that reads as one), and at its own width at the row's end otherwise.
func controlFit(ctl widget.Widget) widget.ItemControlFit {
	switch c := ctl.(type) {
	case interface{ fit() widget.ItemControlFit }:
		return c.fit()
	case interface{ stacked() }:
		return widget.ItemControlWide
	case interface{ columnField() }:
		return widget.ItemControlColumn
	case interface{ Role() widget.Role }:
		if r := c.Role(); r == widget.RoleComboBox || r == widget.RoleEntry {
			return widget.ItemControlColumn
		}
	}
	return widget.ItemControlNatural
}

// rowProblem is why a row did not do what it was asked: summary says
// so in the app's words, err is the failure behind it. The zero
// rowProblem is none.
type rowProblem struct {
	summary string
	err     error
}

// notSaved is the problem of a write that failed with err, none for
// nil.
func notSaved(err error) rowProblem {
	if err == nil {
		return rowProblem{}
	}
	return rowProblem{summary: locale.T().Get("settings-row-not-saved"), err: err}
}

// problemLine is the row's problem line, made and placed under its text
// the first time it is asked for.
func (r *settingRow) problemLine() *inlineState {
	if r.problem == nil {
		r.problem = r.k.inlineState()
		r.AddDetail(r.problem)
	}
	return r.problem
}

// showProblem shows p's summary on the problem line, its error behind
// the disclosure, or hides the line for none.
func (r *settingRow) showProblem(p rowProblem) {
	if p.err == nil && r.problem == nil {
		return
	}
	r.problemLine().Result(p.summary, p.err)
}

// showPending says the value is being checked before it is written.
func (r *settingRow) showPending(value any) {
	r.problemLine().Loading(locale.T().Get("settings-checking", i18n.Str("value", fmt.Sprint(value))))
}

// errorText is a caption in the error color, wrapping: why something
// failed.
func (k *kit) errorText(text string) *widget.Label {
	return wrapped(k.label(text, css.Caption, css.Error))
}

// note is a dim caption, wrapping, with classes: a status, a hint.
func (k *kit) note(text string, classes ...string) *widget.Label {
	return wrapped(k.label(text, append(classes, css.Caption, css.DimLabel)...))
}

// fieldRow is SettingRow, the row of a config field: its unit and
// source badge on the title's line, the reset button and the field's
// control.
type fieldRow struct {
	*settingRow
	k     *kit
	be    backend
	path  string
	ctl   control
	badge *widget.Badge
	reset resetButton
	// action is the row's RowBehavior::Action.
	action bool
}

// newFieldRow is the row for spec with its field's control, whose refused
// writes the row reports under its label. A row with a vet writes a value,
// its reset's included, only once the vet passed it.
func newFieldRow(k *kit, be backend, spec rowSpec) *fieldRow {
	var r *fieldRow
	report := func(err error) {
		if r != nil {
			r.showProblem(notSaved(err))
		}
	}
	if spec.vet == nil {
		r = newSettingRow(k, be, spec, fieldControl(k, reportingBackend{be, report}, spec))
		return r
	}
	v := &vetter{k: k, check: func(ctx context.Context, value any) rowProblem { return spec.vet(ctx, k, value) }}
	vetted := vettingBackend{reportingBackend{be, report}, v}
	r = newSettingRow(k, vetted, spec, fieldControl(k, vetted, spec))
	v.row = r
	return r
}

// vettingBackend hands every write and reset to its vetter, which makes it
// once the vet passed the value it would leave.
type vettingBackend struct {
	backend
	v *vetter
}

func (b vettingBackend) slot(path string) slot {
	s := b.backend.slot(path)
	return slot{
		get: s.get,
		set: func(value any) error {
			b.v.start(value, func() error { return s.set(value) })
			return nil
		},
		unset: s.unset,
	}
}

func (b vettingBackend) reset(path string) {
	d, ok := b.defaultOf(path)
	if !ok {
		b.v.row.showProblem(rowProblem{summary: locale.T().Get("settings-row-not-reset"), err: fmt.Errorf("settings: %s has no default to check before resetting it", path)})
		return
	}
	b.v.start(d, func() error {
		b.backend.reset(path)
		return nil
	})
}

// vetter runs a row's vet off the loop while the row says the value is
// being checked: a value that passed is written, one that failed or took
// longer than commandTimeout on the kit's clock is refused. A newer value
// drops the check of the one before.
type vetter struct {
	k       *kit
	check   func(ctx context.Context, value any) rowProblem
	row     *fieldRow
	current uint64
	cancel  context.CancelFunc
}

func (v *vetter) start(value any, write func() error) {
	v.drop()
	v.current++
	run := v.current
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	v.row.showPending(value)
	stop := v.k.clock.AfterFunc(commandTimeout, func() {
		v.k.invoke(func() { v.settle(run, checkTimedOut(value), write) })
	})
	async(v.k, func() rowProblem { return v.check(ctx, value) }, func(why rowProblem) {
		stop()
		v.settle(run, why, write)
	})
}

func (v *vetter) drop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *vetter) settle(run uint64, why rowProblem, write func() error) {
	if run != v.current || v.cancel == nil {
		return
	}
	v.drop()
	if why.err == nil {
		why = notSaved(write())
	}
	v.row.showProblem(why)
	v.row.refresh()
}

// checkTimedOut is the problem of value, whose check took longer than
// commandTimeout.
func checkTimedOut(value any) rowProblem {
	return rowProblem{
		summary: locale.T().Get("settings-check-timed-out", i18n.Int("seconds", int64(commandTimeout/time.Second))),
		err:     fmt.Errorf("settings: checking %v did not finish within %s", value, commandTimeout),
	}
}

// reportingBackend hands report the outcome of every write through its
// slots: the error a refused value gets, nil once one lands.
type reportingBackend struct {
	backend
	report func(error)
}

func (b reportingBackend) slot(path string) slot {
	s := b.backend.slot(path)
	return slot{
		get: s.get,
		set: func(v any) error {
			err := s.set(v)
			b.report(err)
			return err
		},
		unset: func() {
			s.unset()
			b.report(nil)
		},
	}
}

// newSettingRow is the row for spec around ctl, its reset button ahead
// of the control so showing it never moves the control.
func newSettingRow(k *kit, be backend, spec rowSpec, ctl control) *fieldRow {
	r := &fieldRow{settingRow: k.keyRow(spec.labelKey()), k: k, be: be, path: spec.path, ctl: ctl, action: spec.action}
	r.badge = k.badge("")
	r.AddTitleSuffix(r.badge)
	if d, ok := ctl.(interface{ dirtyBadge() *widget.Badge }); ok {
		r.AddTitleSuffix(d.dirtyBadge())
	}
	r.reset = k.resetButton("settings-reset-setting", func() {
		if d, ok := ctl.(interface{ discardDraft() }); ok {
			d.discardDraft()
		}
		r.showProblem(rowProblem{})
		be.reset(spec.path)
	})
	r.AddAction(r.reset.Button)
	fit := controlFit(ctl)
	if spec.fullWidth {
		fit = widget.ItemControlWide
	}
	r.SetControl(ctl, fit)
	r.refreshSource()
	return r
}

// refresh re-reads the source and the control's value.
func (r *fieldRow) refresh() {
	r.refreshSource()
	r.ctl.refresh()
}

// refreshSource shows the badge and arms the reset button
// (update_source_info): the button keeps its place, invisible and
// inert, while there is no runtime override.
func (r *fieldRow) refreshSource() {
	src := sourceInfo{}
	if !r.action {
		src = r.be.source(r.path)
	}
	r.badge.SetVisible(src.badge)
	r.badge.SetText(src.label)
	r.badge.SetTooltip(src.tooltip)
	r.badge.RemoveClass(badgeStatuses...)
	if src.class != "" {
		r.badge.AddClass(src.class)
	}
	r.reset.show(src.reset)
}

// description is the help a message's description attribute gives, ""
// for a message without one.
func description(key string) string {
	t := locale.T()
	if !t.HasAttr(key, "description") {
		return ""
	}
	return t.Attr(key, "description")
}

// formGrid lines a form's fields up: their labels share one column,
// and the fields (dropdowns, spins, entries) at the end of their row
// share another, so the controls of one form start and end at one x.
type formGrid struct {
	k        *kit
	labels   *widget.SizeGroup
	controls *widget.SizeGroup
}

func (k *kit) formGrid() formGrid {
	return formGrid{k: k, labels: widget.NewSizeGroup(widget.SizeGroupHorizontal), controls: widget.NewSizeGroup(widget.SizeGroupHorizontal)}
}

// label is text in the grid's label column, with classes.
func (g formGrid) label(text string, classes ...string) widget.Widget {
	return g.labels.Add(g.k.label(text, append([]string{css.CaptionHeading, css.DimLabel}, classes...)...))
}

// row is a field: label (from label) then ctl, which takes the spare
// width with fill and otherwise sits at the row's end, in the field
// column when controlFit holds it there; more follow ctl.
func (g formGrid) row(label widget.Widget, ctl widget.Widget, fill bool, more ...widget.Widget) *widget.Box {
	row := gapBox(widget.Row)
	row.AppendAligned(label, !fill, widget.AlignCenter)
	if !fill && controlFit(ctl) == widget.ItemControlColumn {
		ctl = g.controls.Add(ctl)
	}
	row.AppendAligned(ctl, fill, widget.AlignCenter)
	for _, w := range more {
		row.AppendAligned(w, false, widget.AlignCenter)
	}
	return row
}
