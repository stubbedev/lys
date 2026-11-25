package i18n

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"unicode"

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

// miscasedInitialisms are initialisms written as words, as the settings
// tree's English must never read them.
var miscasedInitialisms = []string{"Id", "Url", "Vpn", "Cpu", "Gpu", "Ram", "Dns", "Ip", "Usb", "Ssh", "Osd", "Ocr", "Qr"}

// The settings tree's English reads an initialism in capitals: "ID",
// never "Id".
func TestSettingsEnglishReadsInitialismsInCapitals(t *testing.T) {
	err := fs.WalkDir(settingslocales.FS, "en-US", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".ftl") {
			return err
		}
		data, err := fs.ReadFile(settingslocales.FS, path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			_, text, ok := strings.Cut(line, "=")
			if !ok || strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, word := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) }) {
				if slices.Contains(miscasedInitialisms, word) {
					t.Errorf("%s:%d reads %q: %s", path, i+1, word, line)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
