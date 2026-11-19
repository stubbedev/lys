package settings

import (
	"reflect"
	"slices"
	"testing"
)

func TestARunningPlayerBecomesItsPattern(t *testing.T) {
	got := values(playerChoices([]string{
		"org.mpris.MediaPlayer2.firefox.instance_1_23", "org.mpris.MediaPlayer2.firefox.instance_2_40",
		"org.mpris.MediaPlayer2.spotify", "org.freedesktop.Notifications", "org.mpris.MediaPlayer2.",
	}))
	if !slices.Equal(got, []string{"*firefox*", "*spotify*"}) {
		t.Fatalf("%q", got)
	}
}

func TestIgnoredPlayersArePickedFromTheRunningOnes(t *testing.T) {
	const path = "modules.media.players-ignored"
	k := testKit(t, "modules:\n  media:\n    players-ignored: [\"*chromium*\"]\n")
	c := choiceList(t, k, "media", path)
	p := openAdd(t, c)
	if !slices.Equal(p.all, []string{"*spotify*", "*firefox*"}) {
		t.Fatalf("offered %q", p.all)
	}
	pickName(t, p, "*spotify*")
	if got := k.store.value(path); !reflect.DeepEqual(got, []any{"*chromium*", "*spotify*"}) {
		t.Errorf("stored %v", got)
	}
}
