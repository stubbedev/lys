package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// Displays' HDR read "fullscreen" and Rotation "flipped-90", lys-wm's
// own words: every value of every enum lys-wm's committed schema
// declares, in a list's items and a mapping's values too, has its label.
func TestEveryLysWMEnumValueHasALabel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "wm", "schema", "lys-wm-config.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	enums := 0
	var walk func(key string, node any)
	walk = func(key string, node any) {
		n, ok := node.(map[string]any)
		if !ok {
			return
		}
		if values, ok := n["enum"].([]any); ok {
			enums++
			typ := wmChoiceType(key)
			if slices.ContainsFunc(values, isBool) {
				typ = wmToggleType
			}
			for _, name := range wmChoiceNames(values) {
				if k := config.EnumLabelKey(typ, name); !locale.T().Has(k) {
					t.Errorf("lys-wm's %s value %q has no label %s", key, name, k)
				}
			}
		}
		if props, ok := n["properties"].(map[string]any); ok {
			for k, v := range props {
				walk(k, v)
			}
		}
		for _, sub := range []string{"items", "additionalProperties"} {
			walk(key, n[sub])
		}
		for _, alt := range []string{"anyOf", "oneOf", "allOf"} {
			if list, ok := n[alt].([]any); ok {
				for _, v := range list {
					walk(key, v)
				}
			}
		}
	}
	walk("", root)
	if enums < 20 {
		t.Fatalf("found %d enums in lys-wm's schema, so the walk is wrong", enums)
	}
}
