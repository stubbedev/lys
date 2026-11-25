package bar

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/testhome"
)

const quietWindow = 200 * time.Millisecond

// pinnedLanguageEnv marks a test process whose shell language is
// already chosen: by this package's TestMain, or by a test starting the
// binary again in a language of its own.
const pinnedLanguageEnv = "LYS_SHELL_TEST_LANGUAGE_PINNED"

// TestMain runs the package against a scratch home: building a theme
// or a config reads, and the shell scaffolds, the user's config dir.
// The shell reads en-US whatever the host's locale is, so a test's
// English holds on any desktop; a process a test started keeps the
// language its parent gave it.
func TestMain(m *testing.M) {
	if err := pinLanguage(); err != nil {
		fmt.Fprintln(os.Stderr, "bar tests:", err)
		os.Exit(1)
	}
	testhome.Run(m)
}

// pinLanguage puts the shell in en-US unless a parent already chose.
func pinLanguage() error {
	if os.Getenv(pinnedLanguageEnv) != "" {
		return nil
	}
	for _, kv := range [...][2]string{{"LANGUAGE", "en_US"}, {pinnedLanguageEnv, "1"}} {
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			return fmt.Errorf("pin the shell language, set %s: %w", kv[0], err)
		}
	}
	return nil
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func TestTheTestsReadTheShellInEnglishWhateverTheHostSpeaks(t *testing.T) {
	if langs := i18n.Shell().Languages(); len(langs) == 0 || !langs[0].Equal(i18n.Fallback) {
		t.Errorf("the shell reads %v, want %s first", langs, i18n.Fallback)
	}
	if got := i18n.Shell().ByteSize(1536); got != "1.5 KiB" {
		t.Errorf("a byte size reads %q, want en-US's 1.5 KiB", got)
	}
}

func TestAHostLanguageGivesWayToEnglish(t *testing.T) {
	t.Setenv(pinnedLanguageEnv, "")
	t.Setenv("LANGUAGE", "de_DE")
	if err := pinLanguage(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("LANGUAGE"); got != "en_US" {
		t.Errorf("LANGUAGE = %q after pinning on a German host, want en_US", got)
	}
}

func TestAParentsLanguageChoiceIsKept(t *testing.T) {
	t.Setenv(pinnedLanguageEnv, "1")
	t.Setenv("LANGUAGE", "fr")
	if err := pinLanguage(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("LANGUAGE"); got != "fr" {
		t.Errorf("LANGUAGE = %q after pinning in a child, want the parent's fr", got)
	}
}
