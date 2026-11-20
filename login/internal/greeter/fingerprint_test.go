package greeter

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/login/internal/auth"
	"github.com/stubbedev/lys/login/internal/fprint"
	loginscreen "github.com/stubbedev/lys/login/internal/screen"
)

func fingerprint(r fprint.Result) auth.Event {
	return auth.Event{Kind: auth.EventPrompt, Prompt: auth.Prompt{Kind: auth.PromptFingerprint, Fingerprint: r}}
}

func rememberedAda(t *testing.T) (*rig, *fakeHandle) {
	t.Helper()
	r := newRig(t, func(i *Init) { i.LastUser = "ada" })
	r.s.start()
	h := r.conv(t)
	if h.username != "ada" || len(r.convs) != 1 {
		t.Fatalf("conversations %d, the last for %q; a remembered user is listened for at once", len(r.convs), h.username)
	}
	h.emit(prompt(auth.PromptSecret, "Password:"))
	return r, h
}

func TestARememberedUserLogsInWithThePasswordTypedWhileTheReaderListens(t *testing.T) {
	r, h := rememberedAda(t)
	if !r.s.awaiting || r.s.prompt.Error.Text() != "" || !r.s.prompt.Entry.Enabled() || len(h.answers) != 0 {
		t.Fatalf("awaiting %v, message %q: the password prompt waits silently in the entry", r.s.awaiting, r.s.prompt.Error.Text())
	}
	h.emit(fingerprint(fprint.Listening))
	if !r.s.prompt.Fingerprint.Visible() || r.s.prompt.Entry.Visible() || r.focused != r.s.prompt.Fingerprint {
		t.Fatal("a listening reader does not stand in for the entry")
	}
	r.s.prompt.Fingerprint.InsertRune('p')
	if r.s.prompt.Fingerprint.Visible() || !r.s.prompt.Entry.Visible() || r.focused != r.s.prompt.Entry {
		t.Fatal("typing over the fingerprint prompt does not bring the entry back")
	}
	r.s.submit("pw")
	if !slices.Equal(h.answers, []string{"pw"}) || r.s.prompt.Entry.Enabled() || r.s.prompt.Username.Enabled() {
		t.Fatalf("answers %v; the typed password answers at once and the form locks", h.answers)
	}
	h.emit(fingerprint(fprint.Listening))
	if r.s.prompt.Fingerprint.Visible() {
		t.Error("the reader came back after the password was given")
	}
	h.emit(auth.Event{Kind: auth.EventSuccess})
	if r.quit != 1 || h.cancelled {
		t.Errorf("quit %d, cancelled %v; the password's success hands over", r.quit, h.cancelled)
	}
}

func TestAFingerprintMatchLogsInWithNoPasswordTyped(t *testing.T) {
	r, h := rememberedAda(t)
	h.emit(fingerprint(fprint.Listening))
	h.emit(auth.Event{Kind: auth.EventSuccess})
	if r.quit != 1 || len(h.answers) != 0 || r.s.prompt.Entry.Enabled() {
		t.Errorf("quit %d, answers %v; the fingerprint's success hands over with nothing typed", r.quit, h.answers)
	}
}

func TestAFingerprintThatDoesNotMatchKeepsAskingForThePassword(t *testing.T) {
	r, h := rememberedAda(t)
	h.emit(fingerprint(fprint.Listening))
	h.emit(fingerprint(fprint.NoMatch))
	if got := r.s.prompt.Error.Text(); got != loginscreen.FingerprintMessage(fprint.NoMatch) || got == "" {
		t.Errorf("message %q, want the no-match", got)
	}
	if r.quit != 0 || h.cancelled || !r.s.awaiting || len(r.convs) != 1 {
		t.Fatalf("quit %d, cancelled %v, awaiting %v: a no-match logs nobody in and leaves the prompt", r.quit, h.cancelled, r.s.awaiting)
	}
	h.emit(fingerprint(fprint.Retry))
	if got := r.s.prompt.Error.Text(); got != loginscreen.FingerprintMessage(fprint.Retry) {
		t.Errorf("message %q, want the retry", got)
	}
	r.s.prompt.Fingerprint.OnClick()
	r.s.submit("pw")
	if !slices.Equal(h.answers, []string{"pw"}) {
		t.Errorf("answers %v, want the password after the no-match", h.answers)
	}
}

func TestAWrongPasswordIsRefusedAndTheUserListenedForAgain(t *testing.T) {
	r, h := rememberedAda(t)
	h.emit(fingerprint(fprint.Listening))
	r.s.prompt.Fingerprint.OnClick()
	r.s.submit("wrong")
	h.emit(auth.Event{Kind: auth.EventFailure, Reason: "pam_authenticate: AUTH_ERR", Refusal: auth.RefusalCredentials})
	if got := r.s.prompt.Error.Text(); got != loginscreen.RefusalText(auth.RefusalCredentials) {
		t.Errorf("message %q, want the refusal", got)
	}
	if len(r.convs) != 2 || r.conv(t).username != "ada" || r.s.pending != nil {
		t.Fatalf("conversations %d; a refused password listens for ada again with nothing typed", len(r.convs))
	}
	again := r.conv(t)
	again.emit(prompt(auth.PromptSecret, "Password:"))
	again.emit(fingerprint(fprint.Listening))
	if r.s.prompt.Fingerprint.Visible() || r.s.prompt.Error.Text() != loginscreen.RefusalText(auth.RefusalCredentials) {
		t.Error("the password chosen for ada gave way to the reader, or the refusal to the prompt")
	}
	r.s.submit("pw")
	if !slices.Equal(again.answers, []string{"pw"}) || len(h.answers) != 1 {
		t.Errorf("answers %v then %v", h.answers, again.answers)
	}
}

func TestAFailureBeforeAnyPromptIsNotRetried(t *testing.T) {
	r := newRig(t, func(i *Init) { i.LastUser = "ada" })
	r.s.start()
	r.conv(t).emit(auth.Event{Kind: auth.EventFailure, Reason: "start a session worker: no such file"})
	if len(r.convs) != 1 || r.s.auth != nil {
		t.Errorf("conversations %d; a login that failed before asking anything must not loop", len(r.convs))
	}
}

func TestAPasswordTypedBeforeThePromptAnswersIt(t *testing.T) {
	r := newRig(t, func(i *Init) { i.LastUser = "ada" })
	r.s.start()
	h := r.conv(t)
	r.s.submit("pw")
	if r.s.prompt.Entry.Enabled() || len(h.answers) != 0 {
		t.Fatal("a password typed before the prompt answered nothing yet and locks the form")
	}
	h.emit(fingerprint(fprint.Listening))
	if r.s.prompt.Fingerprint.Visible() {
		t.Error("the reader stood in for a password already typed")
	}
	h.emit(prompt(auth.PromptSecret, "Password:"))
	if !slices.Equal(h.answers, []string{"pw"}) || len(r.convs) != 1 {
		t.Errorf("answers %v over %d conversations, want the typed password", h.answers, len(r.convs))
	}
}

func TestPickingAUserListensForThem(t *testing.T) {
	r := newRig(t, nil)
	r.s.users[1].Button.OnClick()
	bob := r.conv(t)
	if bob.username != "bob" {
		t.Fatalf("conversation for %q, want bob", bob.username)
	}
	r.s.users[0].Button.OnClick()
	if !bob.cancelled || r.conv(t).username != "ada" || len(r.convs) != 2 {
		t.Errorf("bob cancelled %v, then %q; picking another user listens for them alone", bob.cancelled, r.conv(t).username)
	}
}

func TestATypedUsernameIsListenedForOnEnter(t *testing.T) {
	r := newRig(t, func(i *Init) { i.Config.Login.Greeter.ShowUserList = false })
	r.s.start()
	r.s.prompt.Username.SetText(" carol ")
	r.s.prompt.Username.OnActivate(" carol ")
	if r.conv(t).username != "carol" || r.focused != r.s.prompt.Entry {
		t.Errorf("conversation for %q, focus on %T; Enter in the username listens for carol and moves to the password", r.conv(t).username, r.focused)
	}
}

func TestAPasswordForAnotherUserCancelsTheOneListenedFor(t *testing.T) {
	r := newRig(t, func(i *Init) { i.LastUser = "carol"; i.Config.Login.Greeter.ShowUserList = false })
	r.s.start()
	carol := r.conv(t)
	carol.emit(prompt(auth.PromptSecret, "Password:"))
	r.s.prompt.Username.SetText("dave")
	r.s.submit("pw")
	dave := r.conv(t)
	if !carol.cancelled || dave.username != "dave" || len(carol.answers) != 0 {
		t.Fatalf("carol cancelled %v with answers %v, then %q", carol.cancelled, carol.answers, dave.username)
	}
	dave.emit(prompt(auth.PromptSecret, "Password:"))
	if !slices.Equal(dave.answers, []string{"pw"}) {
		t.Errorf("dave's answers %v", dave.answers)
	}
}
