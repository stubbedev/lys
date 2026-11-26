package launcher

import (
	"context"

	"github.com/stubbedev/lys/desktop/shell/launcherview"

	engine "github.com/stubbedev/lys/shell/service/launcher"
)

// multiSelect is MultiSelect: whether the mode takes several rows, the
// rows picked so far (by item index), and the ballots that mark them.
type multiSelect struct {
	enabled                          bool
	picked                           map[uint32]bool
	ballotSelected, ballotUnselected string
}

// ballot is the mark item index's row shows, nil while the mode takes
// one row.
func (m *multiSelect) ballot(index uint32) *string {
	switch {
	case !m.enabled:
		return nil
	case m.picked[index]:
		return &m.ballotSelected
	}
	return &m.ballotUnselected
}

// viewRow is how the list shows an item with ballot.
func viewRow(it engine.Item, ballot *string) launcherview.Row {
	return launcherview.Row{
		Text: it.Display, Markup: it.Flags.Has(engine.FlagMarkup), Icon: viewIcon(it.Icon),
		Urgent: it.Flags.Has(engine.FlagUrgent), Active: it.Flags.Has(engine.FlagActive),
		Unselectable: it.Flags.Has(engine.FlagNonselectable), Ballot: ballot,
	}
}

// viewIcon is the icon a row shows right away: a thumbnail shows its
// fallback until thumbnailPending's file is ready.
func viewIcon(icon engine.Icon) launcherview.Icon {
	switch ic := icon.(type) {
	case engine.IconName:
		return launcherview.IconName(ic)
	case engine.IconFile:
		return launcherview.IconFile(ic)
	case engine.IconThumbnail:
		return launcherview.IconName(ic.Fallback)
	}
	return nil
}

// thumbnailPending is the file a row's icon waits on, "" when it shows
// its final icon.
func thumbnailPending(icon engine.Icon) string {
	if ic, ok := icon.(engine.IconThumbnail); ok {
		return ic.Path
	}
	return ""
}

// generateThumbnail finds or makes path's thumbnail off the loop, on a
// goroutine spawn starts, and hands the file to done (nothing when none
// could be made).
func generateThumbnail(ctx context.Context, spawn func(func()), t engine.Thumbnailer, path string, done func(string)) {
	spawn(func() {
		if made, ok := t.Generate(ctx, path); ok {
			done(made)
		}
	})
}
