package i18n

import (
	"io/fs"
	"strings"
	"testing"

	settingslocales "github.com/stubbedev/lys/desktop/i18n/locales/settings"
)

// lys-settings shows config descriptions as plain text, so a Markdown
// code span read as literal backticks ("`lys-shell lock`"). No message
// of the settings tree carries one; comments may.
func TestSettingsMessagesCarryNoMarkdownCodeSpans(t *testing.T) {
	err := fs.WalkDir(settingslocales.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".ftl") {
			return err
		}
		data, err := fs.ReadFile(settingslocales.FS, path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "`") {
				t.Errorf("%s:%d carries a code span: %s", path, i+1, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
