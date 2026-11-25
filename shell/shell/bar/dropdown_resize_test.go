package bar

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// recordingWrites has ctx's config writes recorded, each answered with
// fail.
func recordingWrites(ctx *ModuleContext, fail error) *[]configWrite {
	var writes []configWrite
	ctx.SetConfig = func(path string, value any, done func(error)) {
		writes = append(writes, configWrite{path, value})
		done(fail)
	}
	return &writes
}

// sizedCard is a dropdown card around a panel width wide over a list
// that scrolls, laid out as the popover lays it out.
func sizedCard(t *testing.T, width, height int) (*panelBox, *widget.Revealer) {
	t.Helper()
	cfg := config.Defaults()
	isolateConfigDir(t)
	col := widget.NewBox(widget.Column, 0, 0)
	col.Append(widget.NewSpacer(10, 100), false)
	col.Append(dropdownScroll(widget.NewSpacer(10, 2000), ""), true)
	panel := newPanelBox(col, width, height)
	panel.clamp(1080)
	card := dropdownTree(panel, cfg)
	apptheme.New(cfg).Attach(card)
	layoutCard(card, widget.Size{W: width, H: 1000})
	return panel, card
}

// layoutCard measures and arranges card within ceiling, as the popover
// does, returning its size.
func layoutCard(card widget.Widget, ceiling widget.Size) widget.Size {
	sz := widget.SizeWithinAsChild(card, ceiling)
	widget.ArrangeAsChild(card, render.Rect{W: sz.W, H: sz.H})
	return sz
}

func TestADropdownWithAConfiguredSizeIsResizable(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	panel, card := sizedCard(t, 382, 400)
	resize := dropdownResize("audio", panel, card, widget.Size{W: 1920, H: 1080}, ctx)
	if resize == nil {
		t.Fatal("the audio dropdown, which has a size, is not resizable")
	}
	if resize.Min != dropdownMinSize || resize.Max != (widget.Size{W: 1920, H: 1080 - panelMonitorMargin}) {
		t.Errorf("the drag is bounded %v..%v, want %v..the monitor less its margin", resize.Min, resize.Max, dropdownMinSize)
	}
	asked := widget.Size{W: 500, H: 420}
	resize.Resizing(asked)
	if got := layoutCard(card, asked); got != asked {
		t.Errorf("a drag to %v left the card %v", asked, got)
	}
	if dropdownResize("recorder", panel, card, widget.Size{W: 1920, H: 1080}, ctx) != nil {
		t.Error("the recorder dropdown, which has no size in the config, is resizable")
	}
	if dropdownResize("nope", panel, card, widget.Size{W: 1920, H: 1080}, ctx) != nil {
		t.Error("an unknown dropdown is resizable")
	}
	if small := dropdownResize("audio", panel, card, widget.Size{}, ctx); small.Max.W < small.Min.W || small.Max.H < small.Min.H {
		t.Errorf("an unknown monitor bounds the drag %v..%v", small.Min, small.Max)
	}
}

// The size a drag ends at is saved so the dropdown opens again as large
// as the drag left its card: the card's border is not saved into the
// height its lists scroll at.
func TestADraggedSizeIsSavedAsTheDropdownsWidthAndHeight(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	writes := recordingWrites(&ctx, nil)
	toasts := 0
	ctx.Toast = func(string, string) { toasts++ }
	panel, card := sizedCard(t, 382, 400)
	resize := dropdownResize("notification", panel, card, widget.Size{W: 1920, H: 1080}, ctx)
	ended := widget.Size{W: 460, H: 610}
	resize.Resizing(ended)
	layoutCard(card, ended)
	resize.Resized(ended)
	if len(*writes) != 1 || (*writes)[0].path != "dropdowns.notification" {
		t.Fatalf("the drag wrote %v, want the notification dropdown's size in one write", *writes)
	}
	value, ok := (*writes)[0].value.(map[string]any)
	if !ok || value["width"] != "460px" {
		t.Fatalf("the drag wrote %v, want 460px wide", (*writes)[0].value)
	}
	var height config.PixelLength
	if err := height.UnmarshalConfig(value["height"]); err != nil {
		t.Fatal(err)
	}
	reopened, again := sizedCard(t, 460, int(height))
	if got := layoutCard(again, widget.Size{W: 460, H: 1000}); got != ended {
		t.Errorf("the saved size reopens the card at %v, want the %v the drag left; panel %dx%d", got, ended, reopened.width, reopened.height)
	}
	if toasts != 0 {
		t.Errorf("a saved size toasted %d times", toasts)
	}
}

func TestAnUnsavedDropdownSizeToasts(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	recordingWrites(&ctx, errors.New("config: save: disk full"))
	var toasts []string
	ctx.Toast = func(label, _ string) { toasts = append(toasts, label) }
	saveDropdownSize(ctx, "audio", widget.Size{W: 400, H: 300})
	want := i18n.T("bar-dropdown-size-not-saved")
	if !slices.Equal(toasts, []string{want}) {
		t.Errorf("a failed save toasted %q, want %q", toasts, want)
	}
}

// The size a drag saves reaches the config file's runtime layer, where
// the settings app reads it, and the shell's own config.
func TestADraggedSizeReachesTheConfig(t *testing.T) {
	dir := t.TempDir()
	svc := config.Load(dir, config.DiscardDiagnostics)
	defer svc.Close()
	w := &configWriter{store: svc}
	ctx := newTestContext(t, config.Defaults())
	ctx.SetConfig = w.set
	saveDropdownSize(ctx, "calendar", widget.Size{W: 420, H: 360})
	w.flush(10 * time.Second)
	reread := config.Load(dir, config.DiscardDiagnostics)
	defer reread.Close()
	got := reread.Config().Dropdowns.Calendar
	if got.Width == nil || *got.Width != config.Px(420) || got.Height == nil || *got.Height != 360 {
		t.Fatalf("the saved calendar size reads back %+v", got)
	}
	if src := reread.Source("dropdowns.calendar.width"); src != config.SourceRuntime {
		t.Errorf("the saved width comes from %v, want the runtime layer", src)
	}
	if width, height, _ := dropdownDims("calendar", reread.Config()); width != 420 || height != 360 {
		t.Errorf("the calendar opens %dx%d, want 420x360", width, height)
	}
}

// A new size in the config resizes the built dropdowns where they are,
// so an open one stays open; a change the bars render rebuilds them.
func TestASizeChangeResizesTheBuiltDropdownsInPlace(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	r := newDropdownRegistry(nil, cfg, ctx.Font, ctx.Style, ctx)
	panel, ok := r.instance("calendar", calendarDropdown).(*panelBox)
	if !ok {
		t.Fatal("the calendar is not a sized panel")
	}
	resized := config.Defaults()
	width := config.Px(480)
	height := config.PixelLength(300)
	resized.Dropdowns.Calendar = config.DropdownSize{Width: &width, Height: &height}
	r.setConfig(resized)
	if r.instance("calendar", calendarDropdown) != panel {
		t.Fatal("a size change rebuilt the calendar")
	}
	if panel.width != 480 || panel.height != 300 {
		t.Errorf("the calendar panel is %dx%d after the size change, want 480x300", panel.width, panel.height)
	}
	moved := config.Defaults()
	moved.Bar.Location = config.LocationBottom
	r.setConfig(moved)
	if r.instance("calendar", calendarDropdown) == panel {
		t.Error("a bar change kept the built calendar")
	}
}
