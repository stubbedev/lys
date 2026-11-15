package fprint

import "testing"

func TestEveryResultReadsBackAsItself(t *testing.T) {
	for _, r := range []Result{Match, NoMatch, Retry, Listening, Stopped} {
		text, err := r.MarshalText()
		if err != nil {
			t.Fatalf("%d: %v", r, err)
		}
		var back Result
		if err := back.UnmarshalText(text); err != nil || back != r {
			t.Errorf("%q reads back as %d, %v; want %d", text, back, err, r)
		}
	}
}

func TestNoResultHasNoName(t *testing.T) {
	if text, err := Result(0).MarshalText(); err == nil {
		t.Errorf("the zero result is named %q", text)
	}
	if text, err := (Stopped + 1).MarshalText(); err == nil {
		t.Errorf("a result past the last is named %q", text)
	}
}

func TestAnUnknownNameIsNoResult(t *testing.T) {
	for _, name := range []string{"", "verify-match", "MATCH"} {
		var r Result
		if err := r.UnmarshalText([]byte(name)); err == nil {
			t.Errorf("%q read as %d", name, r)
		}
	}
}
