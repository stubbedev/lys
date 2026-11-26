package kms

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/drm"
	"github.com/stubbedev/lys/lib/udevmonitor"
	"golang.org/x/sys/unix"
)

// firmwareDrivers are the platform drivers of the DRM drivers that show the
// firmware's framebuffer until the native driver takes the GPU over:
// simpledrm, efidrm, vesadrm and ofdrm.
var firmwareDrivers = []string{"simple-framebuffer", "efi-framebuffer", "vesa-framebuffer", "of-display"}

// driversRemovedWithClients are the drivers whose PCI shutdown removes the DRM
// device and frees its state, so a client that closes its card afterwards
// oopses the kernel and the machine never reaches the reset.
var driversRemovedWithClients = []string{"nvidia"}

// Paths are where the display finds card nodes and their sysfs entries.
type Paths struct {
	Dev   string
	Sysfs string
}

// SystemPaths are the kernel's: /dev/dri and /sys/class/drm.
var SystemPaths = Paths{Dev: "/dev/dri", Sysfs: "/sys/class/drm"}

// Event is something that happened to the display: a card's page flip, a
// uevent, something failing off the loop, or a card worker's result.
// Display.Handle applies it.
type Event struct {
	card   *Card
	flip   drm.FlipEvent
	uevent *udevmonitor.Event
	err    error
	lit    *litCard
	unlit  *unlitCard
	scan   *scanResult
	lost   bool
}

// Display is every DRM card's outputs. It is not safe for concurrent use:
// one goroutine opens it, handles its events and draws, while a worker per card
// does whatever could keep that goroutine waiting.
type Display struct {
	paths       Paths
	open        func(path string) (device, error)
	log         *slog.Logger
	clock       clock
	scaler      Scaler
	cards       map[string]*Card
	workers     map[string]*cardWorker
	events      chan Event
	done        chan struct{}
	wg          sync.WaitGroup
	workersDone sync.WaitGroup
	uevents     *udevmonitor.Listener
	paused      bool
	outs        []*Output
	// awaiting says the last output went with a firmware framebuffer's card,
	// whose native driver's card is on its way: no card has been lit, or
	// failed to light, since.
	awaiting bool
}

// Open opens every card under paths and lights its outputs, then follows
// cards appearing and disappearing. A card that fails to open is logged and
// left out, unless another process is master of it, when it is tried again
// for a while; with none at all, Outputs is empty and the splash has no
// graphical output.
func Open(paths Paths, log *slog.Logger) (*Display, error) {
	return open(paths, func(path string) (device, error) { return drm.Open(path) }, true, systemClock, log)
}

func open(paths Paths, opener func(string) (device, error), uevents bool, clk clock, log *slog.Logger) (*Display, error) {
	d := &Display{
		paths:   paths,
		open:    opener,
		log:     log,
		clock:   clk,
		cards:   map[string]*Card{},
		workers: map[string]*cardWorker{},
		events:  make(chan Event, 16),
		done:    make(chan struct{}),
	}
	if uevents {
		s, err := udevmonitor.Listen(subsystems)
		if err != nil {
			return nil, fmt.Errorf("kms: follow the cards: %w", err)
		}
		d.uevents = s
		d.wg.Go(d.readUevents)
	}
	entries, err := os.ReadDir(paths.Dev)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, errors.Join(fmt.Errorf("kms: list %s: %w", paths.Dev, err), d.Close(false))
	}
	for _, e := range entries {
		if isCard(e.Name()) {
			d.lightNow(e.Name())
		}
	}
	return d, nil
}

func (p Paths) driver(name string) (string, error) {
	link, err := os.Readlink(filepath.Join(p.Sysfs, name, "device", "driver"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("kms: the driver of %s: %w", name, err)
	}
	return filepath.Base(link), nil
}

func (d *Display) firmware(name string) (bool, error) {
	driver, err := d.paths.driver(name)
	if err != nil {
		return false, err
	}
	return slices.Contains(firmwareDrivers, driver), nil
}

func (p Paths) ClientsMustCloseBeforeRemoval() (bool, error) {
	entries, err := os.ReadDir(p.Sysfs)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("kms: list %s: %w", p.Sysfs, err)
	}
	for _, e := range entries {
		if !isCard(e.Name()) {
			continue
		}
		driver, err := p.driver(e.Name())
		if err != nil {
			return false, err
		}
		if slices.Contains(driversRemovedWithClients, driver) {
			return true, nil
		}
	}
	return false, nil
}

// lightNow lights a card before the loop runs, so the first frame shows on it:
// nothing is waiting yet. A card another master holds goes on in its worker.
func (d *Display) lightNow(name string) {
	w := d.newWorker(name)
	w.lighting, w.until, w.wait = true, d.clock.now().Add(busyRetryFor), firstBusyRetry
	lit, _ := w.attempt(false, 0)
	d.workersDone.Go(w.run)
	if lit == nil {
		return
	}
	if err := d.take(lit); err != nil {
		d.log.Warn("lighting a card", "card", name, "err", err)
	}
}

// take brings a card its worker opened into the display.
func (d *Display) take(lit *litCard) error {
	c := lit.card
	d.cards[c.name] = c
	d.awaiting = false
	d.wg.Go(func() { d.readFlips(c, lit.reader) })
	c.apply(lit.scan)
	d.outs = nil
	if d.paused {
		return errors.Join(lit.scan.err, c.Deactivate())
	}
	return lit.scan.err
}

func eventReader(fd int, path string) (*os.File, error) {
	dup, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("kms: duplicate the fd of %s: %w", path, err)
	}
	if err := unix.SetNonblock(dup, true); err != nil {
		return nil, errors.Join(fmt.Errorf("kms: make %s non-blocking: %w", path, err), unix.Close(dup))
	}
	return os.NewFile(uintptr(dup), path), nil
}

func (d *Display) send(e Event) bool {
	select {
	case d.events <- e:
		return true
	case <-d.done:
		return false
	}
}

func (d *Display) readFlips(c *Card, r *os.File) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if errors.Is(err, os.ErrClosed) {
			return
		}
		if err != nil {
			d.send(Event{card: c, err: fmt.Errorf("kms: read the events of %s: %w", r.Name(), err)})
			return
		}
		if err := drm.ParseEvents(buf[:n], func(e drm.FlipEvent) error {
			if !d.send(Event{card: c, flip: e}) {
				return os.ErrClosed
			}
			return nil
		}); err != nil {
			d.send(Event{card: c, err: fmt.Errorf("kms: the events of %s: %w", r.Name(), err)})
			return
		}
	}
}

func (d *Display) readUevents() {
	for {
		ev, err := d.uevents.Next()
		if errors.Is(err, os.ErrClosed) {
			return
		}
		if errors.Is(err, udevmonitor.ErrLost) {
			if !d.send(Event{lost: true}) {
				return
			}
			continue
		}
		if err != nil {
			d.send(Event{err: err})
			return
		}
		if !d.send(Event{uevent: &ev}) {
			return
		}
	}
}

// Events delivers what Handle applies.
func (d *Display) Events() <-chan Event {
	return d.events
}

// Handle applies e and says whether Outputs or AwaitingNative changed: a flip
// lets its output show the next frame, a card that disappears is dropped at
// once, and what takes time, a card appearing or a hotplug, goes to the card's
// worker, whose result is another Event.
func (d *Display) Handle(e Event) (bool, error) {
	switch {
	case e.err != nil:
		return false, e.err
	case e.uevent != nil:
		return d.uevent(*e.uevent), nil
	case e.lost:
		return d.resync()
	case e.lit != nil:
		if e.lit.gen != d.workers[e.lit.card.name].gen {
			return false, nil
		}
		return true, d.take(e.lit)
	case e.unlit != nil:
		if e.unlit.gen != d.workers[e.unlit.name].gen || !d.awaiting {
			return false, nil
		}
		d.awaiting = false
		return true, nil
	case e.scan != nil:
		return d.scanned(e.scan)
	case e.card != nil:
		if d.cards[e.card.name] != e.card {
			return false, nil
		}
		return false, e.card.Flipped(e.flip)
	}
	return false, nil
}

func (d *Display) scanned(r *scanResult) (bool, error) {
	if d.cards[r.card.name] != r.card {
		return false, nil
	}
	r.card.apply(*r)
	d.outs = nil
	if len(r.added) > 0 {
		d.awaiting = false
	}
	if len(r.dropped) > 0 {
		dropped := r.dropped
		d.workers[r.card.name].ask(func(j *cardJobs) { j.retired = append(j.retired, dropped...) })
	}
	return len(r.added)+len(r.dropped) > 0, r.err
}

func (d *Display) uevent(ev udevmonitor.Event) bool {
	name, ok := card(ev)
	if !ok {
		return false
	}
	switch ev.Action {
	case udevmonitor.Add:
		until := d.clock.now().Add(busyRetryFor)
		d.worker(name).ask(func(j *cardJobs) { j.open, j.until = true, until })
	case udevmonitor.Remove:
		return d.remove(name)
	case udevmonitor.Change:
		if _, ok := d.cards[name]; ok && ev.Props["HOTPLUG"] == "1" && !d.paused {
			d.worker(name).ask(func(j *cardJobs) { j.rescan = true })
		}
	}
	return false
}

// resync stands in for uevents the socket dropped when its buffer overflowed:
// a card node that appeared is opened as an add would, one that went is removed,
// and every card still there is probed again for hotplugs.
func (d *Display) resync() (bool, error) {
	entries, err := os.ReadDir(d.paths.Dev)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("kms: list %s after lost uevents: %w", d.paths.Dev, err)
	}
	present := map[string]bool{}
	for _, e := range entries {
		if isCard(e.Name()) {
			present[e.Name()] = true
		}
	}
	changed := false
	for _, c := range d.sortedCards() {
		if !present[c.name] {
			changed = d.remove(c.name) || changed
		}
	}
	until := d.clock.now().Add(busyRetryFor)
	for name := range present {
		if _, ok := d.cards[name]; !ok {
			d.worker(name).ask(func(j *cardJobs) { j.open, j.until = true, until })
		} else if !d.paused {
			d.worker(name).ask(func(j *cardJobs) { j.rescan = true })
		}
	}
	d.log.Warn("uevents were lost, so the cards were listed again", "cards", len(present))
	return changed, nil
}

// remove forgets a card at once, so nothing more is drawn on it, and leaves its
// worker to free it. A card still being lit is told apart by the generation.
func (d *Display) remove(name string) bool {
	d.worker(name).removed()
	c, ok := d.cards[name]
	if !ok {
		return false
	}
	delete(d.cards, name)
	d.outs = nil
	d.log.Info("a card went away", "card", name)
	if c.firmware && len(d.Outputs()) == 0 {
		d.awaiting = true
		d.log.Info("waiting for the native driver's card, which takes the firmware framebuffer's place", "card", name)
	}
	return true
}

// AwaitingNative says the display has no outputs because a firmware
// framebuffer's card went away for its native driver's, which has not been lit
// yet nor failed to light. The kernel's console takes the native driver's
// framebuffer as it registers, and a VT in text mode would show it until the
// card lit.
func (d *Display) AwaitingNative() bool {
	return d.awaiting
}

// Outputs are every lit output, ordered by card and connector. The list is
// built once per change, and a change gives a new list.
func (d *Display) Outputs() []*Output {
	if d.outs != nil {
		return d.outs
	}
	d.outs = []*Output{}
	for _, c := range d.sortedCards() {
		d.outs = append(d.outs, c.outputs...)
	}
	return d.outs
}

func (d *Display) sortedCards() []*Card {
	cards := make([]*Card, 0, len(d.cards))
	for _, c := range d.cards {
		cards = append(cards, c)
	}
	slices.SortFunc(cards, func(a, b *Card) int { return cmp.Compare(a.name, b.name) })
	return cards
}

// Deactivate drops DRM master on every card, leaving the frames on screen;
// cards that appear meanwhile are opened deactivated.
func (d *Display) Deactivate() error {
	d.paused = true
	for _, w := range d.workers {
		w.setPaused(true)
	}
	var errs []error
	for _, c := range d.sortedCards() {
		errs = append(errs, c.Deactivate())
	}
	return errors.Join(errs...)
}

// Reactivate takes DRM master back on every card and shows the splash again.
func (d *Display) Reactivate() error {
	d.paused = false
	for _, w := range d.workers {
		w.setPaused(false)
	}
	var errs []error
	for _, c := range d.sortedCards() {
		errs = append(errs, c.Reactivate())
	}
	for _, w := range d.workers {
		w.ask(func(j *cardJobs) { j.rescan = true })
	}
	return errors.Join(errs...)
}

// Close releases every card and stops following uevents. With retain, the
// frames on screen stay there after the process exits (see Card.Release).
func (d *Display) Close(retain bool) error {
	close(d.done)
	d.workersDone.Wait()
	var errs []error
	if d.uevents != nil {
		errs = append(errs, d.uevents.Close())
	}
	names := slices.Sorted(maps.Keys(d.workers))
	for _, name := range names {
		errs = append(errs, d.workers[name].closeCard(retain))
	}
	clear(d.cards)
	d.outs = nil
	d.wg.Wait()
	return errors.Join(errs...)
}
