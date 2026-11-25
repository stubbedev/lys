package bar

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/testhome"
)

const quietWindow = 200 * time.Millisecond

// TestMain runs the package against a scratch home: building a theme
// or a config reads, and the shell scaffolds, the user's config dir.
// The scratch home reads en-US whatever the host's locale is, so a
// test's English holds on any desktop.
func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func TestTheTestsReadTheShellInEnglishWhateverTheHostSpeaks(t *testing.T) {
	if langs := i18n.Shell().Languages(); len(langs) == 0 || !langs[0].Equal(i18n.Fallback) {
		t.Errorf("the shell reads %v, want %s first", langs, i18n.Fallback)
	}
	if got := i18n.Shell().ByteSize(1536); got != "1.5 KiB" {
		t.Errorf("a byte size reads %q, want en-US's 1.5 KiB", got)
	}
}
