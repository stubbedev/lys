package config

import (
	"errors"
	"testing"
	"time"
)

var shotTime = time.Date(2026, 9, 30, 12, 34, 56, 0, time.UTC)

func TestAScreenshotNameWithTheCounterIsNumbered(t *testing.T) {
	f, err := ParseScreenshotFilename("Shot_%Y-%m-%d_{counter}.png")
	if err != nil {
		t.Fatal(err)
	}
	n := f.At(shotTime)
	if !f.Counted() || !n.Counted() {
		t.Fatal("a name holding the counter is not counted")
	}
	if got := n.Numbered(7); got != "Shot_2026-09-30_7.png" {
		t.Errorf("numbered 7 = %q", got)
	}
	if f.String() != "Shot_%Y-%m-%d_{counter}.png" {
		t.Errorf("source = %q", f.String())
	}
}

func TestTheNextCounterIsOnePastTheHighestOfTheSameName(t *testing.T) {
	n := mustScreenshotFilename("Shot_%Y-%m-%d_{counter}.png").At(shotTime)
	for _, c := range []struct {
		taken []string
		want  uint64
	}{
		{nil, 1},
		{[]string{"Shot_2026-09-30_1.png", "Shot_2026-09-30_2.png"}, 3},
		{[]string{"Shot_2026-09-30_9.png", "Shot_2026-09-30_3.png"}, 10},
		{[]string{"Shot_2026-09-30_007.png"}, 8},
		{[]string{"Shot_2026-09-29_40.png", "Shot_2026-09-30_.png", "Shot_2026-09-30_2a.png", "Shot_2026-09-30_5.jpg", "Shot_2026-09-30_-4.png", "notes.txt"}, 1},
	} {
		if got := n.Next(c.taken); got != c.want {
			t.Errorf("next after %v = %d, want %d", c.taken, got, c.want)
		}
	}
}

func TestAScreenshotNameWithoutTheCounterIsTheSameForEveryNumber(t *testing.T) {
	f := mustScreenshotFilename("Screenshot_%H-%M-%S.png")
	n := f.At(shotTime)
	if f.Counted() || n.Counted() {
		t.Fatal("a name without the counter is counted")
	}
	if a, b := n.Numbered(1), n.Numbered(5); a != "Screenshot_12-34-56.png" || a != b {
		t.Errorf("numbered = %q and %q, want the one name", a, b)
	}
}

func TestAScreenshotNameRefusesATwiceCounterAndABadPattern(t *testing.T) {
	for _, s := range []string{"{counter}-{counter}.png", "%Q_{counter}.png", "{counter}_%Q.png"} {
		if _, err := ParseScreenshotFilename(s); err == nil {
			t.Errorf("%q parsed", s)
		}
	}
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  screenshot:\n    filename-format: \"{counter}{counter}.png\"\n"))
	var d Diagnostic
	if !errors.As(err, &d) || d.Field("Field") != "modules.screenshot.filename-format" {
		t.Errorf("a twice counter = %v, want a diagnostic for its field", err)
	}
	if cfg.Screenshot.FilenameFormat.String() != Defaults().Screenshot.FilenameFormat.String() {
		t.Errorf("a bad name left %q, want the default", cfg.Screenshot.FilenameFormat)
	}
	cfg, err = LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  screenshot:\n    filename-format: \"shot {counter}.png\"\n"))
	if err != nil || !cfg.Screenshot.FilenameFormat.Counted() {
		t.Errorf("a counted name loads as %q, %v", cfg.Screenshot.FilenameFormat, err)
	}
}
