package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stubbedev/lys/power/internal/config"
)

func TestABadPolicyFailsOnlyTheCommandsThatManageDevices(t *testing.T) {
	t.Setenv("LYS_POWER_DEVICE_POLICY", "sometimes")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	paths := config.LoadPaths()
	for _, command := range []string{"version", "--version", "help", "--help", "-h"} {
		if err := dispatch([]string{command}, paths, log); err != nil {
			t.Errorf("lys-power %s under a bad LYS_POWER_DEVICE_POLICY = %v, want it to run", command, err)
		}
	}
	for _, command := range []string{"run", "once", "status"} {
		if err := dispatch([]string{command}, paths, log); !errors.Is(err, errConfiguration) {
			t.Errorf("lys-power %s under a bad LYS_POWER_DEVICE_POLICY = %v, want errConfiguration", command, err)
		}
	}
}
