package settings

import (
	"fmt"
	"os"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/lyswm/lyswmtest"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/settings/internal/locale"
	"golang.org/x/sys/unix"
)

const noHostDir = "/nonexistent/lys-settings-test"

var messageLocaleEnv = [...]string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"}

// TestMain runs the package against a scratch home and no session:
// building a theme or a config reads, and the shell scaffolds, the
// user's config dir, and a test that wants a compositor starts its own.
// It also fixes the app's language to the en-US fallback before any
// test runs: every loader negotiates the desktop locale once, so a test
// that sets LC_ALL for a time locale would otherwise pick it for the
// rest of the run.
func TestMain(m *testing.M) {
	for _, key := range []string{"XDG_CONFIG_DIRS", "XDG_DATA_DIRS"} {
		if err := os.Setenv(key, noHostDir); err != nil {
			panic(err)
		}
	}
	speakTheFallbackLanguage()
	if err := lyswmtest.Build(); err != nil {
		panic(err)
	}
	testhome.Run(m)
}

func speakTheFallbackLanguage() {
	for _, key := range messageLocaleEnv {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	for i, l := range messageLoaders() {
		if !speaksOnlyTheFallback(l) {
			panic(fmt.Sprintf("message loader %d negotiated %v with no locale set", i, l.Languages()))
		}
	}
}

func messageLoaders() []*i18n.Loader {
	return []*i18n.Loader{locale.T(), i18n.Shell(), i18n.Settings(), i18n.Greeter()}
}

func speaksOnlyTheFallback(l *i18n.Loader) bool {
	langs := l.Languages()
	return len(langs) == 1 && langs[0].Equal(i18n.Fallback)
}

func TestATestsTimeLocaleLeavesTheAppSpeakingTheFallback(t *testing.T) {
	for _, key := range messageLocaleEnv {
		t.Setenv(key, "de_DE.utf8")
	}
	for i, l := range messageLoaders() {
		if !speaksOnlyTheFallback(l) {
			t.Errorf("message loader %d speaks %v under a German locale, want only %v", i, l.Languages(), i18n.Fallback)
		}
	}
	if got := locale.T().Get("settings-search-placeholder"); got != "Search settings" {
		t.Errorf("the search placeholder under a German locale reads %q", got)
	}
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func TestTemporaryDirectoriesAreOnATmpfs(t *testing.T) {
	var fs unix.Statfs_t
	dir := t.TempDir()
	if err := unix.Statfs(dir, &fs); err != nil || fs.Type != unix.TMPFS_MAGIC {
		t.Fatalf("%s: statfs %v, type %#x, want a tmpfs", dir, err, fs.Type)
	}
}
