package preview

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
)

func TestByteSizeIsTheLocaleByteSize(t *testing.T) {
	for _, n := range []int64{0, 512, 1536, 3 << 30} {
		if got, want := ByteSize(n), i18n.Shell().ByteSize(uint64(n)); got != want {
			t.Errorf("ByteSize(%d) = %q, want the locale's %q", n, got, want)
		}
	}
	if got, want := ByteSize(-1), i18n.Shell().ByteSize(0); got != want {
		t.Errorf("ByteSize(-1) = %q, want a negative size clamped to %q", got, want)
	}
}

func TestModifiedIsTheLocaleDateTimeInLocalTime(t *testing.T) {
	if got := Modified(time.Time{}); got != "" {
		t.Errorf("no time = %q, want empty", got)
	}
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if got, want := Modified(at), i18n.Shell().DateTime(at.Local()); got != want {
		t.Errorf("Modified = %q, want the locale's local %q", got, want)
	}
}
