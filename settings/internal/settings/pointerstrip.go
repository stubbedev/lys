package settings

import (
	"errors"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

type pointerStrip struct {
	*widget.Label
	k       *kit
	w       *wmBackend
	known   bool
	devices []wmipc.Device
	err     error
}

type pointersRead struct {
	devices []wmipc.Device
	err     error
}

func newPointerStrip(k *kit, w *wmBackend) *pointerStrip {
	l := k.label("", "wm-pointers", css.Caption, css.DimLabel)
	l.SetWrap(true)
	l.SetVisible(false)
	return &pointerStrip{Label: l, k: k, w: w}
}

func (s *pointerStrip) load(then func()) {
	run := s.w.run
	async(s.k, func() pointersRead {
		devices, err := wmstate.Pointers(run)
		return pointersRead{devices, err}
	}, func(r pointersRead) {
		s.known, s.devices, s.err = true, r.devices, r.err
		s.SetText(s.text())
		s.SetVisible(true)
		then()
	})
}

func (s *pointerStrip) allows(class wmipc.DeviceClass) bool {
	if class == "" || s == nil || !s.known || s.err != nil {
		return true
	}
	return slices.ContainsFunc(s.devices, func(d wmipc.Device) bool { return d.Class == class })
}

func (s *pointerStrip) text() string {
	t := locale.T()
	switch {
	case errors.Is(s.err, wmstate.ErrNotRunning):
		return t.Get("wm-pointers-not-running")
	case errors.Is(s.err, wmstate.ErrOutdated):
		return t.Get("wm-pointers-outdated")
	case s.err != nil:
		return t.Get("wm-pointers-unreadable", i18n.Str("reason", s.err.Error()))
	case len(s.devices) == 0:
		return t.Get("wm-pointers-none")
	}
	names := make([]string, len(s.devices))
	for i, d := range s.devices {
		names[i] = t.Get("wm-pointers-device", i18n.Str("name", d.Name), i18n.Str("kind", t.Get("wm-pointers-class-"+string(d.Class))))
	}
	return t.Get("wm-pointers-connected", i18n.Str("devices", strings.Join(names, ", ")))
}
