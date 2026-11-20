package launchermatch

import (
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/config"
)

// OptionsOf are the matching options launcher configures.
func OptionsOf(l config.LauncherConfig) Options {
	return Options{
		Method:       MethodOf(l.Matching),
		Case:         caseOf(l.Case),
		Tokenize:     l.Tokenize,
		Normalize:    l.NormalizeMatch,
		NegationChar: NegationCharOf(l.NegateChar, DefaultOptions().NegationChar),
		Sort:         l.Sort,
		SortMethod:   SortMethodOf(string(l.SortingMethod)),
	}
}

// MethodOf is the method a matching name selects; a name lys-shell does
// not know is MatchNormal, as rofi's -matching.
func MethodOf(m config.LauncherMatching) MatchMethod {
	switch m {
	case config.MatchingFuzzy:
		return MatchFuzzy
	case config.MatchingPrefix:
		return MatchPrefix
	case config.MatchingRegex:
		return MatchRegex
	case config.MatchingGlob:
		return MatchGlob
	default:
		return MatchNormal
	}
}

// SortMethodOf is the sort a sorting name selects: fzf and fzf-v2 rank
// by match quality, any other name by edit distance, as rofi's
// -sorting-method.
func SortMethodOf(name string) SortMethod {
	switch name {
	case string(config.SortingFzf), "fzf-v2":
		return SortFzf
	default:
		return SortLevenshtein
	}
}

// NegationCharOf is the first character of s, or unset when s is empty.
func NegationCharOf(s string, unset rune) rune {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return unset
	}
	return r
}

func caseOf(c config.LauncherCase) CaseMode {
	switch c {
	case config.CaseSmart:
		return CaseModeSmart
	case config.CaseSensitive:
		return CaseModeSensitive
	default:
		return CaseModeInsensitive
	}
}
