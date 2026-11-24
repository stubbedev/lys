package configdocs

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

var unescapedPipe = regexp.MustCompile(`(^|[^\\])\|`)

func requireTable(t *testing.T, rendered string, columns int, wantRows ...string) {
	t.Helper()
	got := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(got) != len(wantRows)+2 {
		t.Fatalf("table has %d lines, want a header, a rule and %d rows:\n%s", len(got), len(wantRows), rendered)
	}
	for i, want := range wantRows {
		row := got[i+2]
		if row != want {
			t.Errorf("row %d = %q, want %q", i, row, want)
		}
		if n := len(unescapedPipe.FindAllString(row, -1)); n != columns+1 {
			t.Errorf("row %d splits into %d cells, want %d: %q", i, n-1, columns, row)
		}
	}
}

func TestEnumValueRowsAreSingleLines(t *testing.T) {
	got, ok := oneOfVariants(config.Schema{"oneOf": []any{
		config.Schema{"const": "genie", "description": "Suck in toward the edge (macOS \"genie\"\nminimize, approximated).   Auto-orients to\n\tthe edge."},
		config.Schema{"const": "fade", "description": "Cross-fade opacity in and out."},
		config.Schema{"const": "pipe", "description": "Either a | b."},
	}})
	if !ok {
		t.Fatal("no table for a oneOf of consts")
	}
	requireTable(t, got, 2,
		"| `\"genie\"` | Suck in toward the edge (macOS \"genie\" minimize, approximated). Auto-orients to the edge. |",
		"| `\"fade\"` | Cross-fade opacity in and out. |",
		"| `\"pipe\"` | Either a \\| b. |",
	)
}

func TestObjectFieldRowsAreSingleLines(t *testing.T) {
	got, ok := objectShape(config.Schema{"type": "object", "properties": config.Schema{
		"wrapped": config.Schema{"description": "First line\nwraps here.\n\nSecond paragraph."},
		"plain":   config.Schema{"description": "One line."},
		"piped":   config.Schema{"description": "Either a | b."},
	}})
	if !ok {
		t.Fatal("no table for an object with properties")
	}
	requireTable(t, got, 2,
		"| `piped` | Either a \\| b. |",
		"| `plain` | One line. |",
		"| `wrapped` | First line wraps here. |",
	)
}

func TestFieldRowsAreSingleLines(t *testing.T) {
	var b strings.Builder
	renderGroup(&b, config.DocGroup{Title: "Fields"}, []field{
		{name: "wrapped", typeCell: "string", defaultCell: "`\"a\nb\"`", description: "First line\n   wraps   here."},
		{name: "plain", typeCell: "bool", defaultCell: "`true`", description: "One line."},
		{name: "piped", typeCell: "string", defaultCell: "`\" | \"`", description: "Either a | b."},
	})
	table := strings.TrimPrefix(b.String(), "## Fields\n\n")
	requireTable(t, table, 4,
		"| `wrapped` | string | `\"a b\"` | First line wraps here. |",
		"| `plain` | bool | `true` | One line. |",
		"| `piped` | string | `\" \\| \"` | Either a \\| b. |",
	)
}

func TestIndexRowsAreSingleLines(t *testing.T) {
	page := indexPage([]config.DocSection{
		{Name: "wrapped", Schema: config.Schema{"description": "First line\nwraps here.\n\nSecond paragraph."}},
		{Name: "piped", Schema: config.Schema{"description": "Either a | b."}},
		{Name: "headed", Schema: config.Schema{"description": "Summary\n## Examples\n\n```yaml\nx: 1\n```"}},
	})
	for _, want := range []string{
		"| [`wrapped`](/shell/config/wrapped) | First line wraps here. |\n",
		"| [`piped`](/shell/config/piped) | Either a \\| b. |\n",
		"| [`headed`](/shell/config/headed) | Summary |\n",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index lacks %q:\n%s", want, page)
		}
	}
}
