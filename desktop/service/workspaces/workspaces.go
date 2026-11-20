// Package workspaces reads the workspaces of the running Hyprland, niri
// or sway as lys-shell's bar models them, and the workspace-ignore entries
// that hide them.
package workspaces

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/service/hyprland"
	"github.com/stubbedev/lys/desktop/service/niri"
	"github.com/stubbedev/lys/desktop/service/sway"
	"github.com/stubbedev/lys/desktop/shell/barview"
)

// ErrNotListed is a compositor whose workspaces this package does not read.
var ErrNotListed = errors.New("lys reads this compositor's workspaces elsewhere")

// FromSway is a sway workspace as the bar models it, occupied when a
// window sits on it.
func FromSway(r sway.Workspace, hasWindows bool) barview.Workspace {
	return barview.Workspace{
		ID: uint64(r.ID), Num: r.Num, Name: r.Name, HasName: true,
		Output: r.Output, HasOutput: true,
		Urgent: r.Urgent, Active: r.Visible, Focused: r.Focused,
		HasWindows: hasWindows,
	}
}

// FromNiri is a niri workspace as the bar models it: its index is its
// number, and it is occupied while it has an active window.
func FromNiri(r niri.Workspace) barview.Workspace {
	ws := barview.Workspace{
		ID: r.ID, Num: int(r.Idx),
		Urgent: r.IsUrgent, Active: r.IsActive, Focused: r.IsFocused,
		HasWindows: r.ActiveWindowID != nil,
	}
	if r.Name != nil {
		ws.Name, ws.HasName = *r.Name, true
	}
	if r.Output != nil {
		ws.Output, ws.HasOutput = *r.Output, true
	}
	return ws
}

// IgnoreNames are the workspace-ignore entries that hide each workspace
// the running compositor on keeps, each once: numbers first in numeric
// order, then names.
func IgnoreNames(on compositor.Kind) ([]string, error) {
	names, err := ignoreNames(on)
	if err != nil {
		return nil, fmt.Errorf("workspaces: list %s's workspaces: %w", on, err)
	}
	slices.SortFunc(names, byNumberThenName)
	return slices.Compact(names), nil
}

func ignoreNames(on compositor.Kind) ([]string, error) {
	switch on {
	case compositor.Hyprland:
		return hyprlandNames()
	case compositor.Niri:
		return niriNames()
	case compositor.Sway:
		return swayNames()
	case compositor.Unknown, compositor.LysWM, compositor.Mango:
	}
	return nil, ErrNotListed
}

func hyprlandNames() ([]string, error) {
	conn, err := hyprland.Connect()
	if err != nil {
		return nil, err
	}
	replies, err := conn.Workspaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(replies))
	for i, r := range replies {
		names[i] = barview.HyprlandIgnoreName(r.ID)
	}
	return names, nil
}

func niriNames() ([]string, error) {
	conn, err := niri.Connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	replies, err := conn.Workspaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(replies))
	for i, r := range replies {
		names[i] = FromNiri(r).IgnoreName()
	}
	return names, nil
}

func swayNames() ([]string, error) {
	conn, err := sway.Connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	replies, err := conn.Workspaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(replies))
	for i, r := range replies {
		names[i] = FromSway(r, false).IgnoreName()
	}
	return names, nil
}

func byNumberThenName(a, b string) int {
	an, aErr := strconv.Atoi(a)
	bn, bErr := strconv.Atoi(b)
	switch {
	case aErr == nil && bErr == nil:
		return cmp.Or(cmp.Compare(an, bn), cmp.Compare(a, b))
	case aErr == nil:
		return -1
	case bErr == nil:
		return 1
	}
	return cmp.Compare(a, b)
}
