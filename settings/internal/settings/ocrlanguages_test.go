package settings

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

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

func TestATesseractLanguageReadsAsItsNameAndAnUnknownCodeAsItself(t *testing.T) {
	namer := display.Tags(language.English)
	for code, want := range map[string]string{
		"deu":          "German",
		"chi_sim":      "Simplified Chinese",
		"chi_tra_vert": "Traditional Chinese",
		"ita_old":      "Italian",
		"equ":          "equ",
		"frk":          "frk",
		"script/Latin": "script/Latin",
	} {
		if got := tesseractLangName(namer, code); got != want {
			t.Errorf("%s reads %q, want %q", code, got, want)
		}
	}
}

func TestTheOCRLanguagesAreListedByNameWithTheirCodeBeneath(t *testing.T) {
	_, c := ocrChecklist(t, "", "fra\ndeu\nequ\n", nil)
	labels := treetest.Labels(c)
	at := func(s string) int { return slices.Index(labels, s) }
	if at("French") < 0 || at("fra") != at("French")+1 || at("German") < at("French") {
		t.Errorf("the languages read %q, want them by name, French before German, each over its code", labels)
	}
	if at("equ") < 0 {
		t.Errorf("an unnamed code is missing from %q", labels)
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
	deu, eng := switchNamed(t, c, "German"), switchNamed(t, c, "English")
	if deu.On() || eng.On() || !deu.Enabled() {
		t.Fatal("with none chosen a language is on or fixed")
	}
	deu.Toggle()
	switchNamed(t, c, "English").Toggle()
	if got := k.store.value(ocrPath); !reflect.DeepEqual(got, []any{"deu", "eng"}) {
		t.Fatalf("choosing German and English stored %#v", got)
	}
	switchNamed(t, c, "German").Toggle()
	switchNamed(t, c, "English").Toggle()
	if got := k.store.value(ocrPath); !reflect.DeepEqual(got, []any{}) {
		t.Errorf("clearing both stored %#v, want none, which follows the session", got)
	}
}

func TestAChosenOCRLanguageThatIsNotInstalledSaysSo(t *testing.T) {
	_, c := ocrChecklist(t, "modules:\n  screenshot:\n    ocr-languages: [jpn]\n", "eng\n", nil)
	if !switchNamed(t, c, "Japanese").On() || !slices.Contains(treetest.Labels(c), locale.T().Get("settings-ocr-not-installed")) {
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
