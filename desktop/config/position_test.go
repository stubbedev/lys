package config

import (
	"errors"
	"strings"
	"testing"
)

func TestTheOsdAndThePopupsSitAtTheSameScreenPositions(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "osd:\n  position: center-left\nmodules:\n  notifications:\n    popup-position: center-left\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Osd.Position != ScreenCenterLeft || cfg.Notification.PopupPosition != ScreenCenterLeft {
		t.Errorf("osd at %q, popups at %q, want both center-left", cfg.Osd.Position, cfg.Notification.PopupPosition)
	}
	osd, _ := Field("osd.position")
	popups, _ := Field("modules.notifications.popup-position")
	if osd.Type != popups.Type || strings.Join(osd.Variants, " ") != strings.Join(popups.Variants, " ") {
		t.Errorf("osd.position offers %s %v, popup-position %s %v", osd.Type, osd.Variants, popups.Type, popups.Variants)
	}
}

func TestAnEdgeNamedWithoutItsMiddleIsNoScreenPosition(t *testing.T) {
	for _, raw := range []string{"bottom", "left", "top", "right"} {
		cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "osd:\n  position: "+raw+"\n"))
		var d Diagnostic
		if !errors.As(err, &d) || d.Field("Field") != "osd.position" || !strings.Contains(d.Field("Error"), "unknown variant `"+raw+"`") {
			t.Errorf("position %s: error %v, want an unknown variant at osd.position", raw, err)
		}
		if cfg.Osd.Position != ScreenBottomCenter {
			t.Errorf("position %s: the OSD sits at %q, want the bottom-center default", raw, cfg.Osd.Position)
		}
	}
}
