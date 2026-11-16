package sharepicker

import (
	"fmt"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/portaldialogs"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/internal/apptest"
)

// A dismissed picker gives its pixels back: once the compositor has
// presented it, the shared memory it was drawn in is resident, and
// after the dismissal no more stays resident than before it opened, so
// a share leaves no picker-sized trace for the rest of the session. The
// application has no other surface whose buffers could come and go
// meanwhile.
func TestADismissedPickerReleasesItsPixels(t *testing.T) {
	t.Setenv("LYS_UI_RENDERER", "cpu")
	a := apptest.Run(t)
	p, presented := headlessPicker(t, a, config.SharePickerWindows)
	t.Cleanup(func() { p.Dismiss("/r/memory") })
	before := apptest.ShmResident(t)
	answers := make(chan string, 1)
	go func() { answers <- p.Pick(portaldialogs.Caller{Handle: "/r/memory"}, Request{Sources: AllSources}) }()
	apptest.Answer(t, "the picker presented", presented)
	shown := apptest.ShmResident(t)
	if shown <= before {
		t.Fatalf("the presented picker holds %d resident bytes, no more than the %d before it opened", shown, before)
	}
	p.Dismiss("/r/memory")
	apptest.Answer(t, "the dismissed picker's answer", answers)
	waittest.Until(t, func() bool { return apptest.ShmResident(t) <= before }, fmt.Sprintf("the dismissed picker to give back its resident bytes, down to the %d before it opened (%d while shown)", before, shown))
}
