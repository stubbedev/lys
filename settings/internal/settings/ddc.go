package settings

import (
	"os"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// i2cAccess is how many I2C buses the system has, which external
// monitors' brightness is set over, and how many of them this user may
// open.
type i2cAccess struct{ buses, usable int }

func (h host) i2cAccess() i2cAccess {
	var a i2cAccess
	for _, e := range entries(h.path("dev")) {
		if !strings.HasPrefix(e.Name(), "i2c-") {
			continue
		}
		a.buses++
		f, err := os.OpenFile(h.path("dev", e.Name()), os.O_RDWR, 0)
		if err != nil {
			continue
		}
		a.usable++
		_ = f.Close()
	}
	return a
}

func (a i2cAccess) words() string {
	t := locale.T()
	switch {
	case a.buses == 0:
		return t.Get("settings-ddc-no-buses")
	case a.usable == 0:
		return t.Get("settings-ddc-no-access")
	}
	return t.Get("settings-ddc-access", i18n.Int("n", int64(a.usable)))
}

// ddcRow is enable-external with what it needs: the I2C buses and the
// right to open them, said beneath it, and switching on held back while
// there is no bus to open.
var ddcRow = withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
	return newDDCSwitch(k, s, k.i2c)
})

type ddcSwitch struct {
	*widget.Box
	toggle *toggle
	status *widget.Label
	access i2cAccess
	known  bool
}

func newDDCSwitch(k *kit, s slot, scan func() i2cAccess) *ddcSwitch {
	c := &ddcSwitch{Box: gapBox(widget.Column), toggle: newToggle(k, s), status: k.label("", css.Caption, css.DimLabel)}
	c.AppendAligned(c.toggle, false, widget.AlignEnd)
	c.AppendAligned(wrapped(c.status), false, widget.AlignEnd)
	async(k, scan, func(a i2cAccess) {
		c.access, c.known = a, true
		c.fit()
	})
	c.fit()
	return c
}

func (c *ddcSwitch) fit() {
	c.status.SetVisible(c.known)
	if !c.known {
		return
	}
	c.status.SetText(c.access.words())
	c.toggle.SetEnabled(c.access.usable > 0 || c.toggle.On())
}

func (c *ddcSwitch) refresh() {
	c.toggle.refresh()
	c.fit()
}
