package settings

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type argHole string

const (
	holeNumber  argHole = "n"
	holeName    argHole = "name"
	holeID      argHole = "id"
	holeMillis  argHole = "ms"
	holeFactor  argHole = "factor"
	holeOutput  argHole = "output"
	holeCommand argHole = "command"
	holeAppID   argHole = "app_id"
	holeDX      argHole = "dx"
	holeDY      argHole = "dy"
	holeKey     argHole = "key"
	holeValue   argHole = "value"
)

type holeKind int

const (
	holeText holeKind = iota
	holeWhole
	holeSigned
	holeFraction
	holeRest
)

var holeKinds = map[argHole]holeKind{
	holeNumber: holeWhole, holeMillis: holeWhole, holeDX: holeSigned, holeDY: holeSigned, holeFactor: holeFraction,
	holeName: holeText, holeID: holeText, holeOutput: holeText, holeAppID: holeText, holeKey: holeText,
	holeCommand: holeRest, holeValue: holeRest,
}

var bareHoles = []argHole{holeNumber, holeName, holeID, holeMillis, holeFactor}

const pixelSuffix = "px"

var holePatterns = map[holeKind]string{
	holeText:     `[^\s:]+`,
	holeWhole:    `[0-9]+`,
	holeSigned:   `-?[0-9]+`,
	holeFraction: `[0-9]+(?:\.[0-9]+)?`,
}

type argPart struct {
	literal string
	hole    argHole
}

type argWord struct {
	parts    []argPart
	optional bool
}

func (w argWord) literal() (string, bool) {
	if len(w.parts) != 1 || w.parts[0].hole != "" {
		return "", false
	}
	return w.parts[0].literal, true
}

func (w argWord) rest() bool {
	return len(w.parts) == 1 && holeKinds[w.parts[0].hole] == holeRest
}

func (w argWord) holes() []argHole {
	var out []argHole
	for _, p := range w.parts {
		if p.hole != "" {
			out = append(out, p.hole)
		}
	}
	return out
}

type argAlt []argWord

type argSlot struct {
	optional bool
	alts     []argAlt
}

func (s argSlot) literals() ([]string, bool) {
	out := make([]string, 0, len(s.alts))
	for _, a := range s.alts {
		if len(a) != 1 || a[0].optional {
			return nil, false
		}
		lit, ok := a[0].literal()
		if !ok {
			return nil, false
		}
		out = append(out, lit)
	}
	return out, true
}

// altKeys name the slot's choices: its alternatives, after the omitted
// choice when it is optional.
func (s argSlot) altKeys() []string {
	keys := make([]string, 0, len(s.alts)+1)
	if s.optional {
		keys = append(keys, argOmitted)
	}
	for _, a := range s.alts {
		keys = append(keys, argAltKey(a))
	}
	return keys
}

type argGrammar []argSlot

var errArgGrammar = errors.New("the grammar has no form the editor builds")

func parseArgGrammar(text string) (argGrammar, error) {
	if strings.ContainsAny(text, ",") {
		return nil, fmt.Errorf("%q: %w", text, errArgGrammar)
	}
	fields, err := splitDepth(text, ' ')
	if err != nil {
		return nil, fmt.Errorf("%q: %w", text, err)
	}
	g := make(argGrammar, 0, len(fields))
	for _, f := range fields {
		slot, err := parseArgSlot(f)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", text, err)
		}
		g = append(g, slot)
	}
	for i, s := range g {
		for _, a := range s.alts {
			for j, w := range a {
				if w.rest() && (i != len(g)-1 || j != len(a)-1) {
					return nil, fmt.Errorf("%q: a rest hole before the end: %w", text, errArgGrammar)
				}
			}
		}
	}
	return g, nil
}

func enclosed(text string, open, closing byte) (string, bool) {
	if len(text) < 2 || text[0] != open || text[len(text)-1] != closing {
		return "", false
	}
	depth := 0
	for i := range len(text) {
		switch text[i] {
		case '<', '[':
			depth++
		case '>', ']':
			depth--
			if depth == 0 && i != len(text)-1 {
				return "", false
			}
		}
	}
	return text[1 : len(text)-1], true
}

func parseArgSlot(text string) (argSlot, error) {
	for _, brackets := range [][2]byte{{'[', ']'}, {'<', '>'}} {
		if inner, ok := enclosed(text, brackets[0], brackets[1]); ok {
			if _, hole := holeKinds[argHole(inner)]; hole {
				return argSlot{optional: brackets[0] == '[', alts: []argAlt{{{parts: []argPart{{hole: argHole(inner)}}}}}}, nil
			}
		}
	}
	if inner, ok := enclosed(text, '[', ']'); ok {
		alts, err := parseArgAlts(inner)
		return argSlot{optional: true, alts: alts}, err
	}
	if inner, ok := enclosed(text, '<', '>'); ok {
		alts, err := parseArgAlts(inner)
		if err == nil && len(alts) == 1 && len(alts[0]) == 1 {
			if _, literal := alts[0][0].literal(); literal {
				return argSlot{}, fmt.Errorf("the hole %q: %w", inner, errArgGrammar)
			}
		}
		return argSlot{alts: alts}, err
	}
	w, err := parseArgWord(text)
	return argSlot{alts: []argAlt{{w}}}, err
}

func parseArgAlts(text string) ([]argAlt, error) {
	choices, err := splitDepth(text, '|')
	if err != nil {
		return nil, err
	}
	alts := make([]argAlt, 0, len(choices))
	for _, c := range choices {
		words, err := splitDepth(c, ' ')
		if err != nil {
			return nil, err
		}
		alt := make(argAlt, 0, len(words))
		for _, w := range words {
			word, err := parseArgWord(w)
			if err != nil {
				return nil, err
			}
			alt = append(alt, word)
		}
		if len(alt) == 0 {
			return nil, fmt.Errorf("an empty choice: %w", errArgGrammar)
		}
		alts = append(alts, alt)
	}
	return alts, nil
}

func parseArgWord(text string) (argWord, error) {
	if inner, ok := enclosed(text, '[', ']'); ok {
		w, err := parseArgWord(inner)
		if err != nil || w.optional {
			return argWord{}, fmt.Errorf("%q: %w", text, errArgGrammar)
		}
		if len(w.holes()) == 0 {
			return argWord{}, fmt.Errorf("an optional word inside a choice that is no hole: %w", errArgGrammar)
		}
		w.optional = true
		return w, nil
	}
	if inner, ok := enclosed(text, '<', '>'); ok {
		if _, known := holeKinds[argHole(inner)]; !known {
			return argWord{}, fmt.Errorf("the hole %q: %w", inner, errArgGrammar)
		}
		return argWord{parts: []argPart{{hole: argHole(inner)}}}, nil
	}
	var parts []argPart
	appendLiteral := func(s string) {
		if n := len(parts); n > 0 && parts[n-1].hole == "" {
			parts[n-1].literal += s
			return
		}
		parts = append(parts, argPart{literal: s})
	}
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '<':
			end := strings.IndexByte(text[i:], '>')
			if end < 0 {
				return argWord{}, fmt.Errorf("%q: %w", text, errArgGrammar)
			}
			name := argHole(text[i+1 : i+end])
			if _, known := holeKinds[name]; !known {
				return argWord{}, fmt.Errorf("the hole %q: %w", name, errArgGrammar)
			}
			parts = append(parts, argPart{hole: name})
			i += end + 1
		case isIdent(c):
			j := i
			for j < len(text) && isIdent(text[j]) {
				j++
			}
			run := text[i:j]
			switch {
			case slices.Contains(bareHoles, argHole(run)):
				parts = append(parts, argPart{hole: argHole(run)})
			case run == string(holeNumber)+pixelSuffix:
				parts = append(parts, argPart{hole: holeNumber})
				appendLiteral(pixelSuffix)
			default:
				appendLiteral(run)
			}
			i = j
		case strings.IndexByte("[]<>|", c) >= 0:
			return argWord{}, fmt.Errorf("%q: %w", text, errArgGrammar)
		default:
			appendLiteral(string(c))
			i++
		}
	}
	if len(parts) == 0 {
		return argWord{}, fmt.Errorf("an empty word: %w", errArgGrammar)
	}
	if lit, ok := (argWord{parts: parts}).literal(); ok && lit == argOmitted {
		return argWord{}, fmt.Errorf("the word %q names the omitted choice: %w", lit, errArgGrammar)
	}
	return argWord{parts: parts}, nil
}

func isIdent(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func splitDepth(text string, sep byte) ([]string, error) {
	var out []string
	depth, start := 0, 0
	for i := range len(text) {
		switch text[i] {
		case '<', '[':
			depth++
		case '>', ']':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("an unbalanced %q: %w", text, errArgGrammar)
			}
		case sep:
			if depth == 0 {
				if part := strings.TrimSpace(text[start:i]); part != "" {
					out = append(out, part)
				}
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("an unbalanced %q: %w", text, errArgGrammar)
	}
	if part := strings.TrimSpace(text[start:]); part != "" {
		out = append(out, part)
	}
	return out, nil
}

type wordValue struct {
	present bool
	holes   []string
}

type slotValue struct {
	alt   int
	words []wordValue
}

type argValues []slotValue

func (g argGrammar) format(v argValues) string {
	var words []string
	for i, s := range g {
		if v[i].alt < 0 {
			continue
		}
		for j, w := range s.alts[v[i].alt] {
			wv := v[i].words[j]
			if !wv.present {
				continue
			}
			var b strings.Builder
			h := 0
			for _, p := range w.parts {
				if p.hole == "" {
					b.WriteString(p.literal)
					continue
				}
				b.WriteString(wv.holes[h])
				h++
			}
			words = append(words, b.String())
		}
	}
	return strings.Join(words, " ")
}

func (w argWord) pattern() *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for _, p := range w.parts {
		if p.hole == "" {
			b.WriteString(regexp.QuoteMeta(p.literal))
			continue
		}
		b.WriteString("(" + holePatterns[holeKinds[p.hole]] + ")")
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func (w argWord) match(tokens []string) (wordValue, int, bool) {
	if w.rest() {
		if len(tokens) == 0 {
			return wordValue{}, 0, false
		}
		return wordValue{present: true, holes: []string{strings.Join(tokens, " ")}}, len(tokens), true
	}
	if len(tokens) == 0 {
		return wordValue{}, 0, false
	}
	m := w.pattern().FindStringSubmatch(tokens[0])
	if m == nil {
		return wordValue{}, 0, false
	}
	return wordValue{present: true, holes: m[1:]}, 1, true
}

func (a argAlt) match(tokens []string, next func(rest []string) bool) ([]wordValue, bool) {
	if len(a) == 0 {
		return nil, next(tokens)
	}
	w := a[0]
	if wv, n, ok := w.match(tokens); ok {
		if rest, ok := a[1:].match(tokens[n:], next); ok {
			return append([]wordValue{wv}, rest...), true
		}
	}
	if !w.optional {
		return nil, false
	}
	rest, ok := a[1:].match(tokens, next)
	if !ok {
		return nil, false
	}
	return append([]wordValue{blankWord(w)}, rest...), true
}

func blankWord(w argWord) wordValue {
	return wordValue{holes: make([]string, len(w.holes()))}
}

func (a argAlt) specificity() (literal int, typed int) {
	for _, w := range a {
		for _, p := range w.parts {
			switch {
			case p.hole == "":
				literal += len(p.literal)
			case holeKinds[p.hole] != holeText && holeKinds[p.hole] != holeRest:
				typed++
			}
		}
	}
	return literal, typed
}

func (s argSlot) order() []int {
	order := make([]int, len(s.alts))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(x, y int) int {
		lx, tx := s.alts[x].specificity()
		ly, ty := s.alts[y].specificity()
		if lx != ly {
			return ly - lx
		}
		return ty - tx
	})
	return order
}

func (g argGrammar) match(args string) (argValues, bool) {
	tokens := strings.Fields(args)
	out := make(argValues, len(g))
	var walk func(i int, tokens []string) bool
	walk = func(i int, tokens []string) bool {
		if i == len(g) {
			return len(tokens) == 0
		}
		s := g[i]
		for _, ai := range s.order() {
			words, ok := s.alts[ai].match(tokens, func(rest []string) bool { return walk(i+1, rest) })
			if ok {
				out[i] = slotValue{alt: ai, words: words}
				return true
			}
		}
		if s.optional && walk(i+1, tokens) {
			out[i] = slotValue{alt: -1}
			return true
		}
		return false
	}
	if !walk(0, tokens) {
		return nil, false
	}
	return out, true
}

func (g argGrammar) blank() argValues {
	out := make(argValues, len(g))
	for i, s := range g {
		if s.optional {
			out[i] = slotValue{alt: -1}
			continue
		}
		out[i] = s.choose(0)
	}
	return out
}

func (s argSlot) choose(alt int) slotValue {
	words := make([]wordValue, len(s.alts[alt]))
	for j, w := range s.alts[alt] {
		words[j] = blankWord(w)
		words[j].present = !w.optional
		for h, hole := range w.holes() {
			if w.optional {
				continue
			}
			words[j].holes[h] = holeStart(hole)
		}
	}
	return slotValue{alt: alt, words: words}
}

func holeStart(h argHole) string {
	switch holeKinds[h] {
	case holeWhole:
		return "1"
	case holeSigned:
		return "0"
	case holeFraction:
		return "1.5"
	}
	return ""
}

func argWordKey(w argWord) string {
	var parts []string
	for _, p := range w.parts {
		if p.hole != "" {
			parts = append(parts, string(p.hole))
			continue
		}
		var b strings.Builder
		for _, c := range p.literal {
			if name, ok := argSymbols[c]; ok {
				if b.Len() > 0 {
					parts = append(parts, b.String())
					b.Reset()
				}
				parts = append(parts, name)
				continue
			}
			b.WriteRune(c)
		}
		if b.Len() > 0 {
			parts = append(parts, b.String())
		}
	}
	return strings.Join(parts, "-")
}

var argSymbols = map[rune]string{'+': "plus", '-': "minus", '%': "percent", ':': "colon"}

func argAltKey(a argAlt) string {
	keys := make([]string, len(a))
	for i, w := range a {
		keys[i] = argWordKey(w)
	}
	return strings.Join(keys, "-")
}
