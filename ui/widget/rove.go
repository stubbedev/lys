package widget

// FocusMove is a step of roving focus along a row of focus stops.
type FocusMove uint8

// The roving focus steps: to the previous or next stop, wrapping at
// the ends, and to the first or last.
const (
	FocusMovePrev FocusMove = iota
	FocusMoveNext
	FocusMoveFirst
	FocusMoveLast
)

// MoveFocusAmong is the stop of stops focus lands on when it takes
// move from target, passing over the stops focus cannot land on: what
// a FocusNavigator returns. It is nil when target is not one of the
// stops focus can land on.
func MoveFocusAmong[W Widget](stops []W, target Widget, move FocusMove) Widget {
	at := -1
	for i, s := range stops {
		if Widget(s) == target {
			at = i
			break
		}
	}
	if at < 0 || !isFocusable(stops[at]) {
		return nil
	}
	n := len(stops)
	switch move {
	case FocusMoveFirst:
		return firstFocusableFrom(stops, 0, 1)
	case FocusMoveLast:
		return firstFocusableFrom(stops, n-1, -1)
	case FocusMovePrev:
		return firstFocusableFrom(stops, (at+n-1)%n, -1)
	case FocusMoveNext:
		return firstFocusableFrom(stops, (at+1)%n, 1)
	}
	panic("widget: MoveFocusAmong: unknown FocusMove")
}

func firstFocusableFrom[W Widget](stops []W, i, dir int) Widget {
	n := len(stops)
	for range n {
		if isFocusable(stops[i]) {
			return stops[i]
		}
		i = (i + dir + n) % n
	}
	return nil
}
