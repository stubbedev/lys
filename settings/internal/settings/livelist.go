package settings

import (
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// liveList is a list read from the system, read again on its reload
// button: its rows, and the state its reading leaves (loading, empty, a
// failure) in their place.
type liveList struct {
	*widget.Box
	rows    *widget.ItemGroup
	state   *cardState
	reading string
}

func newLiveList(k *kit, readingKey, reloadKey string, reload func()) *liveList {
	l := &liveList{Box: gapBox(widget.Column), rows: widget.NewItemGroup(), state: k.cardState(), reading: locale.T().Get(readingKey)}
	l.AppendAligned(k.reloadButton(reloadKey, reload), false, widget.AlignEnd)
	l.Append(l.rows, false)
	l.clearRows()
	return l
}

// clearRows empties the list down to its state.
func (l *liveList) clearRows() {
	l.rows.Clear()
	l.rows.Append(l.state)
}

func (l *liveList) startReading() { l.state.Loading(l.reading) }
