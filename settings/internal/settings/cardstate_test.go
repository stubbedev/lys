package settings

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// dialFailure is the error a client gets for a service whose socket is
// not there, the Go error chain a page must never show as its text.
func dialFailure(what, socket string) error {
	return fmt.Errorf("connect to %s: %w", what, &net.OpError{Op: "dial", Net: "unix", Addr: &net.UnixAddr{Name: socket, Net: "unix"}, Err: os.NewSyscallError("connect", syscall.ENOENT)})
}

// titleLabel is the title of the row w is or holds first.
func titleLabel(w widget.Widget) *widget.Label {
	return treetest.WithClass(w, "title")[0].(*widget.Label)
}

// mediaTile is the media tile of the row w is or holds first.
func mediaTile(w widget.Widget) widget.Widget { return treetest.WithClass(w, "media")[0] }

func boundsOf(w widget.Widget) render.Rect { return w.(widget.Boundser).Bounds() }

// cardOf is the card holding w under root.
func cardOf(t *testing.T, root, w widget.Widget) widget.Widget {
	t.Helper()
	var card widget.Widget
	walkAncestry(root, nil, func(x widget.Widget, above []widget.Widget) {
		if x != w {
			return
		}
		for i := len(above) - 1; i >= 0 && card == nil; i-- {
			if widget.HasClass(above[i], css.Card) {
				card = above[i]
			}
		}
	})
	if card == nil {
		t.Fatalf("no card holds the %T", w)
	}
	return card
}

// rowInset is where a row with a media tile sets its tile and its title
// in a page's card, from the card's top-left corner.
func rowInset(t *testing.T, k *kit) (media, title widget.Point) {
	t.Helper()
	row := k.row("Row", "")
	row.SetMedia(k.icon("ld-shield-check-symbolic"), false)
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(newSection(k, "settings-section-sound-output", widget.NewItemGroup(row)), false)
	styledLayOut(k, root, 900)
	return insetIn(t, root, row)
}

// insetIn is where the row w sets its media tile and its title in its
// card under root.
func insetIn(t *testing.T, root, w widget.Widget) (media, title widget.Point) {
	t.Helper()
	card := boundsOf(cardOf(t, root, w))
	m, l := boundsOf(mediaTile(w)), boundsOf(titleLabel(w))
	return widget.Point{X: m.X - card.X, Y: m.Y - card.Y}, widget.Point{X: l.X - card.X, Y: l.Y - card.Y}
}

// Each kind of state is one row of its card: its glyph (a spinning
// spinner while loading) on the media tile and its summary as the
// title, both where a row with a media tile sets them; cleared, the
// row is gone.
func TestEveryCardStateKindPaintsItsGlyphAndSummaryInsetLikeARow(t *testing.T) {
	k := testKit(t, "")
	wantMedia, wantTitle := rowInset(t, k)
	for _, c := range []struct {
		name, summary string
		set           func(s *cardState)
		kind          cardStateKind
		glyph         string
	}{
		{"empty", "Nothing here", func(s *cardState) { s.Empty("ld-shield-check-symbolic", "Nothing here") }, stateEmpty, "ld-shield-check-symbolic"},
		{"loading", "Reading", func(s *cardState) { s.Loading("Reading") }, stateLoading, ""},
		{"unavailable", "Cannot reach it", func(s *cardState) {
			s.Unavailable("ld-wifi-zero-symbolic", "Cannot reach it", dialFailure("the session bus", "/run/bus"))
		}, stateUnavailable, "ld-wifi-zero-symbolic"},
		{"error", "It failed", func(s *cardState) { s.Error("It failed", errors.New("boom"), func() {}) }, stateError, cardStateErrorIcon},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := k.cardState()
			c.set(s)
			root := widget.NewBox(widget.Column, 0, 0)
			root.Append(newSection(k, "settings-section-sound-output", widget.NewItemGroup(s)), false)
			styledLayOut(k, root, 900)
			if !shown(s) || s.kind != c.kind || titleLabel(s).Text() != c.summary {
				t.Fatalf("shows %v, kind %v, title %q; want %v %q", shown(s), s.kind, titleLabel(s).Text(), c.kind, c.summary)
			}
			media, title := insetIn(t, root, s)
			if media.X != wantMedia.X || title.X != wantTitle.X {
				t.Errorf("the tile at %v and the title at %v, want a row's %v and %v", media, title, wantMedia, wantTitle)
			}
			spinners := treetest.All[*widget.Spinner](mediaTile(s))
			switch {
			case c.kind == stateLoading:
				if len(spinners) != 1 || !spinners[0].Spinning() {
					t.Errorf("loading shows %d spinners on its tile", len(spinners))
				}
			case len(spinners) != 0 || treetest.First[*widget.Icon](t, mediaTile(s)).Name() != c.glyph:
				t.Errorf("the tile shows %d spinners and %q, want %q", len(spinners), treetest.First[*widget.Icon](t, mediaTile(s)).Name(), c.glyph)
			}
			s.Clear()
			styledLayOut(k, root, 900)
			if s.Visible() || s.spinner.Spinning() {
				t.Errorf("cleared, the state shows %v and spins %v", s.Visible(), s.spinner.Spinning())
			}
		})
	}
}

// shownTexts are the texts of the labels laid out and shown under root.
func shownTexts(root widget.Widget) []string {
	var out []string
	walkShown(root, func(w widget.Widget) {
		if l, ok := w.(*widget.Label); ok && shown(l) {
			out = append(out, l.Text())
		}
	})
	return out
}

// walkShown visits the shown widgets under root, not looking inside a
// hidden one.
func walkShown(w widget.Widget, visit func(widget.Widget)) {
	if v, ok := w.(interface{ Visible() bool }); ok && !v.Visible() {
		return
	}
	visit(w)
	if c, ok := w.(interface{ Children() []widget.Widget }); ok {
		for _, k := range c.Children() {
			walkShown(k, visit)
		}
	}
}

// An unreachable service's error is the state's detail, out of sight
// until its disclosure opens it, in a dim monospace caption; a state
// with no error offers no disclosure.
func TestAnUnavailableStateKeepsItsErrorOutOfSightUntilOpened(t *testing.T) {
	k := testKit(t, "")
	tr := locale.T()
	err := dialFailure("the session bus", "/run/bus")
	s := k.cardState()
	s.Unavailable("ld-wifi-zero-symbolic", "Cannot reach it", err)
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(newSection(k, "settings-section-sound-output", widget.NewItemGroup(s)), false)
	styledLayOut(k, root, 900)
	if texts := shownTexts(root); slices.ContainsFunc(texts, func(x string) bool { return strings.Contains(x, "dial unix") }) || !slices.Contains(texts, "Cannot reach it") {
		t.Fatalf("closed, the card reads %q", texts)
	}
	if !shown(s.reveal.button) || s.reveal.button.TooltipText() != tr.Get("settings-card-expand") {
		t.Fatalf("the disclosure shows %v, reads %q", shown(s.reveal.button), s.reveal.button.TooltipText())
	}
	s.reveal.button.OnClick()
	styledLayOut(k, root, 900)
	if !shown(s.detail) || s.detail.Text() != err.Error() || !widget.HasClass(s.detail, css.Monospace) || !widget.HasClass(s.detail, css.DimLabel) {
		t.Fatalf("open, the detail shows %v reading %q, classes %v", shown(s.detail), s.detail.Text(), s.detail.Classes())
	}
	if s.reveal.button.TooltipText() != tr.Get("settings-card-collapse") || !s.reveal.chevron.HasState(widget.StateChecked) {
		t.Errorf("open, the disclosure reads %q, its chevron turned %v", s.reveal.button.TooltipText(), s.reveal.chevron.HasState(widget.StateChecked))
	}
	s.reveal.button.OnClick()
	styledLayOut(k, root, 900)
	if shown(s.detail) {
		t.Error("closed again, the detail still shows")
	}
	s.reveal.button.OnClick()
	s.Unavailable("ld-wifi-zero-symbolic", "Cannot reach it again", err)
	if s.detail.Visible() {
		t.Error("switched to a new state, the detail stays open")
	}
	for name, set := range map[string]func(){
		"empty":          func() { s.Empty("ld-shield-check-symbolic", "Nothing") },
		"loading":        func() { s.Loading("Reading") },
		"no error given": func() { s.Unavailable("ld-wifi-zero-symbolic", "Cannot reach it", nil) },
	} {
		set()
		styledLayOut(k, root, 900)
		if shown(s.reveal.button) || shown(s.detail) {
			t.Errorf("%s: a disclosure shows %v over a detail %v", name, shown(s.reveal.button), shown(s.detail))
		}
	}
}

// A failure's retry runs once per click; a failure without one, and
// every other state, offers none.
func TestAFailuresRetryRunsOncePerClick(t *testing.T) {
	k := testKit(t, "")
	s := k.cardState()
	runs := 0
	s.Error("It failed", errors.New("boom"), func() { runs++ })
	if !s.retry.Visible() || s.retry.Classes() == nil {
		t.Fatal("a failure with a retry offers none")
	}
	for want := 1; want <= 3; want++ {
		s.retry.OnClick()
		if runs != want {
			t.Fatalf("after %d clicks the retry ran %d times", want, runs)
		}
	}
	s.Error("It failed", errors.New("boom"), nil)
	if s.retry.Visible() {
		t.Error("a failure without a retry offers one")
	}
	s.Error("It failed", errors.New("boom"), func() { runs++ })
	s.Empty("ld-shield-check-symbolic", "Nothing")
	if s.retry.Visible() {
		t.Error("an empty list offers the retry of the failure before it")
	}
}

// Sound with no output device says so in the Output card's one state,
// set in from the card's edge where a row sets its tile and title.
func TestSoundWithoutAnOutputDeviceSaysSoInsetLikeARow(t *testing.T) {
	srv := pulsetest.New(t)
	srv.PutSource(pulsetest.Source(10, "mic", "Microphone", 0, ""))
	srv.SetDefaults("", "mic")
	r := startSoundOn(t, srv)
	st := r.p.output.state
	r.l.until("the output said empty", func() bool { return st.kind == stateEmpty })
	if st.Title() != locale.T().Get("settings-sound-no-output") || r.p.output.rows.Visible() {
		t.Fatalf("the output card reads %q, its rows shown %v", st.Title(), r.p.output.rows.Visible())
	}
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(r.p, false)
	styledLayOut(r.p.k, root, 900)
	wantMedia, wantTitle := rowInset(t, r.p.k)
	if media, title := insetIn(t, root, st); media.X != wantMedia.X || title.X != wantTitle.X {
		t.Errorf("no output device is said at %v, %v in its card, want a row's %v, %v", media, title, wantMedia, wantTitle)
	}
}

// Displays with no display connected say so in the arrangement card's
// one state, inset like a row, and show no output card.
func TestDisplaysWithNoDisplaySaySoInsetLikeARow(t *testing.T) {
	k := testKit(t, "")
	fakeHeads(t, nil, nil)
	p := displaysLead(k).(*displaysPanel)
	t.Cleanup(p.close)
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(p, false)
	styledLayOut(k, root, 900)
	if p.state.kind != stateEmpty || p.state.Title() != locale.T().Get("settings-displays-none") || p.output.Visible() || p.canvas.Visible() {
		t.Fatalf("with no display the page shows %v %q, the output card %v, the canvas %v", p.state.kind, p.state.Title(), p.output.Visible(), p.canvas.Visible())
	}
	wantMedia, wantTitle := rowInset(t, k)
	if media, title := insetIn(t, root, p.state); media.X != wantMedia.X || title.X != wantTitle.X {
		t.Errorf("no display is said at %v, %v in its card, want a row's %v, %v", media, title, wantMedia, wantTitle)
	}
}

// goErrorChain is a label reading as a Go error: a lowercase phrase,
// then at least two more each after ": ".
var goErrorChain = regexp.MustCompile(`^[a-z][^:\n]*: [^:\n]+: `)

// rawError reports text that is an error's own words, not a sentence of
// the app's.
func rawError(text string) bool {
	return strings.Contains(text, "dial unix") || goErrorChain.MatchString(text)
}

// servicelessKit is a kit no service answers, each failing as a client
// does whose socket is not there.
func servicelessKit(t *testing.T) *kit {
	t.Helper()
	k := testKit(t, geometryYAML)
	bus := dialFailure("the system bus", "/run/dbus/system_bus_socket")
	session := dialFailure("the session bus", "/run/user/0/bus")
	k.systemBus = func() (*dbus.Conn, error) { return nil, bus }
	k.network = func() (netControl, error) { return nil, bus }
	k.power = func() (powerControl, error) { return nil, bus }
	k.pausing = func() (pauseControl, error) { return nil, bus }
	k.bluetooth = func() (btControl, error) { return nil, bus }
	k.dnd = func() (dndControl, error) { return nil, session }
	k.nightlight = func() (nightlightControl, error) { return nil, session }
	k.brightness = func() (brightnessControl, error) { return nil, session }
	k.vault = func() (vaultControl, error) { return nil, session }
	k.cameras = func() (cameraSource, func(), error) { return nil, nil, dialFailure("udev", "/run/udev/control") }
	k.screenShares = func() (shareSource, func(), error) { return nil, nil, session }
	return k
}

// Every page, with no service to ask (no session or system bus, no
// sound server, no NetworkManager, no display server), says so the
// app's own way: no card is left blank, none shows more than one
// state, and no label outside a state's closed detail reads as an
// error's own words.
func TestEveryPageWithoutItsServicesSaysSoInOneCardState(t *testing.T) {
	k := servicelessKit(t)
	cfg := k.store.svc.Config()
	w := newWindow(k, cfg, func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	apptheme.New(cfg).Attach(w.root)
	pages := 0
	for _, sec := range w.sections {
		for _, p := range sec.pages {
			w.show(p.id)
			w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
			pages++
			checkServicelessPage(t, p.id, w.page)
		}
	}
	if pages == 0 {
		t.Fatal("no page laid out")
	}
}

func checkServicelessPage(t *testing.T, id string, page widget.Widget) {
	t.Helper()
	walkShown(page, func(x widget.Widget) {
		if l, ok := x.(*widget.Label); ok && shown(l) && rawError(l.Text()) {
			t.Errorf("%s: a label reads an error's own words: %q", id, l.Text())
		}
		if !widget.HasClass(x, css.Card) || !shown(x) {
			return
		}
		leaves, states := 0, 0
		walkShownIn(x, func(y widget.Widget) {
			if _, ok := y.(*cardState); ok && shown(y) {
				states++
			}
			if (leaf(y) || widget.HasClass(y, css.Card)) && shown(y) {
				leaves++
			}
		})
		if leaves == 0 {
			t.Errorf("%s: a card at %v shows nothing at all, not even why", id, boundsOf(x))
		}
		if states > 1 {
			t.Errorf("%s: a card shows %d states, want one", id, states)
		}
	})
}

// walkShownIn is walkShown under card, not looking inside a card it
// holds, which answers for itself and is the outer card's content.
func walkShownIn(card widget.Widget, visit func(widget.Widget)) {
	c, ok := card.(interface{ Children() []widget.Widget })
	if !ok {
		return
	}
	for _, k := range c.Children() {
		if v, ok := k.(interface{ Visible() bool }); ok && !v.Visible() {
			continue
		}
		visit(k)
		if !widget.HasClass(k, css.Card) {
			walkShownIn(k, visit)
		}
	}
}

// leaf reports a widget that paints something of its own: one with no
// children, but a spacer or a separator.
func leaf(w widget.Widget) bool {
	switch w.(type) {
	case *widget.Spacer, *widget.Separator:
		return false
	}
	c, ok := w.(interface{ Children() []widget.Widget })
	return !ok || len(c.Children()) == 0
}

// The sweep's raw error test tells a Go error chain or a failed dial
// from the app's own sentences, a colon among them.
func TestARawErrorIsToldFromTheAppsSentences(t *testing.T) {
	for _, raw := range []string{
		dialFailure("the session bus", "/run/bus").Error(),
		"displays: connect: dial unix /run/wayland-0: connect: no such file or directory",
		"read the items registered with org.kde.StatusNotifierWatcher: no owner: exit",
	} {
		if !rawError(raw) {
			t.Errorf("%q passes as the app's own words", raw)
		}
	}
	tr := locale.T()
	for _, own := range []string{
		tr.Get("settings-bluetooth-no-adapter"),
		tr.Get("settings-sound-unreachable"),
		"lys-wm’s settings cannot be read",
		"12:30",
	} {
		if rawError(own) {
			t.Errorf("%q is taken for an error's own words", own)
		}
	}
}
