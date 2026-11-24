package xkb

import (
	"fmt"
	"strings"
	"testing"
)

func keymapWithKeys(n int) string {
	var b strings.Builder
	b.WriteString("xkb_keymap {\nxkb_keycodes \"t\" {\n")
	for i := range n {
		fmt.Fprintf(&b, "<K%d> = %d;\n", i, 8+i)
	}
	b.WriteString("};\nxkb_symbols \"t\" {\n")
	for i := range n {
		fmt.Fprintf(&b, "key <K%d> { [ a, b ] };\n", i)
	}
	b.WriteString("};\n};\n")
	return b.String()
}

func TestParsingMoreKeysAllocatesOnlyForChunks(t *testing.T) {
	ctx := NewContext(func(string) string { return "" })
	ctx.SetLogLevel(LogCritical)
	parse := func(text string) func() {
		return func() {
			pf, err := ctx.newSession(FormatTextV1).parseText("(input string)", text)
			if err != nil {
				t.Fatal(err)
			}
			f, err := pf.defaultMap()
			if err != nil {
				t.Fatal(err)
			}
			if len(f.sections) != 2 {
				t.Fatalf("parsed %d sections, want keycodes and symbols", len(f.sections))
			}
		}
	}
	const few, many = 64, 4096
	base := testing.AllocsPerRun(5, parse(keymapWithKeys(few)))
	more := testing.AllocsPerRun(5, parse(keymapWithKeys(many)))
	perKey := (more - base) / (many - few)
	t.Logf("parsing %d keys allocates %v times, %d keys %v times: %.3f a key", few, base, many, more, perKey)
	if perKey > 0.1 {
		t.Errorf("each further keycode and key symbols declaration allocates %.3f times, want its syntax tree carved from shared chunks", perKey)
	}
}

func TestCompilingMoreKeysCarvesTheirGroupsAndLevelsFromChunks(t *testing.T) {
	ctx := NewContext(func(string) string { return "" })
	ctx.SetLogLevel(LogCritical)
	compile := func(text string) func() {
		return func() {
			km, err := ctx.NewKeymapFromBuffer([]byte(text), FormatTextV1)
			if err != nil {
				t.Fatal(err)
			}
			if got := km.KeySymsByLevel(8, 0, 0); len(got) != 1 || got[0] != Keya {
				t.Fatalf("the first key's first level is %v, want a", got)
			}
		}
	}
	const few, many = 64, 1024
	base := testing.AllocsPerRun(5, compile(keymapWithKeys(few)))
	more := testing.AllocsPerRun(5, compile(keymapWithKeys(many)))
	perKey := (more - base) / (many - few)
	t.Logf("compiling %d keys allocates %v times, %d keys %v times: %.3f a key", few, base, many, more, perKey)
	if perKey > 8.5 {
		t.Errorf("each further key allocates %.3f times to compile, want its groups and levels carved from shared chunks", perKey)
	}
}
