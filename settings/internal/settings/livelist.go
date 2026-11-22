package settings

import (
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type liveList struct {
	*widget.Box
	status  *widget.Label
	list    *widget.Box
	reading string
}

func newLiveList(k *kit, readingKey, reloadKey string, reload func()) *liveList {
	l := &liveList{Box: gapBox(widget.Column), status: k.label("", css.Caption, css.DimLabel), list: gapBox(widget.Column), reading: locale.T().Get(readingKey)}
	head := gapBox(widget.Row)
	head.AppendAligned(l.status, true, widget.AlignCenter)
	head.AppendAligned(k.reloadButton(reloadKey, reload), false, widget.AlignCenter)
	l.Append(head, false)
	l.Append(l.list, false)
	return l
}

func (l *liveList) startReading() {
	l.status.SetText(l.reading)
	l.status.SetVisible(true)
}

func (l *liveList) doneReading() { l.status.SetVisible(false) }
