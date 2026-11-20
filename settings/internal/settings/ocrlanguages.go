package settings

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	tesseractBinary     = "tesseract"
	tesseractTimeout    = 5 * time.Second
	tesseractOSD        = "osd"
	tesseractListHeader = "List of available languages"
)

var errNoTesseract = errors.New(tesseractBinary + " is not on PATH")

// tesseractListLangs is what `tesseract --list-langs` prints (a seam for
// tests).
var tesseractListLangs = func() (string, error) {
	if _, err := exec.LookPath(tesseractBinary); err != nil {
		return "", errNoTesseract
	}
	ctx, cancel := context.WithTimeout(context.Background(), tesseractTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, tesseractBinary, "--list-langs").Output()
	if err != nil {
		return "", fmt.Errorf("%s --list-langs: %w", tesseractBinary, err)
	}
	return string(out), nil
}

var tesseractScripts = map[string]string{"sim": "Hans", "tra": "Hant", "cyrl": "Cyrl", "latn": "Latn"}

func tesseractLangName(namer display.Namer, code string) string {
	parts := strings.Split(code, "_")
	tag := parts[0]
	if len(parts) > 1 {
		if script, ok := tesseractScripts[parts[1]]; ok {
			tag += "-" + script
		}
	}
	t, err := language.Parse(tag)
	if err != nil {
		return code
	}
	if name := namer.Name(t); name != "" {
		return name
	}
	return code
}

func parseTesseractLangs(out string) []string {
	var langs []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == tesseractOSD || strings.HasPrefix(line, tesseractListHeader) {
			continue
		}
		langs = append(langs, line)
	}
	return langs
}

var ocrLanguagesRow = checklistRow(checklistSpec{
	readingKey: "settings-ocr-reading", reloadKey: "settings-ocr-reload",
	missingKey: "settings-ocr-not-installed", emptyKey: "settings-ocr-none",
	icon:  "ld-type-symbolic",
	label: func(code string) string { return tesseractLangName(display.Tags(interfaceLanguage()), code) },
	read: func() checkListing {
		out, err := tesseractListLangs()
		if errors.Is(err, errNoTesseract) {
			return checkListing{err: errors.New(locale.T().Get("settings-ocr-no-tesseract"))}
		}
		if err != nil {
			return checkListing{err: err}
		}
		langs := parseTesseractLangs(out)
		items := make([]checkItem, len(langs))
		for i, l := range langs {
			items[i] = checkItem{value: l}
		}
		return checkListing{items: items}
	},
})
