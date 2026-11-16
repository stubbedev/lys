package widget

import "slices"

// Politeness is how urgently an announcement interrupts an assistive
// technology: AT-SPI's live politeness, ARIA's aria-live.
type Politeness uint8

const (
	// Polite waits for the screen reader to finish what it says.
	Polite Politeness = iota + 1
	// Assertive interrupts it.
	Assertive
)

// Announcement is text an assistive technology speaks without the focus
// moving: the widget it comes from, the text, and how urgently.
type Announcement struct {
	Widget     Widget
	Text       string
	Politeness Politeness
}

type announceQueue struct{ pending []Announcement }

var listeners perLoop[announceQueue]

// SetAnnouncementsHeard tells the toolkit whether an assistive technology
// listens to the announcements made on the calling loop goroutine; the
// accessibility bridge of the application running there sets it as it
// starts and stops, so each application's bridge hears its own
// announcements and stopping one leaves the others speaking. Turning it
// off drops what waits. Loop goroutine only.
func SetAnnouncementsHeard(on bool) {
	checkLoop("SetAnnouncementsHeard")
	if !on {
		listeners.unbindCurrent()
		return
	}
	if listeners.current() == nil {
		listeners.bind(&announceQueue{})
	}
}

// Announce has assistive technology speak text for w without moving the
// focus: a notification appearing, a value the user asked for. It costs
// nothing while no assistive technology listens to the calling loop.
// Call it on the loop goroutine.
func Announce(w Widget, text string, p Politeness) {
	if text == "" || w == nil {
		return
	}
	q := listeners.current()
	if q == nil {
		return
	}
	checkLoop("Announce")
	q.add(Announcement{Widget: w, Text: text, Politeness: p})
}

func (q *announceQueue) add(a Announcement) {
	if i := slices.IndexFunc(q.pending, func(o Announcement) bool { return o.Widget == a.Widget }); i >= 0 {
		q.pending[i] = a
	} else {
		q.pending = append(q.pending, a)
	}
	MarkA11yChanged()
}

// TakeAnnouncements appends what waits on the calling loop to buf, in the
// order it was announced, and empties the queue: the bridge's pass speaks
// each once. Loop goroutine only.
func TakeAnnouncements(buf []Announcement) []Announcement {
	q := listeners.current()
	if q == nil {
		return buf
	}
	buf = append(buf, q.pending...)
	clear(q.pending)
	q.pending = q.pending[:0]
	return buf
}

// SetLiveRegion makes w a live region: text changing inside it (a
// label's, the widget's own) is announced with politeness p, once per
// accessibility pass however often it changes, with the latest text.
// Zero makes it an ordinary widget again.
func SetLiveRegion(w Widget, p Politeness) {
	if n := nodeOf(w); n != nil {
		n.live = p
	}
}

// LiveRegion returns w's politeness as a live region, zero when it is
// none.
func LiveRegion(w Widget) Politeness {
	if n := nodeOf(w); n != nil {
		return n.live
	}
	return 0
}

// liveTextChanged announces text when w is in a live region: the region
// is w itself or its nearest live ancestor.
func liveTextChanged(w Widget, text string) {
	if text == "" {
		return
	}
	q := listeners.current()
	if q == nil {
		return
	}
	for cur := w; cur != nil; cur = parentOf(cur) {
		if n := nodeOf(cur); n != nil && n.live != 0 {
			q.add(Announcement{Widget: cur, Text: text, Politeness: n.live})
			return
		}
	}
}
