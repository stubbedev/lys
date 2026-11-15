package theme

import (
	"encoding/binary"
	"fmt"
	"image"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

var shortFrames = []image.Point{image.Pt(640, 400), image.Pt(640, 480)}

func TestGoldenShortFrames(t *testing.T) {
	t.Parallel()
	for _, size := range shortFrames {
		for _, tc := range []struct {
			name  string
			state State
		}{
			{"boot", State{Message: "Starting Disk Encryption", Progress: known(50), Phase: 3}},
			{"password", State{Prompt: password(rootPrompt, 7), Hints: Hints{CapsLock: true, Keymap: "de"}}},
		} {
			name := fmt.Sprintf("%s-%dx%d", tc.name, size.X, size.Y)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				f := frame(size)
				draw(t, renderer(t, Default(), nil), f, 1, nil, tc.state)
				check(t, name, f)
			})
		}
	}
}

func TestEveryStateStaysInsideAShortFrame(t *testing.T) {
	t.Parallel()
	dots := Default()
	dots.Throbber = Dots
	states := []struct {
		name  string
		theme Theme
		state State
	}{
		{"boot", Default(), State{Phase: 3}},
		{"dots", dots, State{Phase: 14}},
		{"message and unit", Default(), State{Message: "Starting Disk Encryption", Status: "systemd-cryptsetup@root.service", Phase: 9}},
		{"progress", Default(), State{Message: "Starting", Progress: known(50), Phase: 3}},
		{"update", Default(), State{Updating: true, Progress: known(42), Message: "Installing updates"}},
		{"password", Default(), State{Prompt: password(rootPrompt, 7), Hints: Hints{CapsLock: true, Keymap: "de"}}},
		{"refused", Default(), State{Prompt: refused(rootPrompt, 0), Message: "cryptsetup: ERROR", Hints: Hints{Keymap: "de"}}},
		{"question", Default(), State{Prompt: Prompt{Kind: QuestionPrompt, Text: "Continue booting?", Shown: "yes"}}},
	}
	for _, size := range shortFrames {
		for _, tc := range states {
			t.Run(fmt.Sprintf("%s at %dx%d", tc.name, size.X, size.Y), func(t *testing.T) {
				t.Parallel()
				r := renderer(t, tc.theme, nil)
				l := r.layout(size, 1)
				whole := image.Rectangle{Max: size}
				for name, row := range map[string]image.Rectangle{"logo": l.Logo, "group": l.Group, "busy": l.Busy, "title": l.Title, "secondary": l.Secondary, "field": l.Field, "hints": l.Hints, "notice": l.Notice} {
					if !row.In(whole) {
						t.Errorf("the %s row %v runs off the %v frame", name, row, size)
					}
				}
				f := frame(size)
				draw(t, r, f, 1, nil, tc.state)
				var ink image.Rectangle
				for y := range size.Y {
					for x := range size.X {
						if render.Color(binary.LittleEndian.Uint32(f.Pix[y*f.Stride+x*4:])) == tc.theme.Background {
							continue
						}
						p := image.Pt(x, y)
						if !p.In(l.Logo) && !p.In(l.Group) {
							t.Fatalf("pixel %v is drawn outside the logo %v and the group %v", p, l.Logo, l.Group)
						}
						ink = ink.Union(image.Rect(x, y, x+1, y+1))
					}
				}
				if ink.Empty() || ink.Max.Y <= l.Busy.Min.Y {
					t.Errorf("the ink %v shows nothing of the group %v", ink, l.Group)
				}
			})
		}
	}
}

func TestAPromptOnAShortFrameShowsItsHints(t *testing.T) {
	t.Parallel()
	for _, size := range shortFrames {
		r := renderer(t, Default(), nil)
		l := r.layout(size, 1)
		if !l.Hints.In(image.Rectangle{Max: size}) {
			t.Errorf("%v: the hints row %v runs off the frame", size, l.Hints)
			continue
		}
		f := frame(size)
		draw(t, r, f, 1, nil, State{Prompt: password(rootPrompt, 3), Hints: Hints{CapsLock: true, Keymap: "de"}})
		inked := false
		for y := l.Hints.Min.Y; y < l.Hints.Max.Y && !inked; y++ {
			for x := l.Hints.Min.X; x < l.Hints.Max.X; x++ {
				if render.Color(binary.LittleEndian.Uint32(f.Pix[y*f.Stride+x*4:])) != Default().Background {
					inked = true
					break
				}
			}
		}
		if !inked {
			t.Errorf("%v: the hints row %v shows no hint", size, l.Hints)
		}
	}
}
