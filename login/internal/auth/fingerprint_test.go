package auth

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
)

type servicesPAM struct {
	byService map[string]*fakePAM
	started   []string
}

func (s *servicesPAM) start(service, user string, converse func([]pamMessage) ([]pamReply, int32)) (pamTxn, error) {
	s.started = append(s.started, service)
	f, ok := s.byService[service]
	if !ok {
		return nil, errors.New("no such service " + service)
	}
	return f.start(service, user, converse)
}

var raceServices = FingerprintServices{Password: "lys-login", Fingerprint: "lys-login-fingerprint"}

func racePAM() (*servicesPAM, *fakePAM, *fakePAM) {
	password := &fakePAM{password: "pw", batches: [][]pamMessage{{{style: pamPromptEchoOff, text: "Password: "}}}}
	fingerprint := &fakePAM{permit: true}
	return &servicesPAM{byService: map[string]*fakePAM{raceServices.Password: password, raceServices.Fingerprint: fingerprint}}, password, fingerprint
}

type fakeScan struct {
	matched chan struct{}
	halted  chan struct{}
	user    string
}

func newFakeScan() *fakeScan {
	return &fakeScan{matched: make(chan struct{}), halted: make(chan struct{})}
}

func (f *fakeScan) scanner(user string) (<-chan struct{}, func()) {
	f.user = user
	return f.matched, func() { close(f.halted) }
}

func (f *fakeScan) wasHalted() bool {
	select {
	case <-f.halted:
		return true
	default:
		return false
	}
}

func typing(answer string) AskUntil {
	return func(p Prompt, _ <-chan struct{}) (string, bool) {
		if !p.WantsInput() {
			return "", false
		}
		return answer, true
	}
}

func waitingForAPassword(onAsked func()) AskUntil {
	return func(p Prompt, withdrawn <-chan struct{}) (string, bool) {
		if !p.WantsInput() {
			return "", false
		}
		onAsked()
		select {
		case <-withdrawn:
			return "", false
		case <-time.After(10 * time.Second):
			return "a password typed long after", true
		}
	}
}

func TestAPasswordTypedWhileTheReaderListensLogsInAndStopsTheScan(t *testing.T) {
	lib, _, _ := racePAM()
	scan := newFakeScan()
	l, err := authenticateOrFingerprint(lib, raceServices, "alice", typing("pw"), scan.scanner, diskKey("luks"))
	if err != nil {
		t.Fatal(err)
	}
	if !l.Typed() || string(l.TakeSecret()) != "pw" {
		t.Errorf("typed %v; want the typed password as the secret", l.Typed())
	}
	if !scan.wasHalted() || scan.user != "alice" {
		t.Errorf("halted %v for %q; the password's login must stop alice's scan", scan.wasHalted(), scan.user)
	}
	if !slices.Equal(lib.started, []string{raceServices.Password}) {
		t.Errorf("transactions %v, want the password's alone", lib.started)
	}
}

func TestAWrongPasswordWhileTheReaderListensIsRefusedWithoutWaitingForIt(t *testing.T) {
	lib, password, _ := racePAM()
	scan := newFakeScan()
	done := make(chan error, 1)
	go func() {
		_, err := authenticateOrFingerprint(lib, raceServices, "alice", typing("wrong"), scan.scanner, diskKey("luks"))
		done <- err
	}()
	err := waittest.For(t, done, "the refusal of a wrong password while the fingerprint reader listens")
	if refused, ok := errors.AsType[*RefusedError](err); !ok || refused.Refusal() != RefusalCredentials {
		t.Fatalf("a wrong password: %v, want the refusal of a wrong password", err)
	}
	if !scan.wasHalted() {
		t.Error("the refused password left the scan running")
	}
	if !slices.Equal(lib.started, []string{raceServices.Password}) || password.endStatus != fakeAuthErr {
		t.Errorf("transactions %v ending %d, want the password's alone, refused", lib.started, password.endStatus)
	}
}

func TestAFingerprintMatchLogsInThroughTheFingerprintService(t *testing.T) {
	lib, _, fingerprint := racePAM()
	scan := newFakeScan()
	asked := func() { close(scan.matched) }
	l, err := authenticateOrFingerprint(lib, raceServices, "alice", waitingForAPassword(asked), scan.scanner, diskKey("luks"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Typed() || l.User() != "alice" {
		t.Errorf("typed %v for %s; want alice let in by her fingerprint", l.Typed(), l.User())
	}
	if !slices.Equal(lib.started, []string{raceServices.Fingerprint}) {
		t.Errorf("transactions %v, want the fingerprint's alone: no password transaction was left to withdraw", lib.started)
	}
	if !slices.Equal(fingerprint.calls, []string{"authenticate", "acct_mgmt"}) {
		t.Errorf("the fingerprint's transaction made %v, want the account checked", fingerprint.calls)
	}
	if !scan.wasHalted() {
		t.Error("the matched scan was not halted")
	}
}

func TestAFingerprintLoginTakesTheDiskKeyForTheKeyring(t *testing.T) {
	lib, _, _ := racePAM()
	scan := newFakeScan()
	l, err := authenticateOrFingerprint(lib, raceServices, "alice", waitingForAPassword(func() { close(scan.matched) }), scan.scanner, diskKey("luks passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(l.TakeSecret()); got != "luks passphrase" {
		t.Errorf("the secret is %q, want the disk passphrase that unlocks the keyring", got)
	}
}

func TestAFingerprintLoginWithoutADiskKeyLeavesTheKeyringLocked(t *testing.T) {
	lib, _, _ := racePAM()
	scan := newFakeScan()
	l, err := authenticateOrFingerprint(lib, raceServices, "alice", waitingForAPassword(func() { close(scan.matched) }), scan.scanner, diskKey(""))
	if err != nil {
		t.Fatal(err)
	}
	if l.HasSecret() {
		t.Errorf("a secret %q with no disk key; nothing may be handed to the keyring", l.TakeSecret())
	}
}

func TestAMatchBeforeThePromptNeverAsksForThePassword(t *testing.T) {
	lib, _, _ := racePAM()
	scan := newFakeScan()
	close(scan.matched)
	asked := 0
	l, err := authenticateOrFingerprint(lib, raceServices, "alice", func(p Prompt, withdrawn <-chan struct{}) (string, bool) {
		select {
		case <-withdrawn:
			return "", false
		default:
		}
		asked++
		return "pw", true
	}, scan.scanner, diskKey(""))
	if err != nil {
		t.Fatal(err)
	}
	if asked != 0 || l.Typed() {
		t.Errorf("asked %d times, typed %v; a match already in withdraws the prompt", asked, l.Typed())
	}
}

func TestAFingerprintLoginTheAccountRefusesIsRefused(t *testing.T) {
	lib, _, fingerprint := racePAM()
	fingerprint.acctRC = fakeAcctErr
	scan := newFakeScan()
	_, err := authenticateOrFingerprint(lib, raceServices, "alice", waitingForAPassword(func() { close(scan.matched) }), scan.scanner, diskKey("luks"))
	refused, ok := errors.AsType[*RefusedError](err)
	if !ok || refused.Refusal() != RefusalExpired {
		t.Fatalf("an expired account's fingerprint: %v, want its refusal", err)
	}
}

func TestAPasswordAnsweredBeforeTheMatchKeepsTheLaterPrompts(t *testing.T) {
	password := &fakePAM{password: "pw", batches: [][]pamMessage{
		{{style: pamPromptEchoOff, text: "Password: "}},
		{{style: pamPromptEchoOn, text: "OTP: "}},
	}}
	lib := &servicesPAM{byService: map[string]*fakePAM{raceServices.Password: password, raceServices.Fingerprint: {permit: true}}}
	scan := newFakeScan()
	var withdrawable []bool
	l, err := authenticateOrFingerprint(lib, raceServices, "alice", func(p Prompt, withdrawn <-chan struct{}) (string, bool) {
		withdrawable = append(withdrawable, withdrawn != nil)
		if p.Kind == PromptSecret {
			close(scan.matched)
			return "pw", true
		}
		return "123456", true
	}, scan.scanner, diskKey(""))
	if err != nil {
		t.Fatal(err)
	}
	if !l.Typed() || !slices.Equal(withdrawable, []bool{true, false}) {
		t.Errorf("typed %v, withdrawable prompts %v; after the password the race is over", l.Typed(), withdrawable)
	}
	if !slices.Equal(lib.started, []string{raceServices.Password}) {
		t.Errorf("transactions %v, want the password's alone", lib.started)
	}
}

func TestACancelledPasswordWithNoMatchLogsNobodyIn(t *testing.T) {
	lib, _, _ := racePAM()
	scan := newFakeScan()
	_, err := authenticateOrFingerprint(lib, raceServices, "alice", func(Prompt, <-chan struct{}) (string, bool) { return "", false }, scan.scanner, diskKey("luks"))
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("a cancelled prompt: %v, want the cancellation", err)
	}
	if len(lib.started) != 0 || !scan.wasHalted() {
		t.Errorf("transactions %v, halted %v; a cancelled login starts none and stops the scan", lib.started, scan.wasHalted())
	}
}
