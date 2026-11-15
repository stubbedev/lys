// Package audiolevel is the volume level the `lys-shell audio
// output-volume` and `input-volume` commands take: a percentage to set,
// or +N and -N to move the current one.
package audiolevel

import (
	"fmt"
	"strconv"
	"strings"
)

// Level is a parsed volume level.
type Level struct {
	// Percent is the level to set, or the signed step of a relative one.
	Percent float64
	// Relative marks a +N or -N step.
	Relative bool
}

// Parse reads "N" as a level to set and "+N" or "-N" as a step.
func Parse(s string) (Level, error) {
	if !strings.HasPrefix(s, "+") && !strings.HasPrefix(s, "-") {
		level, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return Level{}, fmt.Errorf("parse volume level %q: %w", s, err)
		}
		return Level{Percent: level}, nil
	}
	delta, err := strconv.ParseFloat(s[1:], 64)
	if err != nil {
		return Level{}, fmt.Errorf("parse volume delta %q: %w", s, err)
	}
	if s[0] == '-' {
		delta = -delta
	}
	return Level{Percent: delta, Relative: true}, nil
}

// Apply is the level reached from current, clamped to 0-100.
func (l Level) Apply(current float64) float64 {
	next := l.Percent
	if l.Relative {
		next += current
	}
	return min(max(next, 0), 100)
}
