package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/desktop/feed"
)

// Profile is one saved NetworkManager connection, the fields the
// service reads.
type Profile struct {
	// Path is the Settings.Connection object, which activation takes.
	Path dbus.ObjectPath
	// UUID is stable across renames.
	UUID string
	// ID is the user-facing name.
	ID string
	// Type is connection.type: "vpn", "wireguard", "802-11-wireless", ...
	Type string
	// SSID is 802-11-wireless.ssid for a wifi profile, nil otherwise.
	SSID []byte
	// Autoconnect is connection.autoconnect: whether NetworkManager
	// brings the profile up by itself when it can.
	Autoconnect bool
}

// Settings is the live list of saved profiles (core/settings), kept
// current by NM's NewConnection / ConnectionRemoved signals and each
// profile's Updated signal (a rename lands without a rebuild of
// anything else).
type Settings struct {
	nm nm
	// owner is whose the VPN profiles made here are, "" when the passwd
	// database has no name for this user (they are then system-wide).
	owner      string
	pluginDirs []string

	mu       sync.Mutex
	profiles []Profile
	changes  feed.Tick
}

// connectionArg is what Settings' NewConnection and ConnectionRemoved
// carry: the profile's object.
type connectionArg struct {
	Path dbus.ObjectPath `dbus:"connection"`
}

var (
	newConnection     = dbusx.NewSignal[connectionArg](settingsIface, "NewConnection")
	connectionRemoved = dbusx.NewSignal[connectionArg](settingsIface, "ConnectionRemoved")
	connectionUpdated = dbusx.NewSignal[struct{}](connectionIface, "Updated")
)

// newSettings reads every profile and follows NM's announcements until
// ctx ends; stop takes its watch off the connection. An announcement
// the watch hears while the first read runs is applied after it.
func newSettings(ctx context.Context, n nm) (*Settings, func() error, error) {
	s := &Settings{nm: n, owner: ownerName(), pluginDirs: pluginDirectories}
	loaded := make(chan struct{})
	watch, err := dbusx.WatchSignals(ctx, n.conn, dbusx.FromOwner(BusName),
		func(err error) { log.Printf("network: settings: %v", err) },
		s.routes(ctx, loaded)...)
	if err != nil {
		return nil, nil, fmt.Errorf("network: watch settings: %w", err)
	}
	err = s.reload(ctx)
	close(loaded)
	if err != nil {
		return nil, nil, errors.Join(err, watch.Stop())
	}
	return s, watch.Stop, nil
}

// reload re-reads the whole list (startup, and NM coming back).
func (s *Settings) reload(ctx context.Context) error {
	var paths []dbus.ObjectPath
	if err := s.nm.object(settingsPath).CallWithContext(ctx, settingsIface+".ListConnections", 0).Store(&paths); err != nil {
		return fmt.Errorf("network: list connections: %w", err)
	}
	profiles := make([]Profile, 0, len(paths))
	for _, path := range paths {
		if p, err := s.read(ctx, path); err == nil {
			profiles = append(profiles, p)
		}
	}
	s.mu.Lock()
	s.profiles = profiles
	s.mu.Unlock()
	feed.Notify(&s.changes)
	return nil
}

func (s *Settings) read(ctx context.Context, path dbus.ObjectPath) (Profile, error) {
	dict, err := s.nm.getSettings(ctx, path)
	if err != nil {
		return Profile{}, err
	}
	r := dictReader(dict)
	p := Profile{
		Path: path,
		UUID: r.str("connection", "uuid"),
		ID:   r.str("connection", "id"),
		Type: r.str("connection", "type"),
	}
	if ssid, ok := dict["802-11-wireless"]["ssid"].Value().([]byte); ok {
		p.SSID = ssid
	}
	auto, err := autoconnectOf(dict)
	if err != nil {
		return Profile{}, fmt.Errorf("read the profile %s: %w", path, err)
	}
	p.Autoconnect = auto
	return p, nil
}

// autoconnectOf is connection.autoconnect, NetworkManager's true when
// the profile leaves it out.
func autoconnectOf(dict ConnectionDict) (bool, error) {
	v, ok := dict["connection"][autoconnectKey]
	if !ok {
		return true, nil
	}
	on, ok := v.Value().(bool)
	if !ok {
		return false, fmt.Errorf("connection.%s is %v, not a boolean", autoconnectKey, v)
	}
	return on, nil
}

const autoconnectKey = "autoconnect"

// SetAutoconnect says whether NetworkManager brings the saved profile at
// path up by itself, leaving the rest of it as it was; it is rewritten
// as SetProxy rewrites one.
func (s *Settings) SetAutoconnect(ctx context.Context, path dbus.ObjectPath, on bool) error {
	dict, err := s.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("set whether %s joins by itself: %w", path, err)
	}
	out := withoutLegacyIP(dict)
	section := maps.Clone(dict["connection"])
	if section == nil {
		section = map[string]dbus.Variant{}
	}
	section[autoconnectKey] = dbus.MakeVariant(on)
	out["connection"] = section
	if err := s.Update(ctx, path, out); err != nil {
		return fmt.Errorf("set whether %s joins by itself: %w", path, err)
	}
	return nil
}

// routes follow NM's profiles once loaded is closed: one added, one
// removed, and one rewritten.
func (s *Settings) routes(ctx context.Context, loaded <-chan struct{}) []dbusx.Route {
	return []dbusx.Route{
		dbusx.On(newConnection, settingsPath, func(added connectionArg, _ dbus.Sequence) error {
			<-loaded
			p, err := s.read(ctx, added.Path)
			if err != nil {
				return fmt.Errorf("new connection %s: %w", added.Path, err)
			}
			s.mu.Lock()
			s.profiles = slices.DeleteFunc(s.profiles, func(q Profile) bool { return q.Path == added.Path })
			s.profiles = append(s.profiles, p)
			s.mu.Unlock()
			feed.Notify(&s.changes)
			return nil
		}),
		dbusx.On(connectionRemoved, settingsPath, func(removed connectionArg, _ dbus.Sequence) error {
			<-loaded
			s.mu.Lock()
			s.profiles = slices.DeleteFunc(s.profiles, func(q Profile) bool { return q.Path == removed.Path })
			s.mu.Unlock()
			feed.Notify(&s.changes)
			return nil
		}),
		dbusx.OnUnder(connectionUpdated, settingsPath, func(path dbus.ObjectPath, _ struct{}, _ dbus.Sequence) error {
			<-loaded
			p, err := s.read(ctx, path)
			if err != nil {
				return fmt.Errorf("updated connection %s: %w", path, err)
			}
			s.mu.Lock()
			for i := range s.profiles {
				if s.profiles[i].Path == path {
					s.profiles[i] = p
				}
			}
			s.mu.Unlock()
			feed.Notify(&s.changes)
			return nil
		}),
	}
}

// Profiles snapshots the saved profiles in NM's order.
func (s *Settings) Profiles() []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.profiles)
}

// Changes ticks whenever a profile is added, removed, or rewritten.
func (s *Settings) Changes(ctx context.Context) <-chan struct{} {
	return s.changes.SubscribeContext(ctx)
}

// byUUID finds the profile with this UUID.
func (s *Settings) byUUID(uuid string) (Profile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.profiles {
		if p.UUID == uuid {
			return p, true
		}
	}
	return Profile{}, false
}

// ForSSID lists the saved profiles for a wifi network.
func (s *Settings) ForSSID(ssid []byte) []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Profile
	for _, p := range s.profiles {
		if p.SSID != nil && bytes.Equal(p.SSID, ssid) {
			out = append(out, p)
		}
	}
	return out
}

// DeleteForSSID deletes every saved profile of a wifi network: one
// refusal does not stop the rest, and every refusal is returned.
func (s *Settings) DeleteForSSID(ctx context.Context, ssid []byte) error {
	var errs []error
	for _, p := range s.ForSSID(ssid) {
		if err := s.Delete(ctx, p.Path); err != nil {
			errs = append(errs, fmt.Errorf("delete saved profile %s of %q: %w", p.Path, ssid, err))
		}
	}
	return errors.Join(errs...)
}

// Add saves a new profile to disk (AddConnection).
func (s *Settings) Add(ctx context.Context, dict ConnectionDict) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	if err := s.nm.object(settingsPath).CallWithContext(ctx, settingsIface+".AddConnection", 0, dict).Store(&path); err != nil {
		return "", fmt.Errorf("dbus operation failed: %w", err)
	}
	return path, nil
}

// Update rewrites a saved profile in place.
func (s *Settings) Update(ctx context.Context, path dbus.ObjectPath, dict ConnectionDict) error {
	if err := s.nm.object(path).CallWithContext(ctx, connectionIface+".Update", 0, dict).Err; err != nil {
		return fmt.Errorf("cannot update connection: %w", err)
	}
	return nil
}

// AddVPN creates a VPN profile of kind from form values. It is the
// running user's own when NetworkManager can run it that way
// (canBePrivate) and system-wide otherwise.
func (s *Settings) AddVPN(ctx context.Context, kind, name string, values map[string]string) error {
	dict := BuildProfile(kind, name, newUUID(), values)
	s.own(kind, dict)
	if _, err := s.Add(ctx, dict); err != nil {
		return fmt.Errorf("add the VPN profile %q: %w", name, err)
	}
	return nil
}

// UpdateVPN rewrites the VPN profile uuid in place, keeping its UUID so
// anything referring to it still matches. Saved from here it becomes
// the user's own when a new profile would be.
func (s *Settings) UpdateVPN(ctx context.Context, uuid, kind, name string, values map[string]string) error {
	p, ok := s.byUUID(uuid)
	if !ok {
		return errNoProfile(uuid)
	}
	dict := BuildProfile(kind, name, uuid, values)
	s.own(kind, dict)
	if err := s.Update(ctx, p.Path, dict); err != nil {
		return fmt.Errorf("update the VPN profile %q: %w", name, err)
	}
	return nil
}

func (s *Settings) own(kind string, dict ConnectionDict) {
	if s.owner != "" && canBePrivate(kind, s.pluginDirs) {
		ownedBy(dict, s.owner)
	}
}

// Delete removes a saved profile.
func (s *Settings) Delete(ctx context.Context, path dbus.ObjectPath) error {
	if err := s.nm.object(path).CallWithContext(ctx, connectionIface+".Delete", 0).Err; err != nil {
		return fmt.Errorf("cannot delete connection: %w", err)
	}
	return nil
}

// Activate brings the saved profile at path up on whatever device
// NetworkManager picks; secrets it needs are asked of the registered
// agent.
func (s *Settings) Activate(ctx context.Context, path dbus.ObjectPath) error {
	if _, err := s.nm.activate(ctx, path, rootPath, rootPath); err != nil {
		return fmt.Errorf("activate the profile %s: %w", path, err)
	}
	return nil
}

// Deactivate takes the profile with uuid down; one not active is not
// an error.
func (s *Settings) Deactivate(ctx context.Context, uuid string) error {
	active, err := s.nm.activeByUUID(ctx)
	if err != nil {
		return fmt.Errorf("list the active connections: %w", err)
	}
	path, ok := active[uuid]
	if !ok {
		return nil
	}
	if err := s.nm.deactivate(ctx, path); err != nil {
		return fmt.Errorf("deactivate the profile %s: %w", uuid, err)
	}
	return nil
}

// ActiveStates is the state of every active profile by UUID.
func (s *Settings) ActiveStates(ctx context.Context) (map[string]ActiveState, error) {
	conns, err := s.nm.activeStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the active connections: %w", err)
	}
	out := make(map[string]ActiveState, len(conns))
	for uuid, c := range conns {
		out[uuid] = c.state
	}
	return out, nil
}

// Get reads a saved profile's settings.
func (s *Settings) Get(ctx context.Context, path dbus.ObjectPath) (ConnectionDict, error) {
	dict, err := s.nm.getSettings(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("cannot get settings: %w", err)
	}
	return dict, nil
}
