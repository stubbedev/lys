package session

import (
	"testing"

	"github.com/stubbedev/lys/wm/internal/testenv"
)

func TestNoVirtualDeviceIsCreatedBesideASession(t *testing.T) {
	t.Setenv(testenv.SeatEnv, "")
	created, skipped := false, false
	t.Run("device", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		newVirtualDevice(t, "lys-wm virtual guard", []uinputBit{{uiSetEvBit, evKey}})
		created = true
	})
	if created || !skipped {
		t.Fatalf("without %s=1 a virtual device was created (%v) or the test did not skip (%v)", testenv.SeatEnv, created, !skipped)
	}
}
