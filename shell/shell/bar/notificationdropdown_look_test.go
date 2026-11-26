package bar

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
)

// An item whose icon is the picture it attaches has a look button that
// previews the picture in Quick Look without invoking the notification
// or closing the dropdown; an item with no attachment, or a dropdown
// without Quick Look, has none.
func TestANotificationItemLooksAtItsAttachment(t *testing.T) {
	const pic = "/tmp/attached.png"
	for _, quickLook := range []bool{true, false} {
		svc := newNotifTestService(t)
		attached := svc.NotifyHints("chat", 0, "", "Photo", "a picture", []string{"default", "Open"}, map[string]dbus.Variant{"image-path": dbus.MakeVariant(pic)}, 0)
		svc.Notify("mail", 0, "", "Hello", "no picture", nil, 0)
		var invoked []string
		svc.SetEmitter(func(member string, args ...any) {
			if strings.HasSuffix(member, ".ActionInvoked") {
				invoked = append(invoked, member)
			}
		})
		var looked []string
		ctx := newTestContext(t, config.Defaults())
		ctx.Notifications = svc
		if quickLook {
			ctx.NotificationLook = func(path string, closed func()) error {
				looked = append(looked, path)
				closed()
				return nil
			}
		}
		v := openView(t, ctx, notificationDropdown).(*notificationView)
		pop := &fakePopover{}
		var buttons []widget.Widget
		var withPicture *notifItem
		doOnLoopOf(ctx, func() {
			v.attachPopover(pop)
			for _, g := range v.groups {
				for _, it := range g.items {
					found := treetest.WithClass(it, "notification-look")
					if it.n.ID == attached {
						withPicture = it
					} else if len(found) != 0 {
						t.Errorf("the item with no attachment has %d look buttons", len(found))
					}
					buttons = append(buttons, found...)
				}
			}
		})
		if !quickLook {
			if len(buttons) != 0 {
				t.Errorf("without Quick Look the dropdown draws %d look buttons", len(buttons))
			}
			continue
		}
		if len(buttons) != 1 || withPicture == nil {
			t.Fatalf("%d look buttons, want the attached item's one", len(buttons))
		}
		doOnLoopOf(ctx, func() {
			widget.SizeWithin(withPicture, widget.Size{W: 360, H: 400})
			withPicture.Arrange(render.Rect{W: 360, H: 400})
			routeClick(withPicture, buttons[0])
		})
		if !slices.Equal(looked, []string{pic}) || len(invoked) != 0 || pop.dismissed != 0 {
			t.Errorf("the look button looked at %q, invoked %v, closed the dropdown %d times", looked, invoked, pop.dismissed)
		}
	}
}
