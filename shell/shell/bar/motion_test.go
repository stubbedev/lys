package bar

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

func TestPopoverMotionFollowsAnimationsMenu(t *testing.T) {
	ms := func(n config.AnimationMS) *config.AnimationMS { return &n }
	kind := func(k config.AnimationType) *config.AnimationType { return &k }
	for _, tc := range []struct {
		name string
		set  func(*config.AnimationsConfig)
		want app.PopoverMotion
	}{
		{"menus are instant by default", func(*config.AnimationsConfig) {}, app.PopoverMotion{}},
		{"the global durations leave menus instant", func(a *config.AnimationsConfig) {
			a.Duration, a.EnterDuration, a.ExitDuration = 900, ms(700), ms(800)
		}, app.PopoverMotion{}},
		{"a long menu duration is honoured as a fade", func(a *config.AnimationsConfig) {
			a.Menu.EnterDuration, a.Menu.ExitDuration = ms(400), ms(250)
		}, app.PopoverMotion{Enter: 400 * time.Millisecond, Exit: 250 * time.Millisecond}},
		{"a moving transition grows the menu", func(a *config.AnimationsConfig) {
			a.Menu.EnterDuration, a.Menu.Enter = ms(400), kind(config.AnimationSlideDown)
		}, app.PopoverMotion{Enter: 400 * time.Millisecond, Grow: true}},
		{"reduced motion fades a moving transition", func(a *config.AnimationsConfig) {
			a.Menu.EnterDuration, a.Menu.Enter, a.ReducedMotion = ms(400), kind(config.AnimationZoom), true
		}, app.PopoverMotion{Enter: 400 * time.Millisecond}},
		{"a none transition is instant whatever its duration", func(a *config.AnimationsConfig) {
			a.Menu.EnterDuration, a.Menu.ExitDuration, a.Menu.Enter = ms(400), ms(300), kind(config.AnimationNone)
		}, app.PopoverMotion{Exit: 300 * time.Millisecond}},
		{"animations off make a long menu instant", func(a *config.AnimationsConfig) {
			a.Menu.EnterDuration, a.Menu.ExitDuration, a.Enabled = ms(400), ms(300), false
		}, app.PopoverMotion{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := config.DefaultsAnimations()
			tc.set(&a)
			if got := popoverMotion(a); got != tc.want {
				t.Errorf("popover motion %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDropdownCardsOwnTheirMotion(t *testing.T) {
	anchor := widget.NewBox(widget.Row, 0, 0)
	if pc := dropdownPopover(config.Defaults(), anchor, anchor, 1000, 0); !pc.OwnMotion {
		t.Error("a bar dropdown's popover takes the menus' motion on top of its card's own")
	}
}
