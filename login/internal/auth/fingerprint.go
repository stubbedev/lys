package auth

import "sync"

type Scanner func(user string) (matched <-chan struct{}, halt func())

type FingerprintServices struct {
	Password, Fingerprint string
}

const passwordPrompt = "Password: "

func AuthenticateOrFingerprint(services FingerprintServices, user string, ask AskUntil, scan Scanner) (*Login, error) {
	return authenticateOrFingerprint(systemPAM{}, services, user, ask, scan, diskPassphrase)
}

func authenticateOrFingerprint(lib pamLib, services FingerprintServices, user string, ask AskUntil, scan Scanner, disk diskPassphraseFunc) (*Login, error) {
	matched, halt := scan(user)
	halted := make(chan struct{})
	stop := sync.OnceFunc(func() {
		go func() {
			halt()
			close(halted)
		}()
	})
	defer func() {
		stop()
		<-halted
	}()
	password, ok := ask(Prompt{Kind: PromptSecret, Text: passwordPrompt}, matched)
	if !ok {
		select {
		case <-matched:
			return passwordless(lib, services.Fingerprint, user, disk)
		default:
			return nil, ErrCancelled
		}
	}
	stop()
	typed := &password
	return authenticate(lib, services.Password, user, &pamConversation{ask: func(p Prompt) (string, bool) {
		if typed == nil || !p.WantsInput() {
			return ask(p, nil)
		}
		answer := *typed
		typed = nil
		return answer, true
	}})
}
