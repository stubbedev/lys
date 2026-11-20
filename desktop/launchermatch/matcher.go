// Package launchermatch is the launcher's matcher: rofi's matching
// methods, case handling and sorting over a port of nucleo's fzf scoring,
// the options launcher configures, and the fuzzy query searches compile
// to. lys-shell filters its lists with it and lys-settings' launcher
// preview matches its test query.
package launchermatch

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/stubbedev/lys/desktop/glob"
)

// MatchMethod is rofi's -matching.
type MatchMethod uint8

// Matching methods.
const (
	// MatchNormal is tokenized substring matching (rofi's default).
	MatchNormal MatchMethod = iota
	// MatchFuzzy is fzf-style fuzzy matching.
	MatchFuzzy
	// MatchPrefix is tokenized prefix matching.
	MatchPrefix
	// MatchRegex treats the whole query as a regular expression.
	MatchRegex
	// MatchGlob is tokenized glob matching (`*token*` per token).
	MatchGlob
)

// CaseMode is rofi's case handling (-case-sensitive and -case-smart
// collapsed).
type CaseMode uint8

// Case modes.
const (
	// CaseModeInsensitive always folds case.
	CaseModeInsensitive CaseMode = iota
	// CaseModeSmart is sensitive only when the query has an uppercase
	// char.
	CaseModeSmart
	// CaseModeSensitive never folds.
	CaseModeSensitive
)

// SortMethod is rofi's -sorting-method.
type SortMethod uint8

// Sort methods.
const (
	// SortFzf ranks by match quality (the fzf-style score).
	SortFzf SortMethod = iota
	// SortLevenshtein ranks by edit distance to the query.
	SortLevenshtein
)

// Options are the matching knobs.
type Options struct {
	Method MatchMethod
	Case   CaseMode
	// Tokenize splits the query into independently matched words.
	Tokenize bool
	// Normalize strips accents while matching.
	Normalize bool
	// NegationChar prefixes a token that must not match
	// (-matching-negate-char).
	NegationChar rune
	// Sort ranks results by match quality; off keeps list order (rofi's
	// default).
	Sort       bool
	SortMethod SortMethod
}

// DefaultOptions are the matching knobs without any flag or config.
func DefaultOptions() Options {
	return Options{Tokenize: true, Normalize: true, NegationChar: '-', SortMethod: SortFzf}
}

// Engine filters and ranks a list's texts against the query. The
// fzf-syntax methods (normal, fuzzy, prefix) rewrite the query into
// nucleo's pattern language and score every text; regex and glob scan.
// It is synchronous: the caller runs it per keystroke, which for
// launcher-sized lists is cheaper than nucleo's worker threads.
type Engine struct {
	texts     []string
	hay       [][]rune
	permanent []uint32
	opts      Options
	query     string
	matched   []uint32
}

// NewEngine builds an engine with no texts.
func NewEngine(opts Options) *Engine {
	return &Engine{opts: opts}
}

// SetTexts replaces the list (a mode load or reload) and re-runs the
// query. permanent are the indices kept after the matches whatever the
// query.
func (e *Engine) SetTexts(texts []string, permanent []uint32) {
	e.texts = texts
	e.hay = make([][]rune, len(texts))
	for i, text := range texts {
		e.hay[i] = graphemeRunes(text)
	}
	e.permanent = permanent
	e.SetQuery(e.query)
}

// SetQuery updates the query and recomputes the matches.
func (e *Engine) SetQuery(query string) {
	e.query = query
	switch e.opts.Method {
	case MatchRegex:
		e.matched = e.scanRegex()
	case MatchGlob:
		e.matched = e.scanGlob()
	default:
		e.matched = e.scanPattern()
	}
}

// Matched returns the ranked matched indices plus the trailing
// permanent ones the filter dropped.
func (e *Engine) Matched() []uint32 {
	out := slices.Clone(e.matched)
	if e.opts.Sort && e.opts.SortMethod == SortLevenshtein {
		slices.SortStableFunc(out, func(a, b uint32) int {
			return levenshtein(e.query, e.texts[a]) - levenshtein(e.query, e.texts[b])
		})
	}
	for _, p := range e.permanent {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// caseSensitive resolves the case mode against the query.
func (e *Engine) caseSensitive() bool {
	switch e.opts.Case {
	case CaseModeSensitive:
		return true
	case CaseModeSmart:
		return strings.IndexFunc(e.query, unicode.IsUpper) >= 0
	default:
		return false
	}
}

// scanPattern runs the nucleo path: every item scored against the
// rewritten pattern, in list order unless sorting by quality, where
// nucleo's order is score, then shorter text, then list order.
func (e *Engine) scanPattern() []uint32 {
	atoms := parsePattern(rewriteQuery(e.query, e.opts), !e.caseSensitive(), e.opts.Normalize)
	type hit struct {
		idx   uint32
		score int
	}
	hits := make([]hit, 0, len(e.texts))
	for i, hay := range e.hay {
		if s, ok := patternScore(atoms, hay); ok {
			hits = append(hits, hit{idx: uint32(i), score: s})
		}
	}
	if e.opts.Sort && e.query != "" {
		slices.SortStableFunc(hits, func(a, b hit) int {
			if a.score != b.score {
				return b.score - a.score
			}
			if la, lb := len(e.hay[a.idx]), len(e.hay[b.idx]); la != lb {
				return la - lb
			}
			return int(a.idx) - int(b.idx)
		})
	}
	out := make([]uint32, len(hits))
	for i, h := range hits {
		out[i] = h.idx
	}
	return out
}

// rewriteQuery turns a rofi query into nucleo's fzf pattern syntax:
// normal -> 'substring atoms, prefix -> ^prefix atoms, fuzzy -> the
// query untouched. Tokens led by the negation char become ! atoms.
// With tokenize off the whole query is one atom, spaces escaped.
func rewriteQuery(query string, opts Options) string {
	if query == "" {
		return ""
	}
	kind := ""
	switch opts.Method {
	case MatchNormal:
		kind = "'"
	case MatchPrefix:
		kind = "^"
	}
	token := func(tok string) string {
		negated, body := false, tok
		if rest, ok := strings.CutPrefix(tok, string(opts.NegationChar)); ok && rest != "" {
			negated, body = true, rest
		}
		out := kind + body
		if negated {
			out = "!" + out
		}
		return out
	}
	if !opts.Tokenize {
		return token(strings.ReplaceAll(query, " ", `\ `))
	}
	fields := strings.Fields(query)
	for i, f := range fields {
		fields[i] = token(f)
	}
	return strings.Join(fields, " ")
}

func (e *Engine) all() []uint32 {
	out := make([]uint32, len(e.texts))
	for i := range out {
		out[i] = uint32(i)
	}
	return out
}

// scanRegex matches the whole query as a regular expression. An
// invalid one (a trailing '[' while typing) matches nothing, as rofi.
func (e *Engine) scanRegex() []uint32 {
	if e.query == "" {
		return e.all()
	}
	expr := e.query
	if !e.caseSensitive() {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil
	}
	var out []uint32
	for i, text := range e.texts {
		if re.MatchString(text) {
			out = append(out, uint32(i))
		}
	}
	return out
}

// scanGlob matches each token as `*token*` (`*` crossing separators); a malformed token matches nothing.
func (e *Engine) scanGlob() []uint32 {
	if e.query == "" {
		return e.all()
	}
	sensitive := e.caseSensitive()
	tokens := []string{e.query}
	if e.opts.Tokenize {
		tokens = strings.Fields(e.query)
	}
	type pattern struct {
		negated bool
		p       glob.Pattern
	}
	patterns := make([]pattern, 0, len(tokens))
	for _, tok := range tokens {
		negated, body := false, tok
		if rest, ok := strings.CutPrefix(tok, string(e.opts.NegationChar)); ok && rest != "" {
			negated, body = true, rest
		}
		// Wrap for substring semantics without forming `**`, which
		// glob.Compile rejects outside a path component.
		lead, trail := "*", "*"
		if strings.HasPrefix(body, "*") {
			lead = ""
		}
		if strings.HasSuffix(body, "*") {
			trail = ""
		}
		p, ok := glob.Compile(lead + body + trail)
		if !ok {
			return nil
		}
		patterns = append(patterns, pattern{negated: negated, p: p})
	}
	var out []uint32
	for i, text := range e.texts {
		keep := true
		for _, p := range patterns {
			if p.p.Matches(text, sensitive) == p.negated {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, uint32(i))
		}
	}
	return out
}

// levenshtein is the edit distance over chars (two-row DP).
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ca := range ar {
		cur[0] = i + 1
		for j, cb := range br {
			cost := 1
			if ca == cb {
				cost = 0
			}
			cur[j+1] = min(prev[j]+cost, prev[j+1]+1, cur[j]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}
