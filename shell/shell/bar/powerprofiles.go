package bar

import (
	"context"
	"errors"
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/jinja"
	"github.com/stubbedev/lys/shell/service/powerprofiles"
)

// powerProfilesLabel renders format: the only variable
// is the active profile's name.
func powerProfilesLabel(format, profile string) string {
	return jinja.ReplaceVars(format, powerProfilesVars(profile))
}

func powerProfilesVars(profile string) map[string]string {
	return map[string]string{"profile": profile}
}

// powerProfiles is the module: the active power profile, cycling on
// the `:cycle` binding.
type powerProfilesModule struct {
	ctx  ModuleContext
	src  powerprofiles.Source
	icon *widget.Icon
	// btn takes the active profile's color as threshold colors.
	btn  *barButton
	snap powerprofiles.Snapshot
	firstRead
}

func newPowerProfiles(ctx ModuleContext) (Module, error) {
	if ctx.PowerProfiles == nil {
		return nil, errors.New("power-profiles: no power-profiles-daemon available")
	}
	m := &powerProfilesModule{ctx: ctx, src: ctx.PowerProfiles}
	m.icon = moduleIcon(ctx, ctx.Config.PowerProfiles.Icons()[config.ProfileBalanced])
	m.btn = newBarButton(ctx, m.icon, "")
	followReads(ctx, "power-profiles", &m.firstRead, ctx.PowerProfiles.Subscribe, m.src.Read, m.apply)
	return m, nil
}

// apply restyles label and icon for the active profile.
func (m *powerProfilesModule) apply(snap powerprofiles.Snapshot) {
	cfg := m.ctx.Config.PowerProfiles
	m.snap = snap
	text := ""
	if snap.Available {
		text = powerProfilesLabel(cfg.Format, snap.Active)
	}
	m.btn.SetLabel(text)
	profile := profileOf(snap.Active)
	if setter := m.icon; setter != nil {
		setter.SetThemeName(cfg.Icons()[profile].Name)
	}
	color := cfg.Colors()[profile]
	m.btn.SetThresholds(config.ThresholdColors{IconColor: &color, LabelColor: &color})
}

// profileOf is the profile the icon and colors use: power-saver and
// performance as they are, balanced for anything else (Unknown).
func profileOf(active string) string {
	switch active {
	case config.ProfilePowerSaver, config.ProfilePerformance:
		return active
	}
	return config.ProfileBalanced
}

// RunAction handles the module's own bindings: `:cycle` moves to the
// next profile in the daemon's available set; the rest fall through
// to the shared executor.
func (m *powerProfilesModule) RunAction(action config.ClickAction) {
	if action.Kind == config.ClickShell && action.Command == ":cycle" {
		m.cycle()
		return
	}
	runClickAction(m.ctx, action)
}

// cycle writes the next profile off the loop; the daemon's
// PropertiesChanged tick redraws, and a failure toasts why.
func (m *powerProfilesModule) cycle() {
	next := powerprofiles.NextProfile(m.snap.Active, m.snap.Profiles)
	serviceWrite(m.ctx, func(ctx context.Context) error {
		return m.src.SetActive(ctx, next)
	}, func(err error) {
		if err == nil {
			return
		}
		title := i18n.T("power-profiles-switch-failed")
		log.Printf("power-profiles: %s (%s): %v", title, next, err)
		if m.ctx.Toast != nil {
			m.ctx.Toast(title+": "+err.Error(), "ld-alert-triangle-symbolic")
		}
	})
}

func (m *powerProfilesModule) Root() widget.Widget { return m.btn }
