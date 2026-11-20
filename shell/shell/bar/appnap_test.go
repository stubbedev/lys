package bar

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/power/nap"
)

func TestEveryConfiguredNapLevelIsALevelAppNapKnows(t *testing.T) {
	got, err := napLevels(map[string]config.NapLevel{
		"a": config.NapLevelDefault, "b": config.NapLevelNever, "c": config.NapLevelEfficiencyCores, "d": config.NapLevelFrozen,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]nap.Level{"a": nap.LevelDefault, "b": nap.LevelNever, "c": nap.LevelEfficiencyCores, "d": nap.LevelFrozen}
	for app, l := range want {
		if got[app] != l {
			t.Errorf("%s: %v, want %v", app, got[app], l)
		}
	}
	if _, err := napLevels(map[string]config.NapLevel{"x": "deep"}); err == nil {
		t.Error("an unknown level was taken")
	}
}
