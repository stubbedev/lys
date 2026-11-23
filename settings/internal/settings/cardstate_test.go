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

	"github.com/stubbedev/lys/desktop/i18n"
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
	if !s.action.Visible() || s.action.Classes() == nil {
		t.Fatal("a failure with a retry offers none")
	}
	for want := 1; want <= 3; want++ {
		s.action.OnClick()
		if runs != want {
			t.Fatalf("after %d clicks the retry ran %d times", want, runs)
		}
	}
	s.Error("It failed", errors.New("boom"), nil)
	if s.action.Visible() {
		t.Error("a failure without a retry offers one")
	}
	s.Error("It failed", errors.New("boom"), func() { runs++ })
	s.Empty("ld-shield-check-symbolic", "Nothing")
	if s.action.Visible() {
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
		if tip, ok := x.(interface{ TooltipText() string }); ok && shown(x) && rawError(tip.TooltipText()) {
			t.Errorf("%s: a %T's tooltip reads an error's own words: %q", id, x, tip.TooltipText())
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

// A state alone in its item group takes the group with it: a cleared
// state leaves no empty group, and its gap, above the card's content.
func TestALoneStateShowsItsGroupOnlyWhileItShows(t *testing.T) {
	k := testKit(t, "")
	s := k.cardState()
	content := k.label("content")
	body := gapBox(widget.Column)
	body.Append(s.lone(), false)
	body.Append(content, false)
	body.Arrange(render.Rect{W: 600, H: 400})
	if content.Bounds().Y != 0 {
		t.Fatalf("a cleared state leaves the content at y %d, want 0", content.Bounds().Y)
	}
	s.Empty("ld-monitor-symbolic", "nothing")
	body.Arrange(render.Rect{W: 600, H: 400})
	if content.Bounds().Y == 0 {
		t.Fatal("an empty state took no room above the content")
	}
	s.Clear()
	body.Arrange(render.Rect{W: 600, H: 400})
	if content.Bounds().Y != 0 {
		t.Fatalf("the cleared state left the content at y %d, want 0", content.Bounds().Y)
	}
}

// actionFailure is a Go error chain as an action gets it from a service
// that refused it.
func actionFailure(action string) error {
	return fmt.Errorf("settings: %s: %w", action, dialFailure("the system bus", "/run/dbus/system_bus_socket"))
}

// refusingSlot swaps a slot for one refusing every write with the
// error it is given; given nil, it restores the slot it first swapped.
func refusingSlot() func(s *slot, err error) {
	var held slot
	return func(s *slot, err error) {
		if held.set == nil {
			held = *s
		}
		*s = held
		if err != nil {
			s.set = func(any) error { return err }
		}
	}
}

// Every page's action failure, with no service to ask, is said the
// app's own way: a summary over the failure's error, which waits in
// the state's closed detail, and no shown label reads as an error's own
// words.
func TestEveryPagesActionFailureIsSaidInTheAppsWords(t *testing.T) {
	k := servicelessKit(t)
	cfg := k.store.svc.Config()
	w := newWindow(k, cfg, func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	apptheme.New(cfg).Attach(w.root)
	tr := locale.T()
	for _, c := range []struct {
		page, summary string
		fail          func(page widget.Widget, err error) *inlineState
	}{
		{bluetoothPageID, tr.Get("settings-bluetooth-failed"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*bluetoothPanel](t, page)
			p.fail(err)
			return p.problem
		}},
		{networkPageID, tr.Get("settings-network-failed"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*networkPanel](t, page)
			p.fail(err)
			return p.problem
		}},
		{powerPageID, tr.Get("settings-power-mode-refused"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*powerPanel](t, page)
			p.fail(powerActionProfile, err)
			return p.problem
		}},
		{powerPageID, tr.Get("settings-power-failed"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*powerPanel](t, page)
			p.fail(powerActionNone, err)
			return p.problem
		}},
		{usersID, tr.Get("settings-users-failed"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*usersPanel](t, page)
			p.fail(err)
			return p.problem
		}},
		{usersID, tr.Get("settings-users-add-no-password", i18n.Str("name", "zoe")), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*usersPanel](t, page)
			if err == nil {
				p.fail(nil)
				return p.problem
			}
			p.fail(&passwordNotSetError{login: "zoe", err: err})
			return p.problem
		}},
		{loginItemsID, tr.Get("settings-login-apps-failed"), func(page widget.Widget, err error) *inlineState {
			l := treetest.First[*loginItems](t, page)
			l.fail(err)
			return l.problem
		}},
		{passwordsID, tr.Get("settings-passwords-failed"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*passwordsPanel](t, page)
			p.fail(err)
			return p.problem
		}},
		{defaultAppsID, tr.Get("settings-default-apps-failed"), func(page widget.Widget, err error) *inlineState {
			d := treetest.First[*defaultApps](t, page)
			d.fail(err)
			return d.problem
		}},
		{displaysID, tr.Get("settings-displays-apply-failed"), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*displaysPanel](t, page)
			if err == nil {
				p.moved(nil, nil)
				return p.status
			}
			p.fail("settings-displays-apply-failed", err)
			return p.status
		}},
		{soundID, tr.Get("settings-sound-alert-not-set"), func() func(widget.Widget, error) *inlineState {
			refuse := refusingSlot()
			return func(page widget.Widget, err error) *inlineState {
				e := treetest.First[*alertSoundList](t, page)
				refuse(&e.slot, err)
				e.choose("bell")
				return e.problem
			}
		}()},
		{dateTimeID, tr.Get("settings-sys-failed", i18n.Str("service", timedateName)), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*sysPanel](t, page)
			p.fail(err)
			return p.problem
		}},
		{regionID, tr.Get("settings-sys-failed", i18n.Str("service", localeName)), func(page widget.Widget, err error) *inlineState {
			p := treetest.First[*sysPanel](t, page)
			p.fail(err)
			return p.problem
		}},
		{displaysID, tr.Get("settings-nightlight-preview-failed"), func(page widget.Widget, err error) *inlineState {
			e := treetest.First[*kelvinEditor](t, page)
			k.nightlight = func() (nightlightControl, error) {
				if err != nil {
					return nil, err
				}
				return &fakeNightlight{}, nil
			}
			e.preview()
			return e.problem
		}},
		{"fonts", tr.Get("settings-font-unavailable", i18n.Str("family", "Sans")), func(page widget.Widget, err error) *inlineState {
			f := treetest.First[*fontEditor](t, page)
			f.shown = "Sans"
			f.showSample(loadedFamily{family: "Sans", face: k.face, err: err})
			return f.problem
		}},
		{"wm-session", tr.Get("wm-palette-add-failed", i18n.Str("name", "mint")), func() func(widget.Widget, error) *inlineState {
			refuse := refusingSlot()
			return func(page widget.Widget, err error) *inlineState {
				p := treetest.First[*paletteGrid](t, page)
				refuse(&p.own, err)
				p.name.SetText("mint")
				p.add()
				return p.problem
			}
		}()},
		{"bitwarden", tr.Get("settings-generator-sample-copy-failed"), func(page widget.Widget, err error) *inlineState {
			g := treetest.First[*generatorSample](t, page)
			k.copySecret = func(string) error { return err }
			g.copySample()
			return g.problem
		}},
	} {
		t.Run(c.page+"/"+c.summary, func(t *testing.T) {
			w.show(c.page)
			err := actionFailure("act on " + c.page)
			st := c.fail(w.page, err)
			w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
			if !shown(st) || st.kind != stateError || st.Text() != c.summary {
				t.Fatalf("the failure shows %v as %v %q, want %q", shown(st), st.kind, st.Text(), c.summary)
			}
			if !strings.Contains(st.detail.Text(), "dial unix") || shown(st.detail) || !shown(st.reveal.button) {
				t.Errorf("the error waits as %q, shown %v, its disclosure shown %v", st.detail.Text(), shown(st.detail), shown(st.reveal.button))
			}
			checkServicelessPage(t, c.page, w.page)
			c.fail(w.page, nil)
			if st.Visible() {
				t.Errorf("an action that went through leaves %q shown", st.Text())
			}
		})
	}
}

// An inline state is a card state's same parts on a line: a failure's
// summary over its error behind the disclosure, a spinner while
// loading, and a plain note; with no error there is no disclosure.
func TestAnInlineStateSaysEachKindOnALine(t *testing.T) {
	k := testKit(t, "")
	s := k.inlineState()
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(s, false)
	lay := func() { styledLayOut(k, root, 600) }
	lay()
	if shown(s) {
		t.Fatal("a new inline state shows")
	}
	err := actionFailure("save")
	s.Error("It failed", err, nil)
	lay()
	if !shown(s) || s.Text() != "It failed" || !widget.HasClass(s.summary, css.Error) || !shown(s.glyph) || shown(s.spinner) || shown(s.action) {
		t.Fatalf("a failure shows %v %q, red %v, glyph %v, spinner %v, action %v", shown(s), s.Text(), widget.HasClass(s.summary, css.Error), shown(s.glyph), shown(s.spinner), shown(s.action))
	}
	if texts := shownTexts(root); slices.ContainsFunc(texts, rawError) {
		t.Fatalf("closed, the line reads %q", texts)
	}
	s.reveal.button.OnClick()
	lay()
	if !shown(s.detail) || s.detail.Text() != err.Error() {
		t.Fatalf("open, the detail shows %v reading %q", shown(s.detail), s.detail.Text())
	}
	s.Error("Refused", nil, nil)
	lay()
	if shown(s.reveal.button) || shown(s.detail) {
		t.Errorf("a refusal without an error offers a disclosure %v over a detail %v", shown(s.reveal.button), shown(s.detail))
	}
	s.Loading("Asking")
	lay()
	if !shown(s.spinner) || !s.spinner.Spinning() || shown(s.glyph) || widget.HasClass(s.summary, css.Error) || !widget.HasClass(s.summary, css.DimLabel) {
		t.Errorf("loading shows spinner %v spinning %v, glyph %v, red %v", shown(s.spinner), s.spinner.Spinning(), shown(s.glyph), widget.HasClass(s.summary, css.Error))
	}
	s.Empty("", "Nothing")
	lay()
	if shown(s.glyph) || shown(s.spinner) || s.spinner.Spinning() || s.Text() != "Nothing" {
		t.Errorf("a plain note shows glyph %v, spinner %v, reads %q", shown(s.glyph), shown(s.spinner), s.Text())
	}
	s.Clear()
	lay()
	if shown(s) {
		t.Error("cleared, the line still shows")
	}
}

// An offer's action is labelled as asked and runs once per click; the
// state after it offers none.
func TestAnOfferRunsItsActionOncePerClick(t *testing.T) {
	k := testKit(t, "")
	card, inline := k.cardState(), k.inlineState()
	for _, c := range []struct {
		name string
		s    *stateParts
	}{{"card", card.stateParts}, {"inline", inline.stateParts}} {
		runs := 0
		c.s.Offer("ld-mouse-symbolic", "No curve", "Draw", func() { runs++ })
		if !c.s.action.Visible() || c.s.actionText.Text() != "Draw" {
			t.Fatalf("%s: the offer shows its action %v reading %q", c.name, c.s.action.Visible(), c.s.actionText.Text())
		}
		c.s.action.OnClick()
		c.s.action.OnClick()
		if runs != 2 {
			t.Errorf("%s: two clicks ran the offer %d times", c.name, runs)
		}
		c.s.Empty("ld-mouse-symbolic", "No curve")
		if c.s.action.Visible() {
			t.Errorf("%s: an empty state keeps the offer's action", c.name)
		}
	}
}
