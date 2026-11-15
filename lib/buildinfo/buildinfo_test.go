package buildinfo

import "testing"

func TestAStampedVersionWins(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "0.5.0"
	if got := Version(); got != "0.5.0" {
		t.Fatalf("Version() = %q with a stamp, want 0.5.0", got)
	}
}

func TestTheModuleVersionLosesItsV(t *testing.T) {
	for in, want := range map[string]string{
		"v0.5.0":                             "0.5.0",
		"v0.5.1-0.20261115120000-abcdef1234": "0.5.1-0.20261115120000-abcdef1234",
		"(devel)":                            Dev,
		"":                                   Dev,
	} {
		if got := fromModule(in); got != want {
			t.Errorf("fromModule(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestATestBinaryIsADevBuild(t *testing.T) {
	if version != "" {
		t.Skip("stamped")
	}
	if got := Version(); got != Dev {
		t.Fatalf("Version() = %q in a test binary, want %q", got, Dev)
	}
}
