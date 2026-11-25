package main

import "testing"

// The replay status speaks English with an en-US byte size whatever the
// desktop's language, as the rest of the CLI's output does; a size
// past a unit's last step reads in the next unit.
func TestTheReplayStatusIsEnglishWhateverTheDesktopSpeaks(t *testing.T) {
	for _, language := range []string{"en_US", "de", "fr"} {
		t.Setenv("LANGUAGE", language)
		for held, want := range map[uint64]string{
			1536 << 20: "Instant replay armed (1.5 GiB)",
			1048575:    "Instant replay armed (1.0 MiB)",
		} {
			got, err := replayArmedStatus(held)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("LANGUAGE=%s: replay status of %d bytes = %q, want %q", language, held, got, want)
			}
		}
	}
}
