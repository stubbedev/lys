package bar

import (
	"context"
	"log"
	"os"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/pause"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/pausedunits"
)

type pausedUnitsSource interface {
	Value() (pause.State, bool)
	Subscribe() (<-chan struct{}, func())
}

func startPausedUnits(ctx *ModuleContext, system, session *dbus.Conn) func() {
	if system == nil {
		return func() {}
	}
	monitor := pausedunits.Follow(system, intconv.Must[uint32](os.Getuid()), func(err error) { log.Printf("paused units: %v", err) })
	ctx.PausedUnits = monitor
	life, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if session != nil {
			pausedunits.Announce(life, monitor, desktopnotify.NewSender(session))
		}
	}()
	return func() {
		cancel()
		<-done
		if err := monitor.Close(); err != nil {
			log.Printf("paused units: %v", err)
		}
	}
}

func pausedUnitDetail(e pause.Entry) string {
	switch e.Status {
	case pause.StatusPauseFailed:
		return i18n.T("dropdown-battery-paused-failed", i18n.Str("error", e.Err))
	case pause.StatusResumeFailed:
		return i18n.T("dropdown-battery-paused-resume-failed", i18n.Str("error", e.Err))
	case pause.StatusPaused:
	}
	return i18n.T("dropdown-battery-paused-now")
}

type pausedSection struct {
	ctx  ModuleContext
	view *batteryView
	box  *widget.Box
	list *widget.ItemGroup
}

func (v *batteryView) pausedSection() widget.Widget {
	s := &pausedSection{ctx: v.ctx, view: v}
	card, list := itemCard()
	card.AddClass("battery-paused")
	s.list = list
	s.box = dropdownSection(sectionLabel(v.font, i18n.T("dropdown-battery-paused")), card)
	v.paused = s
	s.apply()
	return s.box
}

func (s *pausedSection) apply() {
	src := s.ctx.PausedUnits
	if src == nil {
		s.box.SetVisible(false)
		return
	}
	st, running := src.Value()
	s.box.SetVisible(running && len(st.Entries) > 0)
	s.list.Clear()
	for _, e := range st.Entries {
		row := widget.NewItem(s.view.font, e.Unit.Name(), pausedUnitDetail(e))
		row.SetSmall(true)
		row.SetEllipsize(true)
		row.AddClass("battery-paused-unit")
		s.list.Append(row)
	}
}

func (s *pausedSection) follow(life context.Context) {
	src := s.ctx.PausedUnits
	if src == nil {
		return
	}
	followTicks(life, src.Subscribe, func() { s.ctx.Invoke(s.apply) })
}
