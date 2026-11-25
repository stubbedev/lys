package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
)

// The hands were drawn with palms in the card's own muted fill, so the
// picture all but vanished: the palms and every finger's outline stand
// off the card at least as far as the card's border does.
func TestTheHandsStandOffTheirCard(t *testing.T) {
	k := testKit(t, "")
	pal := k.palette()
	th := apptheme.WidgetTheme(&pal, k.store.svc.Config())
	gap := func(c float64) float64 {
		d := c - trackLuma(th.CardColor())
		if d < 0 {
			return -d
		}
		return d
	}
	ink, border := handsInk(th), gap(trackLuma(th.Border))
	if palm := gap(trackLuma(ink.palm)); palm < border {
		t.Errorf("the palms are %.1f luma off the card, its border %.1f", palm, border)
	}
	if outline := gap(trackLuma(ink.outline)); outline < border {
		t.Errorf("the fingers' outlines are %.1f luma off the card, its border %.1f", outline, border)
	}
}
