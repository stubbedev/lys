package preview

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// mdKind is what a block of Markdown is.
type mdKind uint8

// The blocks: a heading (its level 1 to 6), a paragraph, a quote, a
// list item (its level the depth it nests at, from 0), a line of code
// or of a table, kept as written, and a rule.
const (
	mdHeading mdKind = iota + 1
	mdParagraph
	mdQuote
	mdItem
	mdCode
	mdRule
)

// mdBlock is one block of Markdown as it shows: its kind, its level,
// and its text as rich-label markup (a code line's as written, escaped;
// a list item's marker apart, in marker).
type mdBlock struct {
	kind   mdKind
	level  int
	marker string
	markup string
}

var (
	mdHeadingLine = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$`)
	mdFence       = regexp.MustCompile("^ {0,3}(```+|~~~+)")
	mdRuleLine    = regexp.MustCompile(`^ {0,3}(?:(?:\*[ \t]*){3,}|(?:-[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	mdItemLine    = regexp.MustCompile(`^([ \t]*)([-*+]|\d{1,9}[.)])[ \t]+(.*)$`)
	mdQuoteLine   = regexp.MustCompile(`^ {0,3}>[ \t]?(.*)$`)
	mdTableLine   = regexp.MustCompile(`^ {0,3}\|`)
)

// markdownBlocks lays src out as the blocks of CommonMark it shows:
// ATX headings, fenced code, rules, quotes, bulleted and numbered
// items, pipe tables kept as written, and paragraphs, each paragraph's
// lines joined. Inline emphasis, code spans, links and images become
// markup.
func markdownBlocks(src string) []mdBlock {
	var blocks []mdBlock
	var para []string
	var paraKind mdKind
	flush := func() {
		if len(para) == 0 {
			return
		}
		blocks = append(blocks, mdBlock{kind: paraKind, markup: inlineMarkup(strings.Join(para, " "))})
		para = nil
	}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := mdFence.FindStringSubmatch(line); m != nil {
			flush()
			fence := m[1]
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimLeft(lines[i], " "), fence); i++ {
				blocks = append(blocks, mdBlock{kind: mdCode, markup: escapeMarkup(expandTabs(lines[i]))})
			}
			continue
		}
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case mdHeadingLine.MatchString(line):
			flush()
			m := mdHeadingLine.FindStringSubmatch(line)
			blocks = append(blocks, mdBlock{kind: mdHeading, level: len(m[1]), markup: inlineMarkup(m[2])})
		case mdRuleLine.MatchString(line):
			flush()
			blocks = append(blocks, mdBlock{kind: mdRule})
		case mdTableLine.MatchString(line):
			flush()
			blocks = append(blocks, mdBlock{kind: mdCode, markup: escapeMarkup(strings.TrimSpace(line))})
		case mdItemLine.MatchString(line):
			flush()
			m := mdItemLine.FindStringSubmatch(line)
			marker := "•"
			if m[2][0] >= '0' && m[2][0] <= '9' {
				marker = m[2]
			}
			blocks = append(blocks, mdBlock{kind: mdItem, level: len(expandTabs(m[1])) / 2, marker: marker, markup: inlineMarkup(m[3])})
		case mdQuoteLine.MatchString(line):
			if paraKind != mdQuote {
				flush()
			}
			paraKind = mdQuote
			para = append(para, mdQuoteLine.FindStringSubmatch(line)[1])
		default:
			if paraKind != mdParagraph {
				flush()
			}
			paraKind = mdParagraph
			para = append(para, strings.TrimSpace(line))
		}
	}
	flush()
	return blocks
}

// expandTabs is line with each tab as four spaces.
func expandTabs(line string) string { return strings.ReplaceAll(line, "\t", "    ") }

// markupEscaper escapes the characters rich-label markup reads.
var markupEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escapeMarkup(s string) string { return markupEscaper.Replace(s) }

// inline is a span of Markdown being turned into markup: the markup so
// far and the tags open in it, innermost last.
type inline struct {
	out  strings.Builder
	open []string
}

// toggle opens tag, or closes it if it is open, closing and opening
// again the tags opened inside it so the markup stays nested.
func (in *inline) toggle(tag string) {
	at := -1
	for i, t := range in.open {
		if t == tag {
			at = i
		}
	}
	if at < 0 {
		in.out.WriteString("<" + tag + ">")
		in.open = append(in.open, tag)
		return
	}
	inner := in.open[at+1:]
	for i := len(in.open) - 1; i >= at; i-- {
		in.out.WriteString("</" + in.open[i] + ">")
	}
	for _, t := range inner {
		in.out.WriteString("<" + t + ">")
	}
	in.open = append(in.open[:at], inner...)
}

// close closes every tag still open.
func (in *inline) close() string {
	for _, t := range slices.Backward(in.open) {
		in.out.WriteString("</" + t + ">")
	}
	in.open = nil
	return in.out.String()
}

// inlineMarkup is the markup of a span of Markdown: **strong** and
// __strong__ bold, *emphasis* and _emphasis_ italic, ~~deleted~~
// struck, `code` monospace, a link's text underlined and an image's
// description in italics, a backslash escaping the punctuation after
// it, and everything else as written. An emphasis left open closes at
// the span's end.
func inlineMarkup(s string) string {
	var in inline
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]):
			in.out.WriteString(escapeMarkup(s[i+1 : i+2]))
			i += 2
		case c == '`':
			n := run(s, i, '`')
			end := strings.Index(s[i+n:], s[i:i+n])
			if end < 0 {
				in.out.WriteString(s[i : i+n])
				i += n
				continue
			}
			in.out.WriteString("<tt>" + escapeMarkup(strings.TrimSpace(s[i+n:i+n+end])) + "</tt>")
			i += 2*n + end
		case c == '!' && i+1 < len(s) && s[i+1] == '[':
			if text, next, ok := link(s, i+1); ok {
				in.out.WriteString("<i>" + escapeMarkup(text) + "</i>")
				i = next
				continue
			}
			in.out.WriteByte('!')
			i++
		case c == '[':
			if text, next, ok := link(s, i); ok {
				in.out.WriteString("<u>" + inlineMarkup(text) + "</u>")
				i = next
				continue
			}
			in.out.WriteByte('[')
			i++
		case c == '~' && run(s, i, '~') == 2:
			in.toggle("s")
			i += 2
		case c == '*' || c == '_':
			n := run(s, i, c)
			if c == '_' && flanked(s, i, n) {
				in.out.WriteString(s[i : i+n])
				i += n
				continue
			}
			if n >= 2 {
				in.toggle("b")
				n -= 2
				i += 2
			}
			if n == 1 {
				in.toggle("i")
				i++
			}
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			in.out.WriteString(escapeMarkup(string(r)))
			i += size
		}
	}
	return in.close()
}

// run is how many times c repeats in s from i.
func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// flanked reports underscores at i, n of them, inside a word: a_b_c
// stays as written.
func flanked(s string, i, n int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:i])
	after, _ := utf8.DecodeRuneInString(s[i+n:])
	word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	return i > 0 && i+n < len(s) && word(before) && word(after)
}

// link reads the link [text](target) starting at the bracket at i: its
// text and where it ends, false when no link starts there.
func link(s string, i int) (text string, next int, ok bool) {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '[':
			depth++
		case ']':
			depth--
			if depth > 0 {
				continue
			}
			if j+1 >= len(s) || s[j+1] != '(' {
				return "", 0, false
			}
			end := strings.IndexByte(s[j+2:], ')')
			if end < 0 {
				return "", 0, false
			}
			return s[i+1 : j], j + 3 + end, true
		}
	}
	return "", 0, false
}

func isASCIIPunct(c byte) bool {
	return c < utf8.RuneSelf && unicode.IsPunct(rune(c)) || strings.IndexByte("$+<=>^`|~", c) >= 0
}
