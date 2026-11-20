package config

import "testing"

func TestAQueryTemplateTakesTheEscapedQuery(t *testing.T) {
	got, ok := FillQueryURL("https://example.org/s?q={query}&again={query}", "a b&c")
	if want := "https://example.org/s?q=a+b%26c&again=a+b%26c"; !ok || got != want {
		t.Fatalf("got %q %v, want %q", got, ok, want)
	}
}

func TestATemplateWithoutTheTokenHasNowhereToPutTheQuery(t *testing.T) {
	if got, ok := FillQueryURL("https://example.org/s?q=", "lys"); ok || got != "" {
		t.Fatalf("got %q %v", got, ok)
	}
}
