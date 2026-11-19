package sni

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
)

func registeredIDs(items []Item) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	slices.Sort(ids)
	return ids
}

func TestRegisteredReadsTheItemsTheWatcherHolds(t *testing.T) {
	bus := dbustest.Start(t)
	a, b := startItem(t, bus, "alpha"), startItem(t, bus, "beta")
	startExternalWatcher(t, bus, a.conn.Names()[0], b.conn.Names()[0]+"/StatusNotifierItem")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := Registered(ctx, bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := registeredIDs(items); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("read %v", got)
	}
	for _, it := range items {
		if it.Title != it.ID+" title" || it.IconName != "first-icon" || len(it.IconPixmap) != 0 {
			t.Errorf("%s reads title %q, icon %q and %d pixmaps", it.ID, it.Title, it.IconName, len(it.IconPixmap))
		}
	}
}

func TestRegisteredNamesAnItemThatDoesNotAnswerAndKeepsTheRest(t *testing.T) {
	bus := dbustest.Start(t)
	a := startItem(t, bus, "alpha")
	startExternalWatcher(t, bus, a.conn.Names()[0], ":1.9999")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := Registered(ctx, bus.Conn(t))
	if err == nil || !strings.Contains(err.Error(), ":1.9999") {
		t.Fatalf("a gone item gives the error %v, want one naming it", err)
	}
	if got := registeredIDs(items); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("beside the gone item read %v, want alpha", got)
	}
}

func TestRegisteredWithoutAWatcherFails(t *testing.T) {
	bus := dbustest.Start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if items, err := Registered(ctx, bus.Conn(t)); err == nil || !strings.Contains(err.Error(), WatcherName) {
		t.Fatalf("with no watcher read %v, error %v", items, err)
	}
}
