// Package wmstate asks the running lys-wm what it knows about the hardware
// and its windows, through lys-wmctl, so that the pages show what a
// display or a device can do rather than guess.
package wmstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const queryLimit = 10 * time.Second

// ErrNotRunning is no lys-wm answering.
var ErrNotRunning = errors.New("wmstate: no lys-wm is running")

// ErrOutdated is a running lys-wm older than this lys-settings, which does
// not report what was asked.
var ErrOutdated = errors.New("wmstate: the running lys-wm predates what lys-settings asks of it")

func query[T any](ctx context.Context, run wmconfig.Runner, kind wmipc.Kind) (T, error) {
	var v T
	out, err := run(ctx, wmconfig.Control, string(kind))
	if wmconfig.IsNotRunning(err) {
		return v, ErrNotRunning
	}
	if err != nil {
		return v, fmt.Errorf("wmstate: ask lys-wm for %s: %w", kind, err)
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return v, fmt.Errorf("wmstate: read lys-wm's %s %q: %w", kind, strings.TrimSpace(string(out)), err)
	}
	return v, nil
}

func bounded[T any](limit time.Duration, ask func(ctx context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	return ask(ctx)
}

// Pointers are the connected devices one of lys-wm's input sections
// configures, in the order lys-wm lists them.
func Pointers(run wmconfig.Runner) ([]wmipc.Device, error) {
	devices, err := bounded(queryLimit, func(ctx context.Context) ([]wmipc.Device, error) {
		return query[[]wmipc.Device](ctx, run, wmipc.KindDevices)
	})
	if err != nil {
		return nil, err
	}
	if i := slices.IndexFunc(devices, func(d wmipc.Device) bool { return d.Class == "" }); i >= 0 {
		return nil, fmt.Errorf("%w: device %s carries no class", ErrOutdated, devices[i].Name)
	}
	return slices.DeleteFunc(devices, func(d wmipc.Device) bool { return d.Class == wmipc.ClassNone }), nil
}
