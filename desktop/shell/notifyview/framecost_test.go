package notifyview

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// framePopup is a notification popup surface as lys-shell's popups
// build it, a card in its revealer in the list in the styled host,
// hosted in a frame driver at the card's size and scale num/120 with
// the default animations, the card hidden.
func framePopup(t *testing.T, mode treetest.DrawMode, num int) (*treetest.Frames, *widget.Revealer, config.AnimationsConfig) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Notification.PopupShadow = true
	theme := apptheme.New(cfg)
	host := widget.NewBox(widget.Column, 0, 0)
	host.SetElement("window")
	host.AddClass("background", "notification-popup-host")
	host.AttachStylesheet(theme.Sheet)
	list := widget.NewBox(widget.Column, 12, 12)
	list.AddClass("notification-popup-list")
	host.Append(list, false)
	c := Content{
		App: "mail", Icon: Icon{Name: FallbackIcon}, Urgency: notifymodel.UrgencyNormal, Added: cardNow,
		Summary: "Lunch on Friday?", Body: "Are we still on for <b>noon</b> at the usual place? I booked a table for four.",
		Actions: []Action{{Label: "Reply"}, {Label: "Mark read"}},
	}
	card := NewCard(cfg, treetest.Font(t), cardNow, c, Handlers{Post: func(fn func()) { fn() }})
	rev := widget.NewRevealer(card.Root)
	rev.SetGenieEdge(reveal.GenieEdge(true))
	list.Append(rev, false)
	reveal.Arm(rev, cfg.Animations, config.AnimNotifications, false)
	rev.Finish()
	size := widget.SizeWithinAsChild(host, widget.Size{W: 1920, H: 1080})
	d := treetest.NewFrames(mode)
	d.Start(t, host, size.W, size.H, num, render.Color(0))
	d.Settle(t, 4)
	return d, rev, cfg.Animations
}

// slide shows the card, or hides it, and draws the frames of its
// transition until it lands.
func slide(t *testing.T, d *treetest.Frames, rev *widget.Revealer, anims config.AnimationsConfig, show bool) treetest.Scenario {
	t.Helper()
	name := "notification slides out"
	landed := false
	if show {
		name = "notification slides in"
		reveal.Show(rev, anims, config.AnimNotifications)
	} else {
		reveal.Hide(rev, anims, config.AnimNotifications, func() { landed = true })
	}
	s := treetest.Scenario{Name: name}
	for range 120 {
		s.Frames = append(s.Frames, d.Frame(t))
		if show && rev.Progress() >= 1 || !show && landed {
			break
		}
	}
	d.Settle(t, 2)
	return s
}

// TestPopupFrameCostReport measures a notification popup sliding in
// and out at scale 1.5 and 1, on the CPU and on Vulkan when
// LYS_TEST_GPU names a device. It runs only when LYS_FRAMECOST is set:
// the times are for reading, not for gating.
func TestPopupFrameCostReport(t *testing.T) {
	treetest.RequireFrameCost(t)
	for _, mode := range treetest.ReportModes() {
		for _, size := range treetest.ReportSizes {
			d, rev, anims := framePopup(t, mode, size.Num)
			tag := size.Tag(mode)
			t.Log(tag, slide(t, d, rev, anims, true))
			t.Log(tag, slide(t, d, rev, anims, false))
		}
	}
}

// TestPopupSlideFramesDoBoundedWork is the regression gate of #479 for
// a notification popup, at the owner's scale through the GPU path's
// recording: a frame of the card sliding in or out paints at most the
// popup's surface, records no more than the card's own List and
// allocates within budget, and the card resting shown paints nothing.
// It counts work, never time.
func TestPopupSlideFramesDoBoundedWork(t *testing.T) {
	d, rev, anims := framePopup(t, treetest.DrawRecorded, treetest.ReportSizes[0].Num)
	surface := d.DeviceRect(rev.Parent().(widget.Boundser).Bounds())
	in := slide(t, d, rev, anims, true)
	rest := d.Run(t, "shown at rest", 20, nil)
	out := slide(t, d, rev, anims, false)
	for _, s := range []treetest.Scenario{in, out} {
		if len(s.Frames) < 4 {
			t.Errorf("%s in %d frames: no transition played", s.Name, len(s.Frames))
		}
		if got, limit := treetest.Median(s, func(f treetest.FrameStat) int { return f.Painted }), surface.W*surface.H; got > limit {
			t.Errorf("a frame as the %s paints %d device px at the median, want at most the surface's %d", s.Name, got, limit)
		}
		if got, limit := treetest.Median(s, func(f treetest.FrameStat) int { return f.Ops }), 128; got > limit {
			t.Errorf("a frame as the %s records %d ops at the median, at most the card's own, %d", s.Name, got, limit)
		}
		if got, limit := treetest.Median(s, func(f treetest.FrameStat) uint64 { return f.Allocs }), uint64(12); got > limit {
			t.Errorf("a frame as the %s allocates %d times at the median, budget %d", s.Name, got, limit)
		}
	}
	if got := treetest.Max(rest, func(f treetest.FrameStat) int { return f.Painted }); got != 0 {
		t.Errorf("the card resting shown painted %d device px in a frame, want none", got)
	}
}
