// Package powerprofiles is the client of the power profile lys-power
// sets: the active profile, the available set, property-change ticks,
// and the cycle action.
package powerprofiles

import (
	"context"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/profile"
)

// Profile names as lys-power spells them.
const (
	ProfilePowerSaver  = string(profile.PowerSaver)
	ProfileBalanced    = string(profile.Balanced)
	ProfilePerformance = string(profile.Performance)
)

// Canonical cycle order: power-saver → balanced →
// performance.
var order = []string{ProfilePowerSaver, ProfileBalanced, ProfilePerformance}

// NextProfile is the successor of current
// in the canonical order restricted to available (empty available
// means all three), falling back to balanced.
func NextProfile(current string, available []string) string {
	cycle := order
	if len(available) > 0 {
		cycle = make([]string, 0, len(available))
		for _, candidate := range order {
			if slices.Contains(available, candidate) {
				cycle = append(cycle, candidate)
			}
		}
	}
	if len(cycle) == 0 {
		return ProfileBalanced
	}
	for i, candidate := range cycle {
		if candidate == current {
			return cycle[(i+1)%len(cycle)]
		}
	}
	return cycle[0]
}

// Snapshot is the profile as lys-power last published it.
type Snapshot struct {
	// Available is lys-power publishing a profile.
	Available bool
	Active    string
	Profiles  []string
}

// BusName is lys-power's power profile service on the system bus.
const BusName = profile.ServiceName

// Source is the module's seam.
type Source interface {
	// Read collects the snapshot.
	Read(ctx context.Context) (Snapshot, error)
	// SetActive switches the profile.
	SetActive(ctx context.Context, profile string) error
	// Subscribe ticks on each change of the active profile. The channel
	// closes when ctx completes; stop terminates the listener and reports
	// what releasing its match met.
	Subscribe(ctx context.Context) (<-chan struct{}, func() error, error)
}

// System reads lys-power's profile over the system bus.
type System struct {
	conn   *dbus.Conn
	client *profile.Client
}

// NewOn reads lys-power over conn, which stays the caller's.
func NewOn(conn *dbus.Conn) *System { return &System{conn: conn, client: profile.NewClient(conn)} }

// Read collects the snapshot; lys-power not publishing a profile reads
// as unavailable with balanced active.
func (s *System) Read(ctx context.Context) (Snapshot, error) {
	active, err := s.client.Active(ctx)
	if dbusx.Unowned(err) {
		return Snapshot{Active: ProfileBalanced}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("powerprofiles: %w", err)
	}
	all := profile.All()
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = string(p)
	}
	return Snapshot{Available: true, Active: string(active), Profiles: names}, nil
}

// SetActive asks lys-power to switch to the profile named name.
func (s *System) SetActive(ctx context.Context, name string) error {
	p, err := profile.Parse(name)
	if err != nil {
		return fmt.Errorf("powerprofiles: %w", err)
	}
	if err := s.client.SetActive(ctx, p); err != nil {
		return fmt.Errorf("powerprofiles: %w", err)
	}
	return nil
}

// Subscribe ticks on lys-power's PropertiesChanged.
func (s *System) Subscribe(ctx context.Context) (<-chan struct{}, func() error, error) {
	ticks, stop, err := dbusx.Ticks(ctx, s.conn, dbusx.FromOwner(BusName), "powerprofiles", routes)
	if err != nil {
		return nil, nil, fmt.Errorf("powerprofiles: %w", err)
	}
	return ticks, stop, nil
}

// routes call changed for a change of lys-power's profile.
func routes(changed func()) []dbusx.Route {
	return []dbusx.Route{
		dbusx.On(dbusx.PropertiesChanged, profile.ServicePath, func(dbusx.ChangedProperties, dbus.Sequence) error {
			changed()
			return nil
		}).WithArg0(profile.Interface),
	}
}
