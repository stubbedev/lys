package screen

import (
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/login/internal/fprint"
)

func FingerprintMessage(r fprint.Result) string {
	switch r {
	case fprint.NoMatch:
		return i18n.T("lock-fingerprint-no-match")
	case fprint.Retry:
		return i18n.T("lock-fingerprint-retry")
	case fprint.Match, fprint.Listening, fprint.Stopped:
	}
	return ""
}

type FingerprintReader struct {
	listening, password bool
	show                func()
}

func NewFingerprintReader(show func()) *FingerprintReader {
	return &FingerprintReader{show: show}
}

func (f *FingerprintReader) Shown() bool { return f.listening && !f.password }

func (f *FingerprintReader) Follow(r fprint.Result) (scanned bool) {
	switch r {
	case fprint.Listening:
		f.listen(true)
		return false
	case fprint.Stopped:
		f.listen(false)
		return false
	case fprint.Match, fprint.NoMatch, fprint.Retry:
	}
	return true
}

func (f *FingerprintReader) Stop() { f.listen(false) }

func (f *FingerprintReader) ChoosePassword() {
	f.password = true
	f.show()
}

func (f *FingerprintReader) Reset() {
	f.listening, f.password = false, false
}

func (f *FingerprintReader) listen(on bool) {
	if f.listening == on {
		return
	}
	f.listening = on
	f.show()
}
