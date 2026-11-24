package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/stubbedev/lys/lib/polkit"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/busauth"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/profiles"
	"github.com/stubbedev/lys/power/internal/statefile"
	"github.com/stubbedev/lys/power/internal/supply"
	"github.com/stubbedev/lys/power/profile"
)

// ProfileAction is the polkit action that switches the power profile.
const ProfileAction polkit.Action = "dev.stubbe.lys.power.profile"

// ProfileSetter switches the power profile. A nil one means the machine
// has no profile knobs, and profile handling is skipped.
type ProfileSetter interface {
	Set(p profile.Profile) error
}

// PowerProfile is the active power profile: applied to the knobs, kept
// in the state directory for the next start, and published on the bus
// once served.
type PowerProfile struct {
	knobs profiles.Knobs
	path  string

	mu        sync.Mutex
	active    profile.Profile
	server    *profile.Server
	authorize busauth.Authorizer
}

// LoadProfile is the profile last set, or the one for the power source
// when none was, over the knobs under cfg's sysfs; nil when there are
// none.
func LoadProfile(cfg config.Config, onAC bool) (*PowerProfile, error) {
	knobs, err := profiles.Find(cfg.SysDir)
	if err != nil || knobs.Empty() {
		return nil, err
	}
	active, err := readProfile(cfg.ProfilePath())
	if err != nil {
		return nil, err
	}
	if active == "" {
		active = profiles.Pick(onAC)
	}
	return &PowerProfile{knobs: knobs, path: cfg.ProfilePath(), active: active}, nil
}

func readProfile(path string) (profile.Profile, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading the power profile: %w", err)
	}
	p, err := profile.Parse(strings.TrimSpace(string(raw)))
	if err != nil {
		return "", fmt.Errorf("reading the power profile in %s: %w", path, err)
	}
	return p, nil
}

// OpenProfile loads the profile and sets the knobs to it; a machine
// without knobs has none. Another daemon setting the profile on conn,
// when there is a bus, is ErrRival.
func OpenProfile(ctx context.Context, cfg config.Config, log *slog.Logger, conn *dbus.Conn) (*PowerProfile, error) {
	if conn != nil {
		if err := profiles.Rival(ctx, conn); err != nil {
			return nil, err
		}
	}
	onAC, err := supply.Supply{Dir: cfg.SupplyDir}.OnAC()
	if err != nil {
		return nil, fmt.Errorf("reading power supplies: %w", err)
	}
	pp, err := LoadProfile(cfg, onAC)
	if err != nil || pp == nil {
		if pp == nil && err == nil {
			log.Info("no power profile to set: neither a platform profile nor a cpufreq energy preference")
		}
		return nil, err
	}
	if err := pp.Reapply(); err != nil {
		log.Error("setting the power profile at start", "err", err)
	}
	return pp, nil
}

// startProfile is OpenProfile, published on conn when there is a bus.
func startProfile(ctx context.Context, cfg config.Config, log *slog.Logger, conn *dbus.Conn) (ProfileSetter, func(), error) {
	pp, err := OpenProfile(ctx, cfg, log, conn)
	if err != nil {
		return nil, nil, err
	}
	if pp == nil {
		return nil, func() {}, nil
	}
	if conn == nil {
		return pp, func() {}, nil
	}
	release, err := pp.Serve(conn, busauth.Polkit(conn, ProfileAction))
	if err != nil {
		return nil, nil, err
	}
	return pp, release, nil
}

// Active is the profile last set.
func (pp *PowerProfile) Active() profile.Profile {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	return pp.active
}

// Reapply sets the knobs to the active profile again, as a start after a
// reboot needs.
func (pp *PowerProfile) Reapply() error {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	if err := pp.knobs.Apply(pp.active); err != nil {
		return fmt.Errorf("applying the power profile %s: %w", pp.active, err)
	}
	return nil
}

// Set applies p, then keeps and publishes it. A knob that cannot be set
// fails it, and the profile kept and published stays the last one set.
func (pp *PowerProfile) Set(p profile.Profile) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	if err := pp.knobs.Apply(p); err != nil {
		return fmt.Errorf("applying the power profile %s: %w", p, err)
	}
	if err := statefile.Write(pp.path, []byte(p)); err != nil {
		return fmt.Errorf("storing the power profile %s: %w", p, err)
	}
	pp.active = p
	if pp.server == nil {
		return nil
	}
	if err := pp.server.Publish(p); err != nil {
		return fmt.Errorf("publishing the power profile %s: %w", p, err)
	}
	return nil
}

// SetActive is Set for a caller root or polkit allows.
func (pp *PowerProfile) SetActive(ctx context.Context, caller profile.Caller, p profile.Profile) error {
	if caller.UID != 0 {
		if err := pp.authorize(ctx, caller); err != nil {
			return err
		}
	}
	return pp.Set(p)
}

// Serve publishes the profile on conn, switched for callers authorize
// allows.
func (pp *PowerProfile) Serve(conn *dbus.Conn, authorize busauth.Authorizer) (func(), error) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.authorize = authorize
	server, release, err := profile.Serve(conn, pp, pp.active)
	if err != nil {
		return nil, err
	}
	pp.server = server
	return release, nil
}
