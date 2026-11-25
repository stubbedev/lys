package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/capture"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/vinput"
	"github.com/stubbedev/lys/secrets/vault"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmtest"
)

const (
	shotBtnLeft = 0x110
	shotKeyEsc  = 1
	shotPoll    = 60 * time.Millisecond
	shotRuns    = 6
	shotBudget  = 6 * time.Second
)

type shooter struct {
	t       *testing.T
	app     *app.Application
	w       *window
	cp      *capture.Client
	out     capture.Output
	dev     *vinput.Device
	dir     string
	winX    int
	winY    int
	only    string
	clicked map[string]int
}

var shotKeymap = []byte("xkb_keymap { xkb_keycodes { include \"evdev\" }; xkb_types { include \"complete\" }; xkb_compat { include \"complete\" }; xkb_symbols { include \"pc+us\" }; };\n")

func shotFakes(t *testing.T, k *kit) {
	t.Helper()
	f := &fakeAccounts{accounts: []fakeAccount{{name: "alice", full: "Alice Example", uid: uint32(os.Getuid()), kind: accounts.Admin}, bob}}
	if icons := os.Getenv("LYS_SETTINGS_SHOTS_ICONS"); icons != "" {
		widget.SetIconSearchPaths(append(widget.IconSearchPaths(), icons))
	}
	k.readAccounts = f.read
	k.changeAccount = f.change
	nets := vpnNets()
	k.network = func() (netControl, error) { return nets, nil }
	bt := homeBT()
	k.bluetooth = func() (btControl, error) { return bt, nil }
	pw := laptop()
	k.power = func() (powerControl, error) { return pw, nil }
	v := &fakeVault{st: vault.Status{State: vault.StateUnlocked, Email: "me@example.org"}, folderSet: []string{"Network", "Portal"}}
	k.vault = func() (vaultControl, error) { return v, nil }
	dnd := &fakeDND{}
	k.dnd = func() (dndControl, error) { return dnd, nil }
	br := &fakeBrightness{at: map[string]float64{"eDP-1": 60}}
	k.brightness = func() (brightnessControl, error) { return br, nil }
	nl := &fakeNightlight{}
	k.nightlight = func() (nightlightControl, error) { return nl, nil }
}

// TestShots screenshots every page of the real window on a headless
// lys-wm, scrolled through, and every dialog, popover and menu a click
// on a page's controls opens, into $LYS_SETTINGS_SHOTS; it skips without
// it. $LYS_SETTINGS_SHOTS_PAGES limits it to comma-separated pages, and
// $LYS_SETTINGS_SHOTS_ICONS and $LYS_SETTINGS_SHOTS_SHELL_ICONS name the
// app's and lys-shell's installed icon themes. `just settings::shots`
// runs it in a throwaway container. The shots are for review: nothing is
// compared.
func TestShots(t *testing.T) {
	dir := os.Getenv("LYS_SETTINGS_SHOTS")
	if dir == "" {
		t.Skip("LYS_SETTINGS_SHOTS names the directory the shots go to")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	s := wmtest.Start(t, "xwayland:\n  enabled: false\ngaps:\n  inner: 0\n  outer: 0\n")
	s.Display()
	k := testKit(t, "")
	shotFakes(t, k)
	wm, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = wm

	sess, err := app.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	application := app.NewApplication(sess)
	if shell := os.Getenv("LYS_SETTINGS_SHOTS_SHELL_ICONS"); shell != "" {
		dir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "lys-shell")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(shell, filepath.Join(dir, "icons")); err != nil {
			t.Fatal(err)
		}
	}
	defer apptheme.Setup(application)()
	if !widget.ThemeIconExists("ld-bluetooth-off-symbolic") {
		t.Fatal("lys-shell's icons are not on the search path")
	}
	cfg := k.store.svc.Config()
	theme := apptheme.New(cfg)
	face, err := app.Font(cfg.General.FontSans, 14)
	if err != nil {
		t.Fatal(err)
	}
	mono, err := app.Font("monospace", 14)
	if err != nil {
		mono = face
	}
	k.face, k.mono, k.monoVariants = app.FontFallback(face), app.FontFallback(mono), app.FontVariants(mono)
	k.invoke = application.Invoke
	k.background = func(fn func()) { go fn() }
	k.forgetImage = application.ForgetImage
	w, _, stop, err := openSettingsWindow(application, sess.Outputs, k, theme)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	w.stack.SetTransition(widget.StackNone, 0)
	w.onClose = func() {}

	cp, err := capture.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cp.Close() }()
	if err := cp.Refresh(); err != nil {
		t.Fatal(err)
	}
	outs := cp.Outputs()
	if len(outs) == 0 {
		t.Fatal("no output")
	}
	dev, err := vinput.Dial("", vinput.Options{Keyboard: true, FallbackKeymap: shotKeymap})
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	defer anim.SetInstant(true)()

	sh := &shooter{t: t, app: application, w: w, cp: cp, out: outs[0], dev: dev, dir: dir, only: os.Getenv("LYS_SETTINGS_SHOTS_PAGES"), clicked: map[string]int{}}
	release := application.Hold()
	done := make(chan error, 1)
	go func() {
		err := sh.all(s)
		application.Invoke(func() {
			release()
			application.Quit()
		})
		done <- err
	}()
	if err := application.Run(); err != nil && !errors.Is(err, app.ErrClosed) {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func (s *shooter) on(fn func()) {
	done := make(chan struct{})
	s.app.Invoke(func() {
		defer close(done)
		fn()
	})
	<-done
}

func (s *shooter) all(wm *wmtest.Session) error {
	time.Sleep(time.Second)
	out, err := wm.Ctl("windows")
	if err != nil {
		return err
	}
	var wins []struct {
		X, Y, Width, Height int
		AppID               string `json:"app_id"`
	}
	if err := json.Unmarshal([]byte(out), &wins); err != nil {
		return fmt.Errorf("windows %s: %w", out, err)
	}
	if len(wins) > 0 {
		s.winX, s.winY = wins[0].X, wins[0].Y
	}
	if err := s.park(); err != nil {
		return err
	}
	for _, p := range pages(config.Defaults(), bothParts) {
		if s.only != "" && !strings.Contains(","+s.only+",", ","+p.ID+",") {
			continue
		}
		if err := s.page(p.ID); err != nil {
			return fmt.Errorf("%s: %w", p.ID, err)
		}
	}
	return nil
}

func (s *shooter) open(id string) {
	s.on(func() {
		s.w.search.SetText("")
		s.w.pageID = ""
		s.w.openPage(id)
	})
}

func (s *shooter) scrollTo(y int) {
	s.on(func() {
		if s.w.page != nil {
			s.w.page.SetOffset(0, y)
		}
	})
}

func (s *shooter) page(id string) error {
	s.open(id)
	var total, view int
	s.on(func() {})
	if _, err := s.settle(nil); err != nil {
		return err
	}
	s.on(func() {
		r := s.w.page.AxisRange(widget.Column)
		total, view = r.Total, r.Page
	})
	step := max(view-80, 100)
	n := 0
	for y := 0; ; y += step {
		s.scrollTo(y)
		img, err := s.settle(nil)
		if err != nil {
			return err
		}
		content := img.Bounds()
		content.Min.X = (s.winX + sidebarEdge(s)) * int(s.out.Scale)
		if err := s.save(fmt.Sprintf("%s-%02d", id, n), img.SubImage(content)); err != nil {
			return err
		}
		n++
		if y+view >= total {
			break
		}
	}
	s.expand()
	return s.crawl(id)
}

func (s *shooter) expand() {
	s.on(func() {
		for _, w := range treetest.WithClass(s.w.page, "settings-section-header") {
			if b, ok := w.(*widget.Button); ok && b.OnClick != nil && widget.Describe(b).Expansion == widget.ExpansionOf(false) {
				b.OnClick()
			}
		}
	})
	_, _ = s.settle(nil)
}

type target struct {
	key  string
	kind string
	w    widget.Widget
}

func clickable(w widget.Widget) (string, bool) {
	name := reflect.TypeOf(w).String()
	switch {
	case strings.Contains(name, "Switch"), strings.Contains(name, "Check"), strings.Contains(name, "Radio"), strings.Contains(name, "Toggle"), strings.Contains(name, "Slider"), strings.Contains(name, "Spin"), strings.Contains(name, "Entry"), strings.Contains(name, "Scale"):
		return name, false
	case strings.Contains(name, "Button"), strings.Contains(name, "DropDown"), strings.Contains(name, "Dropdown"), strings.Contains(name, "Combo"), strings.Contains(name, "Pick"), strings.Contains(name, "Chooser"), strings.Contains(name, "MenuButton"):
		return name, true
	}
	return name, false
}

func (s *shooter) targets() []target {
	var out []target
	s.on(func() {
		seen := map[widget.Widget]bool{}
		i := 0
		treetest.Walk(s.w.page, func(w widget.Widget) {
			name, ok := clickable(w)
			if !ok || seen[w] {
				return
			}
			if !shownIn(s.w.page, w) {
				return
			}
			seen[w] = true
			out = append(out, target{key: strconv.Itoa(i), kind: name, w: w})
			i++
		})
	})
	return out
}

func shotBounds(w widget.Widget) (render.Rect, bool) {
	b, ok := w.(widget.Boundser)
	if !ok {
		return render.Rect{}, false
	}
	return b.Bounds(), true
}

func (s *shooter) reveal(w widget.Widget) (render.Rect, bool) {
	var r render.Rect
	var ok bool
	s.on(func() {
		r, ok = shotBounds(w)
		if !ok || r.W <= 0 || r.H <= 0 {
			ok = false
			return
		}
		view := s.w.page.Bounds()
		_, off := s.w.page.Offset()
		contentY := r.Y - view.Y + off
		s.w.page.SetOffset(0, contentY-view.H/3)
	})
	if !ok {
		return r, false
	}
	if _, err := s.settle(nil); err != nil {
		return r, false
	}
	s.on(func() {
		r, ok = shotBounds(w)
		view := s.w.page.Bounds()
		ok = ok && r.W > 0 && r.H > 0 && r.Y >= view.Y && r.Y+r.H <= view.Y+view.H && r.X >= view.X && r.X+r.W <= view.X+view.W
	})
	return r, ok
}

func (s *shooter) crawl(id string) error {
	ts := s.targets()
	for i, tg := range ts {
		cur := s.targets()
		if i >= len(cur) {
			break
		}
		tg = cur[i]
		r, ok := s.reveal(tg.w)
		if !ok {
			continue
		}
		before, err := s.settle(nil)
		if err != nil {
			return err
		}
		if err := s.click(r.X+r.W/2, r.Y+r.H/2); err != nil {
			return err
		}
		time.Sleep(250 * time.Millisecond)
		after, err := s.settle(before)
		if err != nil {
			return err
		}
		var nav bool
		s.on(func() { nav = s.w.pageID != id })
		if nav {
			s.open(id)
			if _, err := s.settle(nil); err != nil {
				return err
			}
			continue
		}
		box := diffBox(before, after)
		sc := int(s.out.Scale)
		widgetBox := image.Rect((s.winX+r.X-8)*sc, (s.winY+r.Y-8)*sc, (s.winX+r.X+r.W+8)*sc, (s.winY+r.Y+r.H+8)*sc)
		if !box.Empty() && !box.In(widgetBox) {
			name := fmt.Sprintf("%s-pop-%02d-%s", id, i, strings.TrimPrefix(strings.ReplaceAll(tg.kind, "*", ""), "settings."))
			crop := box.Union(widgetBox).Inset(-48).Intersect(after.Bounds())
			if err := s.save(name, after.SubImage(crop)); err != nil {
				return err
			}
		}
		if err := s.key(shotKeyEsc); err != nil {
			return err
		}
		time.Sleep(150 * time.Millisecond)
		now, err := s.settle(nil)
		if err != nil {
			return err
		}
		if !diffBox(before, now).Empty() {
			if err := s.key(shotKeyEsc); err != nil {
				return err
			}
			s.open(id)
			if _, err := s.settle(nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func diffBox(a, b *image.RGBA) image.Rectangle {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
		return image.Rect(0, 0, 1, 1)
	}
	var box image.Rectangle
	r := a.Bounds()
	w := r.Dx()
	for y := range r.Dy() {
		ra := a.Pix[y*a.Stride : y*a.Stride+w*4]
		rb := b.Pix[y*b.Stride : y*b.Stride+w*4]
		if string(ra) == string(rb) {
			continue
		}
		x0, x1 := 0, w-1
		for x0 < w && string(ra[x0*4:x0*4+3]) == string(rb[x0*4:x0*4+3]) {
			x0++
		}
		for x1 > x0 && string(ra[x1*4:x1*4+3]) == string(rb[x1*4:x1*4+3]) {
			x1--
		}
		if x0 < w {
			box = box.Union(image.Rect(x0, y, x1+1, y+1))
		}
	}
	return box
}

func (s *shooter) capture() (*image.RGBA, error) {
	f, err := s.cp.CaptureOutputOnce(s.out, capture.Options{})
	if err != nil {
		return nil, err
	}
	return f.Image()
}

func (s *shooter) settle(prev *image.RGBA) (*image.RGBA, error) {
	deadline := time.Now().Add(shotBudget)
	last, err := s.capture()
	if err != nil {
		return nil, err
	}
	same := 0
	for same < shotRuns {
		if time.Now().After(deadline) {
			return last, nil
		}
		time.Sleep(shotPoll)
		img, err := s.capture()
		if err != nil {
			return nil, err
		}
		if diffBox(last, img).Empty() {
			same++
		} else {
			same = 0
		}
		last = img
	}
	_ = prev
	return last, nil
}

func sidebarEdge(s *shooter) int {
	var x int
	s.on(func() { x = s.w.page.Bounds().X })
	return x
}

func (s *shooter) save(name string, img image.Image) error {
	f, err := os.Create(filepath.Join(s.dir, name+".png"))
	if err != nil {
		return err
	}
	return errors.Join(png.Encode(f, img), f.Close())
}

func (s *shooter) move(x, y int) error {
	sc := int(s.out.Scale)
	if err := s.dev.MotionAbsolute(uint32(x*sc), uint32(y*sc), uint32(s.out.Width), uint32(s.out.Height)); err != nil {
		return err
	}
	return s.dev.Roundtrip()
}

func (s *shooter) park() error {
	sc := int(s.out.Scale)
	return s.move(int(s.out.Width)/sc-1, int(s.out.Height)/sc-1)
}

func (s *shooter) click(x, y int) error {
	if err := s.move(s.winX+x, s.winY+y); err != nil {
		return err
	}
	for _, down := range []bool{true, false} {
		if err := s.dev.Button(shotBtnLeft, down); err != nil {
			return err
		}
	}
	if err := s.dev.Roundtrip(); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return s.park()
}

func (s *shooter) key(code uint32) error {
	for _, down := range []bool{true, false} {
		if err := s.dev.Key(code, down); err != nil {
			return err
		}
	}
	return s.dev.Roundtrip()
}
