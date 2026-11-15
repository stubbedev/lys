package daemon

import (
	"testing"

	"github.com/stubbedev/lys/splash/internal/protocol"
	"github.com/stubbedev/lys/splash/internal/theme"
)

func TestUnitNamesGoToTheStatusLineOnlyWithUnitsOn(t *testing.T) {
	t.Parallel()
	d := &daemon{units: true}
	d.state.Message = "Unlocking"
	if err := d.apply(protocol.Update{Status: "systemd-udevd.service"}); err != nil {
		t.Fatal(err)
	}
	if d.state.Status != "systemd-udevd.service" || d.state.Message != "Unlocking" {
		t.Errorf("with units on, a unit made the status %q and the message %q; want the unit and the message kept", d.state.Status, d.state.Message)
	}
	off := &daemon{}
	if err := off.apply(protocol.Update{Status: "systemd-udevd.service"}); err != nil {
		t.Fatal(err)
	}
	if off.state.Status != "" || off.state.Message != "" {
		t.Errorf("with units off, a unit showed status %q and message %q", off.state.Status, off.state.Message)
	}
}

func TestAThemeWithoutUnitsClearsTheStatusLine(t *testing.T) {
	t.Parallel()
	r, err := theme.NewRenderer(theme.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d := &daemon{units: true}
	d.state.Status = "systemd-udevd.service"
	d.themeLoaded(&themeLoad{renderer: r, units: true})
	if d.state.Status != "systemd-udevd.service" {
		t.Errorf("a theme with units cleared the status line to %q", d.state.Status)
	}
	d.themeLoaded(&themeLoad{renderer: r, units: false})
	if d.state.Status != "" {
		t.Errorf("a theme without units left the status line %q", d.state.Status)
	}
}
