package barview

import (
	"math"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
)

// WorkspaceHandlers are what a workspace strip's input does; a nil
// handler leaves its input unbound. i is the index of a button in
// WorkspaceStrip.Shown.
type WorkspaceHandlers struct {
	Select  func(i int)
	Pointer func(i int, e widget.PointerEvent) bool
	Scroll  func(e widget.ScrollEvent) bool
	Hover   func(i int, seg widget.Boundser, on bool)
}

// WorkspaceStrip is the workspaces module's row: lys-ui's
// SegmentIndicator, one segment per workspace button, with the
// travelling active mark —
//
//	segments.workspaces.<kind>[.border-*]    (the --ws-* variables inline)
//	├ indicator.indicator-<kind>[.vertical]
//	└ button.segment.workspace.<state>[...]  (--ws-override-color inline)
//	  └ [box (workspace-padding), for content past the circle]
//	    └ box.workspace-content
//	      ├ box: label.workspace-label, the mapped icon, label.workspace-divider
//	      └ box.workspace-icons > image.workspace-icon...
type WorkspaceStrip struct {
	segs     *widget.SegmentIndicator
	cfg      config.CompositorWorkspacesConfig
	vertical bool
	face     render.Font
	sizes    workspaceSizes
	circle   int
	handlers WorkspaceHandlers
	shown    []WorkspaceButton
	segShown []segmentShown
}

type workspaceSizes struct {
	label            float64
	icon, gap, inset int
}

func newWorkspaceSizes(c config.CompositorWorkspacesConfig, barScale float64) workspaceSizes {
	gap := float64(c.IconGap.Value)
	if c.IconGap.Unit != config.SizePixels {
		gap *= styling.RemBase
	}
	return workspaceSizes{
		label: math.Round(c.LabelSize.ResolvePx(config.WorkspaceLabelBaseRem*styling.RemBase, barScale)),
		icon:  int(math.Round(c.IconSize.ResolvePx(config.WorkspaceIconBaseRem*styling.RemBase, barScale))),
		gap:   int(math.Round(gap)),
		inset: int(math.Round(c.WorkspacePad.ResolvePx(styling.RemBase, barScale))),
	}
}

// WorkspaceFace is the bold face of the bar's font a workspace label
// is drawn in.
func WorkspaceFace(font render.Font) render.Font {
	variants := app.FontVariantsOf(font)
	if variants == nil {
		return font
	}
	return app.FontFallback(variants(true, false))
}

// NewWorkspaceStrip is an empty strip under cfg's bar and the
// workspaces config c, its labels in face. kind names the compositor
// as a class; otherMonitor is Hyprland's active-on-other-monitor
// color, nil elsewhere.
func NewWorkspaceStrip(cfg *config.Config, c config.CompositorWorkspacesConfig, kind string, otherMonitor *config.ColorValue, face render.Font, h WorkspaceHandlers) *WorkspaceStrip {
	s := &WorkspaceStrip{
		segs: widget.NewSegmentIndicator(0), cfg: c, vertical: cfg.Bar.Location.IsVertical(), face: face,
		sizes: newWorkspaceSizes(c, float64(cfg.Bar.Scale)), handlers: h,
	}
	if h.Select != nil {
		s.segs.OnSelect = h.Select
	}
	s.segs.AddClass("workspaces")
	if kind != "" {
		s.segs.AddClass(kind)
	}
	if class, ok := cfg.Bar.ButtonBorderLocation.CSSClass(); ok && c.BorderShow {
		s.segs.AddClass(class)
	}
	s.circle = s.measureCircle()
	s.segs.SetInlineStyle(InlineDecls(styling.WorkspacesCSS(c, otherMonitor, cfg.Bar)) + " --ws-circle: " + strconv.Itoa(s.circle) + "px;")
	drop := s.segs.EnableDrop()
	drop.AddClass(c.ActiveIndicator.CSSClass())
	drop.SetClass("vertical", s.vertical)
	return s
}

// Segments is the strip's widget.
func (s *WorkspaceStrip) Segments() *widget.SegmentIndicator { return s.segs }

// Shown is the buttons the strip shows, in order.
func (s *WorkspaceStrip) Shown() []WorkspaceButton { return s.shown }

// Vertical reports a strip down a side bar.
func (s *WorkspaceStrip) Vertical() bool { return s.vertical }

func (s *WorkspaceStrip) measureCircle() int {
	digit := s.measureLabel("8")
	return max(s.measureLabel("88").W, digit.W+2*s.sizes.inset, digit.H, s.sizes.icon)
}

func (s *WorkspaceStrip) measureLabel(text string) widget.Size {
	label := widget.NewLabel(s.face, s.sizes.label, text, 0)
	width := label.Measure(widget.Row, widget.Unknown).Nat
	return widget.Size{W: width, H: label.Measure(widget.Column, width).Nat}
}

type segmentShown struct {
	classes  []string
	override string
	hasContent,
	showLabel, showIcon, showDivider bool
	label, icon string
	appNames    []string
	appIcons    []*widget.Icon
}

func (c *segmentShown) sameContent(model WorkspaceButton, cfg config.CompositorWorkspacesConfig) bool {
	return c.hasContent && c.label == model.Label && c.icon == model.Icon &&
		c.showLabel == model.ShowLabel(cfg.DisplayMode) && c.showIcon == model.ShowIcon(cfg.DisplayMode) && c.showDivider == model.ShowDivider(cfg) &&
		slices.EqualFunc(c.appNames, model.AppIcons, func(name string, app WorkspaceAppIcon) bool { return name == app.Name })
}

// Set shows one segment per button: the active one through SetActive,
// or ShiftActive for the same workspace at a new index, its occupancy
// and urgency, then the button's classes, the workspace-map color and
// its content. A new segment takes the handlers; what a kept one
// already holds stays.
func (s *WorkspaceStrip) Set(models []WorkspaceButton) {
	segs := s.segs
	previous, hadActive := s.shownActive()
	kept := min(segs.Count(), len(models))
	segs.SetCount(len(models))
	s.segShown = append(s.segShown[:kept], make([]segmentShown, len(models)-kept)...)
	active, activeOverride := -1, ""
	for i, model := range models {
		seg := segs.Segment(i)
		shown := &s.segShown[i]
		segs.SetSegmentState(i, segmentState(model))
		segs.SetSegmentUrgent(i, hasClass(model.Classes, "urgent"))
		classes := slices.DeleteFunc(slices.Clone(model.Classes), segmentOwned)
		if !slices.Equal(shown.classes, classes) {
			seg.RemoveClass(shown.classes...)
			seg.AddClass(classes...)
			shown.classes = classes
		}
		override := ""
		if color, ok := overrideColor(model.Classes, s.cfg.WorkspaceMap); ok {
			override = "--ws-override-color: " + color.ToCSS() + ";"
		}
		if hasClass(model.Classes, "active") {
			active, activeOverride = i, override
		}
		if shown.override != override {
			seg.SetInlineStyle(override)
			shown.override = override
		}
		if i >= kept {
			s.bind(i, seg)
		}
		if shown.sameContent(model, s.cfg) {
			for j, app := range model.AppIcons {
				shown.appIcons[j].SetClass("urgent", app.Urgent)
			}
			continue
		}
		segs.SetSegmentContent(i, s.segmentContent(model, shown))
	}
	if active >= 0 && hadActive && models[active].WS.ID == previous {
		segs.ShiftActive(active)
	} else {
		segs.SetActive(active)
	}
	s.dressDrop(models, active, activeOverride)
	s.shown = models
}

func (s *WorkspaceStrip) bind(i int, seg *widget.Button) {
	h := s.handlers
	if h.Pointer != nil {
		seg.SetOnPointer(func(e widget.PointerEvent) bool { return h.Pointer(i, e) })
	}
	if h.Scroll != nil {
		seg.SetOnScroll(h.Scroll)
	}
	if h.Hover != nil {
		seg.SetOnHoverWithin(func(on bool) { h.Hover(i, seg, on) })
	}
}

func (s *WorkspaceStrip) shownActive() (uint64, bool) {
	i := s.segs.Active()
	if i < 0 {
		return 0, false
	}
	return s.shown[i].WS.ID, true
}

func (s *WorkspaceStrip) dressDrop(models []WorkspaceButton, active int, override string) {
	var classes []string
	if active >= 0 {
		classes = models[active].Classes
	}
	drop := s.segs.EnableDrop()
	drop.SetInlineStyle(override)
	drop.SetClass("urgent", hasClass(classes, "urgent") && !hasClass(classes, "urgent-application"))
	drop.SetClass("special", hasClass(classes, "special"))
}

func segmentOwned(class string) bool {
	return class == "active" || class == "occupied" || class == "urgent"
}

func segmentState(model WorkspaceButton) widget.SegmentState {
	if hasClass(model.Classes, "occupied") {
		return widget.SegmentOccupied
	}
	return widget.SegmentEmpty
}

func (s *WorkspaceStrip) segmentContent(model WorkspaceButton, shown *segmentShown) widget.Widget {
	cfg := s.cfg
	shown.hasContent, shown.label, shown.icon = true, model.Label, model.Icon
	shown.showLabel, shown.showIcon, shown.showDivider = model.ShowLabel(cfg.DisplayMode), model.ShowIcon(cfg.DisplayMode), model.ShowDivider(cfg)
	shown.appNames, shown.appIcons = shown.appNames[:0], shown.appIcons[:0]
	axis := widget.Row
	if s.vertical {
		axis = widget.Column
	}
	content := widget.NewBox(axis, 0, 0)
	content.AddClass("workspace-content")

	fits := !cfg.AppIconsShow
	if shown.showLabel || shown.showIcon || shown.showDivider {
		identity := widget.NewBox(widget.Row, 0, 0)
		if shown.showLabel {
			label := widget.NewLabel(s.face, s.sizes.label, model.Label, 0)
			label.AddClass("workspace-label")
			identity.Append(label, false)
			fits = fits && s.measureLabel(model.Label).W <= s.circle
		}
		if shown.showIcon {
			icon := widget.NewThemeIcon(model.Icon, s.sizes.icon)
			if model.Flavor == FlavorHyprland {
				icon.AddClass("workspace-custom-icon")
			} else {
				icon.AddClass("workspace-icon")
			}
			identity.Append(icon, false)
		}
		if shown.showDivider {
			divider := widget.NewLabel(s.face, s.sizes.label, cfg.Divider, 0)
			divider.AddClass("workspace-divider")
			identity.Append(divider, false)
		}
		content.Append(identity, false)
	}
	if cfg.AppIconsShow {
		icons := widget.NewBox(axis, s.sizes.gap, 0)
		icons.AddClass("workspace-icons")
		if len(model.AppIcons) == 0 {
			icon := widget.NewThemeIcon(cfg.AppIconsEmpty, s.sizes.icon)
			icon.AddClass("workspace-icon", "workspace-icon-empty")
			icons.Append(icon, false)
		}
		for _, app := range model.AppIcons {
			icon := widget.NewThemeIcon(app.Name, s.sizes.icon)
			icon.AddClass("workspace-icon")
			icon.SetClass("urgent", app.Urgent)
			icons.Append(icon, false)
			shown.appNames, shown.appIcons = append(shown.appNames, app.Name), append(shown.appIcons, icon)
		}
		content.Append(icons, false)
	}
	if fits {
		return content
	}
	padded := widget.NewBox(widget.Row, 0, 0)
	padded.Append(content, true)
	if s.vertical {
		padded.SetPadding(render.Insets{Top: s.sizes.inset, Bottom: s.sizes.inset})
	} else {
		padded.SetPadding(render.Insets{Left: s.sizes.inset, Right: s.sizes.inset})
	}
	return padded
}
