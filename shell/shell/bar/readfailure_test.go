package bar

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/ocr"

	"github.com/stubbedev/lys/shell/shell/screenshot"
)

func TestAFailedReadingToastsWhatWentWrong(t *testing.T) {
	for _, c := range []struct {
		err error
		id  string
	}{
		{fmt.Errorf("read: %w", ocr.ErrBlank), "screenshot-read-blank"},
		{ocr.ErrNotText, "screenshot-read-no-text"},
		{screenshot.ErrNoCode, "screenshot-read-no-code"},
		{errors.New("cannot copy the text"), "screenshot-read-failed"},
	} {
		want := i18n.T(c.id)
		if strings.HasPrefix(want, "No localization") {
			t.Fatalf("%s is not in the catalogue", c.id)
		}
		if got := readFailure(c.err); got != want {
			t.Errorf("readFailure(%v) = %q, want %s (%q)", c.err, got, c.id, want)
		}
	}
}
