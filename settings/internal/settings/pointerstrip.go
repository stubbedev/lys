package settings

import (
	"errors"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

type pointerStrip struct {
	*inlineState
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
	s := &pointerStrip{inlineState: k.inlineState(), k: k, w: w}
	s.AddClass("wm-pointers")
	return s
}

func (s *pointerStrip) load(then func()) {
	run := s.w.run
	async(s.k, func() pointersRead {
		devices, err := wmstate.Pointers(run)
		return pointersRead{devices, err}
	}, func(r pointersRead) {
		s.known, s.devices, s.err = true, r.devices, r.err
		s.Note(s.text())
		then()
	})
}

func (s *pointerStrip) allows(class wmipc.DeviceClass) bool {
	if class == "" || s == nil || !s.known || s.err != nil {
		return true
	}
	return slices.ContainsFunc(s.devices, func(d wmipc.Device) bool { return d.Class == class })
}

// text is what the strip says, and the error behind it when lys-wm's
// answer could not be read.
func (s *pointerStrip) text() (string, error) {
	t := locale.T()
	switch {
	case errors.Is(s.err, wmstate.ErrNotRunning):
		return t.Get("wm-pointers-not-running"), nil
	case errors.Is(s.err, wmstate.ErrOutdated):
		return t.Get("wm-pointers-outdated"), nil
	case s.err != nil:
		return t.Get("wm-pointers-unreadable"), s.err
	case len(s.devices) == 0:
		return t.Get("wm-pointers-none"), nil
	}
	names := make([]string, len(s.devices))
	for i, d := range s.devices {
		names[i] = t.Get("wm-pointers-device", i18n.Str("name", d.Name), i18n.Str("kind", t.Get("wm-pointers-class-"+string(d.Class))))
	}
	return t.Get("wm-pointers-connected", i18n.Str("devices", strings.Join(names, ", "))), nil
}
