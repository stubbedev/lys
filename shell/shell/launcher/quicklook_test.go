package launcher

import (
	"bytes"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
	engine "github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/launcher/search"
)

// lookedAtHistory is a text file to look at, and a clipboard history
// holding it as a copied file and a copied picture 4 by 3 pixels.
func lookedAtHistory(t *testing.T, own func(func())) (string, *launcherClipboard) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("first line\nsecond line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return path, newLauncherClipboard(t, own, [2]string{"image/png", pic.String()}, [2]string{"text/uri-list", uri + "\r\n"})
}

// quickLookPanel is a search panel over that history, and the file.
func quickLookPanel(t *testing.T) (*panelHarness, *panel, string) {
	t.Helper()
	h, path := quickLookHarness(t)
	p, _ := h.openPanel()
	return h, p, path
}

// quickLookHarness is the harness of a search panel over that history,
// the panel not open yet, and the file.
func quickLookHarness(t *testing.T) (*panelHarness, string) {
	t.Helper()
	h := newPanelHarness(t)
	path, clip := lookedAtHistory(t, h.own)
	h.cfg.Launcher.Search.Providers = append(h.cfg.Launcher.Search.Providers, config.SearchClipboard)
	deps := h.s.d.Search
	h.s.d.Search = func() search.Deps {
		d := deps()
		d.Clipboard = clip
		return d
	}
	return h, path
}

func (p *panel) selectedRow() search.Row {
	row, _ := p.selected()
	return row
}

func isPicture(r search.Row) bool { return r.Entry != nil && r.Entry.Preview.Load != nil }

func isFile(path string) func(search.Row) bool {
	return func(r search.Row) bool { return r.Entry != nil && r.Entry.Preview.Path == path }
}

// lookShowsFile waits for Quick Look to show path's text.
func lookShowsFile(h *harness, q *quickLook, path string) {
	h.t.Helper()
	h.until("Quick Look showing "+path, func() bool {
		return q.shows == quickLookFile && q.file.Path() == path && q.file.Renderer() == preview.Text
	})
}

// lookShowsPicture waits for Quick Look to show the 4 by 3 picture in
// the shared preview's zoomable view, at its size, over a detail line
// giving its kind and size.
func lookShowsPicture(h *harness, q *quickLook) {
	h.t.Helper()
	h.until("Quick Look showing the picture", func() bool {
		views := treetest.All[*widget.ImageView](q.card)
		return q.shows == quickLookPicture && q.card.Visible() && q.file.Renderer() == preview.Image && q.file.Path() == "" &&
			len(views) == 1 && views[0].Measure(widget.Row, widget.Unknown).Nat == 4 && views[0].Measure(widget.Column, 4).Nat == 3 &&
			slices.ContainsFunc(treetest.All[*widget.Label](q.card), func(l *widget.Label) bool { return strings.Contains(l.Text(), " · 4 × 3 · ") })
	})
}

func overlaid(f *panelFrame) bool {
	return slices.Contains(f.root.Children(), widget.Widget(f.overlay))
}

// Shift+Space opens Quick Look on the selected file over the panel,
// larger than the side pane; a second press closes it, and so does
// Escape, which leaves the panel open. A plain Space in a field with
// text in it is the field's.
func TestQuickLookOpensAndClosesByItsKey(t *testing.T) {
	h, p, path := quickLookPanel(t)
	q := p.v.ql
	h.typed(p, "notes.txt")
	h.until("the copied file selected", func() bool { return isFile(path)(p.selectedRow()) })

	if h.key("space") {
		t.Error("a plain Space in a field with text in it must type")
	}
	if q.shown() {
		t.Fatal("a plain Space in a field with text in it opened Quick Look")
	}

	if !h.key("Shift+space") {
		t.Fatal("Shift+Space was not taken")
	}
	lookShowsFile(h.harness, q, path)
	if !overlaid(p.v.panelFrame) {
		t.Error("Quick Look's card is not over the panel")
	}
	p.v.root.Arrange(render.Rect{W: 1920, H: 1080})
	if b := q.card.Bounds(); b.W <= p.v.pane.width || b.X != (1920-b.W)/2 || b.Y != (1080-b.H)/2 {
		t.Errorf("Quick Look lies at %+v, not centered or no larger than the %d-wide pane", b, p.v.pane.width)
	}
	if !h.key("Shift+space") {
		t.Fatal("a second Shift+Space was not taken")
	}
	if q.shown() || q.file.Renderer() != preview.Empty || overlaid(p.v.panelFrame) {
		t.Fatalf("a second press left Quick Look showing %v", q.file.Renderer())
	}

	h.key("Shift+space")
	lookShowsFile(h.harness, q, path)
	if !h.key("Escape") {
		t.Fatal("Escape was not taken")
	}
	if q.shown() {
		t.Error("Escape left Quick Look open")
	}
	h.loop.pump()
	if h.s.panel != p || p.closing {
		t.Error("closing Quick Look closed the panel")
	}
}

// In the clipboard history, whose field starts empty, a plain Space
// opens Quick Look: a copied picture shows at its size, the arrows step
// the history underneath to the copied file, and Space closes it.
func TestSpaceOnAnEmptyFieldLooksAtTheClipboardHistory(t *testing.T) {
	h, p, path := quickLookPanel(t)
	q := p.v.ql
	h.typed(p, "cliphist")
	h.until("the clipboard history's row", func() bool { return p.selectedTitle() == i18n.T("launcher-picker-clipboard") })
	h.key("Return")
	h.until("the history", func() bool { return p.depth == 1 && len(p.rowTitles()) == 2 && p.v.Entry.Text() == "" })
	for !isPicture(p.selectedRow()) {
		h.key("Down")
	}

	if !h.key("space") {
		t.Fatal("Space on an empty field was not taken")
	}
	lookShowsPicture(h.harness, q)
	if !h.key("Down") {
		t.Fatal("Down was not Quick Look's")
	}
	lookShowsFile(h.harness, q, path)
	if !isFile(path)(p.selectedRow()) {
		t.Error("Down left the history's selection where it was")
	}
	if !h.key("Up") {
		t.Fatal("Up was not Quick Look's")
	}
	lookShowsPicture(h.harness, q)
	if !h.key("space") || q.shown() {
		t.Error("Space did not close Quick Look")
	}
	if p.depth != 1 {
		t.Error("closing Quick Look left the history")
	}
}

// [launcher.keybindings] quick-look rebinds the key: the new one opens
// and closes Quick Look, the default no longer does.
func TestTheQuickLookKeyIsConfigurable(t *testing.T) {
	h := newPanelHarness(t)
	path, clip := lookedAtHistory(t, h.own)
	h.cfg.Launcher.Keybindings = map[string]string{"quick-look": "F3"}
	h.cfg.Launcher.Search.Providers = append(h.cfg.Launcher.Search.Providers, config.SearchClipboard)
	deps := h.s.d.Search
	h.s.d.Search = func() search.Deps {
		d := deps()
		d.Clipboard = clip
		return d
	}
	p, _ := h.openPanel()
	h.typed(p, "notes.txt")
	h.until("the copied file selected", func() bool { return isFile(path)(p.selectedRow()) })
	if h.key("Shift+space") || p.v.ql.shown() {
		t.Error("Shift+Space opened Quick Look though quick-look is F3")
	}
	if !h.key("F3") {
		t.Fatal("F3 was not taken")
	}
	lookShowsFile(h.harness, p.v.ql, path)
	if !h.key("F3") || p.v.ql.shown() {
		t.Error("a second F3 did not close Quick Look")
	}
}

// Quick Look's Open opens the file shown through the opener the shell
// gives the launcher and closes the launcher, so the app it opens in
// takes the keyboard; without an opener it offers no Open.
func TestQuickLookOpensTheFileThroughTheOpener(t *testing.T) {
	for _, opener := range []bool{true, false} {
		h := newPanelHarness(t)
		var opened []string
		if opener {
			h.s.d.OpenFile = func(path string) { opened = append(opened, path) }
		}
		path, clip := lookedAtHistory(t, h.own)
		h.cfg.Launcher.Search.Providers = append(h.cfg.Launcher.Search.Providers, config.SearchClipboard)
		deps := h.s.d.Search
		h.s.d.Search = func() search.Deps {
			d := deps()
			d.Clipboard = clip
			return d
		}
		p, _ := h.openPanel()
		h.typed(p, "notes.txt")
		h.until("the copied file selected", func() bool { return isFile(path)(p.selectedRow()) })
		h.key("Shift+space")
		lookShowsFile(h.harness, p.v.ql, path)
		buttons := treetest.All[*widget.Button](p.v.ql.card)
		if !opener {
			if len(buttons) != 0 {
				t.Errorf("without an opener Quick Look offers %d buttons", len(buttons))
			}
			continue
		}
		if len(buttons) != 1 || !buttons[0].Visible() {
			t.Fatalf("Quick Look offers %d buttons, want Open", len(buttons))
		}
		buttons[0].OnClick()
		if !slices.Equal(opened, []string{path}) {
			t.Errorf("Open opened %q, want %q", opened, path)
		}
		h.until("the panel to close", func() bool { return h.s.panel == nil })
	}
}

// In a rofi mode Space is select-element, never Quick Look: Shift+Space
// opens it on the selected file even before the selection rests,
// Escape closes it and leaves the session open.
func TestARofiModeLooksByShiftSpaceAlone(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("first line\nsecond line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.s.d.Vault = func() engine.Mode { return &filedMode{path: path} }
	h.open(launcheripc.SessionOptions{Modes: []string{VaultModeName}}, false)
	h.until("the rows", func() bool { return h.s.cur != nil && h.s.model.len() == 2 })
	a := h.s.cur
	q := a.views.ql
	if h.key("space") || q.shown() {
		t.Error("a plain Space in a rofi mode opened Quick Look")
	}
	if !h.key("Shift+space") {
		t.Fatal("Shift+Space was not taken")
	}
	lookShowsFile(h, q, path)
	if !overlaid(a.views.panelFrame) {
		t.Error("Quick Look's card is not over the panel")
	}
	if !h.key("Escape") || q.shown() {
		t.Fatal("Escape did not close Quick Look")
	}
	h.loop.pump()
	if h.s.cur != a {
		t.Error("Escape closing Quick Look ended the session")
	}
	h.key("Shift+space")
	lookShowsFile(h, q, path)
	h.key("Down")
	h.until("Quick Look to close on a row it does not show", func() bool { return !q.shown() && !a.lookWanted })
	if h.s.model.textAt(h.s.sel) != "facts" {
		t.Errorf("Down moved to %q, want the facts row", h.s.model.textAt(h.s.sel))
	}
}

// The rofi clipboard mode looks at a copied picture.
func TestTheClipboardModeLooksAtACopiedPicture(t *testing.T) {
	h := newHarness(t)
	_, clip := lookedAtHistory(t, h.own)
	h.s.d.Clipboard = clip
	mode := "clipboard"
	h.open(launcheripc.SessionOptions{Mode: &mode}, false)
	h.until("the rows", func() bool { return h.s.cur != nil && h.s.model.len() == 2 })
	h.key("Down")
	h.key("Shift+space")
	q := h.s.cur.views.ql
	lookShowsPicture(h, q)
	if !h.key("Shift+space") || q.shown() {
		t.Error("a second Shift+Space did not close Quick Look")
	}
}

func TestQuickLookKeys(t *testing.T) {
	accel := func(spec string) app.Accel {
		a, err := app.ParseAccel(spec)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	toggles := []app.Accel{accel("F3")}
	for spec, want := range map[string]quickLookKey{
		"F3": quickLookClose, "space": quickLookClose, "Escape": quickLookClose,
		"Up": quickLookPrevious, "Left": quickLookPrevious, "Down": quickLookNext, "Right": quickLookNext,
		"Return": quickLookPass, "a": quickLookPass, "Control+space": quickLookPass, "Shift+Down": quickLookPass,
	} {
		if got := quickLookKeyFor(accel(spec), toggles); got != want {
			t.Errorf("%s does %d, want %d", spec, got, want)
		}
	}
	if accel("space") != spaceAccel {
		t.Error("spaceAccel is not a plain Space")
	}
}

// In a rofi mode too, Open opens the file and ends the session.
func TestARofiModesQuickLookOpensTheFile(t *testing.T) {
	h := newHarness(t)
	var opened []string
	h.s.d.OpenFile = func(path string) { opened = append(opened, path) }
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.s.d.Vault = func() engine.Mode { return &filedMode{path: path} }
	_, reply := h.open(launcheripc.SessionOptions{Modes: []string{VaultModeName}}, false)
	h.until("the rows", func() bool { return h.s.cur != nil && h.s.model.len() == 2 })
	q := h.s.cur.views.ql
	h.key("Shift+space")
	lookShowsFile(h, q, path)
	buttons := treetest.All[*widget.Button](q.card)
	if len(buttons) != 1 {
		t.Fatalf("Quick Look offers %d buttons, want Open", len(buttons))
	}
	buttons[0].OnClick()
	if !slices.Equal(opened, []string{path}) {
		t.Errorf("Open opened %q, want %q", opened, path)
	}
	if f := h.frame(reply); f.Type != launcheripc.FrameCancelled {
		t.Errorf("the session answered %+v, want cancelled", f)
	}
	if q.shown() {
		t.Error("the ended session's Quick Look still shows")
	}
}

// A copied picture that does not decode keeps Quick Look open, saying
// why as the shared preview says it of a file.
func TestQuickLookSaysWhyACopiedPictureCannotShow(t *testing.T) {
	h := newHarness(t)
	h.s.d.Clipboard = newLauncherClipboard(t, h.own, [2]string{"image/png", "not a picture"})
	mode := "clipboard"
	h.open(launcheripc.SessionOptions{Mode: &mode}, false)
	h.until("the row", func() bool { return h.s.cur != nil && h.s.model.len() == 1 })
	h.key("Shift+space")
	q := h.s.cur.views.ql
	h.until("Quick Look saying why", func() bool { return q.shown() && q.file.Renderer() == preview.Failed })
	if !q.card.Visible() || len(treetest.All[*widget.ImageView](q.card)) != 0 {
		t.Errorf("the undecodable picture leaves the card visible %v, showing %d image views", q.card.Visible(), len(treetest.All[*widget.ImageView](q.card)))
	}
}

// footerSays reports the footer naming key for Quick Look.
func footerSays(p *panel, key string) bool {
	return strings.Contains(p.v.Hints.Markup(), "<b>"+key+"</b> "+i18n.T("launcher-footer-quick-look"))
}

// The footer names the configured Quick Look key on a row Quick Look
// shows, a copied file or picture, and says nothing of it on a row it
// does not, a quicklink.
func TestTheFooterNamesTheQuickLookKeyOnARowItShows(t *testing.T) {
	for _, keys := range []string{"", "F3"} {
		h := newPanelHarness(t)
		path, clip := lookedAtHistory(t, h.own)
		want := "Shift+space"
		if keys != "" {
			h.cfg.Launcher.Keybindings = map[string]string{"quick-look": keys}
			want = keys
		}
		h.cfg.Launcher.Search.Providers = append(h.cfg.Launcher.Search.Providers, config.SearchClipboard)
		deps := h.s.d.Search
		h.s.d.Search = func() search.Deps {
			d := deps()
			d.Clipboard = clip
			return d
		}
		p, _ := h.openPanel()
		h.typed(p, "notes.txt")
		h.until("the copied file selected", func() bool { return isFile(path)(p.selectedRow()) })
		h.until("the footer naming "+want, func() bool { return footerSays(p, want) })

		h.typed(p, "Alpha Docs")
		h.until("the quicklink selected", func() bool { return p.selectedTitle() == "Alpha Docs" })
		h.loop.pump()
		if strings.Contains(p.v.Hints.Markup(), i18n.T("launcher-footer-quick-look")) {
			t.Errorf("a quicklink's footer offers Quick Look: %q", p.v.Hints.Markup())
		}
		if !strings.Contains(p.v.Hints.Markup(), "Esc") {
			t.Errorf("the quicklink's footer was not written: %q", p.v.Hints.Markup())
		}
	}
}
