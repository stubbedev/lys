package calendar

import (
	"slices"
	"testing"
	"time"
)

func TestRezoneMovesZonedEventsAndKeepsFloatingOnes(t *testing.T) {
	s, _ := newFixtureStore(t, map[string]string{
		"zoned.ics":    "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:zoned\r\nSUMMARY:Zoned\r\nDTSTART:20270310T120000Z\r\nDTEND:20270310T130000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
		"floating.ics": "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:floating\r\nSUMMARY:Floating\r\nDTSTART:20270310T090000\r\nDTEND:20270310T100000\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
	})
	changes, stop := s.Changes()
	defer stop()
	if got, want := dayText(s.Month(2027, time.March), 10), []string{"09:00 Floating", "13:00 Zoned"}; !slices.Equal(got, want) {
		t.Fatalf("in Copenhagen March 10 = %q, want %q", got, want)
	}
	s.Rezone(time.FixedZone("UTC+5", 5*3600))
	select {
	case <-changes:
	default:
		t.Error("Rezone did not tick the store's changes")
	}
	if got, want := dayText(s.Month(2027, time.March), 10), []string{"09:00 Floating", "17:00 Zoned"}; !slices.Equal(got, want) {
		t.Errorf("five hours east of UTC March 10 = %q, want %q", got, want)
	}
}
