package idlepolicy

import (
	"slices"
	"testing"
	"time"
)

func mustTimeout(t *testing.T, d time.Duration) Timeout {
	t.Helper()
	to, err := NewTimeout(d)
	if err != nil {
		t.Fatal(err)
	}
	return to
}

func attentionPolicy(t *testing.T, a Attention) Policy {
	t.Helper()
	var p Policy
	p.Timeouts[KeyboardBacklight] = Timeouts{AC: Never, Battery: mustTimeout(t, time.Minute)}
	p.Timeouts[Dim] = Timeouts{AC: mustTimeout(t, 4*time.Minute), Battery: mustTimeout(t, 2*time.Minute)}
	p.Timeouts[Lock] = Timeouts{AC: mustTimeout(t, 5*time.Minute), Battery: mustTimeout(t, 90*time.Second)}
	p.Timeouts[ScreenOff] = Timeouts{AC: mustTimeout(t, 10*time.Minute), Battery: mustTimeout(t, 5*time.Minute)}
	p.Timeouts[Suspend] = Timeouts{AC: Never, Battery: mustTimeout(t, 15*time.Minute)}
	p.Attention = a
	return p
}

func TestPresenceSpellsAndParsesBothWays(t *testing.T) {
	for _, p := range []Presence{Present, Absent, Unseen} {
		got, err := ParsePresence(p.String())
		if err != nil || got != p {
			t.Errorf("ParsePresence(%q) = %v, %v", p.String(), got, err)
		}
	}
	if _, err := ParsePresence("looking"); err == nil {
		t.Error("an unknown presence parsed")
	}
	if got := Presence(0).String(); got != "Presence(0)" {
		t.Errorf("an invalid presence spells %q", got)
	}
}

func TestWithoutAttentionEveryStageIsWatchedAndNoneFollows(t *testing.T) {
	p := attentionPolicy(t, AttentionOff)
	if got, want := p.Watched(Battery), p.Arms(Battery); !slices.Equal(got, want) {
		t.Errorf("watched %v, want every arm %v", got, want)
	}
	if got := p.Followers(Battery, true); got != nil {
		t.Errorf("followers %v without attention", got)
	}
	if p.Attending(Battery) {
		t.Error("attending with attention off")
	}
}

func TestWithoutADimNothingWaitsForOne(t *testing.T) {
	p := attentionPolicy(t, AttentionSooner)
	p.Timeouts[Dim] = Timeouts{AC: Never, Battery: Never}
	if p.Attending(AC) {
		t.Error("attending without a dim")
	}
	if got, want := p.Watched(AC), p.Arms(AC); !slices.Equal(got, want) {
		t.Errorf("watched %v, want every arm %v", got, want)
	}
}

func TestStagesAfterTheDimFollowItAtTheirOwnDistance(t *testing.T) {
	p := attentionPolicy(t, AttentionPostpone)
	if got, want := p.Watched(Battery), []Arm{{KeyboardBacklight, time.Minute}, {Dim, 2 * time.Minute}, {Lock, 90 * time.Second}}; !slices.Equal(got, want) {
		t.Errorf("watched on battery %v, want %v: a lock set before the dim keeps its own watch", got, want)
	}
	if got, want := p.Followers(Battery, false), []Follow{{ScreenOff, 3 * time.Minute}, {Suspend, 13 * time.Minute}}; !slices.Equal(got, want) {
		t.Errorf("followers on battery %v, want %v", got, want)
	}
	if got, want := p.Followers(AC, false), []Follow{{Lock, time.Minute}, {ScreenOff, 6 * time.Minute}}; !slices.Equal(got, want) {
		t.Errorf("followers on AC %v, want %v", got, want)
	}
	p.Timeouts[Lock] = Timeouts{AC: mustTimeout(t, 4*time.Minute), Battery: Never}
	if got, want := p.Followers(AC, false)[0], (Follow{Lock, 0}); got != want {
		t.Errorf("a lock at the dim's own timeout follows as %v, want %v: it must not lock a reader", got, want)
	}
}

func TestAHurriedDimBringsTheLockAndTheScreenOffWithinTheGrace(t *testing.T) {
	p := attentionPolicy(t, AttentionSooner)
	if got, want := p.Followers(AC, true), []Follow{{Lock, AbsentGrace}, {ScreenOff, AbsentGrace}}; !slices.Equal(got, want) {
		t.Errorf("hurried followers on AC %v, want %v", got, want)
	}
	if got, want := p.Followers(Battery, true), []Follow{{ScreenOff, AbsentGrace}, {Suspend, 13 * time.Minute}}; !slices.Equal(got, want) {
		t.Errorf("hurried followers on battery %v, want %v: suspend keeps its distance", got, want)
	}
	p.Timeouts[Lock] = Timeouts{AC: mustTimeout(t, 4*time.Minute+10*time.Second), Battery: Never}
	if got := p.Followers(AC, true)[0]; got.After != 10*time.Second {
		t.Errorf("a lock 10s after the dim, hurried: %v, want it no later", got)
	}
}

func TestAFacePostponesTheDimUpToTheCapAndNobodyDimsAtOnce(t *testing.T) {
	for _, mode := range []Attention{AttentionPostpone, AttentionSooner} {
		p := attentionPolicy(t, mode)
		for postponed := range MaxPostponements {
			if got, want := p.AtDim(AC, Present, postponed), (DimStep{Postpone: 4 * time.Minute}); got != want {
				t.Errorf("mode %d, a face after %d postponements: %v, want %v", mode, postponed, got, want)
			}
		}
		if got := p.AtDim(AC, Present, MaxPostponements); got != (DimStep{}) {
			t.Errorf("mode %d, a face at the cap: %v, want the dim now, unhurried", mode, got)
		}
		if got := p.AtDim(Battery, Present, 0); got.Postpone != 2*time.Minute {
			t.Errorf("mode %d, a face on battery postpones %v, want the battery dim interval", mode, got.Postpone)
		}
		if got, want := p.AtDim(AC, Absent, 1), (DimStep{Hurried: mode == AttentionSooner}); got != want {
			t.Errorf("mode %d, nobody: %v, want %v", mode, got, want)
		}
		if got := p.AtDim(AC, Unseen, 0); got != (DimStep{}) {
			t.Errorf("mode %d, no answer: %v, want the dim now, unhurried", mode, got)
		}
	}
}
