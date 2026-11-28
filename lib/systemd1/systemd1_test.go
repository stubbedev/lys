package systemd1_test

import (
	"testing"

	"github.com/stubbedev/lys/lib/systemd1"
)

func TestAUnitUpOrComingUpIsActive(t *testing.T) {
	for _, state := range []string{"active", "activating", "reloading"} {
		if !systemd1.IsActive(state) {
			t.Errorf("%s is not active", state)
		}
	}
	for _, state := range []string{"inactive", "deactivating", "failed", "maintenance", "refreshing", ""} {
		if systemd1.IsActive(state) {
			t.Errorf("%s is active", state)
		}
	}
}
