package popups

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
)

// A popup whose icon is the picture it attaches has a look button that
// previews the picture in Quick Look without invoking the notification;
// one whose icon is no attachment has none.
func TestAPopupLooksAtItsAttachment(t *testing.T) {
	s := newSurface(t, config.Defaults().Notification)
	var looked []string
	s.p.look = func(path string, closed func()) error {
		looked = append(looked, path)
		closed()
		return nil
	}
	const pic = "/tmp/attached.png"
	attached := s.svc.NotifyHints("chat", 0, "", "Photo", "a picture", []string{"default", "Open"}, map[string]dbus.Variant{"image-path": dbus.MakeVariant(pic)}, -1)
	plain := s.notify("mail", "Hello")
	s.sync()

	buttons := treetest.WithClass(s.card(attached).Root, "notification-look")
	if len(buttons) != 1 {
		t.Fatalf("the attached popup has %d look buttons, want one", len(buttons))
	}
	if n := len(treetest.WithClass(s.card(plain).Root, "notification-look")); n != 0 {
		t.Errorf("the popup with no attachment has %d look buttons", n)
	}
	s.click(buttons[0].(widget.Boundser))
	if !slices.Equal(looked, []string{pic}) {
		t.Errorf("the look button looked at %q, want %q", looked, pic)
	}
	if _, ok := s.p.cards[attached]; !ok {
		t.Error("looking at the attachment invoked the notification")
	}
}

// Without Quick Look a popup draws no look button.
func TestWithoutQuickLookAPopupHasNoLookButton(t *testing.T) {
	s := newSurface(t, config.Defaults().Notification)
	id := s.svc.NotifyHints("chat", 0, "", "Photo", "a picture", nil, map[string]dbus.Variant{"image-path": dbus.MakeVariant("/tmp/attached.png")}, -1)
	s.sync()
	if n := len(treetest.WithClass(s.card(id).Root, "notification-look")); n != 0 {
		t.Errorf("%d look buttons without Quick Look", n)
	}
}
