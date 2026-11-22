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

// Row text limits.
const (
	descriptionMaxChars        = 120
	descriptionTooltipMinChars = 80
)

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

// insetRow holds w as a row of its card, inset like the rows around it:
// a list's search field, a note.
func insetRow(w widget.Widget) *widget.Box {
	row := widget.NewBox(widget.Row, 0, 0)
	row.SetElement("item")
	row.Append(w, true)
	return row
}

// emptyRow is the row of a list holding nothing: icon on lys-ui's muted
// tile beside text.
func (k *kit) emptyRow(icon, text string) *widget.Item {
	it := widget.NewItem(k.face, text, "")
	it.SetMedia(k.icon(icon), false)
	return it
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
var badgeStatuses = []string{"success", "warning", "error", "info"}

// control is a row's editor: its widget, and refresh re-reading the
// value after the config changed.
type control interface {
	widget.Widget
	refresh()
}

// settingRow is SettingRow: the label with its unit and source badge,
// the description, the reset button and the control.
type settingRow struct {
	*widget.Box
	kit   *kit
	be    backend
	path  string
	ctl   control
	title *widget.Label
	badge *widget.Badge
	reset resetButton
	// problem says why the last value the control wrote was refused.
	problem *widget.Label
	// action is the row's RowBehavior::Action.
	action bool
}

// newFieldRow is the row for spec with its field's control, whose refused
// writes the row reports under its label. A row with a vet writes a value,
// its reset's included, only once the vet passed it.
func newFieldRow(k *kit, be backend, spec rowSpec) *settingRow {
	var r *settingRow
	report := func(err error) {
		if r != nil {
			r.showProblem(err)
		}
	}
	if spec.vet == nil {
		r = newSettingRow(k, be, spec, fieldControl(k, reportingBackend{be, report}, spec))
		return r
	}
	v := &vetter{k: k, check: func(ctx context.Context, value any) error { return spec.vet(ctx, k, value) }}
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
		b.v.row.showProblem(fmt.Errorf("settings: %s has no default to check before resetting it", path))
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
	check   func(ctx context.Context, value any) error
	row     *settingRow
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
		v.k.invoke(func() { v.settle(run, checkTimedOut{}, write) })
	})
	async(v.k, func() error { return v.check(ctx, value) }, func(err error) {
		stop()
		v.settle(run, err, write)
	})
}

func (v *vetter) drop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *vetter) settle(run uint64, err error, write func() error) {
	if run != v.current || v.cancel == nil {
		return
	}
	v.drop()
	if err == nil {
		err = write()
	}
	v.row.showProblem(err)
	v.row.refresh()
}

type checkTimedOut struct{}

func (checkTimedOut) Error() string {
	return locale.T().Get("settings-check-timed-out", i18n.Int("seconds", int64(commandTimeout/time.Second)))
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

// showProblem shows why a value was refused, or hides the line for nil.
func (r *settingRow) showProblem(err error) {
	r.problem.SetClass(css.Error, true)
	r.problem.SetVisible(err != nil)
	if err != nil {
		r.problem.SetText(err.Error())
	}
}

// showPending says the value is being checked before it is written.
func (r *settingRow) showPending(value any) {
	r.problem.SetClass(css.Error, false)
	r.problem.SetText(locale.T().Get("settings-checking", i18n.Str("value", fmt.Sprint(value))))
	r.problem.SetVisible(true)
}

// newSettingRow lays a row out: a full-width control
// stacks under the text, with the reset button beside the label.
func newSettingRow(k *kit, be backend, spec rowSpec, ctl control) *settingRow {
	t := locale.T()
	if _, ok := ctl.(interface{ stacked() }); ok {
		spec.fullWidth = true
	}
	axis := widget.Row
	if spec.fullWidth {
		axis = widget.Column
	}
	th := widget.Current()
	r := &settingRow{kit: k, be: be, path: spec.path, ctl: ctl, action: spec.action, Box: gapBox(axis)}
	r.SetElement("item")
	r.AddClass(css.Small, "setting-row")

	info := widget.NewBox(widget.Column, th.Spacing/4, 0)
	labelRow := widget.NewBox(widget.Row, th.Spacing/2, 0)
	title := k.label(t.Get(spec.labelKey()), "setting-label")
	r.title = title
	title.SetEllipsize(widget.EllipsizeEnd)
	title.SetTooltip(t.Get(spec.labelKey()))
	labelRow.AppendAligned(title, false, widget.AlignCenter)
	if spec.unit != "" {
		labelRow.AppendAligned(k.label("("+spec.unit+")", "setting-unit", css.Caption, css.DimLabel, css.Monospace), false, widget.AlignCenter)
	}
	r.badge = k.badge("")
	labelRow.AppendAligned(r.badge, false, widget.AlignCenter)
	if d, ok := ctl.(interface{ dirtyBadge() *widget.Badge }); ok {
		labelRow.AppendAligned(d.dirtyBadge(), false, widget.AlignCenter)
	}
	info.Append(labelRow, false)
	if desc := description(spec.labelKey()); desc != "" {
		dl := k.label(desc, "setting-description", css.Caption, css.DimLabel)
		dl.SetEllipsize(widget.EllipsizeEnd)
		dl.SetMaxWidthChars(descriptionMaxChars)
		if len(desc) > descriptionTooltipMinChars {
			dl.SetTooltip(desc)
		}
		info.Append(dl, false)
	}
	r.problem = k.label("", "setting-problem", css.Caption, css.Error)
	r.problem.SetWrap(true)
	r.problem.SetVisible(false)
	info.Append(r.problem, false)

	r.reset = k.resetButton("settings-reset-setting", func() {
		if d, ok := ctl.(interface{ discardDraft() }); ok {
			d.discardDraft()
		}
		r.showProblem(nil)
		be.reset(spec.path)
	})
	slot := widget.NewBox(widget.Row, 0, 0)
	slot.AddClass("setting-control")
	slot.AppendAligned(ctl, spec.fullWidth, widget.AlignCenter)

	if spec.fullWidth {
		r.AddClass("vertical")
		labelRow.Append(widget.NewSpacer(0, 0), true)
		labelRow.Append(r.reset.Button, false)
		r.Append(info, false)
		r.Append(slot, false)
	} else {
		r.AppendAligned(info, true, widget.AlignCenter)
		r.AppendAligned(r.reset.Button, false, widget.AlignCenter)
		r.AppendAligned(slot, false, widget.AlignCenter)
	}
	r.refreshSource()
	return r
}

// refresh re-reads the source and the control's value.
func (r *settingRow) refresh() {
	r.refreshSource()
	r.ctl.refresh()
}

// refreshSource shows the badge and arms the reset button
// (update_source_info): the button keeps its place, invisible and
// inert, while there is no runtime override.
func (r *settingRow) refreshSource() {
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
