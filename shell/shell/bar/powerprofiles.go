package bar

import (
	"context"
	"errors"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/jinja"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/power/profile"
	"github.com/stubbedev/lys/ui/widget"
)

// powerProfilesLabel renders format: the only variable
// is the active profile's name.
func powerProfilesLabel(format, active string) string {
	return jinja.ReplaceVars(format, powerProfilesVars(active))
}

func powerProfilesVars(active string) map[string]string {
	return map[string]string{"profile": active}
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
		return nil, errors.New("power-profiles: lys-power publishes no power profile")
	}
	m := &powerProfilesModule{ctx: ctx, src: ctx.PowerProfiles}
	m.icon = barview.Icon(ctx.Config.PowerProfiles.Icon())
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
	shown := profileOf(snap.Active)
	if setter := m.icon; setter != nil {
		setter.SetThemeName(cfg.Icons()[shown].Name)
	}
	color := cfg.Colors()[shown]
	m.btn.SetThresholds(config.ThresholdColors{IconColor: &color, LabelColor: &color})
}

// profileOf is the profile the icon and colors use: the active one,
// balanced for a name that is none (Unknown).
func profileOf(active string) profile.Profile {
	p, err := profile.Parse(active)
	if err != nil {
		return profile.Balanced
	}
	return p
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
		toastFailure(m.ctx.Toast, "power-profiles", i18n.T("power-profiles-switch-failed"), err)
	})
}

func (m *powerProfilesModule) Root() widget.Widget { return m.btn }
