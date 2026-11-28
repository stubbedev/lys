package main

import (
	"testing"

	"github.com/stubbedev/lys/login/internal/dm"
)

func TestAWorkerOfNoKnownKindIsAUsageError(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"Greeter"}} {
		if got := worker(args); got != 2 {
			t.Errorf("worker(%q) = %d, want 2", args, got)
		}
	}
}

func TestAGreeterWorkerWithoutACompositorIsAUsageError(t *testing.T) {
	if got := worker([]string{string(dm.GreeterWorker), "--service", "lys-login"}); got != 2 {
		t.Errorf("a greeter worker without a compositor command = %d, want 2", got)
	}
}
