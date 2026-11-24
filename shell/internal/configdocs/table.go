package configdocs

import "strings"

// tableRow is one Markdown table row: every cell on one line, its
// whitespace runs collapsed to single spaces and its pipes escaped.
func tableRow(cells ...string) string {
	var b strings.Builder
	b.WriteByte('|')
	for _, c := range cells {
		b.WriteByte(' ')
		b.WriteString(strings.ReplaceAll(strings.Join(strings.Fields(c), " "), "|", `\|`))
		b.WriteString(" |")
	}
	b.WriteByte('\n')
	return b.String()
}
