package theme

import (
	"strings"
	"unicode/utf8"
)

const (
	caretPhases  = 16
	breathPhases = 8
	breathSteps  = len(breathLevels)
	shakeSteps   = len(shakeEighths)
)

var breathLevels = [...]uint32{100, 98, 92, 83, 73, 62, 53, 47, 45, 47, 53, 62, 73, 83, 92, 98}

var shakeEighths = [...]int{8, -7, 5, -4, 2, -1}

type Ask struct {
	Caret  int
	Breath int
	Shake  int
}

func (a Ask) caretShown() bool {
	return a.Caret < caretPhases
}

func (a Ask) breath() int {
	return a.Breath / breathPhases
}

func (a Ask) shakeOffset(amplitude int) int {
	if a.Shake <= 0 {
		return 0
	}
	return shakeEighths[shakeSteps-a.Shake] * amplitude / 8
}

func (a Ask) phasesToNextPicture() int {
	if a.Shake > 0 {
		return 1
	}
	return min(caretPhases-a.Caret%caretPhases, breathPhases-a.Breath%breathPhases)
}

func (a Ask) advance(n int) Ask {
	return Ask{
		Caret:  (a.Caret + n) % (2 * caretPhases),
		Breath: (a.Breath + n) % (breathSteps * breathPhases),
		Shake:  max(a.Shake-n, 0),
	}
}

func (s State) WithPrompt(p Prompt) State {
	prev := s.Prompt
	s.Prompt = p
	switch {
	case p.Kind == NoPrompt:
		s.Ask = Ask{}
	case p.Kind != prev.Kind || p.Text != prev.Text || p.Retry != prev.Retry:
		s.Ask = Ask{}
		if p.Retry {
			s.Ask.Shake = shakeSteps
		}
	case p.Shown != prev.Shown:
		s.Ask.Caret = 0
	}
	return s
}

func (s State) Advance(n int) State {
	if s.Prompt.Kind != NoPrompt {
		s.Ask = s.Ask.advance(n)
		return s
	}
	s.Phase = (s.Phase + n%Frames) % Frames
	s.Progress = s.Progress.advance(n)
	return s
}

type promptWords struct {
	title       string
	device      string
	placeholder string
	refused     string
}

type passwordPrompt struct {
	prefix string
	words  promptWords
}

var passphrase = promptWords{title: "Unlock disk", placeholder: "Passphrase", refused: "Incorrect passphrase"}

var passwordPrompts = [...]passwordPrompt{
	{"Please enter passphrase for disk ", passphrase},
	{"Please enter recovery key for disk ", promptWords{title: "Unlock disk", placeholder: "Recovery key", refused: "Incorrect recovery key"}},
	{"Please unlock disk ", passphrase},
	{"Enter passphrase for ", passphrase},
}

func wordsOf(p Prompt) promptWords {
	if p.Kind == QuestionPrompt {
		return promptWords{title: p.Text, placeholder: "Answer"}
	}
	for _, known := range passwordPrompts {
		rest, ok := strings.CutPrefix(p.Text, known.prefix)
		if !ok {
			continue
		}
		device := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), ":"))
		if device == "" || strings.ContainsAny(rest, "\n\r") {
			break
		}
		w := known.words
		w.device = device
		return w
	}
	return promptWords{title: p.Text, placeholder: "Password", refused: "Incorrect password"}
}

func (p Prompt) typed() int {
	return utf8.RuneCountInString(p.Shown)
}
