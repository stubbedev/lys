package sharepicker

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/portaldialogs"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/internal/apptest"
	"github.com/stubbedev/lys/shell/service/sharepreview"
)

// A dismissed picker gives its pixels back: once the compositor has
// presented it, the shared memory it was drawn in is held, and after
// the dismissal no more stays held than before it opened, so a share
// leaves no picker-sized trace for the rest of the session. The
// previews never land while it is shown, so its spinners animate and
// the pool grows by a spare buffer before the first present, moving
// the mapping the pixels were drawn through; the loop waits in the
// presented callback while the test reads, so no later frame touches
// the pool meanwhile. The application has no other surface whose
// buffers could come and go.
func TestADismissedPickerReleasesItsPixels(t *testing.T) {
	t.Setenv("LYS_UI_RENDERER", "cpu")
	a := apptest.Run(t)
	p, _ := headlessPicker(t, a, config.SharePickerWindows)
	t.Cleanup(func() { p.Dismiss("/r/memory") })
	previews := make(chan struct{})
	t.Cleanup(func() { close(previews) })
	p.windowPreview = func(sharepreview.Toplevel, int) (*image.RGBA, error) {
		<-previews
		return nil, errors.New("the preview is held until the test ends")
	}
	presented, measured := make(chan struct{}), make(chan struct{})
	resume := sync.OnceFunc(func() { close(measured) })
	t.Cleanup(resume)
	p.presented = sync.OnceFunc(func() {
		close(presented)
		<-measured
	})
	before := apptest.ShmHeld(t)
	answers := make(chan string, 1)
	go func() { answers <- p.Pick(portaldialogs.Caller{Handle: "/r/memory"}, Request{Sources: AllSources}) }()
	apptest.Answer(t, "the picker presented", presented)
	shown := apptest.ShmHeld(t)
	resume()
	if shown <= before {
		t.Fatalf("the presented picker holds %d bytes, no more than the %d before it opened", shown, before)
	}
	p.Dismiss("/r/memory")
	apptest.Answer(t, "the dismissed picker's answer", answers)
	waittest.Until(t, func() bool { return apptest.ShmHeld(t) <= before }, fmt.Sprintf("the dismissed picker to give back its held bytes, down to the %d before it opened (%d while shown)", before, shown))
}
