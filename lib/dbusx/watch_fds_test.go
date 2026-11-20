package dbusx_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

func copiesOf(t *testing.T, fd dbus.UnixFD) []dbus.UnixFD {
	t.Helper()
	held := fdtest.Holders(t, int(fd))
	copies := make([]dbus.UnixFD, len(held))
	for i, h := range held {
		copies[i] = intconv.Must[dbus.UnixFD](h)
	}
	return copies
}

func inodeOf(t *testing.T, fd dbus.UnixFD) uint64 {
	t.Helper()
	var st unix.Stat_t
	if err := unix.Fstat(int(fd), &st); err != nil {
		t.Fatalf("descriptor %d is not open: %v", fd, err)
	}
	return st.Ino
}

func TestWatchesOfOneSignalEachOwnItsDescriptor(t *testing.T) {
	w := startWatched(t)
	takers := []chan dbus.UnixFD{make(chan dbus.UnixFD, 1), make(chan dbus.UnixFD, 1)}
	failed := func(err error) { t.Errorf("a watch failed: %v", err) }
	for _, took := range takers {
		watch, err := dbusx.WatchSignals(t.Context(), w.client, dbusx.FromOwner(watchedName), failed,
			dbusx.On(handSignal, watchedPath, func(h handed, _ dbus.Sequence) error {
				took <- h.FD
				return nil
			}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = watch.Stop() })
	}
	ticked := make(chan uint32, 1)
	bystander, err := dbusx.WatchSignals(t.Context(), w.client, dbusx.FromOwner(watchedName), failed,
		dbusx.On(tickSignal, watchedPath, func(c counted, _ dbus.Sequence) error {
			ticked <- c.N
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bystander.Stop() })

	pipe := dbustest.NewPipe(t)
	write := pipe.Write
	inode := inodeOf(t, write)
	if err := handSignal.Emit(w.service, watchedPath, handed{FD: write}); err != nil {
		t.Fatal(err)
	}
	if err := tickSignal.Emit(w.service, watchedPath, counted{N: 1}); err != nil {
		t.Fatal(err)
	}
	first, second := waittest.For(t, takers[0], "descriptor for the first watch"), waittest.For(t, takers[1], "descriptor for the second watch")
	waittest.For(t, ticked, "Tick behind the Hand, by which the third watch has dropped its copy")
	if first == second || first == write || second == write || first < 0 || second < 0 {
		t.Fatalf("the watches were handed descriptors %d and %d of the sender's %d, want one of its own each", first, second, write)
	}
	for _, fd := range []dbus.UnixFD{first, second} {
		if got := inodeOf(t, fd); got != inode {
			t.Errorf("descriptor %d is inode %d, want the pipe's %d", fd, got, inode)
		}
	}
	if got, want := copiesOf(t, write), slices.Sorted(slices.Values([]dbus.UnixFD{first, second})); !slices.Equal(got, want) {
		t.Errorf("descriptors %v hold the sent pipe with the signal handled, want one per watch that routes it, %v", got, want)
	}
	for _, fd := range []dbus.UnixFD{first, second} {
		if err := unix.Close(int(fd)); err != nil {
			t.Errorf("close descriptor %d: %v", fd, err)
		}
	}
	if got := copiesOf(t, write); len(got) != 0 {
		t.Errorf("descriptors %v hold the sent pipe once both watches closed their own", got)
	}
	pipe.RequireEveryCopyClosed(t)
}

func TestAWatchThatEndsClosesTheDescriptorsItNeverRead(t *testing.T) {
	w := startWatched(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var mu sync.Mutex
	var took []dbus.UnixFD
	watch, err := dbusx.WatchSignals(ctx, w.client, dbusx.FromOwner(watchedName), func(err error) { t.Errorf("the watch failed: %v", err) },
		dbusx.On(handSignal, watchedPath, func(h handed, _ dbus.Sequence) error {
			mu.Lock()
			took = append(took, h.FD)
			mu.Unlock()
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	pipe := dbustest.NewPipe(t)
	write := pipe.Write
	const sent = 8
	for range sent {
		if err := handSignal.Emit(w.service, watchedPath, handed{FD: write}); err != nil {
			t.Fatal(err)
		}
	}
	waittest.For(t, entered, "first Hand in its handler")
	arrived := make(chan uint32, 1)
	marker, err := dbusx.WatchSignals(t.Context(), w.client, dbusx.FromOwner(watchedName), func(error) {},
		dbusx.On(tickSignal, watchedPath, func(c counted, _ dbus.Sequence) error {
			arrived <- c.N
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = marker.Stop() })
	if err := tickSignal.Emit(w.service, watchedPath, counted{N: 1}); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, arrived, "Tick behind every Hand, by which the connection has read them all")
	cancel()
	close(release)
	if err := watch.Stop(); err != nil {
		t.Fatalf("Stop = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(took) == 0 || len(took) > sent {
		t.Fatalf("the handler took %d descriptors of %d sent", len(took), sent)
	}
	if got, want := copiesOf(t, write), slices.Sorted(slices.Values(took)); !slices.Equal(got, want) {
		t.Errorf("descriptors %v hold the sent pipe after the watch ended, want the %v its handler took: the unread ones leaked", got, want)
	}
	for _, fd := range took {
		if err := unix.Close(int(fd)); err != nil {
			t.Errorf("close descriptor %d: %v", fd, err)
		}
	}
	pipe.RequireEveryCopyClosed(t)
}

func TestAWatchClosesTheDescriptorsInArgumentsItsSignalDoesNotDeclare(t *testing.T) {
	w := startWatched(t)
	ticked := make(chan uint32, 2)
	watch, err := dbusx.WatchSignals(t.Context(), w.client, dbusx.FromOwner(watchedName), func(err error) { t.Errorf("the watch failed: %v", err) },
		dbusx.On(tickSignal, watchedPath, func(c counted, _ dbus.Sequence) error {
			ticked <- c.N
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watch.Stop() })

	pipe := dbustest.NewPipe(t)
	write := pipe.Write
	if err := w.service.Emit(watchedPath, tickSignal.Name(), uint32(1), write, []dbus.UnixFD{write}); err != nil {
		t.Fatal(err)
	}
	if err := tickSignal.Emit(w.service, watchedPath, counted{N: 2}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []uint32{1, 2} {
		if got := waittest.For(t, ticked, "Tick, the first with two arguments appended"); got != want {
			t.Fatalf("Tick %d, want %d", got, want)
		}
	}
	if got := copiesOf(t, write); len(got) != 0 {
		t.Errorf("descriptors %v hold the sent pipe with the Tick handled: those in the arguments Tick does not declare belong to nobody", got)
	}
	pipe.RequireEveryCopyClosed(t)
}
