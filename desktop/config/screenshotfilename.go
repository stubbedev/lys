package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/strftime"
)

// ScreenshotCounter is the token a screenshot file name numbers its
// files with.
const ScreenshotCounter = "{counter}"

// ScreenshotFilename is a screenshot file name: a strftime pattern that
// may hold ScreenshotCounter once, compiled at load. In the schema it is
// a string of format screenshot-filename.
type ScreenshotFilename struct {
	src           string
	before, after *strftime.Layout
	counted       bool
}

// ParseScreenshotFilename compiles one file name.
func ParseScreenshotFilename(s string) (ScreenshotFilename, error) {
	before, after, counted := strings.Cut(s, ScreenshotCounter)
	if strings.Contains(after, ScreenshotCounter) {
		return ScreenshotFilename{}, fmt.Errorf("%s appears more than once in %q", ScreenshotCounter, s)
	}
	b, err := strftime.Compile(before)
	if err != nil {
		return ScreenshotFilename{}, err
	}
	a, err := strftime.Compile(after)
	if err != nil {
		return ScreenshotFilename{}, err
	}
	return ScreenshotFilename{src: s, before: b, after: a, counted: counted}, nil
}

func mustScreenshotFilename(s string) ScreenshotFilename {
	f, err := ParseScreenshotFilename(s)
	if err != nil {
		panic(err)
	}
	return f
}

// String is the file name as written.
func (f ScreenshotFilename) String() string { return f.src }

// Counted reports whether the name holds ScreenshotCounter.
func (f ScreenshotFilename) Counted() bool { return f.counted }

// At is the name a screenshot taken at t gets.
func (f ScreenshotFilename) At(t time.Time) ScreenshotName {
	return ScreenshotName{before: f.before.Format(t), after: f.after.Format(t), counted: f.counted}
}

// ScreenshotName is a screenshot file name rendered for one moment.
type ScreenshotName struct {
	before, after string
	counted       bool
}

// Counted reports whether the name is numbered by ScreenshotCounter.
func (n ScreenshotName) Counted() bool { return n.counted }

// Numbered is the name with the counter at i; a name without the
// counter is the same for every i.
func (n ScreenshotName) Numbered(i uint64) string {
	if !n.counted {
		return n.before
	}
	return n.before + strconv.FormatUint(i, 10) + n.after
}

// Next is the counter after the highest one among taken, the names
// already in the folder, so numbers keep rising past deleted files; it
// is 1 when none of taken is this name with a number.
func (n ScreenshotName) Next(taken []string) uint64 {
	var highest uint64
	for _, name := range taken {
		middle, ok := strings.CutPrefix(name, n.before)
		if !ok {
			continue
		}
		digits, ok := strings.CutSuffix(middle, n.after)
		if !ok || digits == "" || strings.TrimLeft(digits, "0123456789") != "" {
			continue
		}
		i, err := strconv.ParseUint(digits, 10, 64)
		if err != nil {
			continue
		}
		highest = max(highest, i)
	}
	return highest + 1
}

// UnmarshalConfig implements Unmarshaler.
func (f *ScreenshotFilename) UnmarshalConfig(v any) error {
	s, ok := v.(string)
	if !ok {
		return invalidType(v, "a string")
	}
	parsed, err := ParseScreenshotFilename(s)
	if err != nil {
		return err
	}
	*f = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (f ScreenshotFilename) MarshalConfig() any { return f.src }

func (ScreenshotFilename) configSchema(*schemaGen) Schema {
	return Schema{"type": "string", "format": "screenshot-filename", "description": "A strftime pattern (`Screenshot_%Y-%m-%d.png`) that may hold `" + ScreenshotCounter + "` once, which numbers the files 1, 2, 3 and on."}
}
