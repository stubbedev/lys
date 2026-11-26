package files

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

// clip is a clipboard in memory: what was written, served back by
// preference.
type clip struct {
	content transfer.Content
	err     error
}

func (c *clip) Write(content transfer.Content) error {
	c.content = content
	return c.err
}

func (c *clip) Read(prefs []string, done func([]byte, string, error)) {
	mime := transfer.Pick(prefs, c.content.Has)
	if mime == "" {
		done(nil, "", app.ErrClipboardUnavailable)
		return
	}
	data, err := c.content.Bytes(mime)
	done(data, mime, err)
}

// asked is a question the harness was asked, and whether it is still
// showing.
type asked struct {
	Question
	open bool
}

// harness runs a file manager over a fake host with a lock-step loop,
// as the browser's tests do.
type harness struct {
	t     *testing.T
	f     *Files
	root  string
	bin   fileops.Bin
	trash string
	clip  *clip
	mods  app.Mods
	// what the host was asked.
	focus     widget.Widget
	title     string
	closed    bool
	questions []*asked
	toasts    []string
	undoToast func()
	opened    []string
	launched  [][]string
	terminals []string
	apps      []desktopentry.App
	queue     chan func()
	qmu       sync.Mutex
	stopped   bool
	drained   chan struct{}
	spawned   atomic.Int64
	work      sync.WaitGroup
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	conf := config.Defaults()
	conf.Animations.Enabled = false
	theme := apptheme.New(conf)
	data := filepath.Join(t.TempDir(), "data")
	hs := &harness{
		t: t, root: browsertest.Tree(t), clip: &clip{}, trash: filepath.Join(data, "Trash"),
		bin: fileops.Bin{
			DataHome: data, UID: os.Getuid(),
			Volume: func(string) (fileops.Volume, error) { return fileops.Volume{Dev: 1, Top: "/"}, nil },
			Now:    func() time.Time { return time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC) },
		},
		queue: make(chan func(), 1024), drained: make(chan struct{}),
	}
	go func() {
		defer close(hs.drained)
		for fn := range hs.queue {
			hs.run(fn)
		}
	}()
	t.Cleanup(hs.end)
	hs.f = New(Config{
		Host: hs, Bin: hs.bin, KeySym: treetest.KeySym, Sheet: theme.Sheet,
		Browser: filebrowser.Deps{
			Invoke:      hs.Invoke,
			Font:        treetest.Font(t),
			MIME:        browsertest.MIME(t),
			Home:        hs.root,
			Places:      func() []filebrowser.Place { return nil },
			Locations:   func() []filebrowser.Place { return nil },
			TrashDirs:   func() []string { return []string{hs.trash} },
			ForgetImage: func(render.ImageID) {},
		},
	})
	return hs
}

func (hs *harness) end() {
	hs.qmu.Lock()
	if !hs.stopped {
		hs.stopped = true
		close(hs.queue)
	}
	hs.qmu.Unlock()
	<-hs.drained
	hs.f.Close()
	hs.work.Wait()
}

func (hs *harness) Invoke(fn func()) {
	hs.qmu.Lock()
	defer hs.qmu.Unlock()
	if !hs.stopped {
		hs.queue <- fn
	}
}

func (hs *harness) Spawn(fn func()) {
	hs.spawned.Add(1)
	hs.work.Go(func() {
		defer hs.spawned.Add(-1)
		fn()
	})
}

func (hs *harness) Focus(w widget.Widget) error {
	hs.focus = w
	return nil
}

func (hs *harness) SetTitle(title string) { hs.title = title }

func (hs *harness) Close() { hs.closed = true }

func (hs *harness) HeldMods() app.Mods { return hs.mods }

func (hs *harness) Clipboard() (Clipboard, error) { return hs.clip, nil }

func (hs *harness) Ask(q Question) func() {
	a := &asked{Question: q, open: true}
	hs.questions = append(hs.questions, a)
	return func() { a.open = false }
}

func (hs *harness) Toast(text string, undo func()) {
	hs.toasts = append(hs.toasts, text)
	hs.undoToast = undo
}

func (hs *harness) OpenFile(path string) { hs.opened = append(hs.opened, path) }

func (hs *harness) Launch(_ desktopentry.App, paths []string) {
	hs.launched = append(hs.launched, paths)
}

func (hs *harness) OpenTerminal(dir string) { hs.terminals = append(hs.terminals, dir) }

func (hs *harness) Apps(string) []desktopentry.App { return hs.apps }

// run runs fn as the loop.
func (hs *harness) run(fn func()) {
	treetest.Loop.Lock()
	defer treetest.Loop.Unlock()
	fn()
}

// flush waits until the loop ran everything queued so far.
func (hs *harness) flush() {
	done := make(chan struct{})
	hs.Invoke(func() { close(done) })
	<-done
}

// do runs a gesture as the loop and waits for what it set off to
// settle.
func (hs *harness) do(fn func()) {
	hs.run(fn)
	hs.settle()
}

// settle waits until nothing runs off the loop, no listing is read and
// every operation ended or waits on a question, and the loop ran what
// that queued.
func (hs *harness) settle() {
	hs.t.Helper()
	waittest.Until(hs.t, func() bool {
		idle := hs.spawned.Load() == 0
		hs.flush()
		quiet := idle && hs.spawned.Load() == 0
		hs.run(func() {
			for _, t := range hs.f.tabs {
				quiet = quiet && !t.b.Busy()
			}
			quiet = quiet && (len(hs.f.jobs.running) == 0 || hs.asking() != nil)
		})
		return quiet
	}, "the file manager to settle")
	hs.flush()
}

// asking is the question showing, nil for none.
func (hs *harness) asking() *asked {
	for _, q := range slices.Backward(hs.questions) {
		if q.open {
			return q
		}
	}
	return nil
}

// answer answers the question showing.
func (hs *harness) answer(t *testing.T, response string) {
	t.Helper()
	q := hs.asking()
	if q == nil {
		t.Fatalf("no question showing to answer %q", response)
	}
	hs.do(func() {
		q.open = false
		q.Answer(response)
	})
}

// open opens the file manager on paths and waits for its tabs.
func (hs *harness) open(paths ...string) {
	hs.do(func() { hs.f.Open(paths) })
}

// tab is the tab showing.
func (hs *harness) tab() *tab {
	var t *tab
	hs.run(func() { t = hs.f.active() })
	return t
}

// pick selects the named entries of the tab showing, once its listing,
// which follows the disk through a watch, holds them all.
func (hs *harness) pick(t *testing.T, names ...string) {
	t.Helper()
	var rows []int
	listed := func() bool {
		rows = rows[:0]
		tb := hs.f.active()
		for _, n := range names {
			i := slices.IndexFunc(tb.b.Entries(), func(e filebrowser.Entry) bool { return e.Name() == n })
			if i < 0 {
				return false
			}
			rows = append(rows, i)
		}
		return true
	}
	waittest.UntilShowing(t, func() bool {
		hs.flush()
		ok := false
		hs.run(func() { ok = listed() })
		return ok
	}, "the listing to hold "+strings.Join(names, ", "), func() string {
		var shown []string
		hs.run(func() {
			for _, e := range hs.f.active().b.Entries() {
				shown = append(shown, e.Name())
			}
		})
		return "the listing holds " + strings.Join(shown, ", ")
	})
	hs.do(func() {
		tb := hs.f.active()
		if !listed() {
			t.Fatalf("the listing of %s lost one of %v", tb.b.Dir(), names)
		}
		view, ok := tb.b.FileView().(interface{ SelectRows(...int) })
		if !ok {
			t.Fatalf("the file view %T selects no rows", tb.b.FileView())
		}
		view.SelectRows(rows...)
	})
}

// selected are the names selected in the tab showing.
func (hs *harness) selected() []string {
	var names []string
	hs.run(func() {
		for _, e := range hs.f.active().b.Selected() {
			names = append(names, e.Name())
		}
	})
	return names
}

// menu clicks the context menu item label for the selection of the tab
// showing; sub names a submenu's item under it.
func (hs *harness) menu(t *testing.T, label string, sub ...string) {
	t.Helper()
	var items []widget.MenuItem
	hs.run(func() {
		tb := hs.f.active()
		items = hs.f.menu(tb, tb.b.Selected())
	})
	hs.click(t, items, label, sub...)
}

// backgroundMenu clicks the item label of the menu a click off the
// rows opens.
func (hs *harness) backgroundMenu(t *testing.T, label string) {
	t.Helper()
	var items []widget.MenuItem
	hs.run(func() { items = hs.f.menu(hs.f.active(), nil) })
	hs.click(t, items, label)
}

// click clicks the item label of items, or sub under it.
func (hs *harness) click(t *testing.T, items []widget.MenuItem, label string, sub ...string) {
	t.Helper()
	for _, name := range append([]string{label}, sub...) {
		i := slices.IndexFunc(items, func(it widget.MenuItem) bool { return it.Label == name })
		if i < 0 {
			t.Fatalf("no %q in the menu %v", name, labels(items))
		}
		if items[i].Disabled {
			t.Fatalf("%q is disabled", name)
		}
		if len(items[i].Items) > 0 {
			items = items[i].Items
			continue
		}
		hs.do(items[i].OnClick)
		return
	}
	t.Fatalf("%q opens a submenu", label)
}

func labels(items []widget.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Label)
	}
	return out
}

// key presses a key on the window, its focus on focused (none for nil).
func (hs *harness) key(sym xkb.Keysym, mods app.Mods, focused widget.Widget) {
	hs.do(func() {
		r := &widget.Router{Root: hs.f.Root()}
		if focused != nil {
			if err := r.SetFocus(focused); err != nil {
				hs.t.Fatalf("focus %T: %v", focused, err)
			}
		}
		hs.f.OnKey()(r, treetest.Code(sym), mods)
	})
}

// path is rel under the fixture's root.
func (hs *harness) path(rel string) string { return filepath.Join(hs.root, rel) }

// exists reports whether rel is under the root.
func (hs *harness) exists(rel string) bool {
	_, err := os.Lstat(hs.path(rel))
	return err == nil
}

// write makes rel under the root hold data.
func (hs *harness) write(t *testing.T, rel, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(hs.path(rel)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hs.path(rel), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// read is what rel under the root holds.
func (hs *harness) read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(hs.path(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// tabs are the folders the tabs show, read as the loop.
func (hs *harness) tabs() []string {
	var dirs []string
	hs.run(func() { dirs = hs.f.Tabs() })
	return dirs
}
