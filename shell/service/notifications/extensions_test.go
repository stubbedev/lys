package notifications

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/dbusx/dbusxtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

func hint(key string, v any) map[string]dbus.Variant {
	return map[string]dbus.Variant{key: dbus.MakeVariant(v)}
}

func TestTheServerAdvertisesHyperlinksAndInlineReply(t *testing.T) {
	bus := dbustest.Start(t)
	svc := newTestService(t)
	server, err := Serve(bus.Conn(t), svc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Release() })
	var caps []string
	if err := bus.Conn(t).Object(Interface, ObjPath).Call(Interface+".GetCapabilities", 0).Store(&caps); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"body-hyperlinks", "inline-reply"} {
		if !slices.Contains(caps, want) {
			t.Errorf("GetCapabilities = %v, missing %s", caps, want)
		}
	}
}

func TestTheValueHintIsAProgressFromZeroToAHundred(t *testing.T) {
	none := func(imageData) (string, bool) { return "", false }
	for v, want := range map[any]uint8{int32(40): 40, uint32(80): 80, int32(-5): 0, int32(250): 100, byte(7): 7, int64(100): 100} {
		got, ok := decodeHints(hint("value", v), none).progress.Percent()
		if !ok || got != want {
			t.Errorf("value %T(%v) = %d %v, want %d", v, v, got, ok, want)
		}
	}
	for _, v := range []any{"40", 4.0, true} {
		if _, ok := decodeHints(hint("value", v), none).progress.Percent(); ok {
			t.Errorf("value %T(%v) read as a progress", v, v)
		}
	}
	if _, ok := decodeHints(nil, none).progress.Percent(); ok {
		t.Error("no value hint read as a progress")
	}
}

func TestAStackTagReplacesTheSameAppsNotification(t *testing.T) {
	for _, key := range []string{stackTagDunst, stackTagCanonical} {
		s := newTestService(t)
		first := s.NotifyHints("volume", 0, "", "Volume 40%", "", nil, hint(key, "vol"), 0)
		second := s.NotifyHints("volume", 0, "", "Volume 50%", "", nil, hint(key, "vol"), 0)
		other := s.NotifyHints("brightness", 0, "", "Brightness 50%", "", nil, hint(key, "vol"), 0)
		untagged := s.Notify("volume", 0, "", "Muted", "", nil, 0)
		if first != second {
			t.Errorf("%s: the same tag from the same app got ids %d and %d", key, first, second)
		}
		if other == first || untagged == first {
			t.Errorf("%s: another app's tag or an untagged notification replaced %d", key, first)
		}
		if got := s.Count(); got != 3 {
			t.Errorf("%s: history holds %d rows, want 3 (one per tag and app, plus the untagged one)", key, got)
		}
		if p := s.Popups(); len(p) != 3 || p[2].Summary != "Volume 50%" {
			t.Errorf("%s: popups = %v", key, p)
		}
	}
}

func TestAStackTagSurvivesTheStore(t *testing.T) {
	st, _ := openTestStore(t)
	s := newService(t)
	s.UseClock(fakeclock.New())
	if err := s.AttachStore(st); err != nil {
		t.Fatal(err)
	}
	h := hint(stackTagDunst, "dl")
	h["value"] = dbus.MakeVariant(int32(30))
	id := s.NotifyHints("files", 0, "", "Copying", "", nil, h, 0)
	s.FlushWrites(5 * time.Second)
	n, ok, err := st.Find(id)
	if err != nil || !ok {
		t.Fatalf("Find = %v %v", ok, err)
	}
	if p, set := n.Progress.Percent(); !set || p != 30 || n.StackTag != "dl" {
		t.Errorf("stored = progress %d %v, tag %q", p, set, n.StackTag)
	}
}

func TestAReplyIsSentAsNotificationRepliedToARealClient(t *testing.T) {
	bus := dbustest.Start(t)
	svc := newTestService(t)
	server, err := Serve(bus.Conn(t), svc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Release() })
	client := bus.Conn(t)
	signals, _ := dbusxtest.Signals(t, client, 8, dbus.WithMatchInterface(Interface))
	obj := client.Object(Interface, ObjPath)
	var chat, plain uint32
	if err := obj.Call(Interface+".Notify", 0, "chat", uint32(0), "", "Ada", "lunch?", []string{InlineReplyActionID, "Reply"}, hint("x-kde-reply-placeholder-text", "Reply to Ada"), int32(-1)).Store(&chat); err != nil {
		t.Fatal(err)
	}
	if err := obj.Call(Interface+".Notify", 0, "mail", uint32(0), "", "New mail", "", []string{}, map[string]dbus.Variant{}, int32(-1)).Store(&plain); err != nil {
		t.Fatal(err)
	}
	if svc.Reply(plain, "hi") {
		t.Error("a notification without inline-reply took a reply")
	}
	if !svc.Reply(chat, "hi") {
		t.Fatal("the reply was refused")
	}
	var got []string
	for len(got) < 2 {
		switch sig := waittest.For(t, signals, "the reply and close signals"); sig.Name {
		case Interface + ".NotificationReplied":
			if sig.Body[0] != chat || sig.Body[1] != "hi" {
				t.Errorf("NotificationReplied%v", sig.Body)
			}
			got = append(got, "replied")
		case Interface + ".NotificationClosed":
			if sig.Body[0] != chat || sig.Body[1] != uint32(Dismissed) {
				t.Errorf("NotificationClosed%v", sig.Body)
			}
			got = append(got, "closed")
		}
	}
	if !slices.Equal(got, []string{"replied", "closed"}) {
		t.Errorf("signals = %v, want the reply then the close", got)
	}
}

func TestTheDoNotDisturbStateIsAnnouncedWhenItChanges(t *testing.T) {
	bus := dbustest.Start(t)
	svc := newTestService(t)
	release, err := ServeLysShell(bus.Conn(t), svc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	watcher := bus.Conn(t)
	signals, stop := dbusxtest.Signals(t, watcher, 4, dbus.WithMatchObjectPath(LysShellPath), dbus.WithMatchInterface(dbusx.PropertiesInterface))
	defer stop()
	svc.SetDND(true)
	changed, err := dbusx.PropertiesChanged.Decode(waittest.For(t, signals, "the do-not-disturb change"))
	if err != nil {
		t.Fatal(err)
	}
	if changed.Interface != LysShellName || changed.Changed["Dnd"].Value() != true || changed.Changed["DndReason"].Value() != "manual" || changed.Changed["DndUntil"].Value() != int64(0) {
		t.Fatalf("turning do-not-disturb on announced %+v", changed)
	}
	var on bool
	if err := watcher.Object(LysShellName, LysShellPath).StoreProperty(LysShellName+".Dnd", &on); err != nil || !on {
		t.Fatalf("Dnd reads %v (%v) after it was announced on", on, err)
	}
	svc.Notify("app", 0, "", "ping", "", nil, 0)
	svc.SetDND(false)
	changed, err = dbusx.PropertiesChanged.Decode(waittest.For(t, signals, "the do-not-disturb change"))
	if err != nil {
		t.Fatal(err)
	}
	if changed.Changed["Dnd"].Value() != false || changed.Changed["DndReason"].Value() != "off" {
		t.Fatalf("turning do-not-disturb off announced %+v; a notification is not a do-not-disturb change", changed)
	}
}
