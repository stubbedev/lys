package i18n

import (
	"errors"
	"slices"
	"strings"
)

// LangID is a Unicode language identifier as unic-langid 0.9 parses
// it: a lowercase language ("" is und), an optional titlecase script,
// an optional uppercase region, and sorted, deduplicated variants.
type LangID struct {
	language string
	script   string
	region   string
	variants []string
}

var errLangID = errors.New("i18n: invalid language identifier")

// ParseLangID ports unic-langid's parse_language_identifier: subtags
// split on '-' or '_', case-normalized; anything past the variants
// (an extension, an empty subtag) is an error.
func ParseLangID(s string) (LangID, error) {
	tags := splitSubtags(s)
	var id LangID
	lang, ok := parseLanguage(tags[0])
	if !ok {
		return LangID{}, errLangID
	}
	id.language = lang
	position := 1
	for _, tag := range tags[1:] {
		script, isScript := parseScript(tag)
		region, isRegion := parseRegion(tag)
		variant, isVariant := parseVariant(tag)
		switch {
		case position == 1 && isScript:
			id.script, position = script, 2
		case position <= 2 && isRegion:
			id.region, position = region, 3
		case isVariant:
			id.variants, position = append(id.variants, variant), 3
		default:
			return LangID{}, errLangID
		}
	}
	if len(id.variants) > 0 {
		slices.Sort(id.variants)
		id.variants = slices.Compact(id.variants)
	}
	return id, nil
}

// MustLangID parses a compile-time identifier.
func MustLangID(s string) LangID {
	id, err := ParseLangID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// splitSubtags splits on '-' and '_', keeping empty subtags.
func splitSubtags(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "_", "-"), "-")
}

func allASCII(s string, pred func(byte) bool) bool {
	for i := range len(s) {
		if !pred(s[i]) {
			return false
		}
	}
	return true
}

func isAlnum(b byte) bool { return isAlpha(b) || isDigit(b) }

func parseLanguage(s string) (string, bool) {
	if len(s) < 2 || len(s) > 8 || len(s) == 4 || !allASCII(s, isAlpha) {
		return "", false
	}
	lower := strings.ToLower(s)
	if lower == "und" {
		return "", true
	}
	return lower, true
}

func parseScript(s string) (string, bool) {
	if len(s) != 4 || !allASCII(s, isAlpha) {
		return "", false
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:]), true
}

func parseRegion(s string) (string, bool) {
	switch {
	case len(s) == 2 && allASCII(s, isAlpha):
		return strings.ToUpper(s), true
	case len(s) == 3 && allASCII(s, isDigit):
		return s, true
	}
	return "", false
}

// parseVariant keeps unic-langid's exact 4-byte test: it rejects only a
// subtag that neither starts with a digit nor is alphanumeric after
// the first byte.
func parseVariant(s string) (string, bool) {
	if len(s) < 4 || len(s) > 8 || !allASCII(s, func(b byte) bool { return b < 0x80 }) {
		return "", false
	}
	if len(s) >= 5 && !allASCII(s, isAlnum) {
		return "", false
	}
	if len(s) == 4 && !isDigit(s[0]) && !allASCII(s[1:], isAlnum) {
		return "", false
	}
	return strings.ToLower(s), true
}

// String renders the identifier canonically ("und" for no language).
func (id LangID) String() string {
	parts := []string{id.Language()}
	if id.script != "" {
		parts = append(parts, id.script)
	}
	if id.region != "" {
		parts = append(parts, id.region)
	}
	parts = append(parts, id.variants...)
	return strings.Join(parts, "-")
}

// Language is the language subtag, "und" when unset.
func (id LangID) Language() string {
	if id.language == "" {
		return "und"
	}
	return id.language
}

// Equal reports whether both identifiers carry the same subtags.
func (id LangID) Equal(other LangID) bool {
	return id.language == other.language && id.script == other.script &&
		id.region == other.region && slices.Equal(id.variants, other.variants)
}

// covers reports whether every subtag id sets is want's: a catalogue
// "pt" covers "pt-BR", a catalogue "de-AT" does not cover "de".
func (id LangID) covers(want LangID) bool {
	sub := func(have, asked string) bool { return have == "" || have == asked }
	return sub(id.language, want.language) && sub(id.script, want.script) &&
		sub(id.region, want.region) &&
		(len(id.variants) == 0 || slices.Equal(id.variants, want.variants))
}
