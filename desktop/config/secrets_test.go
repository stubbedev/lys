package config

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
)

func TestAnUnsetSecretReferenceIsLoggedWithoutItsName(t *testing.T) {
	logs := logtest.Capture(t)

	const secret = "LYS_SHELL_TEST_sk_live_unset"
	if got, ok := ResolveSecret("$" + secret); ok || got != "" {
		t.Fatalf("ResolveSecret = %q, %v, want an unresolved secret", got, ok)
	}
	if !strings.Contains(logs.String(), "environment variable is not set") {
		t.Errorf("logs %q lack the warning", logs.String())
	}
	if strings.Contains(logs.String(), secret) {
		t.Errorf("logs %q carry the referenced name", logs.String())
	}
}
