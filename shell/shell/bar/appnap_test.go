package bar

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/power/nap"
)

func TestEveryConfiguredNapLevelIsALevelAppNapKnows(t *testing.T) {
	configured := map[string]config.NapLevel{}
	for _, l := range config.NapLevels() {
		configured[string(l)] = l
	}
	got, err := napLevels(configured)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(nap.Levels()) {
		t.Fatalf("the config names %d levels and App Nap knows %d", len(got), len(nap.Levels()))
	}
	for name, l := range got {
		if l.String() != name {
			t.Errorf("%s became %v", name, l)
		}
	}
	if _, err := napLevels(map[string]config.NapLevel{"x": "deep"}); err == nil {
		t.Error("an unknown level was taken")
	}
}
