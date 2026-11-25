package popup

import "testing"

// A popup's surface opacity is its content's own fade, times its
// motion's while the motion fades through the compositor (a blurred
// popup), and goes on the wire only when it changes: a popup whose
// content fades out hands each step to the compositor, and one that
// never fades sends nothing.
func TestAPopupsSurfaceFadeCarriesItsContentsFade(t *testing.T) {
	for _, tc := range []struct {
		name              string
		content, motion   float64
		motionFades, sent bool
		last              float64
		wantFade          float64
		wantSend          bool
	}{
		{"an unfaded popup sends nothing", 1, 0.3, false, false, 0, 1, false},
		{"its content fading out starts the fade", 0.6, 1, false, false, 0, 0.6, true},
		{"each step of the content's fade goes out", 0.3, 1, false, true, 0.6, 0.3, true},
		{"a step that changes nothing stays off the wire", 0.3, 1, false, true, 0.3, 0.3, false},
		{"the content landing whole sends the last step", 1, 1, false, true, 0.3, 1, true},
		{"the motion's fade alone is the paint's without blur", 1, 0.5, false, true, 1, 1, false},
		{"a blurred popup's motion fades through the compositor", 1, 0.5, true, false, 0, 0.5, true},
		{"a blurred popup's content and motion fades multiply", 0.5, 0.5, true, true, 0.5, 0.25, true},
		{"a blurred popup at rest still sends its first opacity", 1, 1, true, false, 0, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fade, send := surfaceFade(tc.content, tc.motion, tc.motionFades, tc.last, tc.sent)
			if fade != tc.wantFade || send != tc.wantSend {
				t.Errorf("surfaceFade = %v, %v; want %v, %v", fade, send, tc.wantFade, tc.wantSend)
			}
		})
	}
}
