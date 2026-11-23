package settings

import (
	"log"
	"slices"
	"time"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// undoLife is how long a removal's undo shows.
const undoLife = 8 * time.Second

// offerUndo shows a toast saying name was removed, its Undo running
// restore while it shows. The removal is already written, so the config
// always holds what the list shows: Undo writes the entry back, and a
// toast that goes leaves it removed. A later removal's toast takes this
// one's place without its Undo, this removal then final, so no restore
// ever lands over a newer removal.
func (k *kit) offerUndo(name string, restore func()) {
	if prev := k.undoing; prev != nil {
		prev.SetAction("", nil)
		prev.Dismiss()
		k.undoing = nil
	}
	t := locale.T()
	toast, err := k.toast(t.Get("settings-list-removed"), undoLife)
	if err != nil {
		log.Printf("settings: offer to undo removing %q: %v", name, err)
		return
	}
	toast.SetDescription(name)
	toast.SetAction(t.Get("settings-list-undo"), func() {
		if k.undoing != toast {
			return
		}
		k.undoing = nil
		restore()
	})
	k.undoing = toast
}

// insertAt is s with v inserted at at, or at the end once s is shorter,
// s itself untouched.
func insertAt[S ~[]E, E any](s S, at int, v E) S {
	return slices.Insert(slices.Clone(s), min(max(at, 0), len(s)), v)
}
