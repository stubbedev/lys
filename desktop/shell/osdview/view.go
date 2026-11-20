// Package osdview draws lys-shell's on-screen display from a config and
// an event: the card with its slider, toggle and track headers, the
// classes the osd stylesheet keys on. It holds no service or surface,
// so lys-shell's OSD maps it on a layer surface and lys-settings
// previews it from a draft config with the same code.
package osdview

import (
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
)

const iconPx = 20

const widthPx = 288

const mediaArtPx = 48

// View is the OSD's tree, built once at a styling scale and updated by
// Apply for each event:
//
//	revealer
//	└ box.osd (vertical; muted, toggle-off, bordered, a toast's class)
//	  ├ box.osd-header        (sliders)  image.osd-icon, label.osd-label, label.osd-value
//	  ├ box.osd-header        (toggles)  image.osd-icon, label.osd-label
//	  ├ box.osd-header.osd-media (tracks) box.osd-media-art, the title and artist
//	  └ progressbar.osd-bar   (sliders)
type View struct {
	font         render.Font
	scale        float64
	rev          *widget.Revealer
	container    *widget.Box
	sliderHeader *widget.Box
	sliderIcon   *widget.Icon
	sliderLabel  *widget.Label
	value        *widget.Label
	toggleHeader *widget.Box
	toggleIcon   *widget.Icon
	toggleLabel  *widget.Label
	mediaHeader  *widget.Box
	art          *widget.Box
	artURL       string
	mediaTitle   *widget.Label
	mediaArtist  *widget.Label
	bar          *widget.ProgressBar
}

// New builds the tree at scale, its labels in font, hidden.
func New(font render.Font, scale float64) *View {
	th := widget.Current()
	v := &View{font: font, scale: scale}
	v.sliderIcon = v.icon()
	v.sliderLabel = v.label("osd-label")
	v.sliderLabel.SetHExpand(true)
	v.sliderLabel.SetHAlign(widget.AlignStart)
	v.sliderLabel.SetEllipsize(widget.EllipsizeEnd)
	v.value = v.label("osd-value")
	v.value.AddClass(css.Heading, css.Numeric)
	v.sliderHeader = header(v.sliderIcon, v.sliderLabel, v.value)

	v.toggleIcon = v.icon()
	v.toggleLabel = v.label("osd-label")
	v.toggleHeader = header(v.toggleIcon, v.toggleLabel)

	v.art = widget.NewBox(widget.Row, 0, 0)
	v.art.AddClass("osd-media-art")
	widgets.Pin(v.art, int(mediaArtPx*scale), int(mediaArtPx*scale))
	v.artURL = "\x00"
	v.mediaTitle = v.label("osd-media-title")
	v.mediaTitle.AddClass(css.Heading)
	v.mediaArtist = v.label("osd-media-artist")
	v.mediaArtist.AddClass(css.DimLabel)
	track := widget.NewBox(widget.Column, th.Spacing/2, 0)
	for _, l := range []*widget.Label{v.mediaTitle, v.mediaArtist} {
		l.SetEllipsize(widget.EllipsizeEnd)
		l.SetHAlign(widget.AlignStart)
		track.Append(l, false)
	}
	track.SetVAlign(widget.AlignCenter)
	track.SetHExpand(true)
	v.mediaHeader = header(v.art, track)
	v.mediaHeader.AddClass("osd-media")

	v.bar = widget.NewProgressBar(0)
	v.bar.AddClass("osd-bar")

	v.container = widget.NewBox(widget.Column, th.Spacing, 2*th.Padding)
	v.container.SetInlineStyle(styling.WidthStyle(int(widthPx * scale)))
	v.container.Append(v.sliderHeader, false)
	v.container.Append(v.toggleHeader, false)
	v.container.Append(v.mediaHeader, false)
	v.container.Append(v.bar, false)
	v.rev = widget.NewRevealer(v.container)
	v.rev.SetFadeFrom(reveal.FadeFrom)
	return v
}

// Preview is the card showing ev under cfg, revealed at once.
func Preview(cfg *config.Config, font render.Font, ev Event) *View {
	v := New(font, float64(cfg.Styling.Scale))
	v.Apply(ev, cfg.Osd)
	v.rev.SetDuration(0)
	v.rev.SetRevealed(true)
	return v
}

// Root is the tree's root, the revealer that shows and hides the card.
func (v *View) Root() *widget.Revealer { return v.rev }

// Scale is the styling scale the tree was built at.
func (v *View) Scale() float64 { return v.scale }

// Host makes the revealer the window node the stylesheet's
// window.background.osd-host rule clears: the surface's root, so its
// fade runs as the surface's opacity instead of repainting the card.
func (v *View) Host(sheet *widget.Stylesheet) {
	v.rev.SetElement("window")
	v.rev.AddClass("background", "osd-host")
	if sheet != nil {
		v.rev.AttachStylesheet(sheet)
	}
}

func (v *View) icon() *widget.Icon {
	icon := widget.NewThemeIcon("", iconPx)
	icon.AddClass("osd-icon")
	icon.SetVAlign(widget.AlignCenter)
	return icon
}

func (v *View) label(class string) *widget.Label {
	l := widget.NewLabel(v.font, widgets.TextPx(styling.Body, v.scale), "", 0)
	l.AddClass(class)
	l.SetVAlign(widget.AlignCenter)
	return l
}

func header(children ...widget.Widget) *widget.Box {
	b := widget.NewBox(widget.Row, widget.Current().Spacing, 0)
	b.AddClass("osd-header")
	for _, c := range children {
		b.Append(c, false)
	}
	return b
}

// Apply shows ev under cfg: a level shows the slider header and the
// bar, a track the media header, everything else the toggle header
// placed by osd.text-align.
func (v *View) Apply(ev Event, cfg config.OsdConfig) {
	v.container.SetClasses(append(classes(ev, cfg.Border), css.Card)...)
	v.sliderHeader.SetVisible(ev.bar)
	v.bar.SetVisible(ev.bar)
	media := ev.kind == kindMedia
	v.toggleHeader.SetVisible(!ev.bar && !media)
	v.mediaHeader.SetVisible(media)
	v.toggleHeader.SetHAlign(textAlign(cfg.TextAlign))
	resolves := widget.ThemeIconExists(ev.icon)
	for _, icon := range [...]*widget.Icon{v.sliderIcon, v.toggleIcon} {
		icon.SetThemeName(ev.icon)
		icon.SetVisible(resolves)
	}
	if media {
		v.showTrack(ev)
		return
	}
	if ev.bar {
		v.sliderLabel.SetText(ev.label)
		v.value.SetText(valueText(ev))
		v.bar.SetValue(fraction(ev))
		return
	}
	v.toggleLabel.SetText(headerLabel(ev))
}

func textAlign(a config.OsdTextAlign) widget.Align {
	switch a {
	case config.OsdTextAlignStart:
		return widget.AlignStart
	case config.OsdTextAlignEnd:
		return widget.AlignEnd
	}
	return widget.AlignCenter
}

func (v *View) showTrack(ev Event) {
	v.mediaTitle.SetText(ev.label)
	v.mediaArtist.SetText(ev.artist)
	v.mediaArtist.SetVisible(ev.artist != "")
	if ev.art == v.artURL {
		return
	}
	v.artURL = ev.art
	v.art.Clear()
	if img, ok := widgets.MediaArt(ev.art); ok {
		v.art.Append(img, true)
		return
	}
	icon := widget.NewThemeIcon(ev.icon, iconPx)
	icon.AddClass("osd-icon")
	v.art.Append(widgets.Center(icon), true)
}
