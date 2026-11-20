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

const (
	queryLimit = 10 * time.Second
	pickLimit  = 3 * time.Minute
)

// ErrNoWindow is a pick that ended on no window.
var ErrNoWindow = errors.New("wmstate: the click was on no window")

// ErrWindowGone is a picked window that closed before it could be read.
var ErrWindowGone = errors.New("wmstate: the picked window closed before it could be read")

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

// Capabilities is what each output lys-wm shows can do, by connector name.
// An output that is off is not among them: lys-wm reads what an output can
// do only while it shows it.
func Capabilities(run wmconfig.Runner) (map[string]wmipc.OutputCapabilities, error) {
	outputs, err := bounded(queryLimit, func(ctx context.Context) ([]wmipc.Output, error) {
		return query[[]wmipc.Output](ctx, run, wmipc.KindOutputs)
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]wmipc.OutputCapabilities, len(outputs))
	for _, o := range outputs {
		if o.Capabilities == nil {
			return nil, fmt.Errorf("%w: output %s carries no capabilities", ErrOutdated, o.Name)
		}
		out[o.Name] = *o.Capabilities
	}
	return out, nil
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

// PickWindow waits for the next click in the session and answers the
// window under it; Escape cancels it with lys-wm's error.
func PickWindow(run wmconfig.Runner) (wmipc.Window, error) {
	return bounded(pickLimit, func(ctx context.Context) (wmipc.Window, error) {
		id, err := query[*uint64](ctx, run, wmipc.KindPickWindow)
		if err != nil {
			return wmipc.Window{}, err
		}
		if id == nil {
			return wmipc.Window{}, ErrNoWindow
		}
		windows, err := query[[]wmipc.Window](ctx, run, wmipc.KindWindows)
		if err != nil {
			return wmipc.Window{}, err
		}
		i := slices.IndexFunc(windows, func(w wmipc.Window) bool { return w.ID == *id })
		if i < 0 {
			return wmipc.Window{}, fmt.Errorf("%w: window %d", ErrWindowGone, *id)
		}
		return windows[i], nil
	})
}

// RunningApps are the app IDs of the windows lys-wm shows, each once, in
// order.
func RunningApps(run wmconfig.Runner) ([]string, error) {
	windows, err := bounded(queryLimit, func(ctx context.Context) ([]wmipc.Window, error) {
		return query[[]wmipc.Window](ctx, run, wmipc.KindWindows)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(windows))
	for _, w := range windows {
		if w.AppID != "" {
			ids = append(ids, w.AppID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

// Workspaces are the IDs of the workspaces lys-wm keeps, each once, in
// order.
func Workspaces(run wmconfig.Runner) ([]int, error) {
	workspaces, err := bounded(queryLimit, func(ctx context.Context) ([]wmipc.Workspace, error) {
		return query[[]wmipc.Workspace](ctx, run, wmipc.KindWorkspaces)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(workspaces))
	for i, w := range workspaces {
		ids[i] = w.ID
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}
