package network

import (
	"maps"
	"slices"
	"strings"
)

// VPNFault is why a field refuses a save.
type VPNFault int

// Field faults.
const (
	// FaultMissing is a required field left empty.
	FaultMissing VPNFault = iota
	// FaultMalformed is a value not in the field's format.
	FaultMalformed
)

// VPNProblem is one field refusing a save, and why.
type VPNProblem struct {
	// Field is the refusing field.
	Field VPNField
	// Fault is why it refuses.
	Fault VPNFault
}

// Check is every field values leave the kind refusing: the missing
// ones first, then the malformed ones, each in drawing order, so the
// first problem is the one worth saying.
func (k VPNKind) Check(values map[string]string) []VPNProblem {
	var missing, malformed []VPNProblem
	for _, f := range k.Fields {
		v := values[f.Key]
		switch {
		case f.Required && strings.TrimSpace(v) == "":
			missing = append(missing, VPNProblem{Field: f, Fault: FaultMissing})
		case !f.Format.Accepts(v):
			malformed = append(malformed, VPNProblem{Field: f, Fault: FaultMalformed})
		}
	}
	return append(missing, malformed...)
}

func (k VPNKind) hasField(key string) bool {
	return slices.ContainsFunc(k.Fields, func(f VPNField) bool { return f.Key == key })
}

// Leftovers are the values the kind's form has no field for, kept in
// the raw editor so an edit never drops them.
func (k VPNKind) Leftovers(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, v := range values {
		if !k.hasField(key) {
			out[key] = v
		}
	}
	return out
}

// KindsWith is kinds plus the kind id names when this machine no longer
// has its plugin: the profile still opens, in the raw editor, and saving
// keeps its type instead of rewriting it as the first kind.
func KindsWith(kinds []VPNKind, id string) []VPNKind {
	if id == "" || slices.ContainsFunc(kinds, func(k VPNKind) bool { return k.ID == id }) {
		return kinds
	}
	return append(slices.Clone(kinds), VPNKind{ID: id, Label: id})
}

// ParseRawValues reads the raw editor: one key = value per line, with
// # comments, blank lines and lines without a key ignored.
func ParseRawValues(text string) map[string]string {
	out := map[string]string{}
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if key = strings.TrimSpace(key); ok && key != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

// RenderRawValues is values as the raw editor shows them, sorted.
func RenderRawValues(values map[string]string) string {
	lines := make([]string, 0, len(values))
	for k, v := range values {
		lines = append(lines, k+" = "+v)
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// TypedValue is one typed field's key and what it holds.
type TypedValue struct {
	// Key is the field's key.
	Key string
	// Value is what the field holds.
	Value string
}

// MergeVPNValues is the raw editor's keys with the typed fields over
// them. A typed field wins, being the one the user sees, and an emptied
// one clears its key rather than reviving the raw one: no blank value
// reaches NetworkManager.
func MergeVPNValues(raw map[string]string, typed []TypedValue) map[string]string {
	values := maps.Clone(raw)
	if values == nil {
		values = map[string]string{}
	}
	for _, t := range typed {
		values[t.Key] = t.Value
	}
	maps.DeleteFunc(values, func(_, v string) bool { return strings.TrimSpace(v) == "" })
	return values
}
