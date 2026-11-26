package launcher

import (
	"context"
	"errors"
	"image"
	"log"
	"math"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/launcherview"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	engine "github.com/stubbedev/lys/shell/service/launcher"
)

const (
	// previewWidth is the pane's width, in logical pixels at scale 1.
	previewWidth = 360
	// previewMinRows is the least the list area is tall beside a preview:
	// a pane one row high shows nothing.
	previewMinRows = 7
	// heroIconSize is the summary's picture, in logical pixels at scale 1.
	heroIconSize = 72
	// previewPad is the pane's padding, and previewTextH the height a
	// file's text shows in, in logical pixels at scale 1.
	previewPad   = 12
	previewTextH = 260
)

// previewSettle is how long the selection must rest before its preview
// loads: arrowing through a list reads no files and captures no windows.
const previewSettle = 40 * time.Millisecond

// previewPane is the side pane every kind of session shows its selected
// row in: a file as Quick Look shows it, a picture, text, the facts that
// describe the row, or, with nothing to load, the row's own summary. It
// is built once; showing another row rebinds its widgets.
type previewPane struct {
	box     *paneBox
	file    *preview.View
	image   *widget.Image
	code    *widgets.CodeView
	facts   *widget.Box
	rows    []factRow
	summary *rowSummary
	font    render.Font
	// sources are what the file shown loads through: the session's.
	// fileShown is the file's preview in the pane, which holds it only
	// while it shows: a hidden subtree still costs every frame.
	sources   atomic.Pointer[preview.Sources]
	fileShown bool
	// shown is the picture the image widget holds.
	shown image.Image
	// width is the pane's width in pixels at the session's scale.
	width int
}

// paneBox is the pane's column, height tall whatever it shows: what is
// previewed never resizes the panel around it.
type paneBox struct {
	*widget.Box
	height int
}

// Measure is the content's width and the fixed height.
func (b *paneBox) Measure(axis widget.Axis, forSize int) widget.Sizes {
	if axis == widget.Column {
		return widget.FixedSizes(b.height)
	}
	return b.Box.Measure(axis, forSize)
}

// factRow is one labelled line under the content.
type factRow struct {
	box   *widget.Box
	label *widget.Label
	value *widget.Label
}

func newPreviewPane(font render.Font, px func(int) int, height int, invoke, spawn func(func())) *previewPane {
	p := &previewPane{box: &paneBox{Box: widget.NewBox(widget.Column, 0, px(previewPad)), height: height}, font: font, width: px(previewWidth)}
	p.box.AddClass("launcher-preview")
	p.box.SetInlineStyle(styling.WidthStyle(p.width))
	inner := p.width - 2*px(previewPad) - 4*widget.Current().Padding
	p.file = preview.New(preview.Config{
		Font: font, Class: "launcher-preview-file", Image: image.Pt(inner, inner*3/4), Text: image.Pt(inner, px(previewTextH)),
		IconPx: px(heroIconSize), NameChars: 28, MIME: mime.System(), Invoke: invoke, Spawn: spawn,
		Load: func(ctx context.Context, req preview.Request) preview.Content {
			return preview.Load(ctx, *p.sources.Load(), req)
		},
	})
	p.image = widget.NewImage(nil)
	p.image.SetScale(widget.ImageFit)
	p.image.AddClass("launcher-preview-image")
	p.code = widgets.NewCodeView(font, launcherview.SmallPx())
	p.facts = widget.NewBox(widget.Column, px(4), 0)
	p.facts.AddClass("launcher-preview-facts")
	p.summary = newRowSummary(font, px)
	p.box.Append(p.image, true)
	p.box.Append(p.code, true)
	p.box.Append(p.facts, false)
	p.box.Append(p.summary.box, false)
	p.show(nil, engine.PreviewContent{}, nil)
	p.box.SetVisible(false)
	return p
}

// showFile shows the file at path as Quick Look does, loaded through
// sources.
func (p *previewPane) showFile(path string, sources preview.Sources) {
	p.sources.Store(&sources)
	p.image.SetVisible(false)
	p.code.SetVisible(false)
	p.showFacts(nil)
	p.summary.box.SetVisible(false)
	if !p.fileShown {
		p.box.InsertAt(0, p.file.Root(), false)
		p.fileShown = true
	}
	p.file.Show(path)
}

// show binds a row's facts and loaded content; err shows the facts
// alone. It reports whether the pane shows anything, so the caller can
// fall back to the row's summary.
func (p *previewPane) show(facts []engine.PreviewFact, c engine.PreviewContent, err error) bool {
	if p.fileShown {
		p.file.Close()
		p.box.Remove(p.file.Root())
		p.fileShown = false
	}
	var img image.Image
	text := ""
	if err == nil {
		img, text = c.Image, c.Text
	}
	p.image.SetVisible(img != nil)
	if img != nil && img != p.shown {
		p.shown = img
		p.image.SetImage(img)
	}
	p.code.SetVisible(text != "")
	if text != "" && text != p.code.Text() {
		p.code.SetText(text, "")
	}
	p.showFacts(facts)
	p.summary.box.SetVisible(false)
	return img != nil || text != "" || len(facts) > 0
}

// showFacts binds facts to the pooled rows, growing the pool only past
// the most facts any row had.
func (p *previewPane) showFacts(facts []engine.PreviewFact) {
	for len(p.rows) < len(facts) {
		r := factRow{box: widget.NewBox(widget.Row, 8, 0)}
		r.label = widget.NewLabel(p.font, launcherview.SmallPx(), "", 0)
		r.label.AddClass("launcher-preview-fact-label", css.DimLabel)
		r.value = widget.NewLabel(p.font, launcherview.SmallPx(), "", 0)
		r.value.AddClass("launcher-preview-fact-value")
		r.value.SetWrap(true)
		r.box.AppendAligned(r.label, false, widget.AlignStart)
		r.box.Append(r.value, true)
		p.facts.Append(r.box, false)
		p.rows = append(p.rows, r)
	}
	for i, r := range p.rows {
		r.box.SetVisible(i < len(facts))
		if i < len(facts) {
			r.label.SetText(facts[i].Label)
			r.value.SetText(facts[i].Value)
		}
	}
	p.facts.SetVisible(len(facts) > 0)
}

// showSummary shows what a row says of itself, in place of content.
func (p *previewPane) showSummary(bind func(*launcherview.AsyncIcon, string), icon, glyph, title, subtitle string) {
	p.show(nil, engine.PreviewContent{}, nil)
	p.summary.box.SetVisible(true)
	p.summary.show(bind, icon, glyph, title, subtitle)
}

// rowSummary is the pane for a row with nothing to load: its picture
// large, its title and its subtitle.
type rowSummary struct {
	box      *widget.Box
	icon     *launcherview.AsyncIcon
	glyph    *widget.Label
	title    *widget.Label
	subtitle *widget.Label
}

func newRowSummary(font render.Font, px func(int) int) *rowSummary {
	s := &rowSummary{box: widget.NewBox(widget.Column, px(8), 0)}
	s.box.AddClass("launcher-preview-summary")
	s.box.SetPadding(render.Insets{Top: px(24)})
	s.icon = launcherview.NewAsyncIcon(px(heroIconSize), "launcher-preview-icon")
	s.box.AppendAligned(s.icon.Slot, false, widget.AlignCenter)
	s.glyph = widget.NewLabel(font, float64(px(heroIconSize))*launcherview.GlyphFontFactor, "", 0)
	s.box.AppendAligned(s.glyph, false, widget.AlignCenter)
	s.title = widget.NewLabel(font, math.Round(launcherview.FontPx()*1.15), "", 0)
	s.title.AddClass("launcher-preview-title")
	s.title.SetWrap(true)
	s.title.SetAlignment(render.AlignCenter)
	s.box.Append(s.title, false)
	s.subtitle = widget.NewLabel(font, launcherview.SmallPx(), "", 0)
	s.subtitle.AddClass(css.DimLabel)
	s.subtitle.SetWrap(true)
	s.subtitle.SetAlignment(render.AlignCenter)
	s.box.Append(s.subtitle, false)
	return s
}

func (s *rowSummary) show(bind func(*launcherview.AsyncIcon, string), icon, glyph, title, subtitle string) {
	s.glyph.SetVisible(glyph != "")
	s.glyph.SetText(glyph)
	s.icon.Slot.SetVisible(glyph == "")
	if glyph == "" {
		bind(s.icon, icon)
	}
	s.title.SetText(title)
	s.subtitle.SetText(subtitle)
	s.subtitle.SetVisible(subtitle != "")
}

// previewLoader waits for the selection to rest and loads the selected
// row's preview off the UI loop. Starting another row drops the last
// one's wait and load, so only the row the selection rests on loads.
type previewLoader struct {
	after   func(time.Duration, func()) func()
	spawn   func(func())
	invoke  func(func())
	sources preview.Sources
	// stopWait and stopLoad end the current row's wait and load.
	stopWait func()
	stopLoad context.CancelFunc
	// loads counts the previews loaded: the reads and captures asked for.
	loads int
	// shown is the pane a file was last shown in.
	shown *previewPane
}

// newPreviewLoader loads previews on goroutines spawn starts, a file
// through sources.
func newPreviewLoader(d Deps, spawn func(func()), sources preview.Sources) *previewLoader {
	return &previewLoader{after: d.After, spawn: spawn, invoke: d.Invoke, sources: sources}
}

// start drops the last row's wait and load, and returns the context the
// next row loads under.
func (l *previewLoader) start(parent context.Context) context.Context {
	l.stop()
	ctx, cancel := context.WithCancel(parent)
	l.stopLoad = cancel
	return ctx
}

// stop drops the current row's wait and load.
func (l *previewLoader) stop() {
	if l.stopWait != nil {
		l.stopWait()
		l.stopWait = nil
	}
	if l.stopLoad != nil {
		l.stopLoad()
		l.stopLoad = nil
	}
}

// close drops the current row's wait and load, and closes the file
// shown, which lets go of what it holds: the session is over.
func (l *previewLoader) close() {
	l.stop()
	if l.shown != nil {
		l.shown.file.Close()
		l.shown = nil
	}
}

// settle runs fn once the selection has rested.
func (l *previewLoader) settle(fn func()) { l.stopWait = l.after(previewSettle, fn) }

// fetch shows p in pane: a file at once, as Quick Look does, which
// loads itself; anything else loaded off the loop, on a goroutine spawn
// starts, and handed to done on the loop, unless ctx ended first.
// Nothing to load is no failure; anything else is logged and handed on,
// so the pane shows what it still can.
func (l *previewLoader) fetch(ctx context.Context, pane *previewPane, p engine.Preview, done func(engine.PreviewContent, error)) {
	l.loads++
	if p.Path != "" {
		l.shown = pane
		pane.showFile(p.Path, l.sources)
		return
	}
	l.spawn(func() {
		content, err := engine.LoadPreview(ctx, p)
		l.invoke(func() {
			if ctx.Err() != nil {
				return
			}
			if err != nil && !errors.Is(err, engine.ErrNoPreview) {
				log.Printf("launcher: preview: %v", err)
			}
			done(content, err)
		})
	})
}
