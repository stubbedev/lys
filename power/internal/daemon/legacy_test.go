package daemon

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/power/internal/config"
)

type legacyFixture struct {
	history string
	apm     string
	bash    string
}

func newLegacyFixture(t *testing.T) legacyFixture {
	t.Helper()
	root := t.TempDir()
	return legacyFixture{
		history: config.HistoryIn(filepath.Join(root, "lys-power")),
		apm:     filepath.Join(root, "adaptive-power-manager"),
		bash:    filepath.Join(root, "power-source"),
	}
}

func (f legacyFixture) seed(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.HistoryIn(dir), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f legacyFixture) adopt() {
	adoptLegacyHistory(f.history, []string{f.apm, f.bash}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (f legacyFixture) want(t *testing.T, content string) {
	t.Helper()
	got, err := os.ReadFile(f.history)
	if err != nil {
		t.Fatalf("reading the adopted history: %v", err)
	}
	if string(got) != content {
		t.Errorf("history = %q, want %q", got, content)
	}
}

func TestAdoptsTheAdaptivePowerManagerHistory(t *testing.T) {
	t.Parallel()
	f := newLegacyFixture(t)
	f.seed(t, f.apm, "1 1020\n")

	f.adopt()

	f.want(t, "1 1020\n")
	if _, err := os.Stat(config.HistoryIn(f.apm)); err != nil {
		t.Errorf("the legacy history is gone (%v), want it left in place on a read-only /var/lib", err)
	}
}

func TestAdoptsTheBashPolicyHistory(t *testing.T) {
	t.Parallel()
	f := newLegacyFixture(t)
	f.seed(t, f.bash, "2 960\n")

	f.adopt()

	f.want(t, "2 960\n")
}

func TestAdoptsTheNewestPredecessorFirst(t *testing.T) {
	t.Parallel()
	f := newLegacyFixture(t)
	f.seed(t, f.apm, "1 1020\n")
	f.seed(t, f.bash, "2 960\n")

	f.adopt()

	f.want(t, "1 1020\n")
}

func TestAnExistingHistoryIsNeverReplaced(t *testing.T) {
	t.Parallel()
	f := newLegacyFixture(t)
	f.seed(t, filepath.Dir(f.history), "3 600\n")
	f.seed(t, f.apm, "1 1020\n")

	f.adopt()

	f.want(t, "3 600\n")
}

func TestNothingToAdoptLeavesNoHistory(t *testing.T) {
	t.Parallel()
	f := newLegacyFixture(t)

	f.adopt()

	if _, err := os.Lstat(f.history); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("history exists (%v) without a predecessor, want none", err)
	}
}

func TestAnUnreadableLegacyHistoryIsNotAdopted(t *testing.T) {
	t.Parallel()
	f := newLegacyFixture(t)
	if err := os.MkdirAll(config.HistoryIn(f.apm), 0o700); err != nil {
		t.Fatal(err)
	}
	f.seed(t, f.bash, "2 960\n")

	f.adopt()

	if _, err := os.Lstat(f.history); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("history exists (%v) after the newest predecessor's failed to read, want none", err)
	}
}
