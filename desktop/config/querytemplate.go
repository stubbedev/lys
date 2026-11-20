package config

import (
	"net/url"
	"strings"
)

// QueryToken is where a search URL template takes the typed query.
const QueryToken = "{query}"

// FillQueryURL puts the escaped query in a search URL template, as the
// launcher's web search and quicklinks open it. A template with no
// QueryToken has nowhere to put it and reports false.
func FillQueryURL(template, query string) (string, bool) {
	if !strings.Contains(template, QueryToken) {
		return "", false
	}
	return strings.ReplaceAll(template, QueryToken, url.QueryEscape(query)), true
}
