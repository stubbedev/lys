package launchermatch

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestTheDefaultLauncherConfigMatchesAsRofi(t *testing.T) {
	got := OptionsOf(config.DefaultsLauncher())
	want := Options{Method: MatchNormal, Case: CaseModeInsensitive, Tokenize: true, Normalize: true, NegationChar: '-', SortMethod: SortLevenshtein}
	if got != want {
		t.Errorf("the default launcher matches with %+v, want %+v", got, want)
	}
}

func TestEveryMatchingAndCaseTheLauncherNamesSelectsItsOwn(t *testing.T) {
	for name, want := range map[config.LauncherMatching]MatchMethod{
		config.MatchingNormal: MatchNormal, config.MatchingFuzzy: MatchFuzzy, config.MatchingPrefix: MatchPrefix,
		config.MatchingRegex: MatchRegex, config.MatchingGlob: MatchGlob,
	} {
		l := config.DefaultsLauncher()
		l.Matching = name
		if got := OptionsOf(l).Method; got != want {
			t.Errorf("launcher.matching %s selects method %d, want %d", name, got, want)
		}
	}
	for name, want := range map[config.LauncherCase]CaseMode{
		config.CaseInsensitive: CaseModeInsensitive, config.CaseSmart: CaseModeSmart, config.CaseSensitive: CaseModeSensitive,
	} {
		l := config.DefaultsLauncher()
		l.Case = name
		if got := OptionsOf(l).Case; got != want {
			t.Errorf("launcher.case %s selects %d, want %d", name, got, want)
		}
	}
	if got := MethodOf("levenshtein"); got != MatchNormal {
		t.Errorf("an unknown matching name selects %d, not normal matching", got)
	}
}

func TestTheConfiguredMatchingDecidesWhatAQueryFinds(t *testing.T) {
	apps := items("Terminal", "Text Editor", "Files")
	l := config.DefaultsLauncher()
	if got := matches(OptionsOf(l), apps, "trm"); len(got) != 0 {
		t.Errorf("normal matching finds %v for trm", got)
	}
	l.Matching = config.MatchingFuzzy
	if got := matches(OptionsOf(l), apps, "trm"); !slices.Equal(got, []uint32{0}) {
		t.Errorf("fuzzy matching finds %v for trm, want Terminal", got)
	}
	l.Matching = config.MatchingPrefix
	if got := matches(OptionsOf(l), apps, "ed"); len(got) != 0 {
		t.Errorf("prefix matching finds %v for ed", got)
	}
	l.Case = config.CaseSensitive
	if got := matches(OptionsOf(l), apps, "te"); len(got) != 0 {
		t.Errorf("case-sensitive matching finds %v for te", got)
	}
	if got := matches(OptionsOf(l), apps, "Te"); !slices.Equal(got, []uint32{0, 1}) {
		t.Errorf("case-sensitive matching finds %v for Te, want Terminal and Text Editor", got)
	}
}

func TestTheNegationCharIsTheFirstCharacterOrTheUnsetOne(t *testing.T) {
	if got := NegationCharOf("!x", '-'); got != '!' {
		t.Errorf("!x negates with %q", got)
	}
	if got := NegationCharOf("", '-'); got != '-' {
		t.Errorf("an empty negate-char negates with %q, want the unset -", got)
	}
}

func TestSortingNamesSelectRofisSorts(t *testing.T) {
	for name, want := range map[string]SortMethod{"fzf": SortFzf, "fzf-v2": SortFzf, "levenshtein": SortLevenshtein, "normal": SortLevenshtein} {
		if got := SortMethodOf(name); got != want {
			t.Errorf("sorting %q selects %d, want %d", name, got, want)
		}
	}
}
