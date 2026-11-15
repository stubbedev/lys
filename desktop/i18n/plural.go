package i18n

import (
	"fmt"
	"slices"
)

// pluralCategory is a CLDR plural category, as a select variant key
// names it.
type pluralCategory string

const (
	pluralZero  pluralCategory = "zero"
	pluralOne   pluralCategory = "one"
	pluralTwo   pluralCategory = "two"
	pluralFew   pluralCategory = "few"
	pluralMany  pluralCategory = "many"
	pluralOther pluralCategory = "other"
)

// pluralCategories is every CLDR plural category.
var pluralCategories = []pluralCategory{pluralZero, pluralOne, pluralTwo, pluralFew, pluralMany, pluralOther}

// isPluralCategory reports whether a variant key names a CLDR plural
// category.
func isPluralCategory(key string) bool {
	return slices.Contains(pluralCategories, pluralCategory(key))
}

// pluralRules is one language's CLDR cardinal rules: the categories it
// has and the one a number falls in.
type pluralRules struct {
	categories []pluralCategory
	category   func(operands) pluralCategory
}

func (r pluralRules) has(c pluralCategory) bool { return slices.Contains(r.categories, c) }

// millions is CLDR's "e = 0 and i != 0 and i % 1000000 = 0 and v = 0":
// a number fluent formats never has a compact exponent, so e is 0.
func millions(o operands) bool { return o.i != 0 && o.i%1_000_000 == 0 && o.v == 0 }

var (
	// oneOther is CLDR's ast de en et fi fy ia ie io ji lij nl sc sv sw
	// ur yi: one is i = 1 and v = 0.
	oneOther = pluralRules{
		categories: []pluralCategory{pluralOne, pluralOther},
		category: func(o operands) pluralCategory {
			if o.i == 1 && o.v == 0 {
				return pluralOne
			}
			return pluralOther
		},
	}
	// belowTwoManyOther is CLDR's fr pt (pt-BR): one is i = 0..1.
	belowTwoManyOther = pluralRules{
		categories: []pluralCategory{pluralOne, pluralMany, pluralOther},
		category: func(o operands) pluralCategory {
			switch {
			case o.i <= 1:
				return pluralOne
			case millions(o):
				return pluralMany
			}
			return pluralOther
		},
	}
	// oneManyOther is CLDR's ca gl it lld pt_PT scn vec: one is i = 1
	// and v = 0.
	oneManyOther = pluralRules{
		categories: []pluralCategory{pluralOne, pluralMany, pluralOther},
		category: func(o operands) pluralCategory {
			switch {
			case o.i == 1 && o.v == 0:
				return pluralOne
			case millions(o):
				return pluralMany
			}
			return pluralOther
		},
	}
	// exactOneManyOther is CLDR's es: one is n = 1, so 1.0 is one too.
	exactOneManyOther = pluralRules{
		categories: []pluralCategory{pluralOne, pluralMany, pluralOther},
		category: func(o operands) pluralCategory {
			switch {
			case o.i == 1 && o.f == 0:
				return pluralOne
			case millions(o):
				return pluralMany
			}
			return pluralOther
		},
	}
)

// cardinalRules is the CLDR plurals.xml cardinal rule set of every
// catalogue lys ships, keyed by its locale directory. The rules follow
// the language the catalogue is written in: pt is European Portuguese,
// CLDR's pt_PT.
var cardinalRules = map[string]pluralRules{
	"de":    oneOther,
	"en-US": oneOther,
	"es":    exactOneManyOther,
	"fr":    belowTwoManyOther,
	"it":    oneManyOther,
	"nl":    oneOther,
	"pt":    oneManyOther,
}

// cardinalRulesFor is a catalogue's rule set; a catalogue without one
// is an error, never another language's rules.
func cardinalRulesFor(catalogue LangID) (pluralRules, error) {
	rules, ok := cardinalRules[catalogue.String()]
	if !ok {
		return pluralRules{}, fmt.Errorf("i18n: catalogue %s has no plural rule set", catalogue)
	}
	return rules, nil
}
