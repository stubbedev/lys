package appicons

import (
	"strings"

	"github.com/stubbedev/lys/desktop/glob"
)

// lowered is Default with every pattern lowercased: Lookup lowercases the name, so the table must match in
// the same case or a mixed-case entry ("Slack", "Tor Browser") never
// does.
var lowered = func() []Entry {
	out := make([]Entry, len(Default))
	for i, e := range Default {
		out[i] = Entry{Pattern: strings.ToLower(e.Pattern), Icon: e.Icon}
	}
	return out
}()

// Lookup is the first Default pattern that matches the name, both
// lowercased (an exact match counts too). Callers pass app ids, window classes,
// or player identities alike.
func Lookup(name string) (string, bool) {
	lower := strings.ToLower(name)
	for _, entry := range lowered {
		if lower == entry.Pattern || glob.Wildcard(entry.Pattern, lower) {
			return entry.Icon, true
		}
	}
	return "", false
}
