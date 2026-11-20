package notifyview

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

var cardNow = time.Date(2026, time.March, 14, 9, 41, 0, 0, time.UTC)

func sampleContent() Content {
	return Content{App: "mail", Icon: Icon{Name: FallbackIcon}, Urgency: notifymodel.UrgencyNormal, Added: cardNow, Summary: "Hello", Body: "a &amp; b"}
}

func onlyCard(t *testing.T, root widget.Widget) *widget.Box {
	t.Helper()
	cards := treetest.WithClass(root, "notification-popup-card")
	if len(cards) != 1 {
		t.Fatalf("%d cards, want one", len(cards))
	}
	card, ok := cards[0].(*widget.Box)
	if !ok {
		t.Fatal("the card is no box")
	}
	return card
}

func TestAPreviewFollowsItsConfig(t *testing.T) {
	cfg := config.Defaults()
	cfg.Notification.PopupShadow = true
	cfg.Notification.PopupUrgencyBar = config.UrgencyBarThresholdNormal
	preview := Preview(cfg, treetest.Font(t), cardNow, sampleContent())
	card := onlyCard(t, preview)
	if !card.HasClass("shadow") || !card.HasClass("urgency-bar") || !card.HasClass("normal") {
		t.Errorf("a shadowed card whose urgency reaches the bar previews as %v", card.Classes())
	}
	labels := treetest.Labels(preview)
	for _, want := range []string{"mail", "Hello", i18n.T("notification-popup-time-just-now")} {
		if !slices.Contains(labels, want) {
			t.Errorf("the preview shows %v, want %q", labels, want)
		}
	}
	if sz := widget.SizeWithinAsChild(preview, widget.Size{W: 1920, H: 1080}); sz.W != Width(float64(cfg.Styling.Scale)) {
		t.Errorf("the preview is %dpx wide, want the card's %d", sz.W, Width(float64(cfg.Styling.Scale)))
	}

	cfg.Notification.PopupShadow = false
	cfg.Notification.PopupUrgencyBar = config.UrgencyBarThresholdCritical
	card = onlyCard(t, Preview(cfg, treetest.Font(t), cardNow, sampleContent()))
	if card.HasClass("shadow") || card.HasClass("urgency-bar") {
		t.Errorf("no shadow and a critical-only bar preview a normal card as %v", card.Classes())
	}
	if widget.CursorNameOf(card) != "" {
		t.Error("a preview takes clicks like a card with a default action")
	}
}

func TestACardShowsAnUpdateInPlace(t *testing.T) {
	var pressed []int
	k := NewCard(config.Defaults(), treetest.Font(t), cardNow, sampleContent(), Handlers{
		Close: func() {}, Action: func(i int) { pressed = append(pressed, i) }, Post: func(fn func()) { fn() },
	})
	if k.Actions.Visible() || len(k.Actions.Children()) != 0 || k.Progress.Visible() {
		t.Fatal("a card without actions or progress shows them")
	}
	next := sampleContent()
	next.Summary = "Bye"
	next.Urgency = notifymodel.UrgencyCritical
	next.Progress = notifymodel.NewProgress(40)
	next.Actions = []Action{{Label: "Open"}, {Label: "Archive"}}
	k.Update(next)
	if k.Summary.Text() != "Bye" || !k.Root.HasClass("critical") || k.Root.HasClass("normal") {
		t.Errorf("the update reads %q with classes %v", k.Summary.Text(), k.Root.Classes())
	}
	if !k.Progress.Visible() || k.Progress.Value() != 0.4 {
		t.Errorf("the update's progress shows %v at %v", k.Progress.Visible(), k.Progress.Value())
	}
	buttons := treetest.All[*widget.Button](k.Actions)
	if len(buttons) != 2 {
		t.Fatalf("the update shows %d action buttons, want 2", len(buttons))
	}
	buttons[1].OnClick()
	if !slices.Equal(pressed, []int{1}) {
		t.Errorf("pressing the second button acted on %v", pressed)
	}
}

func TestPlacementPutsThePopupsWherePopupPositionSays(t *testing.T) {
	for pos, want := range map[config.PopupPosition][2]widget.Align{
		config.PopupPositionTopLeft:      {widget.AlignStart, widget.AlignStart},
		config.PopupPositionTopCenter:    {widget.AlignCenter, widget.AlignStart},
		config.PopupPositionTopRight:     {widget.AlignEnd, widget.AlignStart},
		config.PopupPositionBottomLeft:   {widget.AlignStart, widget.AlignEnd},
		config.PopupPositionBottomCenter: {widget.AlignCenter, widget.AlignEnd},
		config.PopupPositionBottomRight:  {widget.AlignEnd, widget.AlignEnd},
		config.PopupPositionCenterLeft:   {widget.AlignStart, widget.AlignCenter},
		config.PopupPositionCenterRight:  {widget.AlignEnd, widget.AlignCenter},
	} {
		if h, v := Placement(pos); h != want[0] || v != want[1] {
			t.Errorf("%s places the popups at %v/%v, want %v/%v", pos, h, v, want[0], want[1])
		}
	}
}
