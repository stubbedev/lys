package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/looptest"
	"github.com/stubbedev/lys/wm/internal/testenv"
)

type refusingOpener struct{ asked []string }

func (o *refusingOpener) TakeDevice(path string) (int, Device, error) {
	o.asked = append(o.asked, path)
	return -1, Device{}, unix.EPERM
}

func (o *refusingOpener) ReleaseDevice(Device) error { return nil }

func libinput(t testing.TB, opener Opener) *Libinput {
	t.Helper()
	return libinputReporting(t, opener, func(err error) { t.Errorf("reported: %v", err) })
}

func keepPriority(func(string, func() error)) (restore func() error) {
	return func() error { return nil }
}

func openLibinput(t testing.TB, opener Opener, report func(error), raise RaisePriority) *Libinput {
	t.Helper()
	l, err := loop.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	path := LibinputPath(os.Getenv)
	in, err := OpenLibinput(l, path, opener, DefaultInputConfig(), report, raise)
	testenv.Library(t, "libinput", path, err)
	return in
}

func libinputReporting(t testing.TB, opener Opener, report func(error)) *Libinput {
	t.Helper()
	in := openLibinput(t, opener, report, keepPriority)
	t.Cleanup(func() {
		if err := in.Close(); err != nil {
			t.Error(err)
		}
	})
	return in
}

func evdevNode(t *testing.T) string {
	t.Helper()
	nodes, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(nodes) == 0 {
		t.Skip("no evdev nodes here")
	}
	slices.Sort(nodes)
	return nodes[0]
}

func (in *Libinput) onReader(t testing.TB, what string, job func(r *inputReader) error) {
	t.Helper()
	if err := in.reader.call(what, func() error { return job(in.reader) }); err != nil {
		t.Fatal(err)
	}
}

func (in *Libinput) addNow(t testing.TB, path string) {
	t.Helper()
	in.onReader(t, "add "+path, func(r *inputReader) error { return r.add(path) })
}

func TestLibinputAsksTheOpenerForDevices(t *testing.T) {
	node := evdevNode(t)
	opener := &refusingOpener{}
	var reports []error
	in := libinputReporting(t, opener, func(err error) { reports = append(reports, err) })
	in.AddLater(node)
	looptest.RunUntil(t, in.loop, "the refused open's report", func() bool { return len(reports) > 0 })
	if err := reports[0]; len(reports) != 1 || !errors.Is(err, unix.EPERM) || errors.Is(err, ErrDeclined) || !strings.Contains(err.Error(), node) {
		t.Fatalf("a refused open reported %v", reports)
	}
	if !slices.Equal(opener.asked, []string{node}) {
		t.Fatalf("the opener was asked for %v", opener.asked)
	}
	in.onReader(t, "suspend, resume and remove", func(r *inputReader) error {
		r.suspend()
		err := r.resume()
		r.remove(node)
		return err
	})
}

func TestLibinputRejectsNonInputDevices(t *testing.T) {
	testenv.UdevRecord(t, SystemRoots.UdevData, os.DevNull)
	in := libinput(t, PlainOpener{})
	err := in.reader.call("add /dev/null", func() error { return in.reader.add(os.DevNull) })
	if !errors.Is(err, ErrDeclined) || !strings.Contains(err.Error(), os.DevNull) {
		t.Fatalf("/dev/null as an input device: %v", err)
	}
	if reason := strings.TrimPrefix(err.Error(), "session: add input device "+os.DevNull+": "+ErrDeclined.Error()+": "); reason == err.Error() || reason == "" {
		t.Fatalf("the decline of /dev/null gives no reason: %v", err)
	}
	t.Logf("libinput declines /dev/null: %v", err)
	if _, err := OpenLibinput(nil, "/nonexistent/libinput.so.10", PlainOpener{}, DefaultInputConfig(), nil, keepPriority); err == nil || !strings.Contains(err.Error(), LibinputEnv) {
		t.Fatalf("a missing library: %v", err)
	}
}

func TestLibinputDeclinesAHeadphoneJack(t *testing.T) {
	jack := newVirtualDevice(t, "lys-wm virtual headphone jack", []uinputBit{
		{uiSetEvBit, evSw},
		{uiSetSwBit, swHeadphoneInsert},
	})
	in := libinput(t, PlainOpener{})
	if err := in.reader.call("add the jack", func() error { return in.reader.add(jack.node) }); !errors.Is(err, ErrDeclined) {
		t.Fatalf("a headphone jack: %v", err)
	}
}

func TestPlainOpenerNamesTheDeviceItOpened(t *testing.T) {
	fd, d, err := PlainOpener{}.TakeDevice("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if d != (Device{Major: 1, Minor: 3}) {
		t.Fatalf("/dev/null is %+v", d)
	}
	if _, _, err := (PlainOpener{}).TakeDevice("/"); err == nil {
		t.Fatal("a directory was taken as a device")
	}
}

type takeOnceOpener struct {
	mu    sync.Mutex
	taken map[Device]bool
}

func (o *takeOnceOpener) holds(path string) bool {
	d, err := DeviceOf(path)
	if err != nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.taken[d]
}

func (o *takeOnceOpener) TakeDevice(path string) (int, Device, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	d, err := DeviceOf(path)
	if err != nil {
		return -1, Device{}, err
	}
	if o.taken[d] {
		return -1, Device{}, unix.EBUSY
	}
	fd, d, err := PlainOpener{}.TakeDevice(path)
	if err == nil {
		o.taken[d] = true
	}
	return fd, d, err
}

func (o *takeOnceOpener) ReleaseDevice(d Device) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.taken, d)
	return nil
}

func TestLibinputResumesOnlyAfterASuspend(t *testing.T) {
	kbd := newVirtualKeyboard(t)
	opener := &takeOnceOpener{taken: map[Device]bool{}}
	in := libinput(t, opener)
	in.addNow(t, kbd.node)
	in.onReader(t, "a resume without a suspend", func(r *inputReader) error { return r.resume() })
	in.onReader(t, "two suspends", func(r *inputReader) error {
		r.suspend()
		r.suspend()
		return nil
	})
	in.reader.work.Wait()
	in.onReader(t, "a resume after a suspend", func(r *inputReader) error { return r.resume() })
	if !opener.holds(kbd.node) {
		t.Error("the keyboard was not taken back after the resume")
	}
}

type blockingOpener struct {
	entered, release chan struct{}
	released         chan Device
}

func (o *blockingOpener) TakeDevice(path string) (int, Device, error) {
	close(o.entered)
	<-o.release
	return PlainOpener{}.TakeDevice(path)
}

func (o *blockingOpener) ReleaseDevice(d Device) error {
	o.released <- d
	return nil
}

func TestAnInputDeviceIsTakenOffTheLoop(t *testing.T) {
	testenv.UdevRecord(t, SystemRoots.UdevData, os.DevNull)
	opener := &blockingOpener{entered: make(chan struct{}), release: make(chan struct{}), released: make(chan Device, 1)}
	var reports []error
	in := libinputReporting(t, opener, func(err error) { reports = append(reports, err) })
	in.AddLater("/dev/null")
	<-opener.entered
	ticks := 0
	if err := in.loop.Post(func() error {
		ticks++
		in.loop.Stop()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := in.loop.Run(); err != nil {
		t.Fatal(err)
	}
	if ticks != 1 {
		t.Fatalf("%d posted ticks ran while an opener was stuck in logind", ticks)
	}
	adding := func() (pending bool) {
		in.onReader(t, "look at the devices in flight", func(r *inputReader) error {
			_, pending = r.adding["/dev/null"]
			return nil
		})
		return pending
	}
	if !adding() {
		t.Fatal("the device was not in flight while the opener held it")
	}
	close(opener.release)
	waittest.Until(t, func() bool { return !adding() }, "the taken device to reach the input reader")
	in.onReader(t, "look at what is left", func(r *inputReader) error {
		if len(r.ready) != 0 || len(r.paths) != 0 {
			return fmt.Errorf("a device libinput declines left ready %v, paths %v", r.ready, r.paths)
		}
		return nil
	})
	waittest.For(t, opener.released, "the declined device's release")
	looptest.RunUntil(t, in.loop, "the decline's report", func() bool { return len(reports) > 0 })
	if err := reports[0]; len(reports) != 1 || !errors.Is(err, ErrDeclined) || !strings.Contains(err.Error(), "/dev/null") {
		t.Fatalf("a device libinput declines reported %v", reports)
	}
}

func TestAReleaseDoesNotWaitForTheOpener(t *testing.T) {
	opener := &blockingOpener{entered: make(chan struct{}), release: make(chan struct{}), released: make(chan Device)}
	in := libinput(t, opener)
	in.reader.releaseLater(Device{Major: 13, Minor: 64})
	if d := waittest.For(t, opener.released, "the release"); d.Major != 13 {
		t.Fatalf("released %+v", d)
	}
}

func endsWithItsGoroutine(tid int) bool { return tid != unix.Getpid() }

func TestTheInputReaderRaisesItsOwnThreadAndReleasesItBeforeTheThreadEnds(t *testing.T) {
	var raised, released atomic.Int64
	in := openLibinput(t, PlainOpener{}, func(err error) { t.Errorf("reported: %v", err) }, func(func(string, func() error)) func() error {
		raised.Store(int64(unix.Gettid()))
		return func() error {
			released.Store(int64(unix.Gettid()))
			return nil
		}
	})
	tid := int(raised.Load())
	var reading int
	in.onReader(t, "read the reader's thread", func(*inputReader) error {
		reading = unix.Gettid()
		return nil
	})
	if tid == 0 || reading != tid || tid == unix.Gettid() {
		t.Fatalf("the priority was raised on thread %d, the reader reads on %d and the test runs on %d, want the reader's own thread", tid, reading, unix.Gettid())
	}
	if released.Load() != 0 {
		t.Fatal("the priority was released while the reader runs")
	}
	if err := in.Close(); err != nil {
		t.Fatal(err)
	}
	if got := int(released.Load()); got != tid {
		t.Fatalf("Close returned with the priority released on thread %d, want the reader's %d", got, tid)
	}
	if !endsWithItsGoroutine(tid) {
		return
	}
	task := fmt.Sprintf("/proc/self/task/%d", tid)
	waittest.Until(t, func() bool {
		_, err := os.Stat(task)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		return err != nil
	}, fmt.Sprintf("the reader's thread %d to end after Close", tid))
}

type staleFirstOpener struct {
	entered, release chan struct{}
	released         chan Device
	takes            int
}

func (o *staleFirstOpener) TakeDevice(path string) (int, Device, error) {
	o.takes++
	if o.takes > 1 {
		return PlainOpener{}.TakeDevice(path)
	}
	close(o.entered)
	<-o.release
	return PlainOpener{}.TakeDevice(os.DevNull)
}

func (o *staleFirstOpener) ReleaseDevice(d Device) error {
	o.released <- d
	return nil
}

func TestADeviceAddedAgainWhileItsFirstOpenIsInFlightIsAddedWithTheSecond(t *testing.T) {
	kbd := newVirtualKeyboard(t)
	opener := &staleFirstOpener{entered: make(chan struct{}), release: make(chan struct{}), released: make(chan Device, 2)}
	in := libinput(t, opener)
	rec := &inputRecorder{}
	in.Start(rec)
	in.AddLater(kbd.node)
	waittest.For(t, opener.entered, "the first open")
	in.onReader(t, "remove the keyboard and add it again", func(r *inputReader) error {
		r.remove(kbd.node)
		r.addLater(kbd.node)
		return nil
	})
	close(opener.release)
	looptest.RunUntil(t, in.loop, "the keyboard's DeviceAdded", func() bool { return len(rec.devices) > 0 })
	if len(rec.devices) != 1 || rec.devices[0].Name() != virtualKeyboard {
		t.Fatalf("added %v", rec.devices)
	}
	var null unix.Stat_t
	if err := unix.Stat(os.DevNull, &null); err != nil {
		t.Fatal(err)
	}
	if d := waittest.For(t, opener.released, "the stale open's release"); d.Major != unix.Major(null.Rdev) || d.Minor != unix.Minor(null.Rdev) {
		t.Fatalf("released %+v, not the stale open of %s", d, os.DevNull)
	}
}
