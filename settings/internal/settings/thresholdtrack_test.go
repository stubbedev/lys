package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"
)

// trackLuma is c's Rec. 709 luma.
func trackLuma(c render.Color) float64 {
	return 0.2126*float64(c.R()) + 0.7152*float64(c.G()) + 0.0722*float64(c.B())
}

// The thresholds track was drawn in the card's muted fill, a shade off
// the card, so it all but vanished: it stands off the card at least as
// far as the card's own border does.
func TestTheThresholdTrackStandsOffItsCard(t *testing.T) {
	k := testKit(t, "")
	pal := k.palette()
	th := apptheme.WidgetTheme(&pal, k.store.svc.Config())
	card := trackLuma(th.CardColor())
	track, border := trackLuma(thresholdTrackColor(th))-card, trackLuma(th.Border)-card
	if track < 0 {
		track = -track
	}
	if border < 0 {
		border = -border
	}
	if track < border {
		t.Errorf("the track is %.1f luma off the card, its border %.1f", track, border)
	}
}
