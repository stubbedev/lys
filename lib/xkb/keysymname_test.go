package xkb

import "testing"

func TestResolvingAKeysymNameAllocatesNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		want Keysym
	}{
		{"BackSpace", KeysymFromName("BackSpace", KeysymNoFlags)},
		{"Cyrillic_a", KeysymFromName("Cyrillic_a", KeysymNoFlags)},
		{"NoSymbol", KeyNoSymbol},
		{"ANY", KeyNoSymbol},
		{"VoidSymbol", KeyVoidSymbol},
		{"NONE", KeyVoidSymbol},
	} {
		if got, ok := resolveKeysymName(tc.name); !ok || got != tc.want {
			t.Errorf("%q resolves to %#x, %v; want %#x", tc.name, got, ok, tc.want)
		}
		if allocs := testing.AllocsPerRun(100, func() { resolveKeysymName(tc.name) }); allocs != 0 {
			t.Errorf("resolving %q allocates %v times", tc.name, allocs)
		}
	}
	if _, ok := resolveKeysymName("NoSymbolX"); ok {
		t.Error("an unknown name resolved")
	}
}
