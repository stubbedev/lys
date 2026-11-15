package theme

import (
	"bytes"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/splash/internal/kms"
)

const rootPrompt = "Please enter passphrase for disk root:"

func password(text string, typed int) Prompt {
	return Prompt{Kind: PasswordPrompt, Text: text, Shown: strings.Repeat("•", typed)}
}

func refused(text string, typed int) Prompt {
	p := password(text, typed)
	p.Retry = true
	return p
}

func TestPromptGoldens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state State
	}{
		{"password-empty", State{Prompt: password(rootPrompt, 0)}},
		{"password", State{Prompt: password(rootPrompt, 7)}},
		{"password-capslock", State{Prompt: password(rootPrompt, 3), Hints: Hints{CapsLock: true}}},
		{"password-keymap", State{Prompt: password(rootPrompt, 3), Hints: Hints{Keymap: "de"}}},
		{"password-error", State{Prompt: refused(rootPrompt, 0), Hints: Hints{Keymap: "de"}}},
		{"password-unparsed", State{Prompt: password("Enter the PIN of the security token", 4)}},
		{"question", State{Prompt: Prompt{Kind: QuestionPrompt, Text: "Continue booting?", Shown: "yes"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkEveryScale(t, tc.name, Default(), tc.state)
		})
	}
}

func TestPromptWords(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prompt Prompt
		want   promptWords
	}{
		{password(rootPrompt, 0), promptWords{"Unlock disk", "root", "Passphrase", "Incorrect passphrase"}},
		{password("Please enter passphrase for disk Samsung SSD 970 (cryptroot): ", 0), promptWords{"Unlock disk", "Samsung SSD 970 (cryptroot)", "Passphrase", "Incorrect passphrase"}},
		{password("Please enter recovery key for disk home:", 0), promptWords{"Unlock disk", "home", "Recovery key", "Incorrect recovery key"}},
		{password("Please unlock disk sda3_crypt", 0), promptWords{"Unlock disk", "sda3_crypt", "Passphrase", "Incorrect passphrase"}},
		{password("Enter passphrase for /dev/sda2: ", 0), promptWords{"Unlock disk", "/dev/sda2", "Passphrase", "Incorrect passphrase"}},
	}
	for _, tc := range tests {
		if got := wordsOf(tc.prompt); got != tc.want {
			t.Errorf("wordsOf(%q) = %+v, want %+v", tc.prompt.Text, got, tc.want)
		}
	}
}

func TestAnUnparseablePromptShowsVerbatim(t *testing.T) {
	t.Parallel()
	for _, p := range []Prompt{
		password("Enter the PIN of the security token", 0),
		password("Please enter passphrase for disk :", 0),
		password("Please enter passphrase for disk \nroot:", 0),
		password("please enter passphrase for disk root:", 0),
		{Kind: QuestionPrompt, Text: "Please enter passphrase for disk root:"},
	} {
		w := wordsOf(p)
		if w.title != p.Text || w.device != "" {
			t.Errorf("wordsOf(%q) = %+v, want the text verbatim as the title and no device", p.Text, w)
		}
	}
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	l := r.layout(size, 1)
	text := "Enter the PIN of the security token"
	asked, said := frame(size), frame(size)
	draw(t, r, asked, 1, nil, State{Prompt: password(text, 2)})
	draw(t, r, said, 1, nil, State{Message: text})
	if !bytes.Equal(region(asked, l.title), region(said, l.title)) {
		t.Error("the unparsed prompt's title row differs from the same text as a message")
	}
}

func region(f kms.Frame, at image.Rectangle) []byte {
	var out []byte
	for y := at.Min.Y; y < at.Max.Y; y++ {
		out = append(out, f.Pix[y*f.Stride+at.Min.X*4:y*f.Stride+at.Max.X*4]...)
	}
	return out
}

func changedOutside(f kms.Frame, before []byte, d kms.Damage) (image.Point, bool) {
	for y := range f.Size.Y {
		for x := range f.Size.X {
			i := y*f.Stride + x*4
			if !bytes.Equal(f.Pix[i:i+4], before[i:i+4]) && !slices.ContainsFunc(d.Rects(), image.Pt(x, y).In) {
				return image.Pt(x, y), true
			}
		}
	}
	return image.Point{}, false
}

func stepDamage(t *testing.T, scale int, from, to State) (kms.Damage, layout, kms.Frame) {
	t.Helper()
	size := image.Pt(800*scale, 600*scale)
	r := renderer(t, Default(), nil)
	f := frame(size)
	draw(t, r, f, scale, nil, from)
	before := bytes.Clone(f.Pix)
	d := draw(t, r, f, scale, &from, to)
	if at, ok := changedOutside(f, before, d); ok {
		t.Errorf("pixel %v changed outside the damage %v", at, d.Rects())
	}
	fresh := frame(size)
	draw(t, renderer(t, Default(), nil), fresh, scale, nil, to)
	if !bytes.Equal(fresh.Pix, f.Pix) {
		t.Error("the partial redraw differs from a full one")
	}
	return d, r.layout(size, scale), f
}

func TestACaretBlinkDamagesOnlyTheCaret(t *testing.T) {
	t.Parallel()
	for _, scale := range []int{1, 2} {
		for _, p := range []Prompt{password(rootPrompt, 0), password(rootPrompt, 5), password(rootPrompt, 80), {Kind: QuestionPrompt, Text: "Go on?", Shown: "yes"}} {
			on := State{Prompt: p}
			off := on
			off.Ask.Caret = caretPhases
			for _, step := range [][2]State{{on, off}, {off, on}} {
				d, l, _ := stepDamage(t, scale, step[0], step[1])
				r := renderer(t, Default(), nil)
				want := r.caret(l, inside(l.entry), p)
				if len(d.Rects()) != 1 || d.Rects()[0] != want {
					t.Errorf("scale %d, %d typed: a blink damaged %v, want the caret's %v alone", scale, p.typed(), d.Rects(), want)
				}
				if !want.In(inside(l.entry)) {
					t.Errorf("scale %d: the caret %v lies outside the field %v", scale, want, inside(l.entry))
				}
			}
		}
	}
}

func TestABreathDamagesOnlyTheLock(t *testing.T) {
	t.Parallel()
	for _, scale := range []int{1, 2} {
		from := State{Prompt: password(rootPrompt, 2)}
		to := from
		to.Ask.Breath = breathPhases
		d, l, _ := stepDamage(t, scale, from, to)
		if len(d.Rects()) != 1 || d.Rects()[0] != l.throb {
			t.Errorf("scale %d: a breath damaged %v, want the lock's %v alone", scale, d.Rects(), l.throb)
		}
		same := from
		same.Ask.Breath = breathPhases - 1
		if d, _, _ := stepDamage(t, scale, from, same); !d.Empty() {
			t.Errorf("scale %d: a phase within one breath step damaged %v", scale, d.Rects())
		}
	}
}

func TestTheBreathChangesTheLock(t *testing.T) {
	t.Parallel()
	pictures := map[string]bool{}
	for step := range breathSteps {
		s := State{Prompt: password(rootPrompt, 0), Ask: Ask{Breath: step * breathPhases}}
		size := image.Pt(800, 600)
		r := renderer(t, Default(), nil)
		f := frame(size)
		draw(t, r, f, 1, nil, s)
		pictures[string(region(f, r.layout(size, 1).throb))] = true
	}
	if len(pictures) < breathSteps/2+1 {
		t.Errorf("a breath shows %d distinct pictures, want %d", len(pictures), breathSteps/2+1)
	}
}

func TestTheShakeEndsStill(t *testing.T) {
	t.Parallel()
	r := renderer(t, Default(), nil)
	size := image.Pt(800, 600)
	l := r.layout(size, 1)
	s := State{Prompt: password(rootPrompt, 0)}.WithPrompt(Prompt{})
	s = s.WithPrompt(refused(rootPrompt, 0))
	if s.Ask.Shake != shakeSteps {
		t.Fatalf("a refused prompt starts with %d phases of shake, want %d", s.Ask.Shake, shakeSteps)
	}
	f := frame(size)
	draw(t, r, f, 1, nil, s)
	offsets := map[int]bool{}
	frames := 0
	for s.Ask.Shake > 0 {
		if n := r.PhasesToNextPicture(s, size, 1); n != 1 {
			t.Fatalf("mid-shake the next picture is %d phases away, want 1", n)
		}
		next := s.Advance(1)
		before := bytes.Clone(f.Pix)
		d := draw(t, r, f, 1, &s, next)
		if !bounds(d).In(l.shakeSpan()) {
			t.Errorf("a shake frame damaged %v, outside the field's span %v", d.Rects(), l.shakeSpan())
		}
		if at, ok := changedOutside(f, before, d); ok {
			t.Errorf("a shake frame changed %v outside its damage", at)
		}
		offsets[l.entryAt(s.Ask).Min.X-l.entry.Min.X] = true
		frames++
		s = next
		if frames > shakeSteps {
			t.Fatal("the shake does not end")
		}
	}
	if frames != shakeSteps || len(offsets) < 3 {
		t.Errorf("the shake took %d frames over %d offsets, want %d frames over at least 3", frames, len(offsets), shakeSteps)
	}
	if l.entryAt(s.Ask) != l.entry {
		t.Errorf("after the shake the field rests at %v, want %v", l.entryAt(s.Ask), l.entry)
	}
	if n := r.PhasesToNextPicture(s, size, 1); n <= 1 {
		t.Errorf("after the shake the next picture is %d phases away, want the caret's or the breath's", n)
	}
	still := frame(size)
	draw(t, r, still, 1, nil, s)
	if !bytes.Equal(still.Pix, f.Pix) {
		t.Error("the field after the shake differs from a still refused prompt")
	}
	typed := s.WithPrompt(refused(rootPrompt, 1))
	if typed.Ask.Shake != 0 {
		t.Error("typing into a refused prompt shakes it again")
	}
}

func TestWithPrompt(t *testing.T) {
	t.Parallel()
	s := State{Phase: 7, Progress: known(40)}
	asked := s.WithPrompt(password(rootPrompt, 0))
	if asked.Ask != (Ask{}) || asked.Phase != 7 {
		t.Errorf("a new prompt starts at %+v, phase %d", asked.Ask, asked.Phase)
	}
	if asked.Ask.Shake != 0 {
		t.Error("a prompt asked the first time shakes")
	}
	waited := asked.Advance(caretPhases + 3)
	if waited.Ask.caretShown() || waited.Phase != 7 {
		t.Errorf("after %d phases the caret shows %v and the throbber is at %d, want hidden and 7", caretPhases+3, waited.Ask.caretShown(), waited.Phase)
	}
	typed := waited.WithPrompt(password(rootPrompt, 1))
	if !typed.Ask.caretShown() || typed.Ask.Breath != waited.Ask.Breath {
		t.Errorf("typing leaves the caret shown %v and the breath at %d, want shown and %d", typed.Ask.caretShown(), typed.Ask.Breath, waited.Ask.Breath)
	}
	other := typed.WithPrompt(password("Please enter passphrase for disk home:", 0))
	if other.Ask != (Ask{}) {
		t.Errorf("another disk's prompt keeps %+v", other.Ask)
	}
}

func TestAnsweringReturnsToTheBootState(t *testing.T) {
	t.Parallel()
	for _, boot := range []State{
		{Phase: 7, Progress: known(40), Message: "Starting"},
		{Phase: 11},
		{Updating: true, Progress: known(30)},
	} {
		size := image.Pt(800, 600)
		r := renderer(t, Default(), nil)
		f := frame(size)
		draw(t, r, f, 1, nil, boot)
		want := bytes.Clone(f.Pix)
		asked := boot.WithPrompt(password(rootPrompt, 0))
		draw(t, r, f, 1, &boot, asked)
		waited := asked.Advance(3*breathPhases + 5)
		draw(t, r, f, 1, &asked, waited)
		answered := waited.WithPrompt(Prompt{})
		if answered != boot {
			t.Errorf("answering left %+v, want the boot state %+v", answered, boot)
		}
		draw(t, r, f, 1, &waited, answered)
		if !bytes.Equal(f.Pix, want) {
			t.Errorf("after answering, %+v differs from before the prompt", boot)
		}
	}
}

func TestAPromptAnimatesAtAFewFramesASecond(t *testing.T) {
	t.Parallel()
	r := renderer(t, Default(), nil)
	s := State{Prompt: password(rootPrompt, 0)}
	period := 2 * caretPhases * breathSteps * breathPhases
	wakes := 0
	for elapsed := 0; elapsed < period; wakes++ {
		n := r.PhasesToNextPicture(s, image.Pt(800, 600), 1)
		if n < min(caretPhases, breathPhases)/2 {
			t.Fatalf("a waiting prompt asks for a picture %d phases on", n)
		}
		s = s.Advance(n)
		elapsed += n
	}
	if most := period/caretPhases + period/breathPhases; wakes > most {
		t.Errorf("%d pictures in %d phases, want at most %d", wakes, period, most)
	}
	hidden := State{Hidden: true, Prompt: password(rootPrompt, 0)}
	if n := r.PhasesToNextPicture(hidden, image.Pt(800, 600), 1); n != 0 {
		t.Errorf("a hidden prompt asks for a picture %d phases on, want none", n)
	}
}

func TestTheHintsRowNamesEsc(t *testing.T) {
	t.Parallel()
	s := State{Prompt: password(rootPrompt, 0), Hints: Hints{CapsLock: true, Keymap: "de"}}
	if got, want := hintsLine(s.hintsShown()), CapsLockHint+hintGap+keymapLabel+"de"+hintGap+EscHint; got != want {
		t.Errorf("the hints row is %q, want %q", got, want)
	}
	if got := hintsLine(State{Hints: Hints{CapsLock: true, Keymap: "de"}}.hintsShown()); got != "" {
		t.Errorf("without a prompt the hints row is %q, want empty", got)
	}
	if got := hintsLine(State{Prompt: password(rootPrompt, 0)}.hintsShown()); got != EscHint {
		t.Errorf("with no hints known the row is %q, want %q", got, EscHint)
	}
}

func TestTheErrorStateShowsOnlyOnARetry(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	l := r.layout(size, 1)
	plain, wrong := frame(size), frame(size)
	draw(t, r, plain, 1, nil, State{Prompt: password(rootPrompt, 0)})
	draw(t, r, wrong, 1, nil, State{Prompt: refused(rootPrompt, 0)})
	if bytes.Equal(region(plain, l.notice), region(wrong, l.notice)) {
		t.Error("a refused prompt shows no error line")
	}
	if !bytes.Equal(region(plain, l.notice), bytes.Repeat([]byte{0, 0, 0, 0xff}, l.notice.Dx()*l.notice.Dy())) {
		t.Error("a first prompt shows something on the error line")
	}
	if bytes.Equal(region(plain, l.entry), region(wrong, l.entry)) {
		t.Error("a refused prompt's field looks like a first one's")
	}
	d, _, _ := stepDamage(t, 1, State{Prompt: refused(rootPrompt, 0)}, State{Prompt: refused(rootPrompt, 0), Message: "cryptsetup: ERROR: wrong"})
	if !d.Empty() {
		t.Errorf("a message under the error line damaged %v; the error line keeps the row", d.Rects())
	}
}
