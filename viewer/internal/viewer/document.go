package viewer

import (
	"errors"
	"image"
	"log"
	"math"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/viewer/internal/locale"
)

const (
	outlineIcon      = "ld-list-symbolic"
	previousPageIcon = "ld-chevron-up-symbolic"
	nextPageIcon     = "ld-chevron-down-symbolic"
	searchIcon       = "ld-search-symbolic"
	closeSearchIcon  = "ld-x-symbolic"
	scrollStepPx     = 40
	dragSlopPx       = 3
	outlineIndentPx  = 12
	pageEntryChars   = 6
	searchEntryChars = 24
)

var (
	matchColor   = render.RGBA(255, 214, 0, 96)
	currentColor = render.RGBA(255, 128, 0, 128)
)

type document struct {
	host Host
	open func(path string) *dochelper.Client
	font render.Font

	root          *widget.Box
	outline       *widget.Scroll
	outlineList   *widget.Box
	outlineToggle *widget.ToggleButton
	pageEntry     *widget.Entry
	pageCount     *widget.Label
	previousPage  *widget.Button
	nextPage      *widget.Button
	searchBar     *widget.Box
	searchEntry   *widget.Entry
	searchStatus  *widget.Label
	message       *widget.Label
	area          *pageArea

	path      string
	client    *dochelper.Client
	gen       uint64
	info      dochelper.Info
	page      int
	view      pageView
	box       image.Point
	shown     dochelper.Tile
	hasShown  bool
	rendering bool
	failed    *dochelper.Tile

	textPage int
	text     pageText
	links    []dochelper.Link

	selection span
	anchor    int
	press     widget.Point
	pressed   bool
	moved     bool
	pinch     pageView

	query   string
	matches []span
	match   int
	land    int
	search  atomic.Uint64
	typing  int
}

const (
	landNone = iota
	landFirst
	landLast
)

func newDocument(font render.Font, host Host, open func(string) *dochelper.Client) *document {
	d := &document{host: host, open: open, font: font, textPage: -1, anchor: -1, match: -1}
	th := widget.Current()
	body := widgets.TextPx(styling.Body, 1)

	d.outlineToggle = widget.NewToggleButton(widget.NewThemeIcon(outlineIcon, toolbarIconPx), 0, 0)
	label(d.outlineToggle, "viewer-outline")
	d.outlineToggle.OnToggled = func(on bool) { d.outline.SetVisible(on) }
	d.previousPage = d.button(previousPageIcon, "viewer-previous-page", func() { d.pageStep(-1) })
	d.nextPage = d.button(nextPageIcon, "viewer-next-page", func() { d.pageStep(1) })
	d.pageEntry = widget.NewEntry(font, body, 0)
	d.pageEntry.SetWidthChars(pageEntryChars, pageEntryChars)
	d.pageEntry.SetAccessibleName(locale.T().Get("viewer-page"))
	d.pageEntry.OnActivate = d.enterPage
	d.typed(d.pageEntry)
	d.pageCount = widget.NewLabel(font, body, "", 0)
	d.pageCount.AddClass(css.DimLabel)
	search := d.button(searchIcon, "viewer-search", func() { d.startSearch() })

	bar := widget.NewBox(widget.Row, th.Spacing, th.Padding)
	bar.AddClass(css.Toolbar)
	bar.Append(d.outlineToggle, false)
	bar.Append(widget.NewSpacer(0, 0), true)
	bar.Append(d.previousPage, false)
	bar.Append(d.pageEntry, false)
	bar.Append(d.pageCount, false)
	bar.Append(d.nextPage, false)
	bar.Append(widget.NewSpacer(0, 0), true)
	bar.Append(search, false)

	d.searchEntry = widget.NewEntry(font, body, 0)
	d.searchEntry.SetWidthChars(searchEntryChars, searchEntryChars)
	d.searchEntry.SetPlaceholder(locale.T().Get("viewer-search-placeholder"))
	d.searchEntry.SetAccessibleName(locale.T().Get("viewer-search"))
	d.searchEntry.OnChanged = d.searchFor
	d.searchEntry.OnActivate = func(string) { d.findNext(1) }
	d.searchEntry.OnEscape = d.endSearch
	d.typed(d.searchEntry)
	d.searchStatus = widget.NewLabel(font, body, "", 0)
	d.searchStatus.AddClass(css.DimLabel)
	d.searchBar = widget.NewBox(widget.Row, th.Spacing, th.Padding)
	d.searchBar.AddClass(css.Toolbar)
	d.searchBar.Append(d.searchEntry, true)
	d.searchBar.Append(d.searchStatus, false)
	d.searchBar.Append(d.button(previousPageIcon, "viewer-previous-match", func() { d.findNext(-1) }), false)
	d.searchBar.Append(d.button(nextPageIcon, "viewer-next-match", func() { d.findNext(1) }), false)
	d.searchBar.Append(d.button(closeSearchIcon, "viewer-close-search", func() { d.endSearch() }), false)
	d.searchBar.SetVisible(false)

	d.message = widget.NewLabel(font, body, "", 0)
	d.message.AddClass("viewer-document-message")
	d.message.SetVisible(false)
	d.area = newPageArea(d)

	d.outlineList = widget.NewBox(widget.Column, 0, th.Padding)
	d.outline = widget.NewScroll(d.outlineList)
	d.outline.AddClass("viewer-outline")
	d.outline.SetVisible(false)

	pages := widget.NewBox(widget.Column, 0, 0)
	pages.Append(bar, false)
	pages.Append(d.searchBar, false)
	pages.Append(d.message, false)
	pages.Append(d.area, true)
	d.root = widget.NewBox(widget.Row, 0, 0)
	d.root.Append(d.outline, false)
	d.root.Append(pages, true)
	d.root.SetVisible(false)
	return d
}

func (d *document) button(icon, id string, click func()) *widget.Button {
	b := widget.NewButton(widget.NewThemeIcon(icon, toolbarIconPx), 0, 0)
	label(b, id)
	b.OnClick = click
	return b
}

func (d *document) typed(e *widget.Entry) {
	e.SetOnFocusChanged(func(focused bool) {
		if focused {
			d.typing++
		} else {
			d.typing = max(d.typing-1, 0)
		}
	})
}

func (d *document) active() bool { return d.client != nil && d.info.Pages() > 0 }

func (d *document) Open(path string) {
	if d.client != nil && path == d.path {
		return
	}
	d.Close()
	d.path = path
	d.root.SetVisible(true)
	d.client = d.open(path)
	d.gen++
	gen, c := d.gen, d.client
	d.host.Spawn(func() {
		info, err := c.Info()
		d.host.Invoke(func() { d.opened(gen, info, err) })
	})
}

func (d *document) opened(gen uint64, info dochelper.Info, err error) {
	if gen != d.gen {
		return
	}
	if err != nil {
		log.Printf("viewer: %v", err)
		d.say(failureMessage(err))
		d.area.SetVisible(false)
		return
	}
	d.info = info
	d.pageCount.SetText(locale.T().Get("viewer-page-count", i18n.Int("pages", int64(info.Pages()))))
	d.fillOutline()
	d.goTo(0, false)
}

func (d *document) Close() {
	if d.client == nil {
		d.root.SetVisible(false)
		return
	}
	c := d.client
	d.host.Spawn(func() {
		if err := c.Close(); err != nil {
			log.Printf("viewer: closing %s: %v", d.path, err)
		}
	})
	d.gen++
	d.search.Add(1)
	d.client, d.info, d.path = nil, dochelper.Info{}, ""
	d.hasShown, d.rendering, d.failed = false, false, nil
	d.textPage, d.text, d.links = -1, pageText{}, nil
	d.selection, d.anchor, d.matches, d.match, d.land = span{}, -1, nil, -1, landNone
	d.area.picture.SetImage(nil, exif.TopLeft)
	d.area.SetVisible(true)
	d.message.SetVisible(false)
	d.outlineList.Clear()
	d.endSearch()
	d.root.SetVisible(false)
}

func (d *document) say(text string) {
	d.message.SetText(text)
	d.message.SetVisible(true)
}

func failureMessage(err error) string {
	for _, f := range []struct {
		err error
		id  string
	}{
		{dochelper.ErrDamaged, "viewer-document-damaged"},
		{dochelper.ErrEncrypted, "viewer-document-encrypted"},
		{dochelper.ErrTooLarge, "viewer-document-too-large"},
		{dochelper.ErrTimeout, "viewer-document-timeout"},
		{dochelper.ErrUnsupported, "viewer-document-unsupported"},
	} {
		if errors.Is(err, f.err) {
			return locale.T().Get(f.id)
		}
	}
	return locale.T().Get("viewer-document-failed")
}

func (d *document) fillOutline() {
	d.outlineList.Clear()
	var add func(entries []dochelper.Outline, depth int)
	add = func(entries []dochelper.Outline, depth int) {
		for _, e := range entries {
			title := widget.NewLabel(d.font, widgets.TextPx(styling.Body, 1), e.Title, 0)
			title.SetEllipsize(widget.EllipsizeEnd)
			b := widget.NewButton(title, 0, 0)
			b.AddClass(css.Flat, "viewer-outline-entry")
			b.SetInlineStyle("padding-left: " + strconv.Itoa(depth*outlineIndentPx) + "px;")
			b.SetAccessibleName(e.Title)
			switch {
			case e.Page >= 0:
				page := e.Page
				b.OnClick = func() { d.goTo(page, false) }
			case e.URI != "":
				uri := e.URI
				b.OnClick = func() { d.host.OpenURI(uri) }
			default:
				b.SetEnabled(false)
			}
			d.outlineList.Append(b, false)
			add(e.Children, depth+1)
		}
	}
	add(d.info.Outline, 0)
	d.outlineToggle.SetEnabled(len(d.info.Outline) > 0)
	if len(d.info.Outline) == 0 {
		d.outlineToggle.SetActive(false)
	}
}

func (d *document) toggleOutline() bool {
	if !d.active() || len(d.info.Outline) == 0 {
		return false
	}
	d.outlineToggle.SetActive(!d.outlineToggle.Active())
	return true
}

func (d *document) goTo(page int, atEnd bool) {
	keep := d.view
	d.page = page
	d.view = fittedView(d.info.Sizes[page])
	if !keep.fit && keep.scale > 0 {
		d.view.fit, d.view.scale = false, keep.scale
	}
	if atEnd {
		d.view = d.view.atEnd(d.box)
	}
	d.selection, d.anchor, d.matches, d.match = span{}, -1, nil, -1
	d.pageEntry.SetText(d.info.Label(page))
	d.previousPage.SetEnabled(page > 0)
	d.nextPage.SetEnabled(page < d.info.Pages()-1)
	d.fetchPage(page)
	d.refresh()
	d.area.Invalidate()
}

func (d *document) fetchPage(page int) {
	if d.textPage == page {
		d.pageArrived()
		return
	}
	d.textPage, d.text, d.links = -1, pageText{}, nil
	gen, c := d.gen, d.client
	d.host.Spawn(func() {
		text, terr := c.Text(page)
		links, lerr := c.Links(page)
		d.host.Invoke(func() {
			if gen != d.gen || page != d.page {
				return
			}
			for _, err := range []error{terr, lerr} {
				if err != nil {
					log.Printf("viewer: reading page %d of %s: %v", page+1, d.path, err)
				}
			}
			d.textPage, d.text, d.links = page, newPageText(text), links
			d.pageArrived()
		})
	})
}

func (d *document) pageArrived() {
	if d.query == "" {
		return
	}
	d.matches = d.text.find(d.query)
	switch {
	case len(d.matches) == 0:
		d.match = -1
	case d.land == landFirst:
		d.match = 0
	case d.land == landLast:
		d.match = len(d.matches) - 1
	}
	d.land = landNone
	d.showMatch()
}

func (d *document) arranged(box image.Point) {
	if box == d.box {
		return
	}
	d.box = box
	d.view = d.view.clamped(box)
	d.refresh()
}

func (d *document) refresh() {
	if !d.active() {
		return
	}
	t, ok := d.view.tile(d.page, d.box)
	if !ok || (d.hasShown && d.shown == t) || d.rendering || (d.failed != nil && *d.failed == t) {
		return
	}
	d.rendering = true
	gen, c := d.gen, d.client
	d.host.Spawn(func() {
		img, err := c.Render(t)
		d.host.Invoke(func() { d.rendered(gen, t, img, err) })
	})
}

func (d *document) rendered(gen uint64, t dochelper.Tile, img *image.RGBA, err error) {
	if gen != d.gen {
		return
	}
	d.rendering = false
	if err != nil {
		log.Printf("viewer: %v", err)
	}
	if img == nil {
		d.failed = &t
		d.host.Toast(failureMessage(err), nil)
		return
	}
	d.failed = nil
	d.area.picture.SetImage(img, exif.TopLeft)
	d.shown, d.hasShown = t, true
	d.area.Invalidate()
	d.refresh()
}

func (d *document) deviceScale() float64 {
	num, denom := widget.DeviceScaleOf(d.area)
	if num == 0 {
		return 1
	}
	return float64(num) / float64(denom)
}

func (d *document) devicePx(px int) int { return int(math.Round(float64(px) * d.deviceScale())) }

func (d *document) boxPoint(p widget.Point) image.Point {
	b := d.area.Bounds()
	s := d.deviceScale()
	return image.Pt(int(float64(p.X-b.X)*s), int(float64(p.Y-b.Y)*s))
}

func (d *document) centre() image.Point { return image.Pt(d.box.X/2, d.box.Y/2) }

func (d *document) setView(v pageView) {
	d.view = v
	d.refresh()
}

func (d *document) pageStep(delta int) bool {
	if !d.active() {
		return false
	}
	next := d.page + delta
	if next < 0 || next >= d.info.Pages() {
		return false
	}
	d.goTo(next, false)
	return true
}

func (d *document) firstPage() bool { return d.jumpPage(0) }

func (d *document) lastPage() bool { return d.jumpPage(d.info.Pages() - 1) }

func (d *document) jumpPage(page int) bool {
	if !d.active() {
		return false
	}
	d.goTo(page, false)
	return true
}

func (d *document) scroll(dx, dy int) bool {
	if !d.active() {
		return false
	}
	v, e := d.view.scrolled(dx, dy, d.box)
	switch e {
	case pastStart:
		if d.page == 0 {
			return false
		}
		d.goTo(d.page-1, dy < 0)
	case pastEnd:
		if d.page == d.info.Pages()-1 {
			return false
		}
		d.goTo(d.page+1, false)
	default:
		d.setView(v)
	}
	return true
}

func (d *document) scrollKey(x, y int) bool {
	step := d.devicePx(scrollStepPx)
	return d.scroll(x*step, y*step)
}

func (d *document) scrollScreen() bool { return d.scroll(0, d.box.Y) }

func (d *document) zoom(r rune) bool {
	if !d.active() {
		return false
	}
	switch r {
	case '+':
		d.setView(d.view.zoomed(zoomStep, d.centre(), d.box))
	case '-':
		d.setView(d.view.zoomed(1/zoomStep, d.centre(), d.box))
	case '0':
		d.setView(fittedView(d.view.size))
	case '1':
		d.setView(d.view.zoomedAt(actualScale*d.deviceScale(), d.centre(), d.box))
	default:
		return false
	}
	return true
}

func (d *document) enterPage(text string) {
	if !d.active() {
		return
	}
	text = strings.TrimSpace(text)
	for i := range d.info.Pages() {
		if d.info.Label(i) == text {
			d.goTo(i, false)
			return
		}
	}
	if n, err := strconv.Atoi(text); err == nil && n >= 1 && n <= d.info.Pages() {
		d.goTo(n-1, false)
		return
	}
	d.pageEntry.SetText(d.info.Label(d.page))
	d.host.Toast(locale.T().Get("viewer-no-such-page", i18n.Str("page", text)), nil)
}

// viewAt is where the page point (x, y) of the shown tile is in the
// view, the inverse of pagePoint.
func (d *document) viewAt(x, y float64) (float64, float64) {
	return d.area.picture.ViewAt(float64(x*d.shown.Scale)-float64(d.shown.Rect.Min.X), float64(y*d.shown.Scale)-float64(d.shown.Rect.Min.Y))
}

func (d *document) pagePoint(p widget.Point) (float64, float64, bool) {
	if !d.hasShown || d.shown.Page != d.page {
		return 0, 0, false
	}
	x, y := d.area.picture.PictureAt(p)
	return (x + float64(d.shown.Rect.Min.X)) / d.shown.Scale, (y + float64(d.shown.Rect.Min.Y)) / d.shown.Scale, true
}

func (d *document) pressAt(p widget.Point) {
	d.press, d.pressed, d.moved = p, true, false
	d.anchor = -1
	if x, y, ok := d.pagePoint(p); ok {
		d.anchor = d.text.charAt(x, y)
	}
	if d.selection != (span{}) {
		d.selection = span{}
		d.area.Invalidate()
	}
}

func (d *document) dragTo(p widget.Point) {
	if !d.pressed {
		return
	}
	if !d.moved && max(abs(p.X-d.press.X), abs(p.Y-d.press.Y)) <= dragSlopPx {
		return
	}
	d.moved = true
	x, y, ok := d.pagePoint(p)
	if !ok || d.anchor < 0 {
		return
	}
	at := d.text.charAt(x, y)
	d.selection = span{from: min(d.anchor, at), to: max(d.anchor, at) + 1}
	d.area.Invalidate()
}

func abs(n int) int { return max(n, -n) }

func (d *document) clickAt(p widget.Point) {
	if d.moved {
		return
	}
	x, y, ok := d.pagePoint(p)
	if !ok {
		return
	}
	l, ok := linkAt(d.links, x, y)
	switch {
	case !ok:
	case l.Page >= 0:
		d.goTo(l.Page, false)
	default:
		d.host.OpenURI(l.URI)
	}
}

func (d *document) cursorAt(p widget.Point) string {
	x, y, ok := d.pagePoint(p)
	if !ok {
		return ""
	}
	if _, ok := linkAt(d.links, x, y); ok {
		return "pointer"
	}
	if i := d.text.charAt(x, y); i >= 0 && d.text.chars[i].Box.Contains(x, y) {
		return "text"
	}
	return ""
}

func (d *document) selectedText() string {
	if d.selection.to <= d.selection.from {
		return ""
	}
	return d.text.text(d.selection)
}

func (d *document) copySelection() bool {
	text := d.selectedText()
	if !d.active() || text == "" {
		return false
	}
	if err := d.host.Copy(transfer.Text(text)); err != nil {
		log.Printf("viewer: copying the selection: %v", err)
		d.host.Toast(locale.T().Get("viewer-copy-text-failed"), nil)
		return true
	}
	d.host.Toast(locale.T().Get("viewer-text-copied"), nil)
	return true
}

func (d *document) startSearch() bool {
	if !d.active() {
		return false
	}
	d.searchBar.SetVisible(true)
	d.host.Focus(d.searchEntry)
	return true
}

func (d *document) endSearch() bool {
	if !d.searchBar.Visible() {
		return false
	}
	d.search.Add(1)
	d.searchBar.SetVisible(false)
	d.searchEntry.SetText("")
	d.query, d.matches, d.match, d.land = "", nil, -1, landNone
	d.searchStatus.SetText("")
	d.area.Invalidate()
	return true
}

func (d *document) searchFor(query string) {
	d.search.Add(1)
	d.query = strings.TrimSpace(query)
	d.matches, d.match = nil, -1
	if d.textPage == d.page {
		d.matches = d.text.find(d.query)
	}
	d.searchStatus.SetText("")
	d.area.Invalidate()
}

func (d *document) findNext(dir int) bool {
	if !d.active() || d.query == "" {
		return false
	}
	if d.textPage == d.page {
		if next := d.match + dir; d.match >= 0 && next >= 0 && next < len(d.matches) || d.match < 0 && len(d.matches) > 0 {
			if d.match < 0 {
				next = map[int]int{1: 0, -1: len(d.matches) - 1}[dir]
			}
			d.match = next
			d.showMatch()
			return true
		}
	}
	seq := d.search.Add(1)
	gen, c, from, query, pages := d.gen, d.client, d.page, d.query, d.info.Pages()
	d.searchStatus.SetText(locale.T().Get("viewer-searching"))
	d.host.Spawn(func() {
		for i := 1; i <= pages; i++ {
			if d.search.Load() != seq {
				return
			}
			page := ((from+dir*i)%pages + pages) % pages
			text, err := c.Text(page)
			if err != nil {
				log.Printf("viewer: searching page %d: %v", page+1, err)
				continue
			}
			if len(newPageText(text).find(query)) > 0 {
				d.host.Invoke(func() { d.found(gen, seq, page, dir) })
				return
			}
		}
		d.host.Invoke(func() { d.found(gen, seq, -1, dir) })
	})
	return true
}

func (d *document) found(gen, seq uint64, page, dir int) {
	if gen != d.gen || seq != d.search.Load() {
		return
	}
	if page < 0 {
		d.searchStatus.SetText(locale.T().Get("viewer-not-found"))
		return
	}
	d.searchStatus.SetText("")
	land := landFirst
	if dir < 0 {
		land = landLast
	}
	if page == d.page {
		d.land = land
		d.pageArrived()
		return
	}
	d.goTo(page, false)
	d.land = land
	if d.textPage == page {
		d.pageArrived()
	}
}

func (d *document) showMatch() {
	if d.match >= 0 && d.match < len(d.matches) {
		boxes := d.text.boxes(d.matches[d.match])
		d.setView(d.view.showing(union(boxes), d.box))
		d.searchStatus.SetText(locale.T().Get("viewer-match", i18n.Int("match", int64(d.match+1)), i18n.Int("matches", int64(len(d.matches)))))
	}
	d.area.Invalidate()
}

func union(boxes []dochelper.Box) dochelper.Box {
	u := boxes[0]
	for _, b := range boxes[1:] {
		u = dochelper.Box{X0: min(u.X0, b.X0), Y0: min(u.Y0, b.Y0), X1: max(u.X1, b.X1), Y1: max(u.Y1, b.Y1)}
	}
	return u
}

func (d *document) marks() []mark {
	if !d.hasShown || d.shown.Page != d.page || d.textPage != d.page {
		return nil
	}
	var out []mark
	for i, m := range d.matches {
		col := matchColor
		if i == d.match {
			col = currentColor
		}
		for _, b := range d.text.boxes(m) {
			out = append(out, mark{box: b, color: col})
		}
	}
	accent := widget.Current().Accent
	selected := render.RGBA(accent.R(), accent.G(), accent.B(), 96)
	if d.selection.to > d.selection.from {
		for _, b := range d.text.boxes(d.selection) {
			out = append(out, mark{box: b, color: selected})
		}
	}
	return out
}

type mark struct {
	box   dochelper.Box
	color render.Color
}

func (d *document) paintMarks(cv *render.Canvas) {
	num, denom := cv.DeviceScale()
	s := float64(num) / float64(denom)
	for _, m := range d.marks() {
		x0, y0 := d.viewAt(m.box.X0, m.box.Y0)
		x1, y1 := d.viewAt(m.box.X1, m.box.Y1)
		r := render.Rect{X: int(math.Floor(x0 * s)), Y: int(math.Floor(y0 * s))}
		r.W, r.H = int(math.Ceil(x1*s))-r.X, int(math.Ceil(y1*s))-r.Y
		cv.FillRectDevice(r, m.color)
	}
}

type pageArea struct {
	widget.Base
	doc     *document
	picture *widget.ImageView
	kids    [1]widget.Widget
	cursor  string
}

func newPageArea(d *document) *pageArea {
	a := &pageArea{Base: widget.NewBase(), doc: d, picture: widget.NewImageView(nil, exif.TopLeft)}
	a.SetElement("viewer-page")
	a.kids[0] = a.picture
	a.SetOnScroll(a.claimScroll)
	return a
}

func (a *pageArea) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

func (a *pageArea) Arrange(r render.Rect) {
	a.ArrangeSelf(r)
	a.picture.Arrange(r)
	widget.SetParents(a, a.kids[:]...)
	s := a.doc.deviceScale()
	a.doc.arranged(image.Pt(int(float64(r.W)*s), int(float64(r.H)*s)))
}

func (a *pageArea) ArrangeRoot(r render.Rect) { a.Arrange(r) }

func (a *pageArea) Paint(cv *render.Canvas) {
	prev := cv.PushClip(a.Bounds())
	widget.PaintChild(cv, a.picture)
	a.doc.paintMarks(cv)
	cv.PopClip(prev)
}

func (a *pageArea) Children() []widget.Widget { return a.kids[:] }

func (a *pageArea) HitTest(p widget.Point) widget.Widget { return a.HitLeaf(a, p) }

func (a *pageArea) claimScroll(e widget.ScrollEvent) bool {
	if e.Mods&widget.ModCtrl == 0 || e.DY == 0 || !a.doc.active() {
		return false
	}
	d := a.doc
	d.setView(d.view.zoomed(math.Pow(zoomStep, float64(-e.DY)), d.boxPoint(e.At), d.box))
	return true
}

func (a *pageArea) ScrollBy(dx, dy int) { a.doc.scrollKey(dx, dy) }

func (a *pageArea) ScrollPixels(dx, dy float64) {
	d := a.doc
	if !d.active() {
		return
	}
	s := d.deviceScale()
	v, _ := d.view.scrolled(int(math.Round(dx*s)), int(math.Round(dy*s)), d.box)
	d.setView(v)
}

func (a *pageArea) Gesture(g widget.Gesture) bool {
	d := a.doc
	if g.Kind != widget.GesturePinch || !d.active() {
		return false
	}
	switch g.Phase {
	case widget.GestureBegin:
		d.pinch = d.view
	case widget.GestureUpdate:
		d.setView(d.pinch.zoomedAt(d.pinch.scaleIn(d.box)*g.Scale, d.boxPoint(g.At), d.box))
	case widget.GestureCancel:
		d.setView(d.pinch)
	}
	return true
}

func (a *pageArea) PressAt(p widget.Point) { a.doc.pressAt(p) }

func (a *pageArea) DragMove(p widget.Point) { a.doc.dragTo(p) }

func (a *pageArea) SetPressed(on bool) {
	if !on {
		a.doc.pressed = false
	}
}

func (a *pageArea) ClickAt(p widget.Point) { a.doc.clickAt(p) }

func (a *pageArea) HoverMove(p widget.Point) { a.cursor = a.doc.cursorAt(p) }

func (a *pageArea) CursorName() string { return a.cursor }
