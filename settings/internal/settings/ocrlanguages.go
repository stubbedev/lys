package settings

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

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
	icon: "ld-type-symbolic",
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
