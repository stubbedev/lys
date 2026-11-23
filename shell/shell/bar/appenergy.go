package bar

import (
	"cmp"
	"context"
	"log"
	"math"
	"os"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/appenergy"
)

type appEnergySource interface {
	Report() (energy.Report, bool)
	Subscribe() (<-chan struct{}, func())
	Watch(life context.Context, failed func(error))
	Quittable(app energy.App) (appenergy.Target, bool)
	Quit(ctx context.Context, target appenergy.Target) error
}

type appEnergy struct {
	*appenergy.Monitor
	session *dbus.Conn
	uid     uint32
}

func (a appEnergy) Quittable(app energy.App) (appenergy.Target, bool) {
	if a.session == nil {
		return appenergy.Target{}, false
	}
	return appenergy.Quittable(app, a.uid)
}

func (a appEnergy) Quit(ctx context.Context, target appenergy.Target) error {
	return appenergy.Quit(ctx, a.session, target)
}

func startAppEnergy(ctx *ModuleContext, system, session *dbus.Conn) func() {
	if system == nil {
		return func() {}
	}
	monitor := appenergy.Follow(system, func(err error) { log.Printf("energy: %v", err) })
	ctx.AppEnergy = appEnergy{Monitor: monitor, session: session, uid: intconv.Must[uint32](os.Getuid())}
	return func() {
		if err := monitor.Close(); err != nil {
			log.Printf("energy: %v", err)
		}
	}
}

func appWindow(windows []wmipc.Window, pids []uint32) (wmipc.Window, bool) {
	var found []wmipc.Window
	for _, w := range windows {
		pid, err := intconv.Exact[uint32](w.PID)
		if err == nil && slices.Contains(pids, pid) {
			found = append(found, w)
		}
	}
	if len(found) == 0 {
		return wmipc.Window{}, false
	}
	return slices.MinFunc(found, func(a, b wmipc.Window) int { return cmp.Compare(a.ID, b.ID) }), true
}

func appEnergyUsage(r energy.Report, a energy.App) string {
	percent := i18n.Str("percent", strconv.FormatFloat(math.Round(a.Impact*100), 'f', 0, 64))
	if r.Source == energy.SourceNone {
		return i18n.T("dropdown-battery-energy-cpu", percent)
	}
	return i18n.T("dropdown-battery-energy-watts", i18n.Str("watts", formatWatts(a.Watts)), percent)
}

type energySection struct {
	ctx   ModuleContext
	view  *batteryView
	box   *widget.Box
	pages *widget.Stack
	list  *widget.ItemGroup
	quits *stopButtons[appenergy.Target]
}

func (v *batteryView) energySection() widget.Widget {
	s := &energySection{ctx: v.ctx, view: v}
	s.quits = newStopButtons[appenergy.Target](v.ctx, "energy", s.apply)
	card, list := itemCard()
	card.AddClass("battery-energy")
	s.list = list
	s.pages = pageStack()
	s.pages.Add("apps", card)
	s.pages.Add("none", v.infoNote("battery-energy-none", i18n.T("dropdown-battery-energy-none")))
	s.pages.Add("measuring", v.infoNote("battery-energy-measuring", i18n.T("dropdown-battery-energy-measuring")))
	s.box = dropdownSection(sectionLabel(v.font, i18n.T("dropdown-battery-energy")), s.pages)
	v.energy = s
	s.apply()
	return s.box
}

func (s *energySection) apply() {
	src := s.ctx.AppEnergy
	if src == nil {
		s.box.SetVisible(false)
		return
	}
	r, running := src.Report()
	s.box.SetVisible(running)
	if !running {
		return
	}
	if !r.Measured() {
		s.pages.Show("measuring")
		return
	}
	apps := r.Significant(energy.SignificantApps)
	if len(apps) == 0 {
		s.pages.Show("none")
		return
	}
	var windows []wmipc.Window
	if s.ctx.Tasks != nil {
		windows, _ = s.ctx.Tasks.geometry()
	}
	s.list.Clear()
	for _, a := range apps {
		s.list.Append(s.row(r, a, windows))
	}
	s.pages.Show("apps")
}

func (s *energySection) row(r energy.Report, a energy.App, windows []wmipc.Window) *widget.Item {
	v := s.view
	w, shown := appWindow(windows, a.PIDs)
	name := a.Command
	if shown {
		name = s.ctx.Tasks.appName(w.AppID)
	}
	row := widget.NewItem(v.font, name, appEnergyUsage(r, a))
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("battery-energy-app")
	if shown {
		id := strconv.FormatUint(w.ID, 10)
		show := dropdownButton(widget.NewLabel(v.font, v.px, i18n.T("dropdown-battery-energy-show"), 0), func() {
			serviceWrite(s.ctx, func(context.Context) error { return lyswm.Dispatch("focuswindow " + id) }, func(err error) {
				if err != nil {
					toastFailure(s.ctx.Toast, "energy", i18n.T("dropdown-battery-energy-show-failed", i18n.Str("app", name)), err)
				}
			})
		}, "battery-energy-show", css.Small)
		row.AddAction(show)
	}
	src := s.ctx.AppEnergy
	if target, ok := src.Quittable(a); ok {
		row.AddAction(s.quits.button(v.font, v.px, i18n.T("dropdown-battery-energy-quit"), target, "battery-energy-quit",
			i18n.T("dropdown-battery-energy-quit-failed", i18n.Str("app", name)),
			func(ctx context.Context) error { return src.Quit(ctx, target) }))
	}
	return row
}

func (s *energySection) follow(life context.Context) {
	src := s.ctx.AppEnergy
	if src == nil {
		return
	}
	s.ctx.Go(func() { src.Watch(life, func(err error) { log.Printf("energy: %v", err) }) })
	followTicks(life, src.Subscribe, func() { s.ctx.Invoke(s.apply) })
}
