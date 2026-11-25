package settings

import (
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// liveList is a list read from the system, read again on its reload
// button, which its row carries on its title line: its rows, and the
// state its reading leaves (loading, empty, a failure) in their place.
type liveList struct {
	*widget.Box
	rows    *widget.ItemGroup
	state   *cardState
	reading string
	reload  *widget.Button
}

func newLiveList(k *kit, readingKey, reloadKey string, reload func()) *liveList {
	l := &liveList{Box: gapBox(widget.Column), rows: widget.NewItemGroup(), state: k.cardState(), reading: locale.T().Get(readingKey), reload: k.reloadButton(reloadKey, reload)}
	l.Append(l.rows, false)
	l.clearRows()
	return l
}

// rowActions is the reload button, an action of the list's row.
func (l *liveList) rowActions() []widget.Widget { return []widget.Widget{l.reload} }

// clearRows empties the list down to its state.
func (l *liveList) clearRows() {
	l.rows.Clear()
	l.rows.Append(l.state)
}

func (l *liveList) startReading() { l.state.Loading(l.reading) }
