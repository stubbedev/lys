package settings

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/region"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
)

// weekPicture is the week card value's picture drawn 70 by 30.
func weekPicture(t *testing.T, c *choiceCards, value string) []byte {
	t.Helper()
	for _, card := range c.cards {
		if card.value != value {
			continue
		}
		if card.pic.draw == nil {
			return nil
		}
		const w, h = 70, 30
		buf := make([]byte, w*h*4)
		card.pic.draw(render.New(buf, w*4, w, h), render.Rect{W: w, H: h}, testBackdrop(t))
		return buf
	}
	t.Fatalf("no %s card", value)
	return nil
}

// Follow region drew the time locale's week while its caption named the
// region's first day: its picture is the week of the day the region
// starts on, the one its caption names, and with the region unreadable
// it draws no week at all.
func TestFollowRegionsWeekPictureIsTheRegionsWeek(t *testing.T) {
	for _, day := range []struct {
		weekday time.Weekday
		value   string
	}{{time.Monday, "monday"}, {time.Sunday, "sunday"}, {time.Saturday, "saturday"}} {
		k := testKit(t, "")
		k.region = func() (region.Prefs, error) { return region.Prefs{WeekStart: day.weekday}, nil }
		h := fieldControl(k, k.store, pageRow(t, dateTimeID, "modules.clock.calendar-weekday-start")).(*regionHint)
		c := h.inner.(*choiceCards)
		if !bytes.Equal(weekPicture(t, c, "auto"), weekPicture(t, c, day.value)) {
			t.Errorf("a region starting its weeks on %s draws Follow region unlike %s", day.weekday, day.value)
		}
	}
	k := testKit(t, "")
	k.region = func() (region.Prefs, error) { return region.Prefs{}, errors.New("no locale archive") }
	c := fieldControl(k, k.store, pageRow(t, dateTimeID, "modules.clock.calendar-weekday-start")).(*regionHint).inner.(*choiceCards)
	if weekPicture(t, c, "auto") != nil {
		t.Error("an unreadable region still draws a week for Follow region")
	}
	if weekPicture(t, c, "monday") == nil {
		t.Error("an unreadable region took Monday's picture too")
	}
}

// The week start's pictures lie under the row's text across the card,
// as every picture choice does, one line of them, not two lines in a
// column beside the text.
func TestTheWeekStartPicturesSpanTheCard(t *testing.T) {
	win := laidOutPage(t, "", dateTimeID, 1000)
	cards := treetest.All[*choiceCards](win.page)
	if len(cards) != 1 {
		t.Fatalf("%d picture choices on Date & Time", len(cards))
	}
	row := treetest.All[*fieldRow](win.page)[0]
	title := treetest.LabelReading(t, row, row.Title())
	c := cards[0]
	if c.Bounds().X != title.Bounds().X {
		t.Errorf("the pictures start at x %d, the row's title at %d", c.Bounds().X, title.Bounds().X)
	}
	if lines := cardLines(c); len(lines) != 1 {
		t.Errorf("the pictures lie on %d lines, want one", len(lines))
	}
}
