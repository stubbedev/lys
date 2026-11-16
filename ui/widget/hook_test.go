package widget

import (
	"os"
	"path/filepath"
	"testing"
)

type hookCase struct {
	name string
	run  func(t *testing.T)
}

func hookOf[F any](name string, installed func() *F, set func(F) func(), first, second, none F) hookCase {
	return hookCase{name, func(t *testing.T) {
		t.Cleanup(func() { set(none) })
		if installed() != nil {
			t.Fatal("a hook is installed before the test installs one")
		}
		remove := set(first)
		if installed() == nil {
			t.Fatal("installing left no hook")
		}
		remove()
		remove()
		if installed() != nil {
			t.Fatal("removing the hook left it installed")
		}

		removeFirst := set(first)
		removeSecond := set(second)
		later := installed()
		removeFirst()
		if installed() != later {
			t.Fatal("removing an earlier hook took out the one installed after it")
		}
		removeSecond()
		if installed() != nil {
			t.Fatal("removing the later hook left it installed")
		}

		set(first)
		set(none)()
		if installed() != nil {
			t.Fatal("installing none left a hook")
		}
	}}
}

func installedWatcher() *StylesheetWatcher {
	watchMu.Lock()
	defer watchMu.Unlock()
	return activeWatcherLocked()
}

func loopHookCases() []hookCase {
	return []hookCase{
		hookOf("error bell", errorBell.current, SetErrorBell, func(Widget) {}, func(Widget) {}, nil),
		hookOf("surface fade", surfaceFade.current, SetSurfaceFade, func(Widget, float64) bool { return true }, func(Widget, float64) bool { return false }, nil),
		hookOf("GPU host", gpuHost.current, SetGPUHost, func(*GPUArea) (GPUSurface, error) { return nil, nil }, func(*GPUArea) (GPUSurface, error) { return nil, os.ErrInvalid }, nil),
		hookOf("video host", videoHost.current, SetVideoHost,
			func(*Video, VideoFormat, func()) (VideoSurface, error) { return nil, nil },
			func(*Video, VideoFormat, func()) (VideoSurface, error) { return nil, os.ErrInvalid }, nil),
		hookOf("texture host", textureHost.current, SetTextureHost,
			func(*Video, int, int, func(), func()) (TextureSurface, error) { return nil, nil },
			func(*Video, int, int, func(), func()) (TextureSurface, error) { return nil, os.ErrInvalid }, nil),
		hookOf("surface origin resolver", originResolver.current, SetSurfaceOriginResolver, func(Widget) (Point, bool) { return Point{}, true }, func(Widget) (Point, bool) { return Point{}, false }, nil),
		hookOf("removed hook", removedHook.current, SetRemovedHook, func(Widget) {}, func(Widget) {}, nil),
	}
}

func TestAHookIsRemovedByWhoInstalledItAndSurvivesAnEarlierRemoval(t *testing.T) {
	resetWatches(t)
	watch := func() StylesheetWatcher {
		return func([]string, func()) (func(), error) { return func() {}, nil }
	}
	cases := append(loopHookCases(), hookOf("stylesheet watcher", installedWatcher, SetStylesheetWatcher, watch(), watch(), nil))
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestALoopsHooksServeOnlyThatLoop(t *testing.T) {
	type rang struct{ first, second int }
	var got rang
	first, second := newTestLoop(t), newTestLoop(t)
	first.do(t, func() { t.Cleanup(SetErrorBell(func(Widget) { got.first++ })) })
	second.do(t, func() { t.Cleanup(SetErrorBell(func(Widget) { got.second++ })) })
	w := NewBox(Row, 0, 0)
	first.do(t, func() { ErrorBell(w) })
	if got != (rang{first: 1}) {
		t.Errorf("a bell rung on the first loop reached %+v, want the first loop's bell alone", got)
	}
	ErrorBell(w)
	if got != (rang{first: 1}) {
		t.Errorf("a bell rung on a goroutine with none reached %+v, want no bell", got)
	}

	for _, c := range loopHookCases() {
		t.Run(c.name+" on another loop", func(t *testing.T) {
			other := newTestLoop(t)
			ran := false
			other.do(t, func() {
				ran = true
				c.run(t)
			})
			if !ran {
				t.Fatal("the other loop never ran the case")
			}
		})
	}
}

func TestRemovingTheRunningWatcherMovesTheWatchToTheOneBeforeIt(t *testing.T) {
	resetWatches(t)
	t.Cleanup(func() { SetStylesheetWatcher(nil) })
	type watcher struct{ armed, stopped int }
	install := func(w *watcher) (remove func()) {
		return SetStylesheetWatcher(func([]string, func()) (func(), error) {
			w.armed++
			return func() { w.stopped++ }, nil
		})
	}
	path := filepath.Join(t.TempDir(), "a.css")
	if err := os.WriteFile(path, []byte("label { color: red; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	var first, second watcher
	removeFirst := install(&first)
	removeSecond := install(&second)
	sheet, err := AddStylesheetFile(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sheet.Remove)
	if first != (watcher{}) || second != (watcher{armed: 1}) {
		t.Fatalf("with a file loaded: first %+v, second %+v; want the watch on the latest", first, second)
	}
	removeSecond()
	if second != (watcher{armed: 1, stopped: 1}) || first != (watcher{armed: 1}) {
		t.Fatalf("removing the latest watcher: first %+v, second %+v; want the watch moved to the first", first, second)
	}
	removeFirst()
	if first != (watcher{armed: 1, stopped: 1}) {
		t.Errorf("removing the last watcher left the first %+v, want its watch ended", first)
	}
}

func TestRemovingTheStylesheetWatcherEndsItsWatchAndNoOther(t *testing.T) {
	resetWatches(t)
	type watcher struct{ armed, stopped int }
	install := func(w *watcher) (remove func()) {
		return SetStylesheetWatcher(func([]string, func()) (func(), error) {
			w.armed++
			return func() { w.stopped++ }, nil
		})
	}
	path := filepath.Join(t.TempDir(), "a.css")
	if err := os.WriteFile(path, []byte("label { color: red; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	var first, second watcher
	removeFirst := install(&first)
	t.Cleanup(func() { SetStylesheetWatcher(nil) })
	sheet, err := AddStylesheetFile(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sheet.Remove)
	if first != (watcher{armed: 1}) {
		t.Fatalf("with a file loaded the first watcher is %+v, want its watch armed", first)
	}
	removeSecond := install(&second)
	if first != (watcher{armed: 1, stopped: 1}) || second != (watcher{armed: 1}) {
		t.Fatalf("after a second install: first %+v, second %+v; want the watch moved to the second", first, second)
	}
	removeFirst()
	if second != (watcher{armed: 1}) {
		t.Fatalf("removing the first watcher left the second %+v, want its watch running", second)
	}
	removeSecond()
	if second != (watcher{armed: 1, stopped: 1}) || first != (watcher{armed: 1, stopped: 1}) {
		t.Errorf("removing the second watcher: first %+v, second %+v; want its watch ended, once", first, second)
	}
}
