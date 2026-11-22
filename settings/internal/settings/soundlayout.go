package settings

import (
	"context"
	"slices"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type gridCell struct{ col, row int }

var (
	listenerCell  = gridCell{2, 2}
	speakerColumn = 5
	speakerCells  = map[native.ChannelPosition]gridCell{
		native.ChannelTopFrontLeft: {0, 0}, native.ChannelTopFrontCenter: {2, 0}, native.ChannelTopFrontRight: {4, 0},
		native.ChannelFrontLeft: {0, 1}, native.ChannelFrontLeftOfCenter: {1, 1}, native.ChannelFrontCenter: {2, 1},
		native.ChannelMono: {2, 1}, native.ChannelFrontRightOfCenter: {3, 1}, native.ChannelFrontRight: {4, 1},
		native.ChannelSideLeft: {0, 2}, native.ChannelLFE: {1, 2}, native.ChannelTopCenter: {3, 2}, native.ChannelSideRight: {4, 2},
		native.ChannelRearLeft: {0, 3}, native.ChannelRearCenter: {2, 3}, native.ChannelRearRight: {4, 3},
		native.ChannelTopRearLeft: {0, 4}, native.ChannelTopRearCenter: {2, 4}, native.ChannelTopRearRight: {4, 4},
	}
	unplacedRow = 5
)

type speakerSpot struct {
	channel int
	cell    gridCell
}

func speakerLayout(m native.ChannelMap) (spots []speakerSpot, listener gridCell) {
	taken := map[gridCell]bool{listenerCell: true}
	unplaced := 0
	for i, position := range m {
		cell, ok := speakerCells[position]
		if !ok || taken[cell] {
			cell = gridCell{unplaced % speakerColumn, unplacedRow + unplaced/speakerColumn}
			unplaced++
		}
		taken[cell] = true
		spots = append(spots, speakerSpot{channel: i, cell: cell})
	}
	rows := make([]int, 0, len(taken))
	for cell := range taken {
		rows = append(rows, cell.row)
	}
	slices.Sort(rows)
	rows = slices.Compact(rows)
	compact := func(c gridCell) gridCell { return gridCell{c.col, slices.Index(rows, c.row)} }
	for i := range spots {
		spots[i].cell = compact(spots[i].cell)
	}
	return spots, compact(listenerCell)
}

func speakerIcon(p native.ChannelPosition) string {
	if p == native.ChannelLFE {
		return "ld-audio-lines-symbolic"
	}
	return "ld-speaker-symbolic"
}

func (c *deviceControls) showSpeakers(m native.ChannelMap) {
	t := locale.T()
	k := c.p.k
	for _, w := range c.tests.Children() {
		c.tests.Remove(w)
	}
	spots, listener := speakerLayout(m)
	you := k.icon("ld-user-symbolic", "sound-listener")
	you.SetTooltip(t.Get("settings-sound-listener"))
	c.tests.Attach(you, listener.col, listener.row, 1, 1)
	c.tests.SetAlign(you, widget.AlignCenter, widget.AlignCenter)
	for _, spot := range spots {
		name := t.Get("settings-sound-channel-" + m[spot.channel].String())
		body := widget.NewBox(widget.Column, widget.Current().Spacing/4, 0)
		body.AppendAligned(k.icon(speakerIcon(m[spot.channel])), false, widget.AlignCenter)
		body.AppendAligned(k.label(name, css.Caption), false, widget.AlignCenter)
		channel := spot.channel
		b := k.roleButton(rolePlain, body, func() {
			sink := c.current.Name
			c.p.act(func(ctx context.Context, svc *pulse.Service) error {
				return playTestTone(ctx, svc, sink, m, channel)
			})
		})
		b.SetAccessibleName(t.Get("settings-sound-test-channel", i18n.Str("channel", name)))
		c.tests.Attach(b, spot.cell.col, spot.cell.row, 1, 1)
	}
}
