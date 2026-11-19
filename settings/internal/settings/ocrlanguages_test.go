package settings

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const ocrPath = "modules.screenshot.ocr-languages"

func TestTesseractsLanguageListIsReadWithoutItsHeadingOrOSD(t *testing.T) {
	for out, want := range map[string][]string{
		"List of available languages in \"/usr/share/tessdata/\" (3):\ndeu\neng\nosd\n": {"deu", "eng"},
		"eng\nfra\n": {"eng", "fra"},
		"List of available languages in \"/x/\" (0):\n": nil,
	} {
		if got := parseTesseractLangs(out); !slices.Equal(got, want) {
			t.Errorf("%q reads %q, want %q", out, got, want)
		}
	}
}

func ocrChecklist(t *testing.T, yaml, out string, err error) (*kit, *checklist) {
	t.Helper()
	was := tesseractListLangs
	tesseractListLangs = func() (string, error) { return out, err }
	t.Cleanup(func() { tesseractListLangs = was })
	k := testKit(t, yaml)
	c, ok := fieldControl(k, k.store, pageRow(t, "screenshot", ocrPath)).(*checklist)
	if !ok {
		t.Fatal("the text recognition languages are not a checklist")
	}
	return k, c
}

func TestTheOCRLanguagesArePickedFromTheInstalledOnes(t *testing.T) {
	k, c := ocrChecklist(t, "", "List of available languages in \"/t/\" (3):\ndeu\neng\nosd\n", nil)
	deu, eng := switchNamed(t, c, "deu"), switchNamed(t, c, "eng")
	if deu.On() || eng.On() || !deu.Enabled() {
		t.Fatal("with none chosen a language is on or fixed")
	}
	deu.Toggle()
	switchNamed(t, c, "eng").Toggle()
	if got := k.store.value(ocrPath); !reflect.DeepEqual(got, []any{"deu", "eng"}) {
		t.Fatalf("choosing German and English stored %#v", got)
	}
	switchNamed(t, c, "deu").Toggle()
	switchNamed(t, c, "eng").Toggle()
	if got := k.store.value(ocrPath); !reflect.DeepEqual(got, []any{}) {
		t.Errorf("clearing both stored %#v, want none, which follows the session", got)
	}
}

func TestAChosenOCRLanguageThatIsNotInstalledSaysSo(t *testing.T) {
	_, c := ocrChecklist(t, "modules:\n  screenshot:\n    ocr-languages: [jpn]\n", "eng\n", nil)
	if !switchNamed(t, c, "jpn").On() || !slices.Contains(treetest.Labels(c), locale.T().Get("settings-ocr-not-installed")) {
		t.Errorf("the missing language reads %q", treetest.Labels(c))
	}
}

func TestWithoutTesseractThePageSaysTextCannotBeRead(t *testing.T) {
	_, c := ocrChecklist(t, "", "", errNoTesseract)
	labels := treetest.Labels(c)
	if !slices.Contains(labels, locale.T().Get("settings-ocr-no-tesseract")) || !slices.Contains(labels, locale.T().Get("settings-ocr-none")) {
		t.Errorf("without tesseract the list reads %q", labels)
	}
	_, c = ocrChecklist(t, "", "", errors.New("tesseract --list-langs: exit status 1"))
	if !slices.Contains(treetest.Labels(c), "tesseract --list-langs: exit status 1") {
		t.Errorf("a failing tesseract reads %q", treetest.Labels(c))
	}
}
