package settings

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// Search fields' placeholders ended in "..." in two places ("Search
// icons...", "Search modules...") and nowhere else. No search
// placeholder ends in an ellipsis, in any language.
func TestNoSearchPlaceholderEndsInAnEllipsis(t *testing.T) {
	available, err := locale.Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, lang := range available {
		src, err := locale.Assets().Source(lang.String())
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(src) {
			id, value, ok := strings.Cut(strings.TrimSpace(line), " = ")
			search := strings.HasSuffix(id, "-search") || strings.HasSuffix(id, "-search-placeholder")
			if !ok || !search || !strings.HasPrefix(id, "settings-") {
				continue
			}
			checked++
			if strings.HasSuffix(value, "...") || strings.HasSuffix(value, "…") {
				t.Errorf("%s: %s reads %q", lang, id, value)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no search placeholder was checked")
	}
}
