package strftime

import "strings"

// HourCycle is how a format writes the hour.
type HourCycle int

// Hour cycles.
const (
	NoHour HourCycle = iota
	Hour12
	Hour24
	MixedHours
)

// HourCycle reports how the format writes the hour: %I with or without
// %p is Hour12, %H or %T is Hour24, both is MixedHours and neither NoHour.
func (l *Layout) HourCycle() HourCycle {
	var twelve, twentyFour bool
	for _, spec := range specs(l.src) {
		switch spec {
		case 'I':
			twelve = true
		case 'H', 'T':
			twentyFour = true
		}
	}
	switch {
	case twelve && twentyFour:
		return MixedHours
	case twelve:
		return Hour12
	case twentyFour:
		return Hour24
	default:
		return NoHour
	}
}

// WithHourCycle rewrites the hour of a Hour12 or Hour24 format to the
// other cycle and leaves every other part of it as written: %H becomes
// %I and %T becomes %I:%M:%S, followed by " %p" after the last time
// field when the format has no %p; the other way %I becomes %H and %p
// is dropped with the space before it. A format already in c, or with
// no hour or mixed hours, comes back unchanged.
func (l *Layout) WithHourCycle(c HourCycle) *Layout {
	from := l.HourCycle()
	if from == c || (from != Hour12 && from != Hour24) || (c != Hour12 && c != Hour24) {
		return l
	}
	if c == Hour12 {
		return &Layout{src: to12(l.src)}
	}
	return &Layout{src: to24(l.src)}
}

func to12(src string) string {
	var b strings.Builder
	lastTime, hasPeriod := -1, false
	eachDirective(src, func(text string, flag, spec byte) {
		switch spec {
		case 'H':
			b.WriteString(directiveText(flag, 'I'))
		case 'T':
			b.WriteString(directiveText(flag, 'I') + ":%M:%S")
		default:
			b.WriteString(text)
		}
		switch spec {
		case 'H', 'T', 'M', 'S':
			lastTime = b.Len()
		case 'p':
			hasPeriod = true
		}
	})
	out := b.String()
	if hasPeriod || lastTime < 0 {
		return out
	}
	return out[:lastTime] + " %p" + out[lastTime:]
}

func to24(src string) string {
	var b strings.Builder
	skipSpace := false
	eachDirective(src, func(text string, flag, spec byte) {
		switch spec {
		case 'I':
			b.WriteString(directiveText(flag, 'H'))
		case 'p':
			kept := b.String()
			trimmed := strings.TrimSuffix(kept, " ")
			skipSpace = trimmed == kept
			b.Reset()
			b.WriteString(trimmed)
			return
		case 0:
			if skipSpace {
				text = strings.TrimPrefix(text, " ")
			}
			b.WriteString(text)
		default:
			b.WriteString(text)
		}
		skipSpace = false
	})
	return b.String()
}

func directiveText(flag, spec byte) string {
	if flag == 0 {
		return "%" + string(spec)
	}
	return "%" + string(flag) + string(spec)
}

func eachDirective(src string, fn func(text string, flag, spec byte)) {
	for i := 0; i < len(src); {
		if src[i] != '%' {
			j := strings.IndexByte(src[i:], '%')
			if j < 0 {
				j = len(src) - i
			}
			fn(src[i:i+j], 0, 0)
			i += j
			continue
		}
		flag, spec, n := directive(src[i:])
		if n == 0 {
			fn(src[i:], 0, 0)
			return
		}
		fn(src[i:i+n], flag, spec)
		i += n
	}
}
