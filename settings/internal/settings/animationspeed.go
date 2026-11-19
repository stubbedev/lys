package settings

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// animationSpeed is a named multiple of the default durations.
type animationSpeed struct {
	name   string
	factor float64
}

var animationSpeeds = []animationSpeed{{"slow", 2}, {"relaxed", 1.5}, {"normal", 1}, {"fast", 0.5}, {"instant", 0}}

// speedDurations are the durations one speed sets together.
var speedDurations = []string{"animations.duration", "animations.ui-duration", "animations.interaction-duration"}

// speedTimes are the durations, in speedDurations' order, factor makes of
// the defaults.
func speedTimes(factor float64) []int64 {
	d := config.DefaultsAnimations()
	base := []config.AnimationMS{d.Duration, d.UIDuration, d.InteractionDuration}
	out := make([]int64, len(base))
	for i, b := range base {
		out[i] = int64(math.Round(float64(b) * factor))
	}
	return out
}

// speedRow sets the three durations at once by a named speed, none chosen
// while they hold times no speed gives.
func speedRow() rowSpec {
	return rowSpec{
		path: speedDurations[0], key: "settings-animations-speed", action: true, fullWidth: true,
		editor: func(k *kit, _ slot, _ config.FieldMeta) control {
			values, labels := make([]string, len(animationSpeeds)), make([]string, len(animationSpeeds))
			pictures := map[string]picture{}
			for i, sp := range animationSpeeds {
				values[i], labels[i] = sp.name, locale.T().Get("settings-animations-speed-"+sp.name)
				pictures[sp.name] = speedPicture(sp.factor)
			}
			return newCards(k, speedSlot(k), values, labels, pictures)
		},
	}
}

func speedSlot(k *kit) slot {
	slots := make([]slot, len(speedDurations))
	for i, p := range speedDurations {
		slots[i] = k.store.slot(p)
	}
	stored := func() []int64 {
		out := make([]int64, len(slots))
		for i, s := range slots {
			out[i], _ = s.get().(int64)
		}
		return out
	}
	return slot{
		get: func() any {
			now := stored()
			if i := slices.IndexFunc(animationSpeeds, func(sp animationSpeed) bool { return slices.Equal(speedTimes(sp.factor), now) }); i >= 0 {
				return animationSpeeds[i].name
			}
			return ""
		},
		set: func(v any) error {
			i := slices.IndexFunc(animationSpeeds, func(sp animationSpeed) bool { return sp.name == v })
			if i < 0 {
				return fmt.Errorf("settings: no animation speed %v", v)
			}
			var errs []error
			for j, ms := range speedTimes(animationSpeeds[i].factor) {
				errs = append(errs, slots[j].set(ms))
			}
			return errors.Join(errs...)
		},
		unset: func() {
			for _, s := range slots {
				s.unset()
			}
		},
	}
}

// speedPicture is a dot with a trail as long as the motion is slow.
func speedPicture(factor float64) picture {
	return drawn(func(cv *render.Canvas, r render.Rect) {
		th := widget.Current()
		dot := r.H / 3
		x := r.X + r.W - dot - pictureInset
		y := r.Y + (r.H-dot)/2
		trail := int(math.Round(float64(x-r.X-pictureInset) * factor / 2))
		for i := range 3 {
			line := render.Rect{X: x - trail*(3-i)/3, Y: y + dot*i/3 + dot/6, W: trail * (3 - i) / 3, H: max(2, dot/6)}
			cv.RoundedRect(line, line.H/2, th.Border)
		}
		cv.RoundedRect(render.Rect{X: x, Y: y, W: dot, H: dot}, dot/2, th.Accent)
	})
}
